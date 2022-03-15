package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var (
	// DefaultUpgrader specifies the parameters for upgrading an HTTP
	// connection to a WebSocket connection.
	DefaultUpgrader = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,

		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// DefaultDialer is a dialer with all fields set to the default zero values.
	// DefaultDialer = websocket.DefaultDialer
	DefaultDialer = &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,

		ReadBufferSize:  1024,
		WriteBufferSize: 1024 * 256,
	}
)

// WebsocketProxy is an HTTP Handler that takes an incoming WebSocket
// connection and proxies it to another server.
type WebsocketProxy struct {
	// Director, if non-nil, is a function that may copy additional request
	// headers from the incoming WebSocket connection into the output headers
	// which will be forwarded to another server.
	Director func(incoming *http.Request, out http.Header)

	// Backend returns the backend URL which the proxy uses to reverse proxy
	// the incoming WebSocket connection. Request is the initial incoming and
	// unmodified request.
	Backend func(*http.Request) *url.URL

	// Upgrader specifies the parameters for upgrading a incoming HTTP
	// connection to a WebSocket connection. If nil, DefaultUpgrader is used.
	Upgrader *websocket.Upgrader

	//  Dialer contains options for connecting to the backend WebSocket server.
	//  If nil, DefaultDialer is used.
	Dialer *websocket.Dialer
}

// NewProxy returns a new Websocket reverse proxy that rewrites the
// URL's to the scheme, host and base path provider in target.
func NewWebsocketProxy(target *url.URL) *WebsocketProxy {
	backend := func(r *http.Request) *url.URL {
		u := *target
		return &u
	}
	return &WebsocketProxy{Backend: backend}
}

