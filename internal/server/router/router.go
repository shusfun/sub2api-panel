package router

import (
	"context"
	"net/http"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/auth"
	"sub2api-panel/internal/server/billing"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/internal/server/observe"
	"sub2api-panel/internal/server/pool"
	"sub2api-panel/internal/server/system"
	"sub2api-panel/internal/server/users"
)

func WireHubs(s *core.Server) {
	s.UsageHub = core.NewUsageStreamHub(observe.BuildUsageStreamSnapshot(s))
	s.UsageHub.Commit = observe.CommitUsageSnapshot(s)
	s.OpsHub = core.NewOpsStreamHub(observe.BuildOpsStreamSnapshot(s))
	s.AccountHub = core.NewOpsStreamHub(pool.BuildAccountActivitySnapshot(s))
}

func StartRuntime(ctx context.Context, s *core.Server) {
	s.EnsurePlugins()
	observe.StartPatrolScheduler(ctx, s)
	s.StartBalanceMonitor(ctx)
	WireHubs(s)
}

func NewHandler(s *core.Server, static http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", static)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		r, end := platform.TraceRequest(r, w, "unknown route")
		defer end()
		platform.WriteError(w, r, platform.Fail(404, "not_found"))
	})

	auth.RegisterRoutes(mux, s)
	pool.RegisterRoutes(mux, s)
	users.RegisterRoutes(mux, s)
	billing.RegisterRoutes(mux, s)
	observe.RegisterRoutes(mux, s)
	system.RegisterRoutes(mux, s)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		policy := "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'none'; form-action 'self'"
		if r.URL.Path == "/verification.html" {
			const vendors = " https://challenges.cloudflare.com https://turing.captcha.qcloud.com https://ca.turing.captcha.qcloud.com https://*.alicdn.com https://*.aliyuncs.com"
			policy = "default-src 'none'; sandbox allow-scripts allow-forms allow-popups; script-src 'self'" + vendors +
				"; style-src 'unsafe-inline'" + vendors + "; img-src data:" + vendors + "; connect-src" + vendors +
				"; frame-src" + vendors + "; frame-ancestors " + s.Origin + "; base-uri 'none'; form-action 'none'"
		} else if strings.HasPrefix(r.URL.Path, "/custom/") && IsPanelDocumentRoute(r.URL.Path) {
			policy = strings.Replace(policy, "frame-src 'self'", "frame-src https: http:", 1)
		} else if strings.HasPrefix(r.URL.Path, "/api/plugins/frame/") {
			policy = "default-src 'none'; sandbox allow-scripts; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data:; frame-ancestors " + s.Origin + "; base-uri 'none'; form-action 'none'"
		} else if (r.URL.Path == "/key-usage" || r.URL.Path == "/batch-image") && platform.CheckOrigin(s.Official, false) && !strings.ContainsAny(s.Official, ";, ") {
			policy = strings.Replace(policy, "connect-src 'self'", "connect-src 'self' "+s.Official, 1)
		}
		// The isolated verification document has an opaque origin and loads one public Vite module.
		if strings.HasPrefix(r.URL.Path, "/assets/verification-") && r.Header.Get("Origin") == "null" {
			w.Header().Set("Access-Control-Allow-Origin", "null")
		}
		w.Header().Set("Content-Security-Policy", policy)
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		mux.ServeHTTP(w, r)
	})
}
