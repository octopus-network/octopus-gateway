package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/mwitkow/grpc-proxy/proxy"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const prefixPathKey = "prefix-path"

// Creates a gRPC server that acts as a proxy and routes incoming requests.
func buildGrpcProxyServer(routeChecker string) *grpc.Server {
	director := func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
		if !ok {
			zap.S().Errorw("grpc: metadata required")
			return nil, nil, status.Errorf(codes.InvalidArgument, "Metadata Required")
		}
		if len(md.Get(prefixPathKey)) != 1 {
			zap.S().Errorw("grpc: prefix path required")
			return nil, nil, status.Errorf(codes.InvalidArgument, "Prefix Path Required")
		}

		prefixPath := md.Get(prefixPathKey)[0]
		target, err := shouldRoute(routeChecker, prefixPath)
		if err != nil {
			zap.S().Errorw(fmt.Sprintf("grpc: route failed %s | %s", prefixPath, err))
			return nil, nil, status.Errorf(codes.Aborted, "Route Failed")
		}

		conn, err := grpc.DialContext(ctx, target,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			zap.S().Errorw(fmt.Sprintf("grpc: couldn't dial to %s | %s", target, err))
		}
		return outCtx, conn, err
	}

	server := grpc.NewServer(
		grpc.UnknownServiceHandler(proxy.TransparentHandler(director)),
	)
	grpc_health_v1.RegisterHealthServer(server, &HealthServer{})
	return server
}

func shouldRoute(routeChecker, prefixPath string) (string, error) {
	re := regexp.MustCompile(`^/(?P<chain>[a-z][-a-z0-9]*[a-z0-9]?)/(?P<project>[a-z0-9]{32})$`)
	params := re.FindStringSubmatch(prefixPath)
	if len(params) < 3 {
		zap.S().Errorw("grpc", "path", prefixPath, "statue", http.StatusBadRequest)
		return "", errors.New(http.StatusText(http.StatusBadRequest))
	}
	chain, project := params[1], params[2]

	// Route URL: http://gateway-api/route/{chain_id}/{project_id}
	client := &http.Client{Timeout: 1 * time.Second}
	url := fmt.Sprintf("%s/%s/%s", routeChecker, chain, project)
	resp, err := client.Get(url)
	if err != nil {
		zap.S().Errorw("grpc", "path", prefixPath, "statue", http.StatusInternalServerError)
		return "", errors.New(http.StatusText(http.StatusInternalServerError))
	}
	defer resp.Body.Close()

	routeResp := RouteResponse{}
	json.NewDecoder(resp.Body).Decode(&routeResp)
	if !routeResp.Route {
		zap.S().Errorw("grpc", "path", prefixPath, "statue", http.StatusForbidden)
		return "", errors.New(http.StatusText(http.StatusForbidden))
	}

	zap.S().Infow("grpc", "path", prefixPath, "target", routeResp.Target.GRPC)
	return routeResp.Target.GRPC, nil
}

// The HealthServer type is a gRPC server that implements the Check method for health checking.
type HealthServer struct{}

func (s *HealthServer) Check(ctx context.Context, in *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

func (s *HealthServer) Watch(in *grpc_health_v1.HealthCheckRequest, srv grpc_health_v1.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "Watch is not implemented")
}
