package testkit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"sub2api-panel/internal/server/observe"
	"sub2api-panel/internal/server/router"
	"sub2api-panel/internal/server/system"
)

func TestPreviewAllWorkspaceAndSettingRoutes(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	admin := browserLogin(t, f.app, "admin")

	for _, section := range system.SettingCatalog {
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

func TestPreviewUsageForAdditionalUsers(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	path := filepath.Join(t.TempDir(), "usage.json")
	records := make([]map[string]any, 0, maxPreviewUser)
	now := time.Now()
	for id := 1; id <= maxPreviewUser; id++ {
		records = append(records, map[string]any{
			"id": id, "user_id": id, "api_key_id": 1000 + id, "created_at": now, "model": "sample-model",
			"input_tokens": 10, "output_tokens": 1, "cache_read_tokens": 0, "cache_creation_tokens": 0, "actual_cost": 0.5,
		})
	}
	payload, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.loadPreviewUsage(path); err != nil {
		t.Fatal(err)
	}
	admin := browserLogin(t, f.app, "admin")
	sample := browserLogin(t, f.app, "sample10")
	all := admin.expect("GET", "/admin/users", "", 200)["items"].([]any)
	if len(all) != maxPreviewUser {
		t.Fatalf("expected %d preview users, got %d", maxPreviewUser, len(all))
	}
	summary := admin.expect("GET", "/admin/usage/summary?user_id=11&api_key_id=1011", "", 200)
	if summary["totals"].(map[string]any)["requests"] != float64(1) {
		t.Fatal(summary)
	}
	admin.expect("GET", "/admin/usage/summary?user_id=11&api_key_id=1002", "", 403)
	sample.expect("GET", "/admin/usage/summary?user_id=2", "", 403)
	rows := sample.expect("GET", "/usage?api_key_id=1011", "", 200)["items"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["id"] != float64(11) {
		t.Fatal(rows)
	}
}

func TestPreviewModelCatalog(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	wantModels := previewModelIDs("composite")
	if len(wantModels) != 18 {
		t.Fatalf("expected 18 models, got %d", len(wantModels))
	}
	var records []map[string]any
	now := time.Now()
	for i := range wantModels {
		for user := 1; user <= 3; user++ {
			records = append(records, map[string]any{
				"id": i*3 + user, "user_id": user, "api_key_id": 1000 + user, "created_at": now,
				"model": fmt.Sprintf("sample-model-%02d", i+1), "input_tokens": 10, "output_tokens": 1,
				"cache_read_tokens": 2, "cache_creation_tokens": 3, "actual_cost": 0.5,
			})
		}
	}
	dir := t.TempDir()
	usagePath, catalogPath := filepath.Join(dir, "usage.json"), filepath.Join(dir, "catalog.json")
	payload, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(usagePath, payload, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.loadPreviewUsage(usagePath); err != nil {
		t.Fatal(err)
	}
	if err := f.loadPreviewCatalog(catalogPath); err != nil {
		t.Fatal(err)
	}
	admin := browserLogin(t, f.app, "admin")
	user := browserLogin(t, f.app, "user")
	compositeID := previewGroupBase + len(f.preview.Groups) - 1
	query := url.Values{"period": {"7d"}, "user_id": {"all"}, "analysis": {"models"}}
	oldScope := observe.DashboardScope(f.panel, 1, query, true)
	f.panel.UsageSampleVersion = previewSampleVersion
	if observe.DashboardScope(f.panel, 1, query, true) == oldScope {
		t.Fatal("preview version reused the old dashboard scope")
	}
	for _, provider := range previewModelCatalog {
		model := provider.models[0].ID
		route := f.previewRoutes[model]
		group := fmt.Sprintf("/admin/groups/%d", route.group.ID)
		account := fmt.Sprintf("/admin/accounts/%d", route.account.ID)
		want := previewModelIDs(provider.platform)
		for _, path := range []string{group + "/model-candidates", "/admin/channels/sync-models?platform=" + provider.platform} {
			var result struct {
				Models []string `json:"models"`
			}
			response := admin.request("GET", path, "")
			checkResponse(t, response, 200)
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || !slices.Equal(result.Models, want) {
				t.Fatal(path, result, err)
			}
		}
		var candidates []previewModel
		response := admin.request("GET", account+"/models", "")
		checkResponse(t, response, 200)
		if err := json.Unmarshal(response.Body.Bytes(), &candidates); err != nil || len(candidates) != len(want) {
			t.Fatal(account, candidates, err)
		}
		for i, candidate := range candidates {
			if candidate.ID != want[i] || candidate.Name != provider.models[i].Name {
				t.Fatal(account, candidate)
			}
		}
		response = admin.request("POST", account+"/test", fmt.Sprintf(`{"model_id":%q}`, model))
		checkResponse(t, response, 200)
		if !strings.Contains(response.Body.String(), `"state":"passed"`) {
			t.Fatal(account, response.Body.String())
		}
		decision := admin.expect("POST", fmt.Sprintf("/admin/groups/%d/composite-routes/preview", compositeID), fmt.Sprintf(`{"model":%q}`, model), 200)
		if decision["matched"] != true || decision["target_platform"] != provider.platform || decision["upstream_model"] != model {
			t.Fatal(decision)
		}
		filters := fmt.Sprintf("user_id=all&group_id=%d&account_id=%d", route.group.ID, route.account.ID)
		summary := admin.expect("GET", "/admin/usage/summary?"+filters, "", 200)
		models := summary["models"].(map[string]any)["items"].([]any)
		if len(models) != len(provider.models) || summary["totals"].(map[string]any)["requests"] != float64(len(models)*3) {
			t.Fatal(provider.platform, summary)
		}
		details := admin.expect("GET", "/admin/usage?"+filters+"&model="+url.QueryEscape(model), "", 200)["items"].([]any)
		if len(details) != 3 {
			t.Fatal(model, details)
		}
		for _, detail := range details {
			row := detail.(map[string]any)
			if row["model"] != model || row["actual_cost"] != 0.5 || row["group_id"] != float64(route.group.ID) || row["account_id"] != float64(route.account.ID) {
				t.Fatal(row)
			}
		}
		own := user.expect("GET", "/usage?model="+url.QueryEscape(model), "", 200)["items"].([]any)
		if len(own) != 1 || own[0].(map[string]any)["id"] != details[1].(map[string]any)["id"] {
			t.Fatal(model, own)
		}
	}
	user.expect("GET", "/admin/usage/summary?user_id=all", "", 403)
	user.expect("GET", "/usage?api_key_id=1003", "", 403)
}

func TestPreviewUsesStaticFallbackForHistoryRoutes(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "development document")
	}))
	defer up.Close()
	target, err := url.Parse(up.URL)
	if err != nil {
		t.Fatal(err)
	}
	handler := previewViteForTarget(router.SPAFileServer(fstest.MapFS{"index.html": {Data: []byte("static document")}}), target)
	request := func(path string, status int, body string) {
		t.Helper()
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "http://"+browserFixtureAddr+path, nil))
		if out.Code != status || !strings.Contains(out.Body.String(), body) {
			t.Fatalf("%s returned %d %q", path, out.Code, out.Body.String())
		}
	}
	request("/admin/accounts/1?tab=config", http.StatusOK, "development document")
	request("/missing.js", http.StatusNotFound, "404")
	up.Close()
	request("/admin/accounts/1?tab=config", http.StatusOK, "static document")
}
