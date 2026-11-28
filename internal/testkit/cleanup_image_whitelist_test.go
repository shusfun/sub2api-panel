package testkit

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestUsageCleanupTasksWhitelist(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	admin := browserLogin(t, f.app, "admin")

	list := admin.request("GET", "/admin/usage/cleanup-tasks", "")
	checkResponse(t, list, 200)
	if strings.Contains(list.Body.String(), fixtureLeak) {
		t.Fatalf("cleanup list leaked fixture secret: %s", list.Body.String())
	}
	if strings.Contains(list.Body.String(), "created_by") || strings.Contains(list.Body.String(), "internal") {
		t.Fatalf("cleanup list leaked upstream fields: %s", list.Body.String())
	}

	created := admin.expect("POST", "/admin/usage/cleanup-tasks",
		`{"start_date":"2026-11-01","end_date":"2026-11-07","user_id":2,"timezone":"Asia/Shanghai","operation_id":"`+operation(1)+`"}`, 201)
	if created["id"] != float64(8) || created["created_by"] != nil || created["error_message"] != nil {
		t.Fatalf("unexpected create projection: %#v", created)
	}

	cancel := admin.expect("POST", "/admin/usage/cleanup-tasks/7/cancel", `{}`, 200)
	if len(cancel) != 2 || cancel["status"] != "canceled" {
		t.Fatalf("unexpected cancel projection: %#v", cancel)
	}

	for name, body := range map[string]string{
		"reversed range": `{"start_date":"2026-11-08","end_date":"2026-11-01","operation_id":"` + operation(2) + `"}`,
		"bad user id":    `{"start_date":"2026-11-01","end_date":"2026-11-02","user_id":0,"operation_id":"` + operation(3) + `"}`,
		"no operation":   `{"start_date":"2026-11-01","end_date":"2026-11-02"}`,
	} {
		if res := admin.request("POST", "/admin/usage/cleanup-tasks", body); res.Code != 400 {
			t.Fatalf("%s: status %d, want 400: %s", name, res.Code, res.Body.String())
		}
	}
}

func TestImageStorageWhitelist(t *testing.T) {
	f := setupFixture(t, http.NotFoundHandler())
	admin := browserLogin(t, f.app, "admin")

	state := admin.expect("GET", "/admin/backups/image-storage", "", 200)
	if encoded, _ := json.Marshal(state); strings.Contains(string(encoded), fixtureLeak) {
		t.Fatalf("image storage leaked fixture secret: %s", encoded)
	}
	if state["secret_configured"] != true || state["secret_access_key"] != nil {
		t.Fatalf("unexpected state projection: %#v", state)
	}
	config, _ := state["config"].(map[string]any)
	if config == nil || config["bucket"] != "images" || config["secret_access_key"] != nil {
		t.Fatalf("config projection leaked or lost fields: %#v", config)
	}

	tested := admin.expect("POST", "/admin/backups/image-storage/test", `{"enabled":true,"bucket":"images"}`, 200)
	if len(tested) != 2 || tested["ok"] != true {
		t.Fatalf("unexpected test projection: %#v", tested)
	}

	if res := admin.request("POST", "/admin/backups/image-storage/test", `{"enabled":true,"bucket":""}`); res.Code != 400 {
		t.Fatalf("enabled without bucket: status %d, want 400", res.Code)
	}
	if res := admin.request("PUT", "/admin/backups/image-storage", `{"enabled":true,"bucket":"","totp":"123456"}`); res.Code != 400 {
		t.Fatalf("PUT enabled without bucket: status %d, want 400", res.Code)
	}
}
