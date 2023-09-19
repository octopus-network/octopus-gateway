package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"go.uber.org/zap"
)

type RestProxy struct {
	Proxy *httputil.ReverseProxy
}

func NewRestProxy(target *url.URL) *RestProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
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
