package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"go.uber.org/zap"
)

type JsonRpcRequest struct {
	Id     *json.RawMessage `json:"id"`
	Method string           `json:"method"`
	Params *json.RawMessage `json:"params"`
}

type JsonRpcResponse struct {
	Id     *json.RawMessage `json:"id"`
	Result *json.RawMessage `json:"result"`
	Error  any              `json:"error"`
}

type JsonRpcProxyTransport struct {
	http.RoundTripper
}

type JsonRpcProxy struct {
	// ReverseProxy is an HTTP Handler that takes an incoming request and
	// sends it to another server, proxying the response back to the
	// client.
	Proxy *httputil.ReverseProxy
}

func NewJsonRpcProxy(target *url.URL) *JsonRpcProxy {
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = target.Path
		req.URL.RawPath = target.RawPath
		req.URL.RawQuery = target.RawQuery
	}
	transport := &JsonRpcProxyTransport{http.DefaultTransport}
	proxy := &httputil.ReverseProxy{
		Director:  director,
		Transport: transport,
	}
	return &JsonRpcProxy{Proxy: proxy}
}

func (h *JsonRpcProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.Proxy.ServeHTTP(rw, req)
}

func (t *JsonRpcProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 20221119 patch cors http method options
	if req.Method != http.MethodPost {
		return http.DefaultTransport.RoundTrip(req)
	}

	ts := time.Now()
	_req, err := parseRequest(req)
	if err != nil {
		zap.S().Errorw(fmt.Sprintf("rpc: Parse Request Error | %s", err))
	}

	resp, err := t.RoundTripper.RoundTrip(req)
	if err != nil {
		zap.S().Errorw(fmt.Sprintf("rpc: Round Trip Error | %s", err))
		return resp, err
	}

	_resp, _len, err := parseResponse(resp)
	if err != nil {
		zap.S().Errorw(fmt.Sprintf("rpc: Parse Response Error | %s", err))
	}

	if _req != nil && _resp != nil {
		switch i := _req.(type) {
		case JsonRpcRequest:
			switch j := _resp.(type) {
			case JsonRpcResponse:
				zap.S().Infow("request",
					"path", req.RequestURI,
					"id", i.Id,
					"method", i.Method,
					"error", j.Error,
					"length", _len,
					"timestamp", ts.Unix(),
					"duration", time.Since(ts))
			default:
				zap.S().Errorw("request",
					"path", req.RequestURI,
					"timestamp", ts.Unix(),
					"duration", time.Since(ts),
					"request", "Type JsonRpcRequest",
					"response", fmt.Sprintf("Type %T", j))
			}
		case []JsonRpcRequest:
			switch j := _resp.(type) {
			case []JsonRpcResponse:
				randomBytes := make([]byte, 8)
				if _, err := rand.Read(randomBytes); err != nil {
					randomBytes = []byte("12345678")
				}
				randomString := base64.URLEncoding.EncodeToString(randomBytes)
				batch := randomString[:8]
				for _, x := range i {
					for _, y := range j {
						if bytes.Equal(*x.Id, *y.Id) {
							zap.S().Infow("request",
								"path", req.RequestURI,
								"batch", batch,
								"id", x.Id,
								"method", x.Method,
								"error", y.Error,
								"length", _len, // batch total length
								"timestamp", ts.Unix(),
								"duration", time.Since(ts))
						}
					}
				}
			default:
				zap.S().Errorw("request",
					"path", req.RequestURI,
					"timestamp", ts.Unix(),
					"duration", time.Since(ts),
					"request", "Type []JsonRpcRequest",
					"response", fmt.Sprintf("Type %T", j))
			}
		default:
			zap.S().Errorw("request",
				"path", req.RequestURI,
				"timestamp", ts.Unix(),
				"duration", time.Since(ts),
				"request", fmt.Sprintf("Type %T", i))
		}
	} else {
		zap.S().Errorw("request",
			"path", req.RequestURI,
			"timestamp", ts.Unix(),
			"duration", time.Since(ts),
			"request", fmt.Sprintf("nil? %v\n", _req == nil),
			"response", fmt.Sprintf("nil? %v\n", _resp == nil))
	}

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

func parseRequest(req *http.Request) (interface{}, error) {
	if req.Body == nil {
		return nil, nil
	}

	var copy []byte
	save, copy, err := drainBody(req.Body)
	if err != nil {
		return nil, err
	}

	var request JsonRpcRequest
	if err = json.Unmarshal(copy, &request); err == nil {
		req.Body = save
		return request, nil
	}

	var requests []JsonRpcRequest
	if err = json.Unmarshal(copy, &requests); err == nil {
		req.Body = save
		return requests, nil
	}

	return nil, errors.New("json: cannot unmarshal array into JsonRpcRequest or []JsonRpcRequest")
}

func parseResponse(resp *http.Response) (interface{}, int, error) {
	if resp.Body == nil {
		return nil, 0, nil
	}

	var copy []byte
	savecl := resp.ContentLength
	save, copy, err := drainBody(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	var response JsonRpcResponse
	if err = json.Unmarshal(copy, &response); err == nil {
		resp.Body = save
		resp.ContentLength = savecl
		return response, len(copy), nil
	}

	var responses []JsonRpcResponse
	if err = json.Unmarshal(copy, &responses); err == nil {
		resp.Body = save
		resp.ContentLength = savecl
		return responses, len(copy), nil
	}

	return nil, 0, errors.New("json: cannot unmarshal array into JsonRpcResponse or []JsonRpcResponse")
}
