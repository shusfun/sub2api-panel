package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (f *fixture) serveAdminExtended(w http.ResponseWriter, r *http.Request, path, role string) bool {
	if strings.HasPrefix(path, "/admin/") && role != "admin" {
		w.WriteHeader(403)
		return true
	}
	switch {
	case path == "/admin/dashboard/users-ranking":
		officialSuccess(w, map[string]any{
			"ranking": []any{
				map[string]any{"user_id": 2, "email": "user@example.test", "requests": 18, "tokens": 48000, "actual_cost": 1.42, "secret": fixtureLeak},
			},
		})
		return true
	case path == "/admin/ops/dashboard/snapshot-v2":
		tpPoints := make([]any, 0, 12)
		errPoints := make([]any, 0, 12)
		sampleReq := []int{42, 48, 55, 61, 58, 76, 94, 88, 79, 85, 91, 82}
		sampleErr := []int{0, 0, 1, 0, 0, 2, 3, 1, 0, 1, 0, 1}
		for i := range 12 {
			ts := fmt.Sprintf("2026-11-21T11:%02d:00Z", i*5)
			reqCount := sampleReq[i]
			errTotal := sampleErr[i]
			tpPoints = append(tpPoints, map[string]any{
				"bucket_start": ts, "request_count": reqCount, "token_consumed": reqCount * 430,
				"switch_count": errTotal, "qps": float64(reqCount) / 30.0, "tps": float64(reqCount * 14),
			})
			errPoints = append(errPoints, map[string]any{
				"bucket_start": ts, "error_count_total": errTotal, "business_limited_count": errTotal / 2,
				"error_count_sla": (errTotal + 1) / 2, "upstream_error_count_excl_429_529": errTotal / 3,
				"upstream_429_count": errTotal, "upstream_529_count": 0,
			})
		}
		officialSuccess(w, map[string]any{
			"generated_at": "2026-11-21T12:00:00Z",
			"overview": map[string]any{
				"start_time": "2026-11-21T11:00:00Z", "end_time": "2026-11-21T12:00:00Z",
				"platform": "", "group_id": nil, "health_score": 98.5,
				"success_count": 850, "error_count_total": 9, "business_limited_count": 3,
				"error_count_sla": 6, "request_count_total": 859, "request_count_sla": 856,
				"token_consumed": 368000, "sla": 99.53, "error_rate": 0.47, "upstream_error_rate": 0.24,
				"upstream_error_count_excl_429_529": 2, "upstream_429_count": 7, "upstream_529_count": 0,
				"qps": map[string]any{"current": 3.4, "peak": 9.2, "avg": 2.4},
				"tps": map[string]any{"current": 1420, "peak": 4800, "avg": 1080},
				"duration": map[string]any{"p50_ms": 540, "p90_ms": 1350, "p95_ms": 1950, "p99_ms": 3200, "avg_ms": 680, "max_ms": 4100},
				"ttft":     map[string]any{"p50_ms": 240, "p90_ms": 580, "p95_ms": 790, "p99_ms": 1150, "avg_ms": 310, "max_ms": 1520},
				"system_metrics": map[string]any{
					"cpu_usage_percent": 18.4, "memory_used_mb": 512, "memory_total_mb": 4096,
					"memory_usage_percent": 12.5, "db_ok": true, "redis_ok": true,
					"goroutine_count": 64, "concurrency_queue_depth": 0, "account_switch_count": 2,
				},
				"internal": fixtureLeak,
			},
			"throughput_trend": map[string]any{"points": tpPoints},
			"error_trend":      map[string]any{"points": errPoints},
		})
		return true
	case path == "/admin/ops/dashboard/latency-histogram":
		officialSuccess(w, map[string]any{
			"buckets": []any{
				map[string]any{"range": "<200ms", "count": 142},
				map[string]any{"range": "200-500ms", "count": 318},
				map[string]any{"range": "500ms-1.5s", "count": 264},
				map[string]any{"range": "1.5s-3s", "count": 98},
				map[string]any{"range": ">3s", "count": 28},
			},
		})
		return true
	case path == "/admin/ops/dashboard/error-distribution":
		officialSuccess(w, map[string]any{
			"items": []any{map[string]any{"status_code": 429, "total": 2, "sla": 1, "business_limited": 1}},
		})
		return true
	case path == "/admin/ops/concurrency":
		officialSuccess(w, map[string]any{
			"enabled": true,
			"platform": map[string]any{
				"openai": map[string]any{"platform": "openai", "current_in_use": 3, "max_capacity": 20, "load_percentage": 15, "waiting_in_queue": 0},
			},
			"group": map[string]any{
				"8": map[string]any{"group_id": 8, "group_name": "Example", "platform": "anthropic", "current_in_use": 2, "max_capacity": 10, "load_percentage": 20, "waiting_in_queue": 0},
			},
			"account": map[string]any{
				"5": map[string]any{"account_id": 5, "account_name": "Demo upstream", "platform": "openai", "group_id": 8, "group_name": "Example", "current_in_use": 2, "max_capacity": 10, "load_percentage": 20, "waiting_in_queue": 0},
			},
			"timestamp": "2026-11-21T12:00:00Z",
		})
		return true
	case path == "/admin/ops/account-availability":
		officialSuccess(w, map[string]any{
			"platform": map[string]any{"openai": map[string]any{"total_accounts": 4, "available_count": 3, "rate_limit_count": 0, "error_count": 1}},
			"group":    map[string]any{"8": map[string]any{"group_id": 8, "group_name": "Example", "platform": "anthropic", "total_accounts": 4, "available_count": 3, "rate_limit_count": 0, "error_count": 1}},
			"account": map[string]any{
				"5": map[string]any{"account_id": 5, "account_name": "Demo upstream", "platform": "openai", "group_id": 8, "group_name": "Example", "status": "active", "is_available": true, "is_rate_limited": false, "is_overloaded": false, "has_error": false, "error_message": fixtureLeak},
			},
		})
		return true
	case path == "/admin/ops/user-concurrency":
		officialSuccess(w, map[string]any{
			"user": map[string]any{"2": map[string]any{"user_id": 2, "user_email": "user@example.test", "username": "demo", "current_in_use": 1, "max_capacity": 5, "load_percentage": 20, "waiting_in_queue": 0}},
		})
		return true
	case path == "/admin/ops/realtime-traffic":
		officialSuccess(w, map[string]any{
			"summary": map[string]any{
				"qps": map[string]any{"current": 3.4, "peak": 9.2, "avg": 2.1},
				"tps": map[string]any{"current": 1420, "peak": 4800, "avg": 980},
			},
		})
		return true
	case path == "/admin/ops/requests":
		officialPage(w, map[string]any{
			"kind": "success", "created_at": "2026-11-21T11:58:00Z", "request_id": "req-demo-001",
			"platform": "openai", "model": "gpt-5.2", "duration_ms": 640, "first_token_ms": 210,
			"status_code": 200, "phase": "completed", "severity": "info",
			"user_id": 2, "api_key_id": 4, "account_id": 5, "group_id": 8, "stream": true,
			"message": fixtureLeak,
		})
		return true
	case path == "/admin/ops/request-errors" || path == "/admin/ops/upstream-errors" || path == "/admin/ops/request-errors/77/upstream-errors":
		officialPage(w, sampleOpsErrorWire())
		return true
	case path == "/admin/ops/request-errors/77" || path == "/admin/ops/upstream-errors/77":
		officialSuccess(w, sampleOpsErrorWire())
		return true
	case path == "/admin/ops/request-errors/77/resolve" || path == "/admin/ops/upstream-errors/77/resolve":
		officialSuccess(w, map[string]any{"secret": fixtureLeak})
		return true
	case path == "/admin/ops/alert-rules":
		if r.Method == "POST" {
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			in["id"] = 11
			officialSuccess(w, in)
			return true
		}
		officialSuccess(w, []any{
			map[string]any{"id": 1, "name": "上游 429 激增", "enabled": true, "metric_type": "upstream_error_rate", "operator": ">", "threshold": 5.0, "window_minutes": 5, "sustained_minutes": 1, "severity": "warning", "cooldown_minutes": 30, "notify_email": true},
		})
		return true
	case strings.HasPrefix(path, "/admin/ops/alert-rules/"):
		if r.Method == "DELETE" {
			officialSuccess(w, map[string]any{})
			return true
		}
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		in["id"] = 1
		officialSuccess(w, in)
		return true
	case path == "/admin/ops/alert-events":
		officialSuccess(w, []any{
			map[string]any{"id": 31, "rule_id": 1, "severity": "warning", "status": "firing", "title": "上游 429 比例达 6.2%", "description": "OpenAI 号池触发限流预警", "metric_value": 6.2, "threshold_value": 5.0, "fired_at": "2026-11-21T11:56:00Z", "resolved_at": nil, "email_sent": false, "created_at": "2026-11-21T11:56:00Z"},
		})
		return true
	case strings.HasPrefix(path, "/admin/ops/alert-events/") && strings.HasSuffix(path, "/status"):
		officialSuccess(w, map[string]any{})
		return true
	case path == "/admin/ops/alert-silences":
		officialSuccess(w, map[string]any{})
		return true
	case path == "/admin/ops/settings/metric-thresholds":
		if r.Method == "PUT" {
			officialSuccess(w, map[string]any{})
			return true
		}
		officialSuccess(w, map[string]any{"sla_percent_min": 99.0, "ttft_p99_ms_max": 5000, "request_error_rate_percent_max": 5.0, "upstream_error_rate_percent_max": 5.0})
		return true
	case path == "/admin/ops/email-notification/config":
		officialSuccess(w, map[string]any{
			"alert":  map[string]any{"enabled": true, "recipients": []string{"ops@example.test"}, "min_severity": "warning", "rate_limit_per_hour": 10, "batching_window_seconds": 60, "include_resolved_alerts": true},
			"report": map[string]any{"enabled": false, "recipients": []string{}, "daily_summary_enabled": false, "daily_summary_schedule": "09:00", "weekly_summary_enabled": false, "weekly_summary_schedule": "MON 09:00", "error_digest_enabled": false, "error_digest_schedule": "10:00", "error_digest_min_count": 10, "account_health_enabled": false, "account_health_schedule": "08:00", "account_health_error_rate_threshold": 5.0},
		})
		return true
	case path == "/admin/ops/runtime/logging" || path == "/admin/ops/runtime/logging/reset":
		officialSuccess(w, map[string]any{"level": "info", "persist_access_logs": true, "enable_sampling": true, "sampling_initial": 100, "sampling_thereafter": 100, "caller": false, "stacktrace_level": "error", "retention_days": 7, "request_retention_days": 7})
		return true
	case path == "/admin/ops/advanced-settings":
		officialSuccess(w, map[string]any{
			"data_retention":                  map[string]any{"cleanup_enabled": true, "cleanup_schedule": "03:00", "error_log_retention_days": 14, "minute_metrics_retention_days": 3, "hourly_metrics_retention_days": 30},
			"aggregation":                     map[string]any{"aggregation_enabled": true},
			"openai_account_quota_auto_pause": map[string]any{"default_threshold_5h": 0.9, "default_threshold_7d": 0.95},
			"ignore_count_tokens_errors":      true, "ignore_context_canceled": true,
			"ignore_no_available_accounts": false, "ignore_invalid_api_key_errors": true,
			"ignore_insufficient_balance_errors": true, "display_openai_token_stats": true,
			"display_alert_events": true, "auto_refresh_enabled": true, "auto_refresh_interval_seconds": 15,
		})
		return true
	case path == "/admin/groups/all":
		officialSuccess(w, []any{sampleGroupWire(8, "Example", "anthropic", "standard"), sampleGroupWire(12, "Pro-Composite", "composite", "subscription")})
		return true
	case path == "/admin/groups/capacity-summary":
		officialSuccess(w, []any{map[string]any{"group_id": 8, "concurrency_used": 2, "concurrency_max": 10, "rpm_used": 12, "rpm_max": 120}})
		return true
	case path == "/admin/groups/usage-summary":
		officialSuccess(w, []any{map[string]any{"group_id": 8, "today_cost": 1.25, "total_cost": 42.8}})
		return true
	case path == "/admin/groups" && r.URL.Query().Get("page_size") != "1":
		if r.Method == "POST" {
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			name, _ := in["name"].(string)
			platform, _ := in["platform"].(string)
			officialSuccess(w, sampleGroupWire(15, name, platform, "standard"))
			return true
		}
		officialSuccess(w, map[string]any{
			"items": []any{sampleGroupWire(8, "Example", "anthropic", "standard"), sampleGroupWire(12, "Pro-Composite", "composite", "subscription")},
			"total": 2, "page": 1, "page_size": 20, "pages": 1,
		})
		return true
	case path == "/admin/groups/8" || path == "/admin/groups/12" || path == "/admin/groups/15":
		if r.Method == "DELETE" {
			officialSuccess(w, map[string]any{})
			return true
		}
		id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/admin/groups/"), 10, 64)
		plat := "anthropic"
		if id == 12 {
			plat = "composite"
		}
		officialSuccess(w, sampleGroupWire(id, fmt.Sprintf("Group-%d", id), plat, "subscription"))
		return true
	case strings.HasSuffix(path, "/duplicate") && strings.HasPrefix(path, "/admin/groups/"):
		officialSuccess(w, sampleGroupWire(19, "Example-Copy", "anthropic", "standard"))
		return true
	case strings.HasSuffix(path, "/model-allowlist-candidates"):
		officialSuccess(w, map[string]any{"models": []string{"claude-sonnet-4-5", "gpt-5.2", "gemini-2.5-pro"}})
		return true
	case strings.HasSuffix(path, "/composite-routes/preview"):
		officialSuccess(w, map[string]any{"matched": true, "source": "route", "group_id": 12, "public_model": "claude-sonnet-4-5", "target_platform": "anthropic", "upstream_model": "claude-sonnet-4-5", "endpoint": "any"})
		return true
	case strings.Contains(path, "/composite-routes"):
		if r.Method == "GET" {
			officialSuccess(w, []any{map[string]any{"id": 1, "group_id": 12, "public_model": "claude-", "match_type": "prefix", "target_platform": "anthropic", "upstream_model": "", "endpoint": "any", "priority": 10, "enabled": true, "notes": ""}})
			return true
		}
		if r.Method == "DELETE" {
			officialSuccess(w, map[string]any{})
			return true
		}
		officialSuccess(w, map[string]any{"id": 2, "group_id": 12, "public_model": "gpt-", "match_type": "prefix", "target_platform": "openai", "upstream_model": "", "endpoint": "any", "priority": 20, "enabled": true, "notes": ""})
		return true
	case strings.HasSuffix(path, "/rate-multipliers"):
		if r.Method == "GET" {
			officialSuccess(w, []any{map[string]any{"user_id": 2, "user_name": "demo", "user_email": "user@example.test", "user_status": "active", "rate_multiplier": 0.85}})
			return true
		}
		officialSuccess(w, map[string]any{})
		return true
	case path == "/admin/channels":
		if r.Method == "POST" {
			officialSuccess(w, sampleChannelWire(3))
			return true
		}
		officialSuccess(w, map[string]any{"items": []any{sampleChannelWire(1)}, "total": 1})
		return true
	case path == "/admin/channels/model-pricing":
		officialSuccess(w, map[string]any{"found": true, "input_price": 0.000003, "output_price": 0.000015})
		return true
	case path == "/admin/channels/pricing/sync-models":
		officialSuccess(w, map[string]any{"models": []string{"gpt-5.2", "claude-sonnet-4-5"}})
		return true
	case strings.HasPrefix(path, "/admin/channels/"):
		if r.Method == "DELETE" {
			officialSuccess(w, map[string]any{})
			return true
		}
		officialSuccess(w, sampleChannelWire(1))
		return true
	case path == "/admin/openai/generate-auth-url" || path == "/admin/accounts/generate-auth-url" || path == "/admin/gemini/oauth/auth-url" || path == "/admin/antigravity/oauth/auth-url" || path == "/admin/grok/oauth/auth-url":
		officialSuccess(w, map[string]any{"auth_url": "https://auth.example.test/authorize", "session_id": "sess-123", "state": "state-456", "secret": fixtureLeak})
		return true
	case path == "/admin/openai/exchange-code" || path == "/admin/accounts/exchange-code" || path == "/admin/gemini/oauth/exchange-code" || path == "/admin/antigravity/oauth/exchange-code" || path == "/admin/grok/oauth/exchange-code":
		officialSuccess(w, map[string]any{"access_token": fixtureLeak + "-oauth-access", "refresh_token": fixtureLeak + "-oauth-refresh"})
		return true
	case path == "/admin/accounts" && r.Method == "POST":
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		name, _ := in["name"].(string)
		plat, _ := in["platform"].(string)
		if name == "" {
			name = "Created Account"
		}
		if plat == "" {
			plat = "openai"
		}
		officialSuccess(w, map[string]any{
			"id": 25, "name": name, "platform": plat, "type": in["type"], "status": "active", "schedulable": true,
			"credentials": map[string]any{"access_token": fixtureLeak, "api_key": fixtureLeak},
			"credentials_status": map[string]bool{"has_access_token": true},
			"group_ids": []int64{8}, "concurrency": 5, "priority": 10, "rate_multiplier": 1.0,
		})
		return true
	case path == "/admin/accounts/5/recover-state" || path == "/admin/accounts/5/clear-error" || path == "/admin/accounts/5/clear-rate-limit" || path == "/admin/accounts/5/reset-quota" || path == "/admin/accounts/5/refresh" || path == "/admin/accounts/5/apply-oauth-credentials":
		officialSuccess(w, map[string]any{
			"id": 5, "name": "Demo upstream", "platform": "openai", "type": "oauth", "status": "active", "schedulable": f.scheduled.Load(),
			"credentials": map[string]any{"access_token": fixtureLeak},
			"extra": map[string]any{"codex_5h_used_percent": 10, "private": fixtureLeak},
		})
		return true
	case path == "/admin/accounts/5/temp-unschedulable":
		officialSuccess(w, map[string]any{})
		return true
	case path == "/admin/accounts/5/stats":
		officialSuccess(w, map[string]any{"summary": map[string]any{"total_requests": 84, "total_tokens": 120000, "total_cost": 3.4, "secret": fixtureLeak}})
		return true
	case path == "/admin/accounts/today-stats/batch" && r.Method == "POST":
		officialSuccess(w, map[string]any{"stats": map[string]any{
			"5": map[string]any{"requests": 19, "tokens": 48000, "cost": 1.2, "user_cost": 1.45, "secret": fixtureLeak},
		}})
		return true
	case path == "/admin/accounts/usage/batch" && r.Method == "POST":
		officialSuccess(w, map[string]any{
			"usage": map[string]any{
				"5": map[string]any{"five_hour": map[string]any{"utilization": 32, "unused": fixtureLeak}, "seven_day": map[string]any{"utilization": 64}, "credentials": fixtureLeak},
			},
			"errors": map[string]any{"9": fixtureLeak},
		})
		return true
	case path == "/admin/cn-providers/accounts/5/balance" && r.Method == "GET":
		officialSuccess(w, map[string]any{
			"provider": "deepseek", "success": true, "balance": 88.5, "currency": "CNY", "available": true,
			"fetched_at": 1795400000, "error": fixtureLeak, "secret": fixtureLeak,
			"balances": []any{map[string]any{"currency": "CNY", "balance": 88.5, "raw": fixtureLeak}},
		})
		return true
	case path == "/admin/accounts/5/models":
		officialSuccess(w, []any{map[string]any{"id": "gpt-5.2", "display_name": "GPT 5.2"}})
		return true
	case path == "/admin/accounts/bulk-update" || path == "/admin/accounts/batch-clear-error" || path == "/admin/accounts/batch-refresh":
		officialSuccess(w, map[string]any{"success": 1, "failed": 0})
		return true
	case path == "/admin/accounts/batch-delete":
		officialSuccess(w, map[string]any{"deleted": 1})
		return true
	case path == "/admin/accounts/import/codex-session":
		officialSuccess(w, map[string]any{"created": 1, "updated": 0, "skipped": 0, "failed": 0})
		return true
	case path == "/admin/proxies" && r.URL.Query().Get("page_size") != "1":
		if r.Method == "POST" {
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			officialSuccess(w, sampleProxyWire(7, fmt.Sprint(in["name"])))
			return true
		}
		officialPage(w, sampleProxyWire(3, "Tokyo-Edge"))
		return true
	case path == "/admin/proxies/3":
		if r.Method == "DELETE" {
			officialSuccess(w, map[string]any{})
			return true
		}
		officialSuccess(w, sampleProxyWire(3, "Tokyo-Edge"))
		return true
	case path == "/admin/proxies/3/test":
		officialSuccess(w, map[string]any{"success": true, "message": "ok", "latency_ms": 42})
		return true
	case path == "/admin/proxies/3/quality-check":
		officialSuccess(w, map[string]any{"proxy_id": 3, "score": 95, "grade": "A", "summary": "原生住宅 IP", "passed_count": 5, "warn_count": 0, "failed_count": 0})
		return true
	case path == "/admin/proxies/3/accounts":
		officialSuccess(w, []any{map[string]any{"id": 5, "name": "Demo upstream", "platform": "openai", "type": "oauth"}})
		return true
	case path == "/admin/users/2/balance" || (path == "/admin/users/2" && r.Method == "PUT"):
		officialSuccess(w, map[string]any{"id": 2, "role": "user", "email": "user@example.test", "balance": 62.5, "status": "active", "concurrency": 5, "secret": fixtureLeak})
		return true
	case path == "/admin/users" && r.Method == "POST":
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		officialSuccess(w, map[string]any{"id": 9, "role": "user", "email": in["email"], "balance": 0, "status": "active", "concurrency": 5})
		return true
	case path == "/admin/users/2/balance-history":
		officialPage(w, map[string]any{"id": 1, "type": "admin_balance", "value": 50.0, "status": "used", "notes": "管理员调额", "used_at": "2026-11-21T10:00:00Z", "created_at": "2026-11-21T10:00:00Z", "code": fixtureLeak})
		return true
	case path == "/admin/users/batch-limits" || path == "/admin/users/batch-disable":
		officialSuccess(w, map[string]any{"affected": 1})
		return true
	case strings.HasPrefix(path, "/admin/api-keys/"):
		officialSuccess(w, map[string]any{"secret": fixtureLeak})
		return true
	case path == "/admin/subscriptions" && r.URL.Query().Get("page_size") != "1":
		officialPage(w, sampleAdminSubWire())
		return true
	case path == "/admin/subscriptions/assign":
		officialSuccess(w, sampleAdminSubWire())
		return true
	case path == "/admin/subscriptions/bulk-assign" || path == "/admin/subscriptions/bulk-action":
		officialSuccess(w, map[string]any{"success_count": 1, "failed_count": 0})
		return true
	case strings.HasPrefix(path, "/admin/subscriptions/11/"):
		if strings.HasSuffix(path, "/extend") {
			officialSuccess(w, sampleAdminSubWire())
			return true
		}
		officialSuccess(w, map[string]any{})
		return true
	case path == "/admin/redeem-codes":
		officialPage(w, map[string]any{
			"id": 51, "code": fixtureLeak + "-9988", "type": "balance", "value": 20.0, "status": "used",
			"used_by": 2, "used_at": "2026-11-21T09:00:00Z", "created_at": "2026-11-20T09:00:00Z",
			"user": map[string]any{"id": 2, "email": "user@example.test", "password_hash": fixtureLeak},
		})
		return true
	case path == "/admin/redeem-codes/create-and-redeem":
		officialSuccess(w, map[string]any{"secret": fixtureLeak})
		return true
	case path == "/admin/redeem-codes/51/expire":
		officialSuccess(w, map[string]any{"id": 51, "code": fixtureLeak + "-9988", "type": "balance", "value": 20.0, "status": "expired", "created_at": "2026-11-20T09:00:00Z"})
		return true
	case path == "/admin/redeem-codes/batch-delete":
		officialSuccess(w, map[string]any{"deleted": 1})
		return true
	case path == "/admin/redeem-codes/batch-update":
		officialSuccess(w, map[string]any{"updated": 1})
		return true
	}
	return false
}

