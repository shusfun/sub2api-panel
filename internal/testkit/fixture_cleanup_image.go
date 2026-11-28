package testkit

import (
	"net/http"
	"strings"
)

// 用量清理任务与生图对象存储的上游夹具：每个响应都混入 fixtureLeak 字段，用来验证面板白名单。
func (f *fixture) serveCleanupAndImage(w http.ResponseWriter, r *http.Request, path, role string) bool {
	if role != "admin" {
		return false
	}
	task := func(id int, status string) map[string]any {
		return map[string]any{
			"id": id, "status": status, "deleted_rows": 12, "created_at": "2026-11-21T10:00:00Z",
			"created_by": fixtureLeak, "error_message": fixtureLeak,
			"filters": map[string]any{"start_time": "2026-11-01T00:00:00Z", "end_time": "2026-11-07T23:59:59Z", "user_id": 2, "internal": fixtureLeak},
		}
	}
	switch {
	case path == "/admin/usage/cleanup-tasks" && r.Method == http.MethodGet:
		officialSuccess(w, map[string]any{"items": []any{task(7, "running")}, "total": 1, "page": 1, "page_size": 20, "pages": 1})
	case path == "/admin/usage/cleanup-tasks" && r.Method == http.MethodPost:
		officialSuccess(w, task(8, "pending"))
	case strings.HasPrefix(path, "/admin/usage/cleanup-tasks/") && strings.HasSuffix(path, "/cancel"):
		officialSuccess(w, map[string]any{"id": 7, "status": "canceled", "created_by": fixtureLeak})
	case path == "/admin/backups/image-storage" && r.Method == http.MethodGet:
		officialSuccess(w, map[string]any{
			"config": map[string]any{
				"enabled": true, "reuse_backup_s3": false, "bucket": "images", "prefix": "gen/", "public_base_url": "",
				"presign_expiry_hours": 24, "max_download_bytes": 1048576, "endpoint": "https://s3.example.test", "region": "us-east-1",
				"access_key_id": "AKIAEXAMPLE", "force_path_style": true, "secret_access_key": fixtureLeak,
			},
			"secret_configured": true, "secret_access_key": fixtureLeak,
		})
	case path == "/admin/backups/image-storage/test" && r.Method == http.MethodPost:
		officialSuccess(w, map[string]any{"ok": true, "message": "ok", "detail": fixtureLeak})
	default:
		return false
	}
	return true
}
