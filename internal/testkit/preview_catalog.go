package testkit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
)

type previewCatalogEntry struct {
	Platform string   `json:"platform,omitempty"`
	Type     string   `json:"type,omitempty"`
	Status   string   `json:"status"`
	Rate     *float64 `json:"rate,omitempty"`
	Priority *int64   `json:"priority,omitempty"`
	Value    *float64 `json:"value,omitempty"`
	Private  bool     `json:"is_exclusive,omitempty"`
}

const previewGroupBase = 8
const previewAccountBase = 5

func previewEntryIndex(id int64, base int64, list []previewCatalogEntry) (int, bool) {
	index := id - base
	return int(index), index >= 0 && index < int64(len(list))
}

type previewCatalog struct {
	Keys          []previewCatalogEntry `json:"keys"`
	Groups        []previewCatalogEntry `json:"groups"`
	Accounts      []previewCatalogEntry `json:"accounts"`
	Codes         []previewCatalogEntry `json:"codes"`
	Announcements []previewCatalogEntry `json:"announcements"`
}

type previewUsageRoute struct {
	group   platform.NamedItem
	account platform.NamedItem
}

func (c *previewCatalog) ensurePlatform(platformName string, account bool) {
	list, kind := &c.Groups, "standard"
	if account {
		list, kind = &c.Accounts, "apikey"
	}
	for _, entry := range *list {
		if entry.Platform == platformName && entry.Status == "active" {
			return
		}
	}
	*list = append(*list, previewCatalogEntry{Platform: platformName, Type: kind, Status: "active"})
}

func (f *fixture) previewUsageRoutes() map[string]previewUsageRoute {
	routes := make(map[string]previewUsageRoute)
	for _, provider := range previewModelCatalog {
		var route previewUsageRoute
		for i, entry := range f.preview.Groups {
			if entry.Platform == provider.platform && entry.Status == "active" {
				route.group = platform.NamedItem{ID: int64(previewGroupBase + i), Name: previewCatalogName(entry.Platform, "分组", i)}
				break
			}
		}
		for i, entry := range f.preview.Accounts {
			if entry.Platform == provider.platform && entry.Status == "active" {
				route.account = platform.NamedItem{ID: int64(previewAccountBase + i), Name: previewCatalogName(entry.Platform, "上游账号", i)}
				break
			}
		}
		for _, model := range provider.models {
			routes[model.ID] = route
		}
	}
	return routes
}

func (f *fixture) loadPreviewCatalog(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	const limit = 1 << 20
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return fmt.Errorf("invalid preview catalog size")
	}
	decoder := json.NewDecoder(io.LimitReader(file, limit+1))
	decoder.DisallowUnknownFields()
	var catalog previewCatalog
	if decoder.Decode(&catalog) != nil || decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("invalid preview catalog")
	}
	for _, list := range [][]previewCatalogEntry{catalog.Keys, catalog.Groups, catalog.Accounts, catalog.Codes, catalog.Announcements} {
		if len(list) > 1000 {
			return fmt.Errorf("preview catalog entry limit exceeded")
		}
		for _, entry := range list {
			if !previewValue(entry.Platform, "", "openai", "anthropic", "gemini", "grok", "deepseek", "antigravity", "openrouter", "composite") ||
				!previewValue(entry.Type, "", "standard", "subscription", "oauth", "apikey", "setup-token", "upstream", "balance", "concurrency", "invitation") ||
				!previewValue(entry.Status, "active", "inactive", "error", "suspended", "expired", "used", "unused", "draft") ||
				entry.Rate != nil && *entry.Rate < 0 || entry.Priority != nil && *entry.Priority < 0 ||
				entry.Value != nil && *entry.Value < 0 {
				return fmt.Errorf("invalid preview catalog entry")
			}
		}
	}
	for _, provider := range previewModelCatalog {
		catalog.ensurePlatform(provider.platform, false)
		catalog.ensurePlatform(provider.platform, true)
	}
	catalog.ensurePlatform("composite", false)
	f.preview = &catalog
	f.previewRoutes = f.previewUsageRoutes()
	for i, key := range f.previewKeys[1] {
		if i < len(catalog.Keys) {
			key["status"] = catalog.Keys[i].Status
		}
	}
	return nil
}

func previewValue(value string, allowed ...string) bool {
	for _, option := range allowed {
		if value == option {
			return true
		}
	}
	return false
}

