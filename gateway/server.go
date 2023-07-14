package main

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/soheilhy/cmux"
)

func main() {
	InitLogger()

	// Route URL: http://gateway-api/route/{chain_id}/{project_id}
	routeChecker := "http://gateway-api/route"
	if value, ok := os.LookupEnv("GATEWAY_API_ROUTE_URL"); ok {
		routeChecker = value
	}

	l, err := net.Listen("tcp", ":80")
	if err != nil {
		log.Fatalln(err)
	}

	m := cmux.New(l)
	// grpcL := m.Match(cmux.HTTP2HeaderField("content-type", "application/grpc"))
	grpcL := m.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	httpL := m.Match(cmux.HTTP1Fast())

	grpcS := buildGrpcProxyServer(routeChecker)
	httpS := &http.Server{Handler: NewRouter(routeChecker)}

	go grpcS.Serve(grpcL)
	go httpS.Serve(httpL)

	m.Serve()
}
