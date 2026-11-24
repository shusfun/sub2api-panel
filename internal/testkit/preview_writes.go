package testkit

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// previewWriteState keeps the in-memory edits made through the preview UI so
// that list, detail and follow-up reads reflect them until the process restarts.
type previewWriteState struct {
	patches  map[string]map[string]any
	attrs    []map[string]any
	attrNext int64
}

// previewPatched merges recorded PUT bodies into freshly generated catalog rows.
func (f *fixture) previewPatched(kind string, rows []map[string]any) []map[string]any {
	f.previewWriteMu.Lock()
	defer f.previewWriteMu.Unlock()
	for _, row := range rows {
		id, _ := row["id"].(int64)
		mergePreviewRow(row, f.previewWrites.patches[kind+"/"+strconv.FormatInt(id, 10)])
	}
	return rows
}

func mergePreviewRow(row, patch map[string]any) {
	for key, value := range patch {
		if key == "id" {
			continue
		}
		next, nested := value.(map[string]any)
		current, exists := row[key].(map[string]any)
		if !nested || !exists {
			row[key] = value
			continue
		}
		merged := make(map[string]any, len(current)+len(next))
		for k, v := range current {
			merged[k] = v
		}
		for k, v := range next {
			merged[k] = v
		}
		row[key] = merged
	}
}

func (f *fixture) servePreviewEditable(w http.ResponseWriter, r *http.Request, path string) bool {
	switch {
	case strings.HasPrefix(path, "/admin/user-attributes"):
		return f.servePreviewAttributes(w, r, path)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/admin/groups/"):
		return f.servePreviewGroupAggregates(w, r, path)
	case r.Method == http.MethodPut && (strings.HasPrefix(path, "/admin/groups/") || strings.HasPrefix(path, "/admin/accounts/")):
		return f.servePreviewCatalogPut(w, r, path)
	}
	return false
}

func (f *fixture) servePreviewCatalogPut(w http.ResponseWriter, r *http.Request, path string) bool {
	kind, idText, ok := strings.Cut(strings.TrimPrefix(path, "/admin/"), "/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if !ok || err != nil {
		return false
	}
	var rows []map[string]any
	if kind == "groups" {
		rows = f.previewGroups()
	} else {
		rows = f.previewAccounts()
	}
	index := slices.IndexFunc(rows, func(row map[string]any) bool { return row["id"] == id })
	if index < 0 {
		w.WriteHeader(http.StatusNotFound)
		return true
	}
	var body map[string]any
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		w.WriteHeader(http.StatusBadRequest)
		return true
	}
	f.previewWriteMu.Lock()
	if f.previewWrites.patches == nil {
		f.previewWrites.patches = map[string]map[string]any{}
	}
	key := kind + "/" + idText
	if f.previewWrites.patches[key] == nil {
		f.previewWrites.patches[key] = map[string]any{}
	}
	mergePreviewRow(f.previewWrites.patches[key], body)
	f.previewWriteMu.Unlock()
	mergePreviewRow(rows[index], body)
	officialSuccess(w, rows[index])
	return true
}

func (f *fixture) servePreviewGroupAggregates(w http.ResponseWriter, r *http.Request, path string) bool {
	idText, section, ok := strings.Cut(strings.TrimPrefix(path, "/admin/groups/"), "/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if !ok || err != nil || (section != "stats" && section != "api-keys") {
		return false
	}
	if !slices.ContainsFunc(f.previewGroups(), func(row map[string]any) bool { return row["id"] == id }) {
		w.WriteHeader(http.StatusNotFound)
		return true
	}
	if section == "stats" {
		officialSuccess(w, map[string]any{"total_api_keys": 2, "active_api_keys": 2, "total_requests": 1280, "total_cost": 42.8})
		return true
	}
	writePreviewPage(w, r, []map[string]any{
		{"id": 1001, "name": "样本 Key 1", "key": "sk-preview-group-0001-aa11", "status": "active", "user_id": 2, "quota": 0.0, "quota_used": 3.4, "user": map[string]any{"id": 2, "email": "user@example.test"}},
		{"id": 1002, "name": "样本 Key 2", "key": "sk-preview-group-0002-bb22", "status": "active", "user_id": 3, "quota": 50.0, "quota_used": 12.5, "user": map[string]any{"id": 3, "email": "other@example.test"}},
	})
	return true
}

func (f *fixture) previewAttributeDefs() []map[string]any {
	if f.previewWrites.attrs == nil {
		f.previewWrites.attrs = []map[string]any{
			{"id": int64(1), "name": "公司", "key": "company", "label": "公司", "type": "text", "description": "所属公司", "required": false, "enabled": true, "sort_order": 1, "options": []any{}},
		}
		f.previewWrites.attrNext = 2
	}
	return f.previewWrites.attrs
}

func (f *fixture) servePreviewAttributes(w http.ResponseWriter, r *http.Request, path string) bool {
	f.previewWriteMu.Lock()
	defer f.previewWriteMu.Unlock()
	defs := f.previewAttributeDefs()
	rest := strings.Trim(strings.TrimPrefix(path, "/admin/user-attributes"), "/")
	switch {
	case rest == "" && r.Method == http.MethodGet:
		out := make([]map[string]any, 0, len(defs))
		for _, def := range defs {
			if r.URL.Query().Get("enabled") == "true" && def["enabled"] != true {
				continue
			}
			out = append(out, def)
		}
		officialSuccess(w, out)
	case rest == "" && r.Method == http.MethodPost:
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		body["id"], body["sort_order"] = f.previewWrites.attrNext, len(defs)+1
		f.previewWrites.attrNext++
		f.previewWrites.attrs = append(defs, body)
		officialSuccess(w, body)
	case rest == "reorder" && r.Method == http.MethodPut:
		var body struct {
			IDs []int64 `json:"ids"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		next := make([]map[string]any, 0, len(defs))
		for position, id := range body.IDs {
			if index := slices.IndexFunc(defs, func(def map[string]any) bool { return def["id"] == id }); index >= 0 {
				defs[index]["sort_order"] = position + 1
				next = append(next, defs[index])
			}
		}
		for _, def := range defs {
			if !slices.Contains(body.IDs, def["id"].(int64)) {
				next = append(next, def)
			}
		}
		f.previewWrites.attrs = next
		officialSuccess(w, map[string]any{})
	default:
		id, err := strconv.ParseInt(rest, 10, 64)
		index := slices.IndexFunc(defs, func(def map[string]any) bool { return def["id"] == id })
		if err != nil || index < 0 {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		switch r.Method {
		case http.MethodDelete:
			f.previewWrites.attrs = slices.Delete(defs, index, index+1)
			officialSuccess(w, map[string]any{})
		case http.MethodPut:
			var body map[string]any
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				w.WriteHeader(http.StatusBadRequest)
				return true
			}
			mergePreviewRow(defs[index], body)
			officialSuccess(w, defs[index])
		default:
			return false
		}
	}
	return true
}