func (f *fixture) servePreview(w http.ResponseWriter, r *http.Request, path, role string, user int64) bool {
	if f.preview != nil && role == "admin" {
		if f.servePreviewProxy(w, r, path) || f.servePreviewBackup(w, r, path) || f.servePreviewAdminPayment(w, r, path) ||
			f.servePreviewChannelRule(w, r, path) || f.servePreviewOfficialPlugin(w, r, path) || f.servePreviewRisk(w, r, path) {
			return true
		}
	}
	if f.preview != nil && f.servePreviewContent(w, r, path, role, user) {
		return true
	}
	if role == "admin" && f.previewKeys != nil && f.servePreviewUsers(w, r, path) {
		return true
	}
	if role == "admin" && f.preview != nil && f.servePreviewModels(w, r, path) {
		return true
	}
	if role == "admin" && f.preview != nil && f.servePreviewAccountTelemetry(w, r, path) {
		return true
	}
	if r.Method != http.MethodGet {
		if role == "admin" && path == "/admin/subscriptions/assign" {
			f.previewSubscribed.Store(true)
		}
		return false
	}
	if f.preview != nil {
		switch path {
		case "/groups/available":
			f.previewMu.Lock()
			allowed := slices.Clone(f.previewUserGroups[user])
			f.previewMu.Unlock()
			available := make([]map[string]any, 0, len(f.preview.Groups))
			for _, group := range f.previewGroups() {
				if group["status"] == "active" && (group["is_exclusive"] != true || slices.Contains(allowed, group["id"].(int64))) {
					available = append(available, group)
				}
			}
			officialSuccess(w, available)
			return true
		case "/subscriptions":
			officialSuccess(w, []any{})
			return true
		}
	}
	if role != "admin" {
		return false
	}
	if f.preview == nil {
		return false
	}
	switch path {
	case "/admin/dashboard/stats":
		officialSuccess(w, f.previewDashboard())
	case "/admin/dashboard/users-ranking":
		officialSuccess(w, map[string]any{"ranking": f.previewRanking(r)})
	case "/admin/groups/all":
		officialSuccess(w, f.previewGroups())
	case "/admin/groups":
		writePreviewPage(w, r, f.previewGroups())
	case "/admin/accounts":
		writePreviewPage(w, r, f.previewAccounts())
	case "/admin/redeem-codes":
		writePreviewPage(w, r, f.previewCodes())
	case "/admin/redeem-codes/stats":
		stats := map[string]int{"total_codes": len(f.preview.Codes)}
		for _, entry := range f.preview.Codes {
			switch entry.Status {
			case "unused", "active":
				stats["active_codes"]++
			case "used":
				stats["used_codes"]++
			case "expired":
				stats["expired_codes"]++
			}
		}
		officialSuccess(w, stats)
	case "/admin/subscriptions":
		if f.previewSubscribed.Load() {
			return false
		}
		writePreviewPage(w, r, []map[string]any{})
	default:
		return f.servePreviewCatalogDetail(w, path)
	}
	return true
}

func (f *fixture) servePreviewCatalogDetail(w http.ResponseWriter, path string) bool {
	parent, idText, ok := strings.Cut(strings.TrimPrefix(path, "/admin/"), "/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if !ok || err != nil || id < 1 {
		return false
	}
	var rows []map[string]any
	switch parent {
	case "groups":
		rows = f.previewGroups()
	case "accounts":
		rows = f.previewAccounts()
	default:
		return false
	}
	for _, row := range rows {
		if row["id"] == id {
			officialSuccess(w, row)
			return true
		}
	}
	w.WriteHeader(http.StatusNotFound)
	return true
}

func (f *fixture) servePreviewUsers(w http.ResponseWriter, r *http.Request, path string) bool {
	if f.preview != nil && f.servePreviewUserKeyOperation(w, r, path) {
		return true
	}
	if path == "/admin/users" && r.Method == http.MethodGet {
		rows := make([]map[string]any, 0, len(f.previewKeys))
		for id := int64(1); id <= maxPreviewUser; id++ {
			if len(f.previewKeys[id]) > 0 {
				rows = append(rows, f.previewUser(id))
			}
		}
		writePreviewPage(w, r, rows)
		return true
	}
	if !strings.HasPrefix(path, "/admin/users/") {
		return false
	}
	first, section, _ := strings.Cut(strings.TrimPrefix(path, "/admin/users/"), "/")
	id, err := strconv.ParseInt(first, 10, 64)
	if err != nil || section != "" && section != "api-keys" || r.Method != http.MethodGet && (r.Method != http.MethodPut || section != "") {
		return false
	}
	if len(f.previewKeys[id]) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return true
	}
	if section == "api-keys" {
		writePreviewPage(w, r, f.keys(id))
	} else {
		if r.Method == http.MethodPut {
			var change struct {
				Groups *[]int64 `json:"allowed_groups"`
			}
			if json.NewDecoder(r.Body).Decode(&change) != nil {
				w.WriteHeader(http.StatusBadRequest)
				return true
			}
			if change.Groups != nil {
				f.previewMu.Lock()
				f.previewUserGroups[id] = *change.Groups
				f.previewMu.Unlock()
			}
		}
		officialSuccess(w, f.previewUser(id))
	}
	return true
}

