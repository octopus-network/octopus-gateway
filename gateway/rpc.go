package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

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
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = target.Path
		req.URL.RawPath = target.RawPath
		req.URL.RawQuery = target.RawQuery
	}
	transport := &ProxyTransport{http.DefaultTransport}
	proxy := &httputil.ReverseProxy{
		Director:  director,
		Transport: transport,
	}
	return &HttpProxy{Proxy: proxy}
}

func (h *HttpProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.Proxy.ServeHTTP(rw, req)
}

func (t *ProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 20221119 patch cors http method options
	if req.Method == http.MethodOptions {
		return http.DefaultTransport.RoundTrip(req)
	}

	ts := time.Now()
	_, id, method, err := parseRequest(req)
	if err != nil {
		zap.S().Errorw(fmt.Sprintf("rpc: couldn't parse request | %s", err))
	}

	var result interface{}
	var _error interface{}
	resp, err := t.RoundTripper.RoundTrip(req)
	if err == nil {
		result, _error, err = parseResponse(resp)
		if err != nil {
			zap.S().Errorw(fmt.Sprintf("rpc: couldn't parse response | %s", err))
		}
	}

	zap.S().Infow("request",
		"path", req.RequestURI,
		"id", id,
		"method", method,
		"error", _error,
		"length", result,
		"timestamp", ts.Unix(),
		"duration", time.Since(ts))

	return resp, err
}

// https://github.com/polkadot-js/api/blob/master/packages/rpc-provider/src/types.ts
// https://github.com/polkadot-js/api/blob/master/packages/rpc-provider/src/coder/index.ts

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

func parseRequest(req *http.Request) (version, id, method interface{}, err error) {
	var copy []byte
	save := req.Body

	if req.Body != nil {
		save, copy, err = drainBody(req.Body)
		if err != nil {
			return
		}

		var jsonMap map[string]interface{}
		if err = json.Unmarshal(copy, &jsonMap); err != nil {
			return
		}
		version = jsonMap["jsonrpc"]
		id = jsonMap["id"]
		method = jsonMap["method"]
	}

	req.Body = save
	return
}

func parseResponse(resp *http.Response) (result, _error interface{}, err error) {
	var copy []byte
	save := resp.Body
	savecl := resp.ContentLength

	if resp.Body != nil {
		save, copy, err = drainBody(resp.Body)
		if err != nil {
			return
		}

		var jsonMap map[string]interface{}
		if err = json.Unmarshal(copy, &jsonMap); err != nil {
			return
		}
		result = len(copy) //jsonMap["result"]
		_error = jsonMap["error"]
	}

	resp.Body = save
	resp.ContentLength = savecl
	return
}
