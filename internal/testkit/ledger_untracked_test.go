package testkit

import (
	"net/http"
	"testing"
)

// 账本记录在首次采样时才建立；未接入的账号必须返回空视图（tracked=false），
// 否则前端拿不到 connection，无法显示接入引导。
func TestLedgerSeriesUntrackedAccount(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	admin := browserLogin(t, f.app, "admin")

	view := admin.expect("GET", "/admin/ledger/accounts/1", "", http.StatusOK)
	if view["tracked"] != false {
		t.Fatalf("tracked = %v, want false", view["tracked"])
	}
	if id, _ := view["account_id"].(float64); id != 1 {
		t.Fatalf("account_id = %v, want 1", view["account_id"])
	}
	if events, ok := view["events"].([]any); !ok || len(events) != 0 {
		t.Fatalf("events = %v, want empty list", view["events"])
	}
	if _, ok := view["points"].([]any); !ok {
		t.Fatalf("points = %v, want list", view["points"])
	}
	if _, ok := view["summary"].(map[string]any); !ok {
		t.Fatalf("summary = %v, want object", view["summary"])
	}
	conn, ok := view["connection"].(map[string]any)
	if !ok {
		t.Fatalf("connection = %v, want object", view["connection"])
	}
	if conn["api_key_configured"] != false || conn["access_token_configured"] != false {
		t.Fatalf("untracked account should report nothing configured: %v", conn)
	}
}
