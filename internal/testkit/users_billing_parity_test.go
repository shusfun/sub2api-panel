package testkit

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestUsersBillingParityEndpoints(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	admin := browserLogin(t, f.app, "admin")

	// 1. User Attribute Schema CRUD
	rec := admin.request("GET", "/admin/user-attributes/all", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"key":"company"`) {
		t.Fatalf("GET /admin/user-attributes/all: %d %s", rec.Code, rec.Body.String())
	}
	created := admin.expect("POST", "/admin/user-attributes", `{"key":"dept","label":"部门","type":"select","required":true,"options":[{"value":"eng","label":"eng"}]}`, 201)
	if created["key"] != "dept" {
		t.Fatalf("POST /admin/user-attributes: %#v", created)
	}
	updated := admin.expect("PUT", "/admin/user-attributes/1", `{"key":"company","label":"公司","type":"text","required":true,"enabled":false}`, 200)
	if updated["enabled"] != false {
		t.Fatalf("PUT /admin/user-attributes/1: %#v", updated)
	}
	admin.expect("PUT", "/admin/user-attributes/reorder", `{"ids":[2,1]}`, 200)
	admin.expect("DELETE", "/admin/user-attributes/1", "", 200)

	// 2. Batch enable users
	batch := admin.expect("POST", "/admin/users/batch-enable", `{"user_ids":[2]}`, 200)
	if batch["affected"] != float64(1) {
		t.Fatalf("POST /admin/users/batch-enable: %#v", batch)
	}

	// 3. Admin API key extended PUT & DELETE (secret must stay masked / never leaked)
	keyPut := admin.expect("PUT", "/admin/api-keys/1", `{"status":"disabled","quota":50,"reset_usage":true,"rate_multiplier":1.5,"allowed_groups":[1],"ip_whitelist":["1.2.3.4"],"ip_blacklist":["5.6.7.8"]}`, 200)
	encoded, _ := json.Marshal(keyPut)
	if strings.Contains(string(encoded), fixtureLeak) {
		t.Fatalf("PUT /admin/api-keys/1 leaked secret: %s", string(encoded))
	}
	admin.expect("DELETE", "/admin/api-keys/1", "", 200)

	// 4. Subscriptions search query
	admin.expect("GET", "/admin/subscriptions?search=user", "", 200)

	// 5. Redeem code single delete
	admin.expect("DELETE", "/admin/redeem-codes/51", "", 200)
}
