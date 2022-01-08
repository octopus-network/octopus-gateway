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

type (
	Proxy struct {
		rpc *HttpProxy
		ws  *WebsocketProxy
	}

	Router struct {
		routes  map[string]*Proxy
		limiter string
	}

	Limiter struct {
		Route  bool `json:"route"`
		Target struct {
			RPC string `json:"rpc"`
			WS  string `json:"ws"`
		} `json:"target"`
	}
)

func NewRouter(limiter string) *Router {
	return &Router{
		limiter: limiter,
		routes:  map[string]*Proxy{},
	}
}

func (r *Router) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Using regexp extract path. /myriad/sbbdluuarbc524e9h3zd2fu4macyl306
	re := regexp.MustCompile(`^/(?P<chain>[a-z][-a-z0-9]*[a-z0-9]?)/(?P<project>[a-z0-9]{32})$`)
	params := re.FindStringSubmatch(req.URL.Path)
	if len(params) < 3 {
		log.Println("[400] Bad Request", req.URL.Path)
		rw.WriteHeader(400)
		rw.Write([]byte("Bad Request"))
		return
	}
	chain, project := params[1], params[2]

	// Check if the request should be routed
	limiter := Limiter{}
	if err := r.shouldRoute(chain, project, &limiter); err != nil {
		log.Println("[500] Internal Server Error", err)
		rw.WriteHeader(500)
		rw.Write([]byte("Internal Server Error"))
		return
	}

	if !limiter.Route {
		log.Println("[403] Forbidden", req.URL.Path)
		rw.WriteHeader(403)
		rw.Write([]byte("Forbidden"))
		return
	}

	// Create proxy if it does not exist
	proxy, found := r.routes[chain]
	if !found {
		proxy = r.addRoute(chain, limiter.Target.RPC, limiter.Target.WS)
	}
	if proxy == nil {
		log.Println("[404] Not Found", req.URL.Path)
		rw.WriteHeader(404)
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
	url := fmt.Sprintf("%s?chain=%s&project=%s", r.limiter, chain, project)
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(target)
}
