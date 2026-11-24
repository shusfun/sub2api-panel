package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func (f *fixture) servePreviewExtra(w http.ResponseWriter, r *http.Request, path, role string, user int64) bool {
	if strings.HasPrefix(path, "/user/passkeys") || strings.HasPrefix(path, "/user/mcp") {
		return f.servePreviewUserSecurity(w, r, path, user)
	}
	if strings.HasPrefix(r.URL.Path, "/v1/images/batches") {
		return f.servePreviewBatchImages(w, r)
	}
	if strings.HasPrefix(path, "/admin/") {
		if role != "admin" {
			w.WriteHeader(http.StatusForbidden)
			return true
		}
		return f.servePreviewAdminExtra(w, r, path)
	}
	return false
}

func (f *fixture) servePreviewUserSecurity(w http.ResponseWriter, r *http.Request, path string, user int64) bool {
	f.previewMu.Lock()
	defer f.previewMu.Unlock()
	now := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	switch {
	case path == "/user/passkeys" && r.Method == http.MethodGet:
		officialSuccess(w, []any{
			map[string]any{"id": 1, "name": "MacBook Touch ID", "created_at": now.Format(time.RFC3339), "last_used_at": now.Add(2 * time.Hour).Format(time.RFC3339)},
		})
		return true
	case strings.HasPrefix(path, "/user/passkeys/") && r.Method == http.MethodDelete:
		officialSuccess(w, map[string]any{})
		return true
	case path == "/user/mcp/config" && r.Method == http.MethodGet:
		officialSuccess(w, map[string]any{"resource": "http://127.0.0.1:28741/mcp/sse"})
		return true
	case path == "/user/mcp/grants" && r.Method == http.MethodGet:
		if f.previewMCPGrants == nil {
			used := now.Add(30 * time.Minute)
			f.previewMCPGrants = []mcpGrant{{
				ID:       "mcp_grant_demo_01",
				Client:   "Claude Desktop",
				Scope:    "user",
				ReadOnly: true,
				Created:  now,
				Expires:  now.Add(30 * 24 * time.Hour),
				LastUsed: &used,
			}}
		}
		officialSuccess(w, f.previewMCPGrants)
		return true
	case strings.HasPrefix(path, "/user/mcp/grants/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(path, "/user/mcp/grants/")
		revoked := time.Now().UTC()
		for i := range f.previewMCPGrants {
			if f.previewMCPGrants[i].ID == id {
				f.previewMCPGrants[i].Revoked = &revoked
			}
		}
		officialSuccess(w, map[string]any{})
		return true
	case path == "/user/mcp/authorize" && r.Method == http.MethodGet:
		officialSuccess(w, map[string]any{
			"client_name":  "Claude Desktop",
			"redirect_uri": "http://127.0.0.1:54123/callback",
			"resource":     "http://127.0.0.1:28741/mcp/sse",
		})
		return true
	case path == "/user/mcp/authorize" && r.Method == http.MethodPost:
		officialSuccess(w, map[string]any{"redirect_url": "http://127.0.0.1:54123/callback?code=demo"})
		return true
	}
	_ = user
	return false
}

func (f *fixture) servePreviewBatchImages(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	switch {
	case path == "/v1/images/batches/models" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []any{map[string]any{"id": "gemini-2.5-flash-image"}, map[string]any{"id": "gemini-2.5-pro-image"}},
		})
		return true
	case path == "/v1/images/batches" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{}, "has_more": false})
		return true
	}
	return false
}

