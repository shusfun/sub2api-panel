package testkit

import (
	"net/http"
	"reflect"
	"testing"
)

func TestPrivateGroupAssignmentKeepsPublicRules(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	f.previewKeys = map[int64][]map[string]any{1: fixtureKeys(1), 2: fixtureKeys(2)}
	f.preview = &previewCatalog{Groups: []previewCatalogEntry{
		{Platform: "openai", Type: "standard", Status: "active"},
		{Platform: "openai", Type: "standard", Status: "active", Private: true},
		{Platform: "anthropic", Type: "standard", Status: "inactive", Private: true},
	}}
	f.previewUserGroups[2] = []int64{8, 10}
	admin := browserLogin(t, f.app, "admin")
	user := browserLogin(t, f.app, "user")
	const path = "/admin/users/2/private-groups"
	user.expect("PUT", path, `{"group_ids":[9]}`, 403)
	admin.expect("PUT", path, `{"group_ids":[8]}`, 400)
	admin.expect("PUT", path, `{"group_ids":[98]}`, 400)
	result := admin.expect("PUT", path, `{"group_ids":[9,9]}`, 200)
	if !reflect.DeepEqual(result["allowed_groups"], []any{float64(9), float64(8)}) {
		t.Fatal(result["allowed_groups"])
	}
	admin.expect("PUT", path, `{"group_ids":[10]}`, 400)
	result = admin.expect("PUT", path, `{"group_ids":[]}`, 200)
	if !reflect.DeepEqual(result["allowed_groups"], []any{float64(8)}) {
		t.Fatal(result["allowed_groups"])
	}
}

func TestAccountSecurityAndGrantChallenge(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	user := browserLogin(t, f.app, "user")
	admin := browserLogin(t, f.app, "admin")
	status := user.expect("GET", "/settings/totp", "", 200)
	if len(status) != 2 || status["enabled"] != false || status["feature_enabled"] != true {
		t.Fatal(status)
	}
	user.expect("GET", "/settings/totp/verification-method", "", 200)
	proof := `{"password":"` + fixturePassword + `"}`
	f.totpFeature.Store(false)
	if user.expect("GET", "/settings/totp", "", 200)["feature_enabled"] != false {
		t.Fatal("site TOTP availability was not projected")
	}
	user.expect("POST", "/settings/totp/setup", proof, 403)
	f.totpFeature.Store(true)
	setup := user.expect("POST", "/settings/totp/setup", proof, 200)
	if len(setup) != 3 || setup["setup_token"] != "local-preview-totp-setup" {
		t.Fatal("setup returned an unexpected field or token")
	}
	user.expect("POST", "/settings/totp/enable", `{"totp":"000000","setup_token":"local-preview-totp-setup"}`, 403)
	user.expect("POST", "/settings/totp/enable", `{"totp":"123456","setup_token":"local-preview-totp-setup"}`, 200)
	if user.expect("GET", "/settings/totp", "", 200)["enabled"] != true {
		t.Fatal("binding did not enable TOTP")
	}
	user.expect("POST", "/settings/totp/disable", proof, 200)
	user.expect("PUT", "/admin/grants/2", `{"invites":1,"budget_cents":0,"amounts_cents":[],"totp":"123456"}`, 403)
	admin.expect("PUT", "/admin/grants/2", `{"invites":1,"budget_cents":0,"amounts_cents":[],"totp":"000000"}`, 403)
}
