package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type HttpProxy struct {
	// Backend returns the backend URL which the proxy uses to reverse proxy
	// the incoming HTTP connection. Request is the initial incoming and
	// unmodified request.
	Backend func(*http.Request) *url.URL
}

func NewHttpProxy(target *url.URL) *HttpProxy {
	backend := func(r *http.Request) *url.URL {
		u := *target
		return &u
	}
	return &HttpProxy{Backend: backend}
}

func (h *HttpProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if h.Backend == nil {
		log.Println("websocketproxy: backend function is not defined")
		http.Error(rw, "internal server error (code: 1)", http.StatusInternalServerError)
		return
	}

	backendURL := h.Backend(req)
	if backendURL == nil {
		log.Println("websocketproxy: backend URL is nil")
		http.Error(rw, "internal server error (code: 2)", http.StatusInternalServerError)
		return
	}

	// Create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	// Update the headers to allow for SSL redirection
	// req.URL.Host = url.Host
	// req.URL.Scheme = url.Scheme
	// req.Host = url.Host

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	proxy.ServeHTTP(rw, req)
}
