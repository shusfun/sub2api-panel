package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func previewVite(static http.Handler, mode string) (http.Handler, error) {
	host := "127.0.0.1"
	switch mode {
	case "":
		return static, nil
	case "docker":
		host = "host.docker.internal"
	case "local":
	default:
		return nil, fmt.Errorf("PANEL_TEST_BROWSER_VITE must be local or docker")
	}
	target := url.URL{Scheme: "http", Host: net.JoinHostPort(host, "28742")}
	return previewViteForTarget(static, &target), nil
}

func previewViteForTarget(static http.Handler, target *url.URL) http.Handler {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.ResponseHeaderTimeout = 20 * time.Second
	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(target)
			request.Out.Host = browserFixtureAddr
			request.Out.Header.Del("Cookie")
			request.Out.Header.Del("Authorization")
		},
		ModifyResponse: func(response *http.Response) error {
			response.Header.Del("Set-Cookie")
			return nil
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		document := isPanelDocumentRoute(path)
		development := path == "/__vite_hmr" || strings.HasPrefix(path, "/src/") || strings.HasPrefix(path, "/@vite/") ||
			strings.HasPrefix(path, "/@id/") || strings.HasPrefix(path, "/@fs/") || strings.HasPrefix(path, "/node_modules/") || path == "/@react-refresh"
		if (r.Method != http.MethodGet && r.Method != http.MethodHead) || !document && !development {
			static.ServeHTTP(w, r)
			return
		}
		if r.Host != browserFixtureAddr || r.URL.Path == "/__vite_hmr" && r.Header.Get("Origin") != "http://"+browserFixtureAddr {
			http.Error(w, "Invalid preview origin", http.StatusForbidden)
			return
		}
		policy := w.Header().Get("Content-Security-Policy")
		if document {
			// Vite's React refresh preamble is inline in the local development document.
			devPolicy := strings.Replace(policy, "script-src 'self'", "script-src 'self' 'unsafe-inline'", 1)
			devPolicy = strings.Replace(devPolicy, "connect-src 'self'", "connect-src 'self' ws://"+browserFixtureAddr, 1)
			w.Header().Set("Content-Security-Policy", devPolicy)
		}
		requestProxy := *proxy
		requestProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, _ error) {
			w.Header().Set("Content-Security-Policy", policy)
			static.ServeHTTP(w, r)
		}
		requestProxy.ServeHTTP(w, r)
	})
}

func TestPreviewUsesStaticFallbackForHistoryRoutes(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "development document")
	}))
	defer up.Close()
	target, err := url.Parse(up.URL)
	if err != nil {
		t.Fatal(err)
	}
	handler := previewViteForTarget(spaFileServer(fstest.MapFS{"index.html": {Data: []byte("static document")}}), target)
	request := func(path string, status int, body string) {
		t.Helper()
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "http://"+browserFixtureAddr+path, nil))
		if out.Code != status || !strings.Contains(out.Body.String(), body) {
			t.Fatalf("%s returned %d %q", path, out.Code, out.Body.String())
		}
	}
	request("/admin/accounts/1?tab=config", http.StatusOK, "development document")
	request("/missing.js", http.StatusNotFound, "404")
	up.Close()
	request("/admin/accounts/1?tab=config", http.StatusOK, "static document")
}