func (f *fixture) previewUser(id int64) map[string]any {
	role := "user"
	if id == 1 {
		role = "admin"
	}
	email := previewEmail(id)
	name, _, _ := strings.Cut(email, "@")
	f.previewMu.Lock()
	groups := f.previewUserGroups[id]
	f.previewMu.Unlock()
	return map[string]any{
		"id": id, "email": email, "username": name, "role": role, "status": "active", "balance": 12.5, "concurrency": 5, "allowed_groups": groups,
	}
}

func writePreviewPage(w http.ResponseWriter, r *http.Request, rows []map[string]any) {
	q := r.URL.Query()
	search := strings.ToLower(strings.TrimSpace(q.Get("search")))
	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		name := strings.ToLower(fmt.Sprint(row["name"], row["email"], row["title"], row["code"]))
		if !strings.Contains(name, search) ||
			q.Get("status") != "" && q.Get("status") != row["status"] ||
			q.Get("role") != "" && q.Get("role") != row["role"] ||
			q.Get("type") != "" && q.Get("type") != row["type"] ||
			q.Get("platform") != "" && q.Get("platform") != row["platform"] {
			continue
		}
		filtered = append(filtered, row)
	}
	page, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("page_size"))
	page, size = max(page, 1), min(max(size, 1), 1000)
	pages := (len(filtered) + size - 1) / size
	start := min(page-1, pages) * size
	start = min(start, len(filtered))
	end := min(start+size, len(filtered))
	officialSuccess(w, map[string]any{"items": filtered[start:end], "total": len(filtered), "pages": pages, "page": page, "page_size": size})
}

func (f *fixture) previewGroups() []map[string]any {
	rows := make([]map[string]any, 0, len(f.preview.Groups))
	for i, entry := range f.preview.Groups {
		id := int64(previewGroupBase + i)
		row := sampleGroupWire(id, previewCatalogName(entry.Platform, "分组", i), entry.Platform, entry.Type)
		row["status"], row["description"], row["sort_order"] = entry.Status, "", i+1
		row["is_exclusive"] = entry.Private
		models := previewModelIDs(entry.Platform)
		row["model_allowlist"] = map[string]any{"enabled": len(models) > 0, "models": models[:min(1, len(models))]}
		count, active := 0, 0
		for _, account := range f.preview.Accounts {
			if account.Platform == entry.Platform {
				count++
				if account.Status == "active" {
					active++
				}
			}
		}
		row["account_count"], row["active_account_count"] = count, active
		if entry.Rate != nil {
			row["rate_multiplier"] = *entry.Rate
		}
		rows = append(rows, row)
	}
	return rows
}

func previewAccountKind(entry previewCatalogEntry, index int) string {
	if entry.Type != "" {
		return entry.Type
	}
	if index%3 == 1 && (entry.Platform == "openai" || entry.Platform == "anthropic") {
		return "oauth"
	}
	return "apikey"
}

func (f *fixture) previewAccounts() []map[string]any {
	rows := make([]map[string]any, 0, len(f.preview.Accounts))
	for i, entry := range f.preview.Accounts {
		kind := previewAccountKind(entry, i)
		groupIDs := []int64{}
		for index, group := range f.preview.Groups {
			if group.Platform == entry.Platform {
				groupIDs = append(groupIDs, int64(previewGroupBase+index))
			}
		}
		row := map[string]any{
			"id": int64(previewAccountBase + i), "name": previewCatalogName(entry.Platform, "上游账号", i), "platform": entry.Platform, "type": kind,
			"status": entry.Status, "schedulable": entry.Status == "active" && (i > 0 || f.scheduled.Load()), "concurrency": 5, "rate_multiplier": 1.0,
			"group_ids": groupIDs, "created_at": "2026-11-01T00:00:00Z", "updated_at": "2026-11-21T00:00:00Z",
		}
		if entry.Priority != nil {
			row["priority"] = *entry.Priority
		}
		if entry.Rate != nil {
			row["rate_multiplier"] = *entry.Rate
		}
		if entry.Platform == "deepseek" && kind == "apikey" {
			row["extra"] = map[string]any{
				"deepseek_balance":           128.50,
				"deepseek_balance_currency":  "CNY",
				"deepseek_balance_available": true,
				"deepseek_balances": []map[string]any{
					{"currency": "CNY", "balance": 128.50},
					{"currency": "USD", "balance": 15.00},
				},
			}
		} else if kind == "oauth" || kind == "setup-token" {
			five := float64((i*17 + 23) % 85)
			week := float64((i*29 + 31) % 90)
			reset5h := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
			reset7d := time.Now().UTC().Add(72 * time.Hour).Format(time.RFC3339)
			row["extra"] = map[string]any{
				"codex_5h_used_percent": five,
				"codex_5h_reset_at":     reset5h,
				"codex_7d_used_percent": week,
				"codex_7d_reset_at":     reset7d,
			}
		}
		rows = append(rows, row)
	}
	return rows
}