func sampleOpsErrorWire() map[string]any {
	return map[string]any{
		"id": 77, "created_at": "2026-11-21T11:57:00Z", "phase": "upstream_http", "type": "rate_limit",
		"error_owner": "provider", "error_source": "upstream_http", "severity": "warning",
		"status_code": 429, "upstream_status_code": 429, "platform": "openai",
		"model": "gpt-5.2", "requested_model": "gpt-5.2", "upstream_model": "gpt-5.2",
		"resolved": false, "is_business_limited": false, "request_id": "req-err-077",
		"user_id": 2, "user_email": "user@example.test", "api_key_id": 4, "api_key_name": "Demo",
		"account_id": 5, "account_name": "Demo upstream", "group_id": 8, "group_name": "Example",
		"stream": true, "auth_latency_ms": 4, "routing_latency_ms": 7, "upstream_latency_ms": 390, "time_to_first_token_ms": 390,
		"error_body": fixtureLeak, "message": fixtureLeak, "upstream_errors": fixtureLeak, "api_key_prefix": fixtureLeak,
	}
}

func sampleGroupWire(id int64, name, platform, subType string) map[string]any {
	return map[string]any{
		"id": id, "name": name, "description": "主力低延迟分组", "platform": platform,
		"rate_multiplier": 1.0, "rpm_limit": 120, "is_exclusive": false, "status": "active",
		"subscription_type": subType, "account_count": 4, "active_account_count": 3,
		"rate_limited_account_count": 0, "sort_order": 1,
		"model_allowlist": map[string]any{"enabled": true, "models": []string{"claude-sonnet-4-5", "gpt-5.2"}},
		"model_pricing":   []any{},
		"created_at": "2026-11-01T00:00:00Z", "updated_at": "2026-11-21T00:00:00Z",
		"profit_min_margin": fixtureLeak,
	}
}

