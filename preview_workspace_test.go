package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

func (f *fixture) servePreviewProxy(w http.ResponseWriter, r *http.Request, path string) bool {
	const base = "/admin/proxies"
	if path != base && !strings.HasPrefix(path, base+"/") {
		return false
	}
	f.previewMu.Lock()
	defer f.previewMu.Unlock()
	if f.previewProxies == nil {
		f.previewProxies = []map[string]any{sampleProxyWire(3, "Tokyo-Edge")}
	}
	if path == base {
		switch r.Method {
		case http.MethodGet:
			writePreviewPage(w, r, f.previewProxies)
			return true
		case http.MethodPost:
			var input map[string]any
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				w.WriteHeader(http.StatusBadRequest)
				return true
			}
			row := sampleProxyWire(int64(len(f.previewProxies))+7, fmt.Sprint(input["name"]))
			for _, key := range []string{"protocol", "host", "port", "username", "password", "fallback_mode", "backup_proxy_id"} {
				row[key] = input[key]
			}
			f.previewProxies = append(f.previewProxies, row)
			officialSuccess(w, row)
			return true
		}
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(path, base+"/"), 10, 64)
	if err != nil || r.Method != http.MethodGet {
		return false
	}
	for _, row := range f.previewProxies {
		if row["id"] == id {
			officialSuccess(w, row)
			return true
		}
	}
	return false
}

func (f *fixture) servePreviewBackup(w http.ResponseWriter, r *http.Request, path string) bool {
	if r.Method != http.MethodGet {
		return false
	}
	switch path {
	case backupPath:
		officialSuccess(w, map[string]any{"items": []backupRow{}})
	case backupPath + "/s3-config":
		officialSuccess(w, backupStorage{})
	case backupPath + "/schedule":
		officialSuccess(w, backupSchedule{RetainDays: 14, Retain: 10})
	default:
		return false
	}
	return true
}

