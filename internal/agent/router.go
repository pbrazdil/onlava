package agent

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
)

func (s *Server) routerMux() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := requestHost(req)
		if host == "console.onlava.localhost" {
			s.handleConsole(w, req)
			return
		}
		kind, sessionID, ok := routeHostParts(host)
		if !ok {
			http.NotFound(w, req)
			return
		}
		session, found := s.registry.Get(sessionID)
		if !found {
			http.NotFound(w, req)
			return
		}
		backend, ok := session.Backends[kind]
		if !ok {
			http.NotFound(w, req)
			return
		}
		if isFrontendSessionBackend(kind) {
			s.handleFrontendRoute(w, req, session, backend)
			return
		}
		proxyBackend(w, req, backend, "")
	})
}

func (s *Server) handleFrontendRoute(w http.ResponseWriter, req *http.Request, session Session, backend Backend) {
	if req.URL.Path == "/__onlava/config" {
		api, ok := session.Backends[RouteAPI]
		if !ok {
			http.NotFound(w, req)
			return
		}
		proxyBackend(w, req, api, "")
		return
	}
	if isProtectedFrontendPath(req.URL.Path) {
		http.NotFound(w, req)
		return
	}
	proxyBackendWithOptions(w, req, backend, proxyBackendOptions{
		spaFallback: shouldUseSPAFallback(req),
	})
}

func (s *Server) handleConsole(w http.ResponseWriter, req *http.Request) {
	if s.dashboard.Addr != "" {
		proxyBackend(w, req, s.dashboard, "")
		return
	}
	path := strings.Trim(req.URL.Path, "/")
	if path == "" {
		s.serveConsoleIndex(w, req)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) >= 2 && parts[0] == "s" {
		sessionID := parts[1]
		session, ok := s.registry.Get(sessionID)
		if !ok {
			http.NotFound(w, req)
			return
		}
		backend, ok := session.Backends[RouteDashboard]
		if !ok {
			http.NotFound(w, req)
			return
		}
		strip := "/s/" + sessionID
		proxyBackend(w, req, backend, strip)
		return
	}
	http.NotFound(w, req)
}

func (s *Server) serveConsoleIndex(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, "<!doctype html><html><head><meta charset=\"utf-8\"><title>onlava Agent</title></head><body><main><h1>onlava Agent</h1><ul>")
	for _, session := range s.registry.List() {
		href := "/s/" + session.SessionID
		_, _ = fmt.Fprintf(w, "<li><a href=\"%s\">%s</a> <code>%s</code></li>", href, session.SessionID, session.AppRoot)
	}
	_, _ = io.WriteString(w, "</ul></main></body></html>")
}

func requestHost(req *http.Request) string {
	host := strings.ToLower(strings.TrimSpace(req.Host))
	if host == "" && req.URL != nil {
		host = strings.ToLower(strings.TrimSpace(req.URL.Host))
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host
}

func routeHostParts(host string) (kind, sessionID string, ok bool) {
	const suffix = ".onlava.localhost"
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if !strings.HasSuffix(host, suffix) {
		return "", "", false
	}
	prefix := strings.TrimSuffix(host, suffix)
	parts := strings.Split(prefix, ".")
	if len(parts) != 2 {
		return "", "", false
	}
	kind = sanitizeLabel(parts[0])
	sessionID = sanitizeLabel(parts[1])
	if kind == "" || sessionID == "" {
		return "", "", false
	}
	return kind, sessionID, true
}

type proxyBackendOptions struct {
	stripPrefix string
	spaFallback bool
}

func proxyBackend(w http.ResponseWriter, req *http.Request, backend Backend, stripPrefix string) {
	proxyBackendWithOptions(w, req, backend, proxyBackendOptions{stripPrefix: stripPrefix})
}

func proxyBackendWithOptions(w http.ResponseWriter, req *http.Request, backend Backend, opts proxyBackendOptions) {
	target := &url.URL{Scheme: "http", Host: backend.Addr}
	transport := http.DefaultTransport
	if backend.Network == "unix" {
		target.Host = "unix"
		addr := backend.Addr
		transport = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", addr)
			},
		}
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = transport
	originalDirector := proxy.Director
	proxy.Director = func(out *http.Request) {
		originalDirector(out)
		out.Host = req.Host
		out.Header.Set("X-Forwarded-Host", req.Host)
		out.Header.Set("X-Forwarded-Proto", forwardedProto(req))
		out.Header.Set("X-Forwarded-Port", forwardedPort(req))
		if opts.stripPrefix != "" {
			out.URL.Path = strings.TrimPrefix(req.URL.Path, opts.stripPrefix)
			if out.URL.Path == "" {
				out.URL.Path = "/"
			}
		}
	}
	if opts.spaFallback {
		proxy.ModifyResponse = func(resp *http.Response) error {
			if resp.StatusCode != http.StatusNotFound || resp.Request == nil || resp.Request.URL == nil {
				return nil
			}
			fallbackReq := resp.Request.Clone(req.Context())
			fallbackReq.URL.Path = "/"
			fallbackReq.URL.RawPath = ""
			fallbackReq.URL.RawQuery = ""
			fallbackResp, err := transport.RoundTrip(fallbackReq)
			if err != nil {
				return nil
			}
			_ = resp.Body.Close()
			resp.StatusCode = fallbackResp.StatusCode
			resp.Status = fallbackResp.Status
			resp.Header = fallbackResp.Header
			resp.Body = fallbackResp.Body
			resp.ContentLength = fallbackResp.ContentLength
			resp.TransferEncoding = fallbackResp.TransferEncoding
			resp.Trailer = fallbackResp.Trailer
			return nil
		}
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, req)
}

func isFrontendSessionBackend(kind string) bool {
	switch kind {
	case "", RouteAPI, RouteDashboard, RouteGrafana, RouteTemporal, "electric", "removed-agent-transport", "sync":
		return false
	default:
		return true
	}
}

func isProtectedFrontendPath(value string) bool {
	value = cleanRequestPath(value)
	for _, prefix := range []string{"/__onlava", "/api", "/sync"} {
		if value == prefix || strings.HasPrefix(value, prefix+"/") {
			return true
		}
	}
	return false
}

func shouldUseSPAFallback(req *http.Request) bool {
	if req == nil || req.URL == nil {
		return false
	}
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return false
	}
	if isConcreteAssetPath(req.URL.Path) {
		return false
	}
	accept := strings.ToLower(req.Header.Get("Accept"))
	return strings.Contains(accept, "text/html")
}

func isConcreteAssetPath(value string) bool {
	value = cleanRequestPath(value)
	if value == "/" {
		return false
	}
	for _, prefix := range []string{"/assets/", "/static/", "/public/"} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	base := path.Base(value)
	return strings.Contains(base, ".")
}

func cleanRequestPath(value string) string {
	if value == "" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return path.Clean(value)
}

func forwardedProto(req *http.Request) string {
	if req.TLS != nil {
		return "https"
	}
	return "http"
}

func forwardedPort(req *http.Request) string {
	if _, port, err := net.SplitHostPort(strings.TrimSpace(req.Host)); err == nil && port != "" {
		return port
	}
	if req.TLS != nil {
		return "443"
	}
	return "80"
}
