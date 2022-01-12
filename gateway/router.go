package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// const rateLimitPath = "/limit"
const healthCheckPath = "/health"

type (
	Proxy struct {
		rpc *HttpProxy
		ws  *WebsocketProxy
	}

	Router struct {
		routes       map[string]*Proxy
		routeChecker string
	}

	RouteResponse struct {
		Route  bool `json:"route"`
		Target struct {
			RPC string `json:"rpc"`
			WS  string `json:"ws"`
		} `json:"target"`
	}
)

func NewRouter(routeChecker string) *Router {
	return &Router{
		routeChecker: routeChecker,
		routes:       map[string]*Proxy{},
	}
}

func (r *Router) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Health Check
	if req.URL.Path == healthCheckPath {
		rw.Header().Set("Content-Type", "text/plain")
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("OK"))
		return
	}

	// Using regexp extract path. /myriad/sbbdluuarbc524e9h3zd2fu4macyl306
	re := regexp.MustCompile(`^/(?P<chain>[a-z][-a-z0-9]*[a-z0-9]?)/(?P<project>[a-z0-9]{32})$`)
	params := re.FindStringSubmatch(req.URL.Path)
	if len(params) < 3 {
		log.Println("[400] Bad Request", req.URL.Path)
		rw.Header().Set("Content-Type", "text/plain")
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte("Bad Request"))
		return
	}
	chain, project := params[1], params[2]

	// Check if the request should be routed
	routeResp := RouteResponse{}
	if err := r.shouldRoute(chain, project, &routeResp); err != nil {
		log.Println("[500] Internal Server Error", err)
		rw.Header().Set("Content-Type", "text/plain")
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Internal Server Error"))
		return
	}

	if !routeResp.Route {
		log.Println("[403] Forbidden", req.URL.Path)
		rw.Header().Set("Content-Type", "text/plain")
		rw.WriteHeader(http.StatusForbidden)
		rw.Write([]byte("Forbidden"))
		return
	}

	// Create proxy if it does not exist
	proxy, found := r.routes[chain]
	if !found {
		proxy = r.addRoute(chain, routeResp.Target.RPC, routeResp.Target.WS)
	}
	if proxy == nil {
		log.Println("[404] Not Found", req.URL.Path)
		rw.Header().Set("Content-Type", "text/plain")
		rw.WriteHeader(http.StatusNotFound)
		rw.Write([]byte("Not Found"))
		return
	}

	// Route request
	switch req.URL.Scheme {
	case "http":
	case "https":
		log.Println("[rpc]", req.URL.Path)
		proxy.rpc.ServeHTTP(rw, req)
	case "ws":
	case "wss":
		log.Println("[ws]", req.URL.Path)
		proxy.ws.ServeHTTP(rw, req)
	default:
		// TODO: connection := req.Header.Get("Connection")
		if upgrade := req.Header.Get("Upgrade"); upgrade == "websocket" {
			log.Println("[ws]", req.URL.Path)
			proxy.ws.ServeHTTP(rw, req)
		} else {
			log.Println("[rpc]", req.URL.Path)
			proxy.rpc.ServeHTTP(rw, req)
		}
	}
}

func (r *Router) addRoute(chain string, rpc string, ws string) *Proxy {
	u1, _ := url.Parse(rpc)
	u2, _ := url.Parse(ws)
	if u1 != nil && u2 != nil {
		proxy := &Proxy{rpc: NewHttpProxy(u1), ws: NewWebsocketProxy(u2)}
		r.routes[chain] = proxy
		return proxy
	}
	return nil
}

func (r *Router) shouldRoute(chain, project string, target interface{}) error {
	client := &http.Client{Timeout: 1 * time.Second}
	url := fmt.Sprintf("%s/%s/%s", r.routeChecker, chain, project)
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}
