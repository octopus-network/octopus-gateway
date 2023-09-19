package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"time"

	"go.uber.org/zap"
)

// const rateLimitPath = "/limit"
const healthCheckPath = "/health"

type (
	Proxy struct {
		rpc  *JsonRpcProxy
		ws   *WebsocketProxy
		rest *RestProxy
	}

	Router struct {
		routes       sync.Map
		routeChecker string
	}

	RouteResponse struct {
		Route  bool `json:"route"`
		Target struct {
			RPC  string `json:"rpc"`
			WS   string `json:"ws"`
			GRPC string `json:"grpc"`
			REST string `json:"rest"`
		} `json:"target"`
	}
)

func NewRouter(routeChecker string) *Router {
	return &Router{
		routeChecker: routeChecker,
	}
}

func (r *Router) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Health Check
	if req.URL.Path == healthCheckPath {
		zap.S().Infow("health", "path", req.URL.Path)
		http.Error(rw, http.StatusText(http.StatusOK), http.StatusOK)
		return
	}

	// - json-rpc
	//    POST /myriad/sbbdluuarbc524e9h3zd2fu4macyl306
	// - cosmos rest via gRPC-gateway
	//    GET  /cosmos/bank/v1beta1/balances/{address}
	//    Header "prefix-path: /myriad/sbbdluuarbc524e9h3zd2fu4macyl306"
	isJsonRpc := true
	re := regexp.MustCompile(`^/(?P<chain>[a-z][-a-z0-9]*[a-z0-9]?)/(?P<project>[a-z0-9]{32})$`)
	params := re.FindStringSubmatch(req.URL.Path)
	if len(params) != 3 {
		params = re.FindStringSubmatch(req.Header.Get(prefixPathKey))
		if len(params) != 3 {
			zap.S().Errorw("router", "path", req.URL.Path, "status", http.StatusBadRequest)
			http.Error(rw, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		isJsonRpc = false
	}
	chain, project := params[1], params[2]

	// Check if the request should be routed
	routeResp := RouteResponse{}
	if err := r.shouldRoute(chain, project, &routeResp); err != nil {
		zap.S().Errorw("router", "path", req.URL.Path, "statue", http.StatusInternalServerError)
		http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if !routeResp.Route {
		zap.S().Errorw("router", "path", req.URL.Path, "statue", http.StatusForbidden)
		http.Error(rw, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	// Create proxy if it does not exist
	value, ok := r.routes.Load(chain)
	if !ok {
		value = r.addRoute(chain, routeResp.Target.RPC, routeResp.Target.WS, routeResp.Target.REST)
	}
	if value == nil {
		zap.S().Errorw("router", "path", req.URL.Path, "statue", http.StatusNotFound)
		http.Error(rw, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	proxy := value.(*Proxy)

	// Route request
	switch req.URL.Scheme {
	case "http":
	case "https":
		if isJsonRpc {
			zap.S().Infow("router", "path", req.URL.Path, "target", routeResp.Target.RPC)
			proxy.rpc.ServeHTTP(rw, req)
		} else {
			zap.S().Infow("router", "path", req.URL.Path, "header", req.Header.Get(prefixPathKey), "target", routeResp.Target.REST)
			proxy.rest.ServeHTTP(rw, req)
		}
	case "ws":
	case "wss":
		zap.S().Infow("router", "path", req.URL.Path, "target", routeResp.Target.WS)
		proxy.ws.ServeHTTP(rw, req)
	default:
		// TODO: connection := req.Header.Get("Connection")
		if upgrade := req.Header.Get("Upgrade"); upgrade == "websocket" {
			zap.S().Infow("router", "path", req.URL.Path, "target", routeResp.Target.WS)
			proxy.ws.ServeHTTP(rw, req)
		} else {
			if isJsonRpc {
				zap.S().Infow("router", "path", req.URL.Path, "target", routeResp.Target.RPC)
				proxy.rpc.ServeHTTP(rw, req)
			} else {
				zap.S().Infow("router", "path", req.URL.Path, "header", req.Header.Get(prefixPathKey), "target", routeResp.Target.REST)
				proxy.rest.ServeHTTP(rw, req)
			}
		}
	}
}

func (r *Router) addRoute(chain string, rpc, ws, rest string) interface{} {
	u1, _ := url.Parse(rpc)
	u2, _ := url.Parse(ws)
	u3, _ := url.Parse(rest)
	if u1 == nil {
		return nil
	}

	proxy := &Proxy{rpc: NewJsonRpcProxy(u1)}
	if u2 != nil {
		proxy.ws = NewWebsocketProxy(u2)
	}
	if u3 != nil {
		proxy.rest = NewRestProxy(u3)
	}
	actual, _ := r.routes.LoadOrStore(chain, proxy)
	return actual
}

func (r *Router) shouldRoute(chain, project string, target interface{}) error {
	client := &http.Client{Timeout: 1 * time.Second}
	// Route URL: http://gateway-api/route/{chain_id}/{project_id}
	url := fmt.Sprintf("%s/%s/%s", r.routeChecker, chain, project)
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}