func (f *fixture) servePreviewAdminPayment(w http.ResponseWriter, r *http.Request, path string) bool {
	if path != adminOrderPath && !strings.HasPrefix(path, adminOrderPath+"/") {
		return false
	}
	f.previewMu.Lock()
	defer f.previewMu.Unlock()
	if path == adminOrderPath && r.Method == http.MethodGet {
		q := r.URL.Query()
		rows := make([]adminPaymentOrder, 0, len(f.previewPayment))
		for user, order := range f.previewPayment {
			row := previewAdminOrder(user, order)
			if q.Get("status") != "" && q.Get("status") != row.Status ||
				q.Get("payment_type") != "" && q.Get("payment_type") != row.Method ||
				q.Get("keyword") != "" && !strings.Contains(fmt.Sprint(row.ID)+row.UserEmail, q.Get("keyword")) {
				continue
			}
			rows = append(rows, row)
		}
		slices.SortFunc(rows, func(a, b adminPaymentOrder) int { return b.CreatedAt.Compare(a.CreatedAt) })
		officialSuccess(w, map[string]any{"items": rows, "total": len(rows), "pages": 1})
		return true
	}
	id, action, _ := strings.Cut(strings.TrimPrefix(path, adminOrderPath+"/"), "/")
	for user, order := range f.previewPayment {
		if id != strconv.FormatInt(order.ID, 10) {
			continue
		}
		switch {
		case r.Method == http.MethodGet && action == "":
			officialSuccess(w, map[string]any{"order": previewAdminOrder(user, order)})
		case r.Method == http.MethodPost && action == "cancel" && order.Status == "PENDING":
			order.Status = "CANCELLED"
			f.previewPayment[user] = order
			officialSuccess(w, struct{}{})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
		return true
	}
	w.WriteHeader(http.StatusNotFound)
	return true
}

func previewAdminOrder(user int64, order paymentOrder) adminPaymentOrder {
	email := "admin@example.test"
	if user == 2 {
		email = "user@example.test"
	} else if user == 3 {
		email = "other@example.test"
	}
	return adminPaymentOrder{paymentOrder: order, UserID: user, UserEmail: email}
}

func (f *fixture) servePreviewChannelRule(w http.ResponseWriter, r *http.Request, path string) bool {
	if path != monitorRulesPath && !strings.HasPrefix(path, monitorRulesPath+"/") {
		return false
	}
	f.previewMu.Lock()
	defer f.previewMu.Unlock()
	if f.previewRules == nil {
		f.previewRules = make(map[int64]channelRule)
	}
	if path == monitorRulesPath && r.Method == http.MethodGet {
		rows := make([]channelRule, 0, len(f.previewRules))
		for _, row := range f.previewRules {
			rows = append(rows, row)
		}
		slices.SortFunc(rows, func(a, b channelRule) int { return int(a.ID - b.ID) })
		officialSuccess(w, map[string]any{"items": rows, "total": len(rows), "pages": 1})
		return true
	}
	var id int64
	if path == monitorRulesPath && r.Method == http.MethodPost {
		f.previewRuleID++
		id = f.previewRuleID
	} else {
		id, _ = strconv.ParseInt(strings.TrimPrefix(path, monitorRulesPath+"/"), 10, 64)
		if _, ok := f.previewRules[id]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
	}
	row := f.previewRules[id]
	switch r.Method {
	case http.MethodPost, http.MethodPut:
		var input channelRuleInput
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		row.ID, row.Name, row.Provider, row.GroupName = id, input.Name, input.Provider, input.Group
		row.Enabled, row.Interval, row.Jitter, row.Check = input.Enabled, input.Interval, input.Jitter, input.Check
		row.APIMode, row.Account, row.Models = input.APIMode, input.Account, input.Models
		if input.Endpoint != nil {
			row.Endpoint = *input.Endpoint
		}
		if input.Model != nil {
			row.PrimaryModel = *input.Model
		}
		if input.Key != "" {
			row.Key = "***"
		}
		f.previewRules[id] = row
	case http.MethodDelete:
		delete(f.previewRules, id)
	}
	officialSuccess(w, row)
	return true
}

func (f *fixture) servePreviewOfficialPlugin(w http.ResponseWriter, r *http.Request, path string) bool {
	const base = "/admin/plugins"
	if path != base && !strings.HasPrefix(path, base+"/") {
		return false
	}
	f.previewMu.Lock()
	defer f.previewMu.Unlock()
	if path == base && r.Method == http.MethodGet {
		rows := []officialPlugin{}
		if f.previewOfficial != nil {
			rows = append(rows, *f.previewOfficial)
		}
		officialSuccess(w, rows)
		return true
	}
	if path == base+"/upload" && r.Method == http.MethodPost {
		file, _, err := r.FormFile("plugin")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		_ = file.Close()
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		row := &officialPlugin{namedItem: namedItem{ID: 1, Name: "预览插件"}, Description: "本地预览夹具", Version: "1.0.0", State: "disabled"}
		row.Compatibility.Status = "compatible"
		f.previewOfficial = row
		f.previewPluginConf = map[string]json.RawMessage{}
		officialSuccess(w, row)
		return true
	}
	if f.previewOfficial == nil {
		w.WriteHeader(http.StatusNotFound)
		return true
	}
	row := f.previewOfficial
	switch path {
	case base + "/1/config":
		if r.Method == http.MethodPut && json.NewDecoder(r.Body).Decode(&f.previewPluginConf) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		writeJSON(w, http.StatusOK, f.previewPluginConf)
	case base + "/1/enable":
		row.State, row.Healthy = "enabled", true
		officialSuccess(w, row)
	case base + "/1/disable":
		row.State, row.Healthy = "disabled", false
		officialSuccess(w, row)
	case base + "/1/test":
		officialSuccess(w, map[string]any{"success": row.Healthy, "latency_ms": 20})
	case base + "/1":
		if r.Method == http.MethodDelete {
			f.previewOfficial = nil
		}
		officialSuccess(w, struct{}{})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
	return true
}

func (f *fixture) servePreviewRisk(w http.ResponseWriter, r *http.Request, path string) bool {
	if path == "/admin/risk-control/logs" {
		writePreviewPage(w, r, []map[string]any{})
		return true
	}
	if path != "/admin/risk-control/config" {
		return false
	}
	f.previewMu.Lock()
	defer f.previewMu.Unlock()
	if f.previewRisk == nil {
		f.previewRisk = &riskConfigView{Engine: "openai", Mode: "observe", Model: "omni-moderation-latest", SampleRate: 100,
			AllGroups: true, BanThreshold: 5, BaseURL: "https://example.test/v1", Timeout: 5000, BlockStatus: 403, HitRetention: 30, OtherRetention: 7}
	}
	if r.Method == http.MethodPut && json.NewDecoder(r.Body).Decode(f.previewRisk) != nil {
		w.WriteHeader(http.StatusBadRequest)
		return true
	}
	officialSuccess(w, f.previewRisk)
	return true
}

func (f *fixture) servePreviewUserKeyOperation(w http.ResponseWriter, r *http.Request, path string) bool {
	keyPath, keyOp := strings.CutPrefix(path, "/admin/api-keys/")
	userPath, userOp := strings.CutPrefix(path, "/admin/users/")
	if keyOp && r.Method == http.MethodPut {
		id, err := strconv.ParseInt(keyPath, 10, 64)
		var input struct {
			Group int64 `json:"group_id"`
		}
		if err != nil || json.NewDecoder(r.Body).Decode(&input) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		f.previewMu.Lock()
		defer f.previewMu.Unlock()
		for user, keys := range f.previewKeys {
			for _, key := range keys {
				if key["id"] != id {
					continue
				}
				index, found := previewEntryIndex(input.Group, previewGroupBase, f.preview.Groups)
				if input.Group != 0 && (!found || f.preview.Groups[index].Status != "active") {
					w.WriteHeader(http.StatusBadRequest)
					return true
				}
				auto := found && f.preview.Groups[index].Private && !slices.Contains(f.previewUserGroups[user], input.Group)
				if auto {
					f.previewUserGroups[user] = append(f.previewUserGroups[user], input.Group)
				}
				if f.previewBindings == nil {
					f.previewBindings = make(map[int64]int64)
				}
				f.previewBindings[id] = input.Group
				officialSuccess(w, map[string]bool{"auto_granted_group_access": auto})
				return true
			}
		}
		w.WriteHeader(http.StatusNotFound)
		return true
	}
	if !userOp || r.Method != http.MethodPost || !strings.HasSuffix(userPath, "/replace-group") {
		return false
	}
	user, err := strconv.ParseInt(strings.TrimSuffix(userPath, "/replace-group"), 10, 64)
	var input struct {
		Old int64 `json:"old_group_id"`
		New int64 `json:"new_group_id"`
	}
	if err != nil || json.NewDecoder(r.Body).Decode(&input) != nil {
		w.WriteHeader(http.StatusBadRequest)
		return true
	}
	f.previewMu.Lock()
	defer f.previewMu.Unlock()
	oldIndex := slices.Index(f.previewUserGroups[user], input.Old)
	newIndex, found := previewEntryIndex(input.New, previewGroupBase, f.preview.Groups)
	if oldIndex < 0 || !found || !f.preview.Groups[newIndex].Private || f.preview.Groups[newIndex].Status != "active" {
		w.WriteHeader(http.StatusBadRequest)
		return true
	}
	groups := f.previewUserGroups[user]
	f.previewUserGroups[user] = slices.Delete(groups, oldIndex, oldIndex+1)
	if !slices.Contains(f.previewUserGroups[user], input.New) {
		f.previewUserGroups[user] = append(f.previewUserGroups[user], input.New)
	}
	migrated := 0
	for _, key := range f.previewKeys[user] {
		id := key["id"].(int64)
		if f.previewBindings[id] == input.Old {
			f.previewBindings[id] = input.New
			migrated++
		}
	}
	officialSuccess(w, map[string]int{"migrated_keys": migrated})
	return true
}
