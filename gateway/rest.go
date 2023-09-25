package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"time"

	"go.uber.org/zap"
)

type RestProxy struct {
	Proxy *httputil.ReverseProxy
}

func NewRestProxy(target *url.URL) *RestProxy {
	// proxy := httputil.NewSingleHostReverseProxy(target)
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		// - v2 cosmos rest via gRPC-gateway
		//    /lcd/myriad/sbbdluuarbc524e9h3zd2fu4macyl306/cosmos/bank/v1beta1/balances/{address}
		//    --> /cosmos/bank/v1beta1/balances/{address}
		re := regexp.MustCompile(v2PathRegex)
		params := re.FindStringSubmatch(req.URL.Path)
		if len(params) == 5 {
			req.URL.Path = params[4]
			req.URL.RawPath = params[4]
		} else {
			req.URL.Path = target.Path
			req.URL.RawPath = target.RawPath
			req.URL.RawQuery = target.RawQuery
		}
	}
	proxy := &httputil.ReverseProxy{Director: director}
	return &RestProxy{Proxy: proxy}
}

func (h *RestProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	prw := &RestProxyResponseWriter{
		ResponseWriter: rw,
		responseSize:   0,
		statusCode:     http.StatusOK,
	}
	ts := time.Now()

	h.Proxy.ServeHTTP(prw, req)

	zap.S().Infow("request",
		"path", req.RequestURI,
		"status", prw.statusCode,
		"length", prw.responseSize,
		"timestamp", ts.Unix(),
		"duration", time.Since(ts))
}

type RestProxyResponseWriter struct {
	http.ResponseWriter
	responseSize int64
	statusCode   int
}

func (w *RestProxyResponseWriter) Write(b []byte) (int, error) {
	size, err := w.ResponseWriter.Write(b)
	w.responseSize += int64(size)
	return size, err
}

func (w *RestProxyResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
