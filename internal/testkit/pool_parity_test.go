package testkit

import (
	"encoding/json"
	"testing"

	"sub2api-panel/internal/server/pool"
)

func TestPoolAccountProjectionAndRedaction(t *testing.T) {
	sched := true
	errMsg := "upstream 429 rate limit exceeded: secret-token-xyz"
	wcl := 12.5
	sticky := 2.0
	rpm := 60
	wire := pool.AccountDetailWire{
		ID:       42,
		Name:     "claude-prod-01",
		Platform: "anthropic",
		Type:     "bedrock",
		Credentials: map[string]json.RawMessage{
			"base_url":          json.RawMessage(`"https://bedrock.example.com"`),
			"region":            json.RawMessage(`"us-east-1"`),
			"access_key_id":     json.RawMessage(`"AKIAEXAMPLE"`),
			"secret_access_key": json.RawMessage(`"super-secret-do-not-leak"`),
			"custom_headers":    json.RawMessage(`{"X-Secret-Header":"top-secret-val"}`),
			"model_mapping":     json.RawMessage(`{"claude-3-5-sonnet":"anthropic.claude-3-5-sonnet"}`),
		},
		Schedulable:  &sched,
		Status:       "active",
		ErrorMessage: &errMsg,
		Extra: pool.AccountExtraWire{
			WindowCostLimit:         &wcl,
			WindowCostStickyReserve: &sticky,
			RPMLimit:                &rpm,
			TLSFingerprint:          true,
			InterceptWarmup:         true,
			ModelAllowlist:          []string{"claude-sonnet-4-5"},
		},
	}

	view, err := pool.ProjectAdminAccount(wire)
	if err != nil {
		t.Fatalf("ProjectAdminAccount failed: %v", err)
	}
	if view.Region != "us-east-1" || view.AccessKeyID != "AKIAEXAMPLE" {
		t.Fatalf("expected bedrock metadata projected, got region=%q key=%q", view.Region, view.AccessKeyID)
	}
	if !view.CustomHeaders["X-Secret-Header"] {
		t.Fatalf("expected custom_headers configured bool true, got %#v", view.CustomHeaders)
	}
	if view.LastErrorCode != "429" || view.LastErrorKind != "rate_limit" {
		t.Fatalf("expected redacted error 429/rate_limit, got %q/%q", view.LastErrorCode, view.LastErrorKind)
	}
	if view.Extra == nil || view.Extra.WindowCostLimit == nil || *view.Extra.WindowCostLimit != 12.5 || !view.Extra.TLSFingerprint {
		t.Fatalf("expected typed AccountExtraView projected, got %#v", view.Extra)
	}
	raw, _ := json.Marshal(view)
	if string(raw) == "" {
		t.Fatal("empty json")
	}
	for _, secret := range []string{"super-secret-do-not-leak", "top-secret-val", "secret-token-xyz"} {
		if containsString(string(raw), secret) {
			t.Fatalf("secret %q leaked in AccountAdminView JSON", secret)
		}
	}
}

func containsString(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && (func() bool {
		for i := 0; i <= len(haystack)-len(needle); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