func (f *fixture) servePreviewAdminExtra(w http.ResponseWriter, r *http.Request, path string) bool {
	f.previewMu.Lock()
	defer f.previewMu.Unlock()

	// 1. 全部 9 个 settingSources 子接口
	for _, subPath := range settingSources {
		if path == subPath {
			if f.previewSubSources == nil {
				f.previewSubSources = map[string]map[string]any{
					"/admin/settings/registration-email-blacklist":            {"emails": []string{}},
					"/admin/settings/panel-rate-limit":                        {"enabled": true, "user_rpm": 120, "heavy_rpm": 30, "public_ip_rpm": 60, "exempt_admin": true},
					"/admin/settings/overload-cooldown":                       {"enabled": true, "cooldown_minutes": 5},
					"/admin/settings/rate-limit-429-cooldown":                 {"enabled": true, "cooldown_seconds": 45},
					"/admin/settings/openai-images-oauth-unavailable-cooldown": {"cooldown_minutes": 10},
					"/admin/settings/stream-timeout":                          {"enabled": true, "action": "temp_unsched", "temp_unsched_minutes": 5, "threshold_count": 3, "threshold_window_minutes": 10},
					"/admin/settings/rectifier":                               {"enabled": true, "thinking_signature_enabled": true, "thinking_budget_enabled": true, "apikey_signature_enabled": false, "apikey_signature_patterns": []string{}},
					"/admin/settings/beta-policy":                             {"rules": []any{}},
					"/admin/settings/web-search-emulation":                    {"enabled": false, "providers": []any{}},
				}
			}
			cur := f.previewSubSources[path]
			if cur == nil {
				cur = map[string]any{}
			}
			if r.Method == http.MethodPut {
				var incoming map[string]any
				if json.NewDecoder(r.Body).Decode(&incoming) != nil {
					w.WriteHeader(http.StatusBadRequest)
					return true
				}
				for k, v := range incoming {
					cur[k] = v
				}
				f.previewSubSources[path] = cur
			}
			officialSuccess(w, cur)
			return true
		}
	}

	now := time.Date(2026, 11, 23, 9, 30, 0, 0, time.UTC)
	actorID := int64(1)
	if f.previewAuditLogs == nil {
		f.previewAuditLogs = []auditEntry{
			{ID: 101, CreatedAt: now, ActorID: &actorID, Email: "admin@example.test", Role: "admin", Auth: "session", Action: "更新站点设置", Method: "PUT", Path: "/api/admin/settings/site", RequestID: "94ac478e5f6289772114db249efdef24", IP: "127.0.0.1", Status: 200, Latency: 14},
			{ID: 102, CreatedAt: now.Add(-18 * time.Minute), ActorID: &actorID, Email: "admin@example.test", Role: "admin", Auth: "session", Action: "同步上游模型", Method: "POST", Path: "/api/admin/accounts/5/models/sync", RequestID: "fb893e74ef4b28590fcb478765285803", IP: "127.0.0.1", Status: 200, Latency: 128},
		}
	}

	switch {
	// 2. 操作审计
	case path == "/admin/audit-logs" && r.Method == http.MethodGet:
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		method := r.URL.Query().Get("method")
		success := r.URL.Query().Get("success")
		filtered := make([]auditEntry, 0, len(f.previewAuditLogs))
		for _, item := range f.previewAuditLogs {
			if method != "" && item.Method != method {
				continue
			}
			if success == "true" && item.Status >= 400 || success == "false" && item.Status < 400 {
				continue
			}
			if q != "" && !strings.Contains(strings.ToLower(item.Path+item.Action+item.Email), q) {
				continue
			}
			filtered = append(filtered, item)
		}
		officialSuccess(w, map[string]any{"items": filtered, "total": len(filtered), "pages": 1})
		return true
	case path == "/admin/audit-logs/clear" && r.Method == http.MethodPost:
		var in struct {
			Code string `json:"totp_code"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Code != "123456" {
			w.WriteHeader(http.StatusForbidden)
			return true
		}
		deleted := len(f.previewAuditLogs)
		f.previewAuditLogs = []auditEntry{}
		officialSuccess(w, map[string]any{"deleted": deleted})
		return true
	case strings.HasPrefix(path, "/admin/audit-logs/") && r.Method == http.MethodGet:
		id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/admin/audit-logs/"), 10, 64)
		for _, item := range f.previewAuditLogs {
			if item.ID == id {
				officialSuccess(w, item)
				return true
			}
		}
		w.WriteHeader(http.StatusNotFound)
		return true

	// 3. 邮件模板与连通测试
	case path == "/admin/settings/test-smtp" || path == "/admin/settings/send-test-email":
		officialSuccess(w, map[string]any{})
		return true
	case path == "/admin/settings/email-templates" && r.Method == http.MethodGet:
		officialSuccess(w, map[string]any{
			"events": []any{
				map[string]any{"value": "verify_code", "label": "邮箱验证码", "category": "认证"},
				map[string]any{"value": "password_reset", "label": "重置密码链接", "category": "认证"},
				map[string]any{"value": "balance_low", "label": "余额不足提醒", "category": "计费"},
			},
			"locales": []string{"zh", "en"},
			"templates": []any{
				map[string]any{"event": "verify_code", "locale": "zh", "subject": "[{{site_name}}] 您的验证码"},
				map[string]any{"event": "password_reset", "locale": "zh", "subject": "[{{site_name}}] 重置登录密码"},
				map[string]any{"event": "balance_low", "locale": "zh", "subject": "[{{site_name}}] 账户余额不足提醒"},
			},
		})
		return true
	case path == "/admin/settings/email-template-preview" && r.Method == http.MethodPost:
		var in mailTemplateInput
		_ = json.NewDecoder(r.Body).Decode(&in)
		officialSuccess(w, in)
		return true
	case strings.HasPrefix(path, "/admin/settings/email-templates/"):
		if r.Method == http.MethodGet {
			officialSuccess(w, map[string]any{
				"subject":      "[{{site_name}}] 系统通知",
				"html":         "<div style=\"font-family:sans-serif;padding:16px\">您好，验证码为 <strong>{{code}}</strong>。</div>",
				"placeholders": []string{"site_name", "code", "email"},
			})
			return true
		}
		officialSuccess(w, map[string]any{})
		return true

	// 4. 旧版管理密钥与细粒度管理访问密钥
	case path == "/admin/settings/admin-api-key":
		if r.Method == http.MethodDelete {
			f.previewAdminKey = ""
			officialSuccess(w, map[string]any{})
			return true
		}
		officialSuccess(w, map[string]any{
			"exists":     f.previewAdminKey != "",
			"masked_key": f.previewAdminKey,
		})
		return true
	case path == "/admin/settings/admin-api-key/regenerate" && r.Method == http.MethodPost:
		f.previewAdminKey = "s2a_adm_****9f10"
		officialSuccess(w, map[string]any{"key": "s2a_adm_preview_plain_9f10"})
		return true
	case path == "/admin/settings/admin-api-keys":
		if r.Method == http.MethodPost {
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			id := fmt.Sprintf("ak_%d", len(f.previewAccessKeys)+1)
			entry := map[string]any{
				"id":          id,
				"name":        in["name"],
				"description": in["description"],
				"enabled":     in["enabled"],
				"expires_at":  in["expires_at"],
				"revoked_at":  nil,
				"allowed_ips": in["allowed_ips"],
				"key_prefix":  "s2a_scoped_****" + id,
				"policy":      in["policy"],
			}
			f.previewAccessKeys = append(f.previewAccessKeys, entry)
			officialSuccess(w, map[string]any{"key": "s2a_scoped_live_" + id})
			return true
		}
		if f.previewAccessKeys == nil {
			f.previewAccessKeys = []map[string]any{{
				"id":          "ak_issuer_01",
				"name":        "本机发码与巡检执行器",
				"description": "仅允许签发兑换码与读取只读分组",
				"enabled":     true,
				"expires_at":  nil,
				"revoked_at":  nil,
				"allowed_ips": []string{"127.0.0.1/32"},
				"key_prefix":  "s2a_scoped_****01",
				"policy": map[string]any{
					"full_access":        false,
					"global_permissions": []string{"redeem.generate"},
					"rules":              []any{},
				},
			}}
		}
		officialSuccess(w, map[string]any{
			"items": f.previewAccessKeys,
			"permissions": []any{
				map[string]any{"id": "redeem.generate", "label": "签发兑换码", "category": "发码与计费", "scope": "global"},
				map[string]any{"id": "settings.write", "label": "修改站点设置", "category": "系统治理", "scope": "global"},
				map[string]any{"id": "audit.clear", "label": "清空操作审计", "category": "系统治理", "scope": "global"},
				map[string]any{"id": "accounts.read", "label": "读取上游账号", "category": "号池管理", "scope": "scoped"},
				map[string]any{"id": "accounts.write", "label": "管理上游账号", "category": "号池管理", "scope": "scoped"},
				map[string]any{"id": "groups.read", "label": "读取调度分组", "category": "号池管理", "scope": "scoped"},
			},
		})
		return true
	case strings.HasPrefix(path, "/admin/settings/admin-api-keys/"):
		rest := strings.TrimPrefix(path, "/admin/settings/admin-api-keys/")
		id, action, _ := strings.Cut(rest, "/")
		for i, item := range f.previewAccessKeys {
			if item["id"] != id {
				continue
			}
			switch {
			case action == "rotate" && r.Method == http.MethodPost:
				officialSuccess(w, map[string]any{"key": "s2a_scoped_rotated_" + id})
			case action == "record" && r.Method == http.MethodDelete:
				f.previewAccessKeys = append(f.previewAccessKeys[:i], f.previewAccessKeys[i+1:]...)
				officialSuccess(w, map[string]any{})
			case action == "" && r.Method == http.MethodDelete:
				item["revoked_at"] = now.Format(time.RFC3339)
				officialSuccess(w, map[string]any{})
			case action == "" && r.Method == http.MethodPut:
				var in map[string]any
				_ = json.NewDecoder(r.Body).Decode(&in)
				for _, k := range []string{"name", "description", "enabled", "expires_at", "allowed_ips", "policy"} {
					item[k] = in[k]
				}
				officialSuccess(w, item)
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
			return true
		}
		w.WriteHeader(http.StatusNotFound)
		return true

	// 5. 系统版本检查与维护动作
	case path == "/admin/system/check-updates" && r.Method == http.MethodGet:
		officialSuccess(w, map[string]any{
			"current_version": "0.2.23",
			"latest_version":  "0.2.23",
			"has_update":      false,
			"build_type":      "docker",
		})
		return true
	case strings.HasPrefix(path, "/admin/system/") && r.Method == http.MethodPost:
		officialSuccess(w, map[string]any{"need_restart": false})
		return true

	// 6. 用户平台配额与自定义属性
	case path == "/admin/user-attributes":
		officialSuccess(w, []any{
			map[string]any{"id": 1, "name": "所属团队", "type": "text", "description": "内部成本归属部门", "required": false, "options": nil},
			map[string]any{"id": 2, "name": "结算层级", "type": "select", "description": "客户服务等级", "required": false, "options": []any{
				map[string]any{"value": "standard", "label": "标准"},
				map[string]any{"value": "enterprise", "label": "企业"},
			}},
		})
		return true
	case strings.HasPrefix(path, "/admin/users/") && strings.Contains(path, "/platform-quotas"):
		rest := strings.TrimPrefix(path, "/admin/users/")
		uidStr, tail, _ := strings.Cut(rest, "/")
		uid, _ := strconv.ParseInt(uidStr, 10, 64)
		if f.previewQuotas == nil {
			daily := 25.0
			f.previewQuotas = map[int64][]platformQuotaView{
				2: {{platformQuotaLimit: platformQuotaLimit{Platform: "openai", Daily: &daily}, DailyUsage: 3.4}},
			}
		}
		if r.Method == http.MethodPut {
			var in struct {
				Quotas []platformQuotaLimit `json:"quotas"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			next := make([]platformQuotaView, 0, len(in.Quotas))
			for _, q := range in.Quotas {
				next = append(next, platformQuotaView{platformQuotaLimit: q})
			}
			f.previewQuotas[uid] = next
		} else if tail == "platform-quotas/reset" && r.Method == http.MethodPost {
			for i := range f.previewQuotas[uid] {
				f.previewQuotas[uid][i].DailyUsage = 0
				f.previewQuotas[uid][i].WeeklyUsage = 0
				f.previewQuotas[uid][i].MonthlyUsage = 0
			}
		}
		officialSuccess(w, map[string]any{"platform_quotas": f.previewQuotas[uid]})
		return true
	case strings.HasPrefix(path, "/admin/users/") && strings.HasSuffix(path, "/attributes"):
		uidStr := strings.TrimSuffix(strings.TrimPrefix(path, "/admin/users/"), "/attributes")
		uid, _ := strconv.ParseInt(uidStr, 10, 64)
		if f.previewAttrs == nil {
			f.previewAttrs = map[int64][]userAttributeValue{2: {{ID: 1, Value: "核心研发组"}}}
		}
		if r.Method == http.MethodPut {
			var in struct {
				Values map[int64]string `json:"values"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			out := make([]userAttributeValue, 0, len(in.Values))
			for k, v := range in.Values {
				out = append(out, userAttributeValue{ID: k, Value: v})
			}
			f.previewAttrs[uid] = out
		}
		officialSuccess(w, f.previewAttrs[uid])
		return true

	// 7. 账号配置导出导入与 CRS 同步
	case path == "/admin/accounts/data":
		if r.Method == http.MethodGet {
			officialSuccess(w, map[string]any{
				"type":        "sub2api-data",
				"version":     1,
				"exported_at": now.Format(time.RFC3339),
				"accounts": []any{
					map[string]any{"name": "OpenAI 主力池 01", "platform": "openai", "type": "oauth", "concurrency": 10, "priority": 1},
				},
			})
			return true
		}
		officialSuccess(w, map[string]any{"account_created": 1, "account_failed": 0})
		return true
	case path == "/admin/accounts/sync/crs/preview" && r.Method == http.MethodPost:
		officialSuccess(w, map[string]any{
			"new_accounts":      []any{map[string]any{"crs_account_id": "crs_01", "name": "CRS-Claude-01", "platform": "anthropic", "type": "oauth"}},
			"existing_accounts": []any{},
		})
		return true
	case path == "/admin/accounts/sync/crs" && r.Method == http.MethodPost:
		officialSuccess(w, map[string]any{
			"created": 1, "updated": 0, "skipped": 0, "failed": 0,
			"items": []any{map[string]any{"crs_account_id": "crs_01", "name": "CRS-Claude-01", "action": "created"}},
		})
		return true
	}
	return false
}

func TestPreviewAllWorkspaceAndSettingRoutes(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	admin := browserLogin(t, f.app, "admin")

	for _, section := range settingCatalog {
		if section.ID == "access-keys" || section.ID == "connection" {
			continue
		}
		admin.expect("GET", "/admin/settings/"+section.ID, "", http.StatusOK)
	}

	endpoints := []string{
		"/admin/audit-logs",
		"/admin/audit-logs/101",
		"/admin/settings-tools/mail/templates",
		"/admin/settings-tools/mail/templates/verify_code/zh",
		"/admin/settings-tools/admin-key",
		"/admin/settings-tools/access-keys",
		"/admin/settings-tools/connection",
		"/admin/settings-tools/system",
		"/settings/profile",
		"/settings/totp",
		"/settings/passkeys",
		"/mcp/config",
		"/mcp/grants",
		"/admin/user-attributes",
		"/admin/users/2/attributes",
		"/admin/users/2/platform-quotas",
		"/affiliate",
		"/channels/available",
		"/model-plaza",
	}
	for _, ep := range endpoints {
		checkResponse(t, admin.request("GET", ep, ""), http.StatusOK)
	}
}
