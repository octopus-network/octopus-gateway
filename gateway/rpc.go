package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"

	"go.uber.org/zap"
)

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

// https://github.com/polkadot-js/api/blob/master/packages/rpc-provider/src/types.ts
// https://github.com/polkadot-js/api/blob/master/packages/rpc-provider/src/coder/index.ts

const DumpBodyMaximumSize = 1024

func dumpJsonRpcRequest(addr, path string, data []byte, body bool) error {
	var jsonMap map[string]interface{}
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		return err
	}

	version := jsonMap["jsonrpc"]
	if version != "2.0" {
		return errors.New("jsonrpc: invalid jsonrpc field in decoded object")
	}

	id := jsonMap["id"]
	method := jsonMap["method"]
	params := jsonMap["params"]
	if id == nil || method == nil {
		return errors.New("jsonrpc: invalid id/method field in decoded object")
	}

	zap.S().Infow("request", "remote", addr, "path", path, "id", id, "method", method, "params", params)
	return nil
}

func dumpJsonRpcResponse(addr, path string, data []byte, body bool) error {
	var jsonMap map[string]interface{}
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		return err
	}

	version := jsonMap["jsonrpc"]
	if version != "2.0" {
		return errors.New("jsonrpc: invalid jsonrpc field in decoded object")
	}

	id := jsonMap["id"]
	if id != nil {
		_error := jsonMap["error"]
		var result interface{}
		if body && len(data) < DumpBodyMaximumSize {
			result = jsonMap["result"]
		}
		zap.S().Infow("response", "remote", addr, "path", path, "id", id, "result", result, "error", _error)
		return nil
	}

	method := jsonMap["method"]
	params, _ := jsonMap["params"].(map[string]interface{})
	if method != nil && params != nil {
		subscription := params["subscription"]
		_error := params["error"]
		var result interface{}
		if body && len(data) < DumpBodyMaximumSize {
			result = params["result"]
		}
		zap.S().Infow("subscription", "remote", addr, "path", path, "method", method, "subscription", subscription, "result", result, "error", _error)
		return nil
	}

	return errors.New("jsonrpc: invalid id field in decoded object")
}
