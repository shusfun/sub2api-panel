package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

const maxPreviewUser = 11

func previewEmail(id int64) string {
	switch id {
	case 1:
		return "admin@example.test"
	case 2:
		return "user@example.test"
	case 3:
		return "other@example.test"
	default:
		return fmt.Sprintf("sample%02d@example.test", id-1)
	}
}

func (f *fixture) previewUserID(email string) int64 {
	for id := int64(1); id <= maxPreviewUser; id++ {
		if len(f.previewKeys[id]) > 0 && email == previewEmail(id) {
			return id
		}
	}
	return 0
}

func (f *fixture) fixtureKeyUsage(id, user int64, admin bool) (today, total float64) {
	if f.previewKeys == nil {
		return 0.25, 2.5
	}
	now := time.Now().In(f.usageRows[0].created.Location())
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, row := range f.usageRows {
		if row.key == id && (admin || row.user == user) {
			total += row.cost
			if !row.created.Before(start) {
				today += row.cost
			}
		}
	}
	return today, total
}

func (f *fixture) keys(user int64) []map[string]any {
	if f.previewKeys != nil {
		if keys := f.previewKeys[user]; keys != nil {
			f.previewMu.Lock()
			defer f.previewMu.Unlock()
			rows := slices.Clone(keys)
			for index, key := range rows {
				if group, ok := f.previewBindings[key["id"].(int64)]; ok {
					rows[index] = maps.Clone(key)
					if group > 0 {
						rows[index]["group_id"] = group
					} else {
						delete(rows[index], "group_id")
					}
				}
			}
			return rows
		}
		return []map[string]any{}
	}
	return fixtureKeys(user)
}

func (f *fixture) loadPreviewUsage(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	const maxSize = 32 << 20
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > maxSize {
		return fmt.Errorf("preview usage sample exceeds %d bytes", maxSize)
	}

	decoder := json.NewDecoder(io.LimitReader(file, maxSize+1))
	decoder.DisallowUnknownFields()
	var input []usageRecordWire
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF || len(input) == 0 {
		return fmt.Errorf("invalid preview usage sample")
	}
	zone, err := loadUsageLocation("")
	if err != nil {
		return err
	}
	rows := make([]fixtureUsageRow, 0, len(input))
	owners := make(map[int64]int64)
	seen := make(map[int64]bool)
	models := make(map[string]string)
	for _, model := range previewModels("composite") {
		models[fmt.Sprintf("sample-model-%02d", model.sample)] = model.ID
	}
	for i, row := range input {
		if row.ID < 1 || seen[row.ID] || row.User < 1 || row.User > maxPreviewUser || row.KeyID < 1000 ||
			row.Created.IsZero() || strings.TrimSpace(row.Model) == "" || len(row.Model) > 128 ||
			row.CacheRead == nil || row.CacheWrite == nil || row.Cost == nil || *row.Cost < 0 || row.Key != nil {
			return fmt.Errorf("invalid preview usage record at row %d", i+1)
		}
		tokens := usageTokens{row.Input, row.Output, *row.CacheRead, *row.CacheWrite}
		if !tokens.valid() || owners[row.KeyID] != 0 && owners[row.KeyID] != row.User {
			return fmt.Errorf("invalid token count or key ownership at row %d", i+1)
		}
		seen[row.ID] = true
		owners[row.KeyID] = row.User
		if strings.HasPrefix(row.Model, "sample-model-") {
			model, ok := models[row.Model]
			if !ok {
				return fmt.Errorf("unmapped preview model at row %d", i+1)
			}
			row.Model = model
		}
		rows = append(rows, fixtureUsageRow{row.ID, row.User, row.KeyID, row.Model, row.Created.In(zone), tokens, *row.Cost})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].created.Equal(rows[j].created) {
			return rows[i].id > rows[j].id
		}
		return rows[i].created.After(rows[j].created)
	})
	keys := make(map[int64][]map[string]any)
	ids := make([]int64, 0, len(owners))
	for id := range owners {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		owner := owners[id]
		keys[owner] = append(keys[owner], map[string]any{
			"id": id, "user_id": owner, "name": fmt.Sprintf("样本 Key %d", len(keys[owner])+1),
			"status": "active", "key": fmt.Sprintf("sk-preview-only-%012d", id),
		})
	}
	f.usageRows, f.previewKeys = rows, keys
	return nil
}

// Local archived data can backfill faster than a live upstream. Use the normal Ent
// sampler and yield after a bounded burst so a wide range can still render promptly.
func (f *fixture) buildPreviewUsageSnapshot(ctx context.Context, token, query string) ([]byte, error) {
	deadline := time.Now().Add(time.Second)
	for {
		payload, err := f.panel.buildUsageStreamSnapshot(ctx, token, query)
		if err != nil {
			return nil, err
		}
		var summary struct {
			ModelSeries usageModelSeries `json:"model_series"`
		}
		if err := json.Unmarshal(payload, &summary); err != nil {
			return nil, err
		}
		series := summary.ModelSeries
		if ctx.Err() != nil || series.Unavailable || series.ReadyDays == series.TotalDays || time.Now().After(deadline) {
			return payload, nil
		}
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
	oldScope := f.panel.dashboardScope(1, query, true)
	f.panel.usageSampleVersion = previewSampleVersion
	if f.panel.dashboardScope(1, query, true) == oldScope {
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
