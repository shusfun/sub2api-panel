package testkit

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func collectKeys(v any, out map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			out[k] = true
			collectKeys(child, out)
		}
	case []any:
		for _, child := range x {
			collectKeys(child, out)
		}
	}
}

// 表驱动：每个接口的响应既不能出现夹具混入的敏感字段，也必须保留界面用到的字段。
func TestWhitelistProjections(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	admin := browserLogin(t, f.app, "admin")

	cases := []struct {
		name, method, path, body string
		status                   int
		forbidden, required      []string
	}{
		{"passthrough list", "GET", "/admin/error-passthrough-rules", "", 200, []string{"created_by", "upstream_body_sample"}, []string{"error_codes", "match_mode"}},
		{"passthrough get", "GET", "/admin/error-passthrough-rules/3", "", 200, []string{"created_by", "upstream_body_sample"}, []string{"keywords"}},
		{"passthrough create", "POST", "/admin/error-passthrough-rules", `{"name":"r","enabled":true,"priority":1,"error_codes":[429],"keywords":[],"match_mode":"any","platforms":["openai"],"passthrough_code":true,"passthrough_body":false,"skip_monitoring":false}`, 201, []string{"created_by", "upstream_body_sample"}, []string{"id"}},
		{"tls list", "GET", "/admin/tls-fingerprint-profiles", "", 200, []string{"ja3_raw", "proxy_password"}, []string{"cipher_suites", "alpn_protocols"}},
		{"tls get", "GET", "/admin/tls-fingerprint-profiles/4", "", 200, []string{"ja3_raw", "proxy_password"}, []string{"extensions"}},
		{"tls create", "POST", "/admin/tls-fingerprint-profiles", `{"name":"p","enable_grease":true,"cipher_suites":[4865],"curves":[29]}`, 201, []string{"ja3_raw", "proxy_password"}, []string{"id"}},
		{"set privacy", "POST", "/admin/accounts/5/set-privacy", `{}`, 200, []string{"credentials", "extra", "refresh_token"}, []string{"privacy_mode"}},
		{"shadow", "POST", "/admin/accounts/5/shadow", `{"name":"shadow","priority":1,"concurrency":1,"group_ids":[1]}`, 201, []string{"credentials", "extra", "token"}, []string{"id", "platform"}},
		{"affiliate users", "GET", "/admin/affiliates/users", "", 200, []string{"password_hash", "balance"}, []string{"aff_code", "aff_count"}},
		{"affiliate overview", "GET", "/admin/affiliates/users/2/overview", "", 200, []string{"api_key"}, []string{"available_quota"}},
		{"risk status", "GET", "/admin/risk-control/status", "", 200, []string{"last_error", "api_key", "max_workers", "request_body"}, []string{"masked", "queue_usage_percent"}},
		{"risk key test", "POST", "/admin/risk-control/api-keys/test", `{}`, 200, []string{"last_error", "api_key", "input"}, []string{"items", "audit_result", "category_scores"}},
		{"risk hash delete", "DELETE", "/admin/risk-control/hashes", `{"input_hash":"abc"}`, 200, []string{"input"}, []string{"deleted"}},
		{"risk hash clear", "DELETE", "/admin/risk-control/hashes/all", `{}`, 200, []string{"input"}, []string{"deleted"}},
		{"prompt config", "GET", "/admin/prompt-audit/config", "", 200, []string{"token", "token_ciphertext"}, []string{"has_token", "token_status"}},
		{"prompt runtime", "GET", "/admin/prompt-audit/runtime", "", 200, []string{"redis_url", "last_error_detail"}, []string{"process_status", "queue"}},
		{"prompt events", "GET", "/admin/prompt-audit/events", "", 200, []string{"full_prompt", "redacted_preview", "scanner_evidence", "request_body", "evidence"}, []string{"prompt_hash", "prompt_length", "issue_summaries"}},
		{"prompt event", "GET", "/admin/prompt-audit/events/5", "", 200, []string{"full_prompt", "redacted_preview", "scanner_evidence", "request_body", "evidence"}, []string{"prompt_hash", "decision"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := admin.request(c.method, c.path, c.body)
			checkResponse(t, res, c.status)
			if strings.Contains(res.Body.String(), fixtureLeak) {
				t.Fatalf("fixture secret leaked: %s", res.Body.String())
			}
			var data any
			if err := json.Unmarshal(res.Body.Bytes(), &data); err != nil {
				t.Fatalf("invalid JSON: %s", res.Body.String())
			}
			keys := map[string]bool{}
			collectKeys(data, keys)
			for _, k := range c.forbidden {
				if keys[k] {
					t.Errorf("field %q leaked: %s", k, res.Body.String())
				}
			}
			for _, k := range c.required {
				if !keys[k] {
					t.Errorf("field %q missing: %s", k, res.Body.String())
				}
			}
		})
	}
}