// ServeHTTP implements the http.Handler that proxies WebSocket connections.
func (w *WebsocketProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if w.Backend == nil {
		http.Error(rw, http.StatusText(http.StatusServiceUnavailable), http.StatusInternalServerError)
		return
	}

	backendURL := w.Backend(req)
	if backendURL == nil {
		http.Error(rw, http.StatusText(http.StatusServiceUnavailable), http.StatusInternalServerError)
		return
	}

	dialer := w.Dialer
	if w.Dialer == nil {
		dialer = DefaultDialer
	}

	// Pass headers from the incoming request to the dialer to forward them to
	// the final destinations.
	requestHeader := http.Header{}
	if origin := req.Header.Get("User-Agent"); origin != "" {
		requestHeader.Add("User-Agent", origin)
	}
	if origin := req.Header.Get("Origin"); origin != "" {
		requestHeader.Add("Origin", origin)
	}
	for _, prot := range req.Header[http.CanonicalHeaderKey("Sec-WebSocket-Protocol")] {
		requestHeader.Add("Sec-WebSocket-Protocol", prot)
	}
	for _, cookie := range req.Header[http.CanonicalHeaderKey("Cookie")] {
		requestHeader.Add("Cookie", cookie)
	}
	if req.Host != "" {
		requestHeader.Set("Host", req.Host)
	}

	// Pass X-Forwarded-For headers too, code below is a part of
	// httputil.ReverseProxy. See http://en.wikipedia.org/wiki/X-Forwarded-For
	// for more information
	// TODO: use RFC7239 http://tools.ietf.org/html/rfc7239
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := req.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		requestHeader.Set("X-Forwarded-For", clientIP)
	}

	// Set the originating protocol of the incoming HTTP request. The SSL might
	// be terminated on our site and because we doing proxy adding this would
	// be helpful for applications on the backend.
	requestHeader.Set("X-Forwarded-Proto", "http")
	if req.TLS != nil {
		requestHeader.Set("X-Forwarded-Proto", "https")
	}

	// Enable the director to copy any additional headers it desires for
	// forwarding to the remote server.
	if w.Director != nil {
		w.Director(req, requestHeader)
	}

	// Connect to the backend URL, also pass the headers we get from the requst
	// together with the Forwarded headers we prepared above.
	// TODO: support multiplexing on the same backend connection instead of
	// opening a new TCP connection time for each request. This should be
	// optional:
	// http://tools.ietf.org/html/draft-ietf-hybi-websocket-multiplexing-01
	connBackend, resp, err := dialer.Dial(backendURL.String(), requestHeader)
	if err != nil {
		zap.S().Errorw(fmt.Sprintf("ws: couldn't dial to remote backend url | %s", err))
		if resp != nil {
			// If the WebSocket handshake fails, ErrBadHandshake is returned
			// along with a non-nil *http.Response so that callers can handle
			// redirects, authentication, etcetera.
			if err := copyResponse(rw, resp); err != nil {
				zap.S().Errorw(fmt.Sprintf("ws: couldn't write response after failed remote backend handshake | %s", err))
			}
		} else {
			http.Error(rw, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		}
		return
	}
	connBackend.EnableWriteCompression(false)
	defer connBackend.Close()

	upgrader := w.Upgrader
	if w.Upgrader == nil {
		upgrader = DefaultUpgrader
	}

	// Only pass those headers to the upgrader.
	upgradeHeader := http.Header{}
	if hdr := resp.Header.Get("Sec-Websocket-Protocol"); hdr != "" {
		upgradeHeader.Set("Sec-Websocket-Protocol", hdr)
	}
	if hdr := resp.Header.Get("Set-Cookie"); hdr != "" {
		upgradeHeader.Set("Set-Cookie", hdr)
	}

	// Now upgrade the existing incoming request to a WebSocket connection.
	// Also pass the header that we gathered from the Dial handshake.
	connPub, err := upgrader.Upgrade(rw, req, upgradeHeader)
	if err != nil {
		zap.S().Errorw(fmt.Sprintf("ws: couldn't upgrade | %s", err))
		return
	}
	defer connPub.Close()

	errClient := make(chan error, 1)
	errBackend := make(chan error, 1)
	replicateWebsocketConn := func(dst, src *websocket.Conn, errc chan error, logger func(data []byte)) {
		for {
			msgType, msg, err := src.ReadMessage()
			if err != nil {
				m := websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("%v", err))
				if e, ok := err.(*websocket.CloseError); ok {
					if e.Code != websocket.CloseNoStatusReceived {
						m = websocket.FormatCloseMessage(e.Code, e.Text)
					}
				}
				errc <- err
				dst.WriteMessage(websocket.CloseMessage, m)
				break
			}
			err = dst.WriteMessage(msgType, msg)
			if err != nil {
				errc <- err
				break
			}

			// Log request/response & subscription
			logger(msg)
		}
	}

	// Log request/response & subscription
	type request struct {
		Id        interface{}
		Method    interface{}
		Timestamp time.Time
	}

	var requestCache = struct {
		sync.RWMutex // read & write simultaneously?
		m            map[interface{}]request
	}{m: make(map[interface{}]request)}

	logRequest := func(data []byte) {
		var jsonMap map[string]interface{}
		if err := json.Unmarshal(data, &jsonMap); err != nil {
			zap.S().Errorw(fmt.Sprintf("rpc: couldn't parse request | %s", err))
			return
		}

		id := jsonMap["id"]
		requestCache.Lock()
		requestCache.m[id] = request{id, jsonMap["method"], time.Now()}
		requestCache.Unlock()
	}

	logResponse := func(data []byte) {
		var jsonMap map[string]interface{}
		if err := json.Unmarshal(data, &jsonMap); err != nil {
			zap.S().Errorw(fmt.Sprintf("rpc: couldn't parse response | %s", err))
			return
		}

		id := jsonMap["id"]
		if id != nil {
			requestCache.RLock()
			v, ok := requestCache.m[id]
			requestCache.RUnlock()
			if ok {
				zap.S().Infow("request",
					"path", req.URL.Path,
					"id", v.Id,
					"method", v.Method,
					"error", jsonMap["error"],
					"length", len(data),
					"timestamp", v.Timestamp.Unix(),
					"duration", time.Since(v.Timestamp))
				requestCache.Lock()
				delete(requestCache.m, id)
				requestCache.Unlock()
			} else {
				zap.S().Errorw(fmt.Sprintf("ws: couldn't get request %v %s", id, req.URL.Path))
			}
		} else {
			method := jsonMap["method"]
			params, _ := jsonMap["params"].(map[string]interface{})
			if method != nil && params != nil {
				zap.S().Infow("subscription",
					"path", req.URL.Path,
					"subscription", params["subscription"],
					"method", method,
					"error", params["error"],
					"length", len(data),
					"timestamp", time.Now().Unix(),
					"duration", 0)
			} else {
				zap.S().Errorw(fmt.Sprintf("ws: response or subscription %s", req.URL.Path))
			}
		}
	}

	go replicateWebsocketConn(connPub, connBackend, errClient, logResponse)
	go replicateWebsocketConn(connBackend, connPub, errBackend, logRequest)

	var message string
	select {
	case err = <-errClient:
		message = "ws: Error when copying from backend to client | %v"
	case err = <-errBackend:
		message = "ws: Error when copying from client to backend | %v"
	}
	if e, ok := err.(*websocket.CloseError); !ok || e.Code == websocket.CloseAbnormalClosure {
		zap.S().Errorw(fmt.Sprintf(message, err))
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func copyResponse(rw http.ResponseWriter, resp *http.Response) error {
	copyHeader(rw.Header(), resp.Header)
	rw.WriteHeader(resp.StatusCode)
	defer resp.Body.Close()

	_, err := io.Copy(rw, resp.Body)
	return err
}
