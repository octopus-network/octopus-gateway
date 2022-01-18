package main

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type JsonRpcRequest struct {
	Id     uint64      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type JsonRpcResponse struct {
	Id     uint64           `json:"id"`
	Result *json.RawMessage `json:"result"`
	Error  interface{}      `json:"error"`
}

type ProxyTransport struct {
	http.RoundTripper
}

type HttpProxy struct {
	// ReverseProxy is an HTTP Handler that takes an incoming request and
	// sends it to another server, proxying the response back to the
	// client.
	Proxy *httputil.ReverseProxy
}

func NewHttpProxy(target *url.URL) *HttpProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	// proxy.Director = func(r *http.Request) {}
	proxy.Transport = &ProxyTransport{http.DefaultTransport}
	return &HttpProxy{Proxy: proxy}
}

func (h *HttpProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.Proxy.ServeHTTP(rw, req)
}

func (t *ProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// TODO: usage statistics
	return t.RoundTripper.RoundTrip(req)
}
