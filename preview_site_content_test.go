package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

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
				input["code"] = "PREVIEW-" + randomHex(4)
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
	writeJSON(w, http.StatusOK, map[string]any{
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
		officialSuccess(w, paymentCheckout{Plans: []checkoutPlan{}, Methods: map[string]checkoutMethod{
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
		row := paymentOrder{ID: f.previewOrders, Amount: input.Amount, Paid: input.Amount, Currency: "CNY",
			Method: input.Method, Type: "balance", Status: "PENDING", CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute)}
		f.previewPayment[user] = row
		officialSuccess(w, map[string]any{"order_id": row.ID, "pay_amount": row.Paid, "currency": row.Currency,
			"qr_code": "https://example.test/preview-pay", "expires_at": row.ExpiresAt})
	case "/payment/orders/my":
		rows := []paymentOrder{}
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
