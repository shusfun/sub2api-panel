package testkit

import (
	"net/http"
	"strings"
)

// 透传规则、TLS 档案、账号隐私/影子、推广、风控状态和提示词审计的上游夹具样本。
// 每个响应都混入敏感字段和 fixtureLeak，用来验证面板白名单。
func (f *fixture) serveWhitelistSamples(w http.ResponseWriter, r *http.Request, path, role string) bool {
	if role != "admin" {
		return false
	}
	page := func(items ...any) map[string]any {
		return map[string]any{"items": items, "total": len(items), "page": 1, "page_size": 20, "pages": 1}
	}
	rule := map[string]any{
		"id": 3, "name": "rate limit", "enabled": true, "priority": 1, "error_codes": []int{429}, "keywords": []string{"quota"},
		"match_mode": "any", "platforms": []string{"openai"}, "passthrough_code": true, "response_code": nil, "passthrough_body": false,
		"custom_message": nil, "skip_monitoring": false, "description": nil, "updated_at": "2026-11-21T10:00:00Z",
		"created_by": fixtureLeak, "upstream_body_sample": fixtureLeak,
	}
	tls := map[string]any{
		"id": 4, "name": "chrome", "description": nil, "enable_grease": true, "cipher_suites": []int{4865}, "curves": []int{29},
		"point_formats": []int{0}, "signature_algorithms": []int{1027}, "alpn_protocols": []string{"h2"}, "supported_versions": []int{772},
		"key_share_groups": []int{29}, "psk_modes": []int{1}, "extensions": []int{0}, "updated_at": "2026-11-21T10:00:00Z",
		"ja3_raw": fixtureLeak, "proxy_password": fixtureLeak,
	}
	keyStatus := map[string]any{
		"index": 0, "key_hash": "h0", "masked": "sk-***abcd", "status": "active", "failure_count": 0, "success_count": 5,
		"last_latency_ms": 120, "last_http_status": 200, "last_error": fixtureLeak, "api_key": fixtureLeak,
	}
	event := map[string]any{
		"id": 5, "decision": "block", "risk_level": "high", "action": "block", "categories": []string{"injection"},
		"matched_scanners": []string{"regex"}, "scanner_scores": map[string]any{"regex": 0.9}, "guard_endpoint_id": "e1",
		"chunk_total": 1, "latency_ms": 30, "created_at": "2026-11-21T10:00:00Z",
		"full_prompt": fixtureLeak, "redacted_preview": fixtureLeak, "scanner_evidence": fixtureLeak,
		"snapshot": map[string]any{
			"request_id": "r1", "user_id": 2, "user_email": "user@example.test", "api_key_id": 1, "api_key_name": "demo", "group_name": "g",
			"provider": "openai", "endpoint": "/v1/chat", "model": "gpt-5.2", "prompt_hash": "abc", "prompt_length": 42, "message_count": 2,
			"stage": "request", "request_body": fixtureLeak,
		},
		"issue_summaries": []any{map[string]any{"category": "injection", "scanner_id": "regex", "title": "t", "severity": "high", "severity_label": "高",
			"action": "block", "action_label": "拦截", "code": "c1", "score": 0.9, "evidence": fixtureLeak}},
	}
	get := r.Method == http.MethodGet
	switch {
	case path == "/admin/error-passthrough-rules" && get:
		officialSuccess(w, []any{rule})
	case path == "/admin/error-passthrough-rules" && r.Method == http.MethodPost,
		path == "/admin/error-passthrough-rules/3" && (get || r.Method == http.MethodPut):
		officialSuccess(w, rule)
	case path == "/admin/tls-fingerprint-profiles" && get:
		officialSuccess(w, []any{tls})
	case path == "/admin/tls-fingerprint-profiles" && r.Method == http.MethodPost,
		path == "/admin/tls-fingerprint-profiles/4" && (get || r.Method == http.MethodPut):
		officialSuccess(w, tls)
	case strings.HasPrefix(path, "/admin/error-passthrough-rules/") && r.Method == http.MethodDelete,
		strings.HasPrefix(path, "/admin/tls-fingerprint-profiles/") && r.Method == http.MethodDelete:
		officialSuccess(w, map[string]any{"deleted_by": fixtureLeak})
	case path == "/admin/accounts/5/set-privacy" && r.Method == http.MethodPost:
		officialSuccess(w, map[string]any{"id": 5, "credentials": fixtureLeak, "extra": map[string]any{"privacy_mode": "training_off", "refresh_token": fixtureLeak}})
	case path == "/admin/accounts/5/shadow" && r.Method == http.MethodPost:
		officialSuccess(w, map[string]any{"id": 9, "name": "shadow", "platform": "openai", "type": "oauth", "credentials": fixtureLeak, "extra": map[string]any{"token": fixtureLeak}})
	case path == "/admin/affiliates/users" && get:
		officialSuccess(w, page(map[string]any{"user_id": 2, "email": "user@example.test", "username": "user", "aff_code": "ABCD1234", "aff_code_custom": false,
			"aff_rebate_rate_percent": 10, "aff_count": 1, "password_hash": fixtureLeak, "balance": fixtureLeak}))
	case path == "/admin/affiliates/users/2/overview" && get:
		officialSuccess(w, map[string]any{"user_id": 2, "email": "user@example.test", "username": "user", "aff_code": "ABCD1234", "rebate_rate_percent": 10,
			"invited_count": 1, "rebated_invitee_count": 1, "available_quota": 3, "history_quota": 5, "api_key": fixtureLeak})
	case path == "/admin/risk-control/status" && get:
		officialSuccess(w, map[string]any{"engine": "openai", "enabled": true, "risk_control_enabled": true, "mode": "observe", "worker_count": 4,
			"active_workers": 1, "queue_size": 100, "queue_length": 3, "queue_usage_percent": 3, "dropped": 0, "processed": 50, "errors": 1,
			"pre_block_checked": 0, "pre_block_blocked": 0, "pre_block_errors": 0, "pre_block_avg_latency_ms": 0, "flagged_hash_count": 2,
			"api_key_statuses": []any{keyStatus}, "max_workers": fixtureLeak, "request_body": fixtureLeak})
	case path == "/admin/risk-control/api-keys/test" && r.Method == http.MethodPost:
		officialSuccess(w, map[string]any{"items": []any{keyStatus}, "audit_result": map[string]any{"flagged": true, "highest_category": "violence",
			"highest_score": 0.8, "composite_score": 0.7, "category_scores": map[string]any{"violence": 0.8}, "thresholds": map[string]any{"violence": 0.5}, "input": fixtureLeak}})
	case path == "/admin/risk-control/hashes" && r.Method == http.MethodDelete:
		officialSuccess(w, map[string]any{"deleted": true, "input": fixtureLeak})
	case path == "/admin/risk-control/hashes/all" && r.Method == http.MethodDelete:
		officialSuccess(w, map[string]any{"deleted": 3, "input": fixtureLeak})
	case path == "/admin/prompt-audit/config" && get:
		officialSuccess(w, map[string]any{"enabled": true, "blocking_enabled": false, "blocking_latest_turn_only": true, "store_pass_events": false,
			"effective_mode": "observe", "strategy": "all", "worker_count": 2, "queue_capacity": 100, "scanners": []string{"regex"}, "all_groups": true,
			"group_ids": []int{}, "config_version": 3, "updated_at": "2026-11-21T10:00:00Z", "change_summary": "",
			"endpoints": []any{map[string]any{"id": "e1", "name": "guard", "protocol": "openai", "base_url": "https://guard.example.test", "model": "m",
				"timeout_ms": 3000, "input_limit": 4000, "enabled": true, "has_token": true, "token_status": "ok", "token": fixtureLeak, "token_ciphertext": fixtureLeak}}})
	case path == "/admin/prompt-audit/runtime" && get:
		officialSuccess(w, map[string]any{"process_status": "running", "effective_mode": "observe", "expected_config_version": 3, "active_config_version": 3,
			"worker_total": 2, "worker_active": 0, "queue_capacity": 100, "queue": map[string]any{"pending": 0}, "processed_total": 10, "failed_total": 0,
			"enqueued_total": 10, "dropped_total": 0, "database_status": "ok", "redis_status": "ok", "endpoints": map[string]any{}, "guard_metrics": map[string]any{},
			"redis_url": fixtureLeak, "last_error_detail": fixtureLeak})
	case path == "/admin/prompt-audit/events" && get:
		officialSuccess(w, page(event))
	case path == "/admin/prompt-audit/events/5" && get:
		officialSuccess(w, event)
	default:
		return false
	}
	return true
}
