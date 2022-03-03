package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/rs/xid"
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
	uuid := xid.New().String()
	dumpRequest(req, uuid, true)
	resp, err := t.RoundTripper.RoundTrip(req)
	if err == nil {
		dumpResponse(resp, uuid, true)
	}
	return resp, err
}

func drainBody(b io.ReadCloser) (r1 io.ReadCloser, r2 []byte, err error) {
	if b == nil || b == http.NoBody {
		// No copying needed. Preserve the magic sentinel meaning of NoBody.
		return http.NoBody, nil, nil
	}
	var buf bytes.Buffer
	if _, err = buf.ReadFrom(b); err != nil {
		return nil, nil, err
	}
	if err = b.Close(); err != nil {
		return nil, nil, err
	}
	return io.NopCloser(&buf), buf.Bytes(), nil
}

func dumpRequest(req *http.Request, uuid string, body bool) error {
	var err error
	var copy []byte
	save := req.Body

	if req.Body != nil {
		save, copy, err = drainBody(req.Body)
		if err != nil {
			return err
		}

		addr := req.RemoteAddr
		path := req.URL.Path
		dumpJsonRpcRequest(uuid, addr, path, copy, body)
	}

	req.Body = save
	return nil
}

func dumpResponse(resp *http.Response, uuid string, body bool) error {
	var err error
	var copy []byte
	save := resp.Body
	savecl := resp.ContentLength

	if resp.Body != nil {
		save, copy, err = drainBody(resp.Body)
		if err != nil {
			return err
		}

		req := resp.Request
		addr := req.RemoteAddr
		path := req.URL.Path
		dumpJsonRpcResponse(uuid, addr, path, copy, body)
	}

	resp.Body = save
	resp.ContentLength = savecl
	return nil
}

// https://github.com/polkadot-js/api/blob/master/packages/rpc-provider/src/types.ts
// https://github.com/polkadot-js/api/blob/master/packages/rpc-provider/src/coder/index.ts

const DumpBodyMaximumSize = 1024

func dumpJsonRpcRequest(uuid, addr, path string, data []byte, body bool) error {
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
	// params := jsonMap["params"]
	if id == nil || method == nil {
		return errors.New("jsonrpc: invalid id/method field in decoded object")
	}

	zap.S().Infow("request",
		"uuid", uuid,
		"remote", addr,
		"path", path,
		"id", id,
		"method", method)
	// "params", params)
	return nil
}

func dumpJsonRpcResponse(uuid, addr, path string, data []byte, body bool) error {
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
		zap.S().Infow("response",
			"uuid", uuid,
			"remote", addr,
			"path", path,
			"id", id,
			"result", result,
			"error", _error,
			"length", len(data))
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
		zap.S().Infow("subscription",
			"uuid", uuid,
			"remote", addr,
			"path", path,
			"method", method,
			"subscription", subscription,
			"result", result,
			"error", _error,
			"length", len(data))
		return nil
	}

	return errors.New("jsonrpc: invalid id field in decoded object")
}