func sampleChannelWire(id int64) map[string]any {
	return map[string]any{
		"id": id, "name": "Enterprise-Pricing", "description": "自定义单价规则", "status": "active",
		"billing_model_source": "requested", "restrict_models": false, "group_ids": []int64{8},
		"model_pricing": []any{
			map[string]any{"id": 1, "platform": "openai", "models": []string{"gpt-5.2"}, "billing_mode": "token", "input_price": 0.000002, "output_price": 0.00001},
		},
		"model_mapping": map[string]any{}, "apply_pricing_to_account_stats": false,
		"created_at": "2026-11-01T00:00:00Z", "updated_at": "2026-11-21T00:00:00Z",
		"features": fixtureLeak,
	}
}

func sampleProxyWire(id int64, name string) map[string]any {
	return map[string]any{
		"id": id, "name": name, "protocol": "socks5", "host": "127.0.0.1", "port": 1080,
		"username": "proxy-user", "password": fixtureLeak, "status": "active", "account_count": 2,
		"latency_ms": 38.0, "latency_status": "success", "quality_status": "healthy", "quality_score": 96.0, "quality_grade": "A",
		"fallback_mode": "direct", "expiry_warn_days": 7,
		"created_at": "2026-11-01T00:00:00Z", "updated_at": "2026-11-21T00:00:00Z",
	}
}

func sampleAdminSubWire() map[string]any {
	return map[string]any{
		"id": 11, "user_id": 2, "group_id": 12, "status": "active",
		"expires_at": "2030-01-01T00:00:00Z", "daily_usage_usd": 1.2, "weekly_usage_usd": 6.4, "monthly_usage_usd": 18.5,
		"user":  map[string]any{"id": 2, "email": "user@example.test", "secret": fixtureLeak},
		"group": map[string]any{"id": 12, "name": "Pro-Composite", "platform": "composite", "extra": fixtureLeak},
	}
}
