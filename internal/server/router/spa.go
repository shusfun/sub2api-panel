package router

import (
	"io/fs"
	"net/http"
	"strings"

	"sub2api-panel/internal/platform"
)

func IsPanelDocumentRoute(p string) bool {
	switch p {
	case "/", "/index.html", "/_ui", "/login", "/register", "/email-verify", "/forgot-password", "/reset-password", "/auth/callback", "/patrol", "/key-usage", "/model-plaza", "/oauth/mcp/authorize", "/mcp", "/purchase", "/payment/result", "/batch-image",
		"/keys", "/usage", "/settings", "/announcements", "/affiliate", "/channels/status", "/admin/ops", "/admin/accounts", "/admin/groups", "/admin/proxies", "/admin/users", "/admin/redeem", "/admin/plugins", "/admin/announcements", "/admin/audit-logs", "/admin/risk-control", "/admin/backups", "/admin/orders",
		"/admin/subscriptions", "/admin/channels", "/admin/upstream-rules", "/admin/affiliates", "/admin/settings":
		return true
	default:
		parent, id, ok := strings.Cut(strings.TrimPrefix(p, "/"), "/")
		if parent == "admin" {
			parent, id, ok = strings.Cut(id, "/")
			switch parent {
			case "accounts", "groups", "proxies", "users":
			default:
				return false
			}
		} else if parent == "custom" {
			return ok && platform.CustomSlugPattern.MatchString(id)
		} else if parent != "keys" {
			return false
		}
		if !ok || len(id) == 0 || len(id) > 16 || id[0] == '0' {
			return false
		}
		for _, digit := range id {
			if digit < '0' || digit > '9' {
				return false
			}
		}
		return true
	}
}

func SPAFileServer(root fs.FS) http.Handler {
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && IsPanelDocumentRoute(r.URL.Path) && r.URL.Path != "/" && r.URL.Path != "/index.html" {
			clone := r.Clone(r.Context())
			clone.URL.Path = "/"
			w.Header().Set("Cache-Control", "no-cache")
			files.ServeHTTP(w, clone)
			return
		}
		files.ServeHTTP(w, r)
	})
}
