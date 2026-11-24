package testkit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/internal/server/observe"
)

const maxPreviewUser = 11

func (f *fixture) servePreviewContent(w http.ResponseWriter, r *http.Request, path, role string, user int64) bool {
	const announcements = "/admin/announcements"
	const promos = "/admin/promo-codes"
	base, suffix := path, ""
	for _, candidate := range []string{announcements, promos, "/announcements"} {
		if rest, match := strings.CutPrefix(path, candidate+"/"); match {
			base, suffix = candidate, rest
			break
		}
	}
	if base != announcements && base != promos && base != "/announcements" {
		return false
	}
	if base != "/announcements" && role != "admin" {
		w.WriteHeader(http.StatusForbidden)
		return true
	}
	f.previewMu.Lock()
	defer f.previewMu.Unlock()
	if f.previewNoticeRead == nil {
		f.previewNoticeRead = map[int64]map[int64]time.Time{}
		for i, entry := range f.preview.Announcements {
			f.previewContentID++
			f.previewNotices = append(f.previewNotices, map[string]any{"id": f.previewContentID, "title": fmt.Sprintf("样本公告 %02d", i+1),
				"status": entry.Status, "content": "供固定预览使用的公告。", "notify_mode": "silent", "targeting": struct{}{}, "created_at": time.Now()})
		}
	}
	idText, section, _ := strings.Cut(suffix, "/")
	id, _ := strconv.ParseInt(idText, 10, 64)
	if base == "/announcements" {
		f.servePreviewUserNotices(w, r, id, section, user)
		return true
	}
	rows := &f.previewNotices
	if base == promos {
		rows = &f.previewPromos
	}
	if suffix == "" && r.Method == http.MethodGet {
		writePreviewPage(w, r, *rows)
		return true
	}
	index := slices.IndexFunc(*rows, func(row map[string]any) bool { return row["id"] == id })
	if r.Method == http.MethodPost && suffix == "" {
		input := map[string]any{}
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		f.previewContentID++
		input["id"], input["created_at"] = f.previewContentID, time.Now()
		if base == promos {
			input["status"], input["used_count"] = "active", 0
			if input["code"] == nil || input["code"] == "" {
				input["code"] = "PREVIEW-" + platform.RandomHex(4)
			}
		}
		previewContentDates(input)
		*rows = append([]map[string]any{input}, *rows...)
		officialSuccess(w, input)
		return true
	}
	if index < 0 {
		w.WriteHeader(http.StatusNotFound)
		return true
	}
	row := (*rows)[index]
	switch {
	case r.Method == http.MethodGet && section == "usages" && base == promos:
		writePreviewPage(w, r, nil)
	case r.Method == http.MethodGet && section == "read-status" && base == announcements:
		var statuses []map[string]any
		for userID := int64(1); userID <= 3; userID++ {
			result := map[string]any{"user_id": userID, "email": previewEmail(userID), "username": "", "eligible": row["status"] == "active"}
			if at, exists := f.previewNoticeRead[id][userID]; exists {
				result["read_at"] = at
			}
			statuses = append(statuses, result)
		}
		writePreviewPage(w, r, statuses)
	case r.Method == http.MethodGet && section == "":
		officialSuccess(w, row)
	case r.Method == http.MethodPut && section == "":
		input := map[string]any{}
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		previewContentDates(input)
		maps.Copy(row, input)
		officialSuccess(w, row)
	case r.Method == http.MethodDelete && section == "":
		*rows = slices.Delete(*rows, index, index+1)
		officialSuccess(w, struct{}{})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
	return true
}

func previewContentDates(row map[string]any) {
	for _, name := range []string{"starts_at", "ends_at", "expires_at"} {
		if seconds, ok := row[name].(float64); ok {
			row[name] = nil
			if seconds > 0 {
				row[name] = time.Unix(int64(seconds), 0)
			}
		}
	}
}

func (f *fixture) servePreviewUserNotices(w http.ResponseWriter, r *http.Request, id int64, section string, user int64) {
	now := time.Now()
	list := []map[string]any{}
	for _, row := range f.previewNotices {
		starts, _ := row["starts_at"].(time.Time)
		ends, _ := row["ends_at"].(time.Time)
		if row["status"] != "active" || now.Before(starts) || !ends.IsZero() && !now.Before(ends) {
			continue
		}
		rowID := row["id"].(int64)
		if r.Method == http.MethodPost && section == "read" && rowID == id {
			if f.previewNoticeRead[id] == nil {
				f.previewNoticeRead[id] = map[int64]time.Time{}
			}
			f.previewNoticeRead[id][user] = now
			officialSuccess(w, struct{}{})
			return
		}
		view := maps.Clone(row)
		if at, exists := f.previewNoticeRead[rowID][user]; exists {
			if r.URL.Query().Get("unread_only") == "true" {
				continue
			}
			view["read_at"] = at
		}
		list = append(list, view)
	}
	if r.Method == http.MethodGet && id == 0 && section == "" {
		officialSuccess(w, list)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func (f *fixture) previewPublicKeyUsage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	key, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || key != "preview-key-usage" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	platform.WriteJSON(w, http.StatusOK, map[string]any{
		"mode": "unrestricted", "planName": "钱包余额", "balance": 0, "remaining": 0,
		"usage": map[string]any{"today": map[string]any{"requests": 2, "total_tokens": 80, "actual_cost": 0.1},
			"total": map[string]any{"requests": 10, "total_tokens": 800, "actual_cost": 1}, "rpm": 1, "tpm": 40},
	})
}

func (f *fixture) servePreviewPayment(w http.ResponseWriter, r *http.Request, path string, user int64, bearer string) bool {
	if !strings.HasPrefix(path, "/payment/") {
		return false
	}
	f.previewMu.Lock()
	defer f.previewMu.Unlock()
	if path == "/payment/public/orders/resolve" && r.Method == http.MethodPost {
		var input struct {
			Token string `json:"resume_token"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil || input.Token != "preview-payment-return" {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		row, ok := f.previewPayment[2]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		officialSuccess(w, row)
		return true
	}
	if bearer == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return true
	}
	switch path {
	case "/payment/checkout-info":
		officialSuccess(w, mockPaymentCheckout{Plans: []mockCheckoutPlan{}, Methods: map[string]mockCheckoutMethod{
			"alipay": {Available: true, Name: "支付宝测试通道", Currency: "CNY", Min: 1, Max: 1000},
		}})
	case "/payment/orders":
		var input struct {
			Amount float64 `json:"amount"`
			Method string  `json:"payment_type"`
			Return string  `json:"return_url"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil || input.Return == "" || r.Referer() != strings.TrimSuffix(input.Return, "/payment/result")+"/purchase" {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		f.previewOrders++
		now := time.Now()
		row := mockPaymentOrder{ID: f.previewOrders, Amount: input.Amount, Paid: input.Amount, Currency: "CNY",
			Method: input.Method, Type: "balance", Status: "PENDING", CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute)}
		f.previewPayment[user] = row
		officialSuccess(w, map[string]any{"order_id": row.ID, "pay_amount": row.Paid, "currency": row.Currency,
			"qr_code": "https://example.test/preview-pay", "expires_at": row.ExpiresAt})
	case "/payment/orders/my":
		rows := []mockPaymentOrder{}
		if row, ok := f.previewPayment[user]; ok {
			rows = append(rows, row)
		}
		officialSuccess(w, map[string]any{"items": rows, "total": len(rows)})
	default:
		row, ok := f.previewPayment[user]
		detail, suffix, _ := strings.Cut(strings.TrimPrefix(path, "/payment/orders/"), "/")
		if !ok || detail != strconv.FormatInt(row.ID, 10) {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		if suffix == "cancel" && r.Method == http.MethodPost {
			row.Status = "CANCELLED"
			f.previewPayment[user] = row
		}
		officialSuccess(w, row)
	}
	return true
}

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

type previewUsageRecordWire struct {
	ID         int64               `json:"id"`
	Model      string              `json:"model"`
	Created    time.Time           `json:"created_at"`
	Input      int64               `json:"input_tokens"`
	Output     int64               `json:"output_tokens"`
	Cost       *float64            `json:"actual_cost"`
	CacheRead  *int64              `json:"cache_read_tokens"`
	CacheWrite *int64              `json:"cache_creation_tokens"`
	User       int64               `json:"user_id"`
	KeyID      int64               `json:"api_key_id"`
	Key        *platform.NamedItem `json:"api_key"`
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
	var input []previewUsageRecordWire
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF || len(input) == 0 {
		return fmt.Errorf("invalid preview usage sample")
	}
	zone, err := core.LoadUsageLocation("")
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
		tokens := observe.UsageTokens{Input: row.Input, Output: row.Output, CacheRead: *row.CacheRead, CacheWrite: *row.CacheWrite}
		if tokens.Input < 0 || tokens.Output < 0 || tokens.CacheRead < 0 || tokens.CacheWrite < 0 || owners[row.KeyID] != 0 && owners[row.KeyID] != row.User {
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

func (f *fixture) buildPreviewUsageSnapshot(ctx context.Context, token, query string) ([]byte, error) {
	deadline := time.Now().Add(time.Second)
	build := observe.BuildUsageStreamSnapshot(f.panel)
	for {
		payload, err := build(ctx, token, query)
		if err != nil {
			return nil, err
		}
		var summary struct {
			ModelSeries observe.UsageModelSeries `json:"model_series"`
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
