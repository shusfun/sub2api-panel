package users

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

const affiliatePath = "/admin/affiliates"

// 推广返利管理的响应白名单：只保留表格和概览实际展示的字段。
type affiliateUser struct {
	UserID        int64    `json:"user_id"`
	Email         string   `json:"email"`
	Username      string   `json:"username"`
	AffCode       string   `json:"aff_code"`
	AffCodeCustom bool     `json:"aff_code_custom"`
	RatePercent   *float64 `json:"aff_rebate_rate_percent,omitempty"`
	AffCount      int      `json:"aff_count"`
}

type affiliateInvite struct {
	InviterID       int64     `json:"inviter_id"`
	InviterEmail    string    `json:"inviter_email"`
	InviterUsername string    `json:"inviter_username"`
	InviteeID       int64     `json:"invitee_id"`
	InviteeEmail    string    `json:"invitee_email"`
	InviteeUsername string    `json:"invitee_username"`
	AffCode         string    `json:"aff_code"`
	TotalRebate     float64   `json:"total_rebate"`
	CreatedAt       time.Time `json:"created_at"`
}

type affiliateRebate struct {
	OrderID         *int64    `json:"order_id"`
	OutTradeNo      string    `json:"out_trade_no"`
	InviterID       int64     `json:"inviter_id"`
	InviterEmail    string    `json:"inviter_email"`
	InviterUsername string    `json:"inviter_username"`
	InviteeID       *int64    `json:"invitee_id"`
	InviteeEmail    string    `json:"invitee_email"`
	InviteeUsername string    `json:"invitee_username"`
	OrderAmount     *float64  `json:"order_amount"`
	PayAmount       *float64  `json:"pay_amount"`
	RebateAmount    float64   `json:"rebate_amount"`
	PaymentType     string    `json:"payment_type"`
	OrderStatus     string    `json:"order_status"`
	CreatedAt       time.Time `json:"created_at"`
}

type affiliateTransfer struct {
	LedgerID            int64     `json:"ledger_id"`
	Action              string    `json:"action"`
	UserID              int64     `json:"user_id"`
	UserEmail           string    `json:"user_email"`
	Username            string    `json:"username"`
	Amount              float64   `json:"amount"`
	AvailableQuotaAfter *float64  `json:"available_quota_after,omitempty"`
	SnapshotAvailable   bool      `json:"snapshot_available"`
	CreatedAt           time.Time `json:"created_at"`
}

type affiliateOverview struct {
	UserID              int64   `json:"user_id"`
	Email               string  `json:"email"`
	Username            string  `json:"username"`
	AffCode             string  `json:"aff_code"`
	RebateRatePercent   float64 `json:"rebate_rate_percent"`
	InvitedCount        int     `json:"invited_count"`
	RebatedInviteeCount int     `json:"rebated_invitee_count"`
	AvailableQuota      float64 `json:"available_quota"`
	HistoryQuota        float64 `json:"history_quota"`
}

type affiliateLookup struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

func (h *handler) registerAffiliateAdminRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/admin/affiliates/invites", true, affiliateRecords[affiliateInvite](h, "/invites", func(r affiliateInvite) error { return platform.RequiredID(r.InviterID) }))
	h.s.Route(mux, "GET /api/admin/affiliates/rebates", true, affiliateRecords[affiliateRebate](h, "/rebates", func(r affiliateRebate) error { return platform.RequiredID(r.InviterID) }))
	h.s.Route(mux, "GET /api/admin/affiliates/transfers", true, affiliateRecords[affiliateTransfer](h, "/transfers", func(r affiliateTransfer) error { return platform.RequiredID(r.LedgerID) }))
	h.s.Route(mux, "GET /api/admin/affiliates/users", true, affiliateRecords[affiliateUser](h, "/users", func(r affiliateUser) error { return platform.RequiredID(r.UserID) }))
	h.s.Route(mux, "GET /api/admin/affiliates/users/lookup", true, h.affiliateLookup)
	h.s.Route(mux, "POST /api/admin/affiliates/users/batch-rate", true, h.affiliateBatchRate)
	h.s.Route(mux, "GET /api/admin/affiliates/users/{user}/overview", true, h.affiliateOverview)
	h.s.Route(mux, "POST /api/admin/affiliates/users/{user}/withdraw", true, h.affiliateWithdraw)
	h.s.Route(mux, "PUT /api/admin/affiliates/users/{user}", true, h.affiliateUpdate)
	h.s.Route(mux, "DELETE /api/admin/affiliates/users/{user}", true, h.affiliateClear)
}

func affiliateRecords[T any](h *handler, suffix string, check func(T) error) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		page, params, err := platform.ListQuery(r, "search", "sort_by", "sort_order", "start_at", "end_at", "timezone")
		if err != nil {
			return err
		}
		rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, affiliatePath+suffix, page, params,
			func(row T) (T, error) { return row, check(row) })
		return platform.Send(w, http.StatusOK, rows, err)
	}
}

func (h *handler) affiliateLookup(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(keyword) > 100 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	if keyword == "" {
		return platform.Send(w, http.StatusOK, []affiliateLookup{}, nil)
	}
	rows, err := platform.Upstream[[]affiliateLookup](r.Context(), h.s.Upstream, http.MethodGet,
		affiliatePath+"/users/lookup?"+url.Values{"q": {keyword}}.Encode(), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	items, err := platform.ProjectList(rows, func(row affiliateLookup) (affiliateLookup, error) { return row, platform.RequiredID(row.ID) })
	return platform.Send(w, http.StatusOK, items, err)
}

func (h *handler) affiliateBatchRate(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		UserIDs []int64  `json:"user_ids"`
		Rate    *float64 `json:"aff_rebate_rate_percent"`
		Clear   bool     `json:"clear"`
	}](r)
	if err != nil || len(input.UserIDs) == 0 || len(input.UserIDs) > 200 || !input.Clear && (input.Rate == nil || !validRate(*input.Rate)) {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	for _, id := range input.UserIDs {
		if id < 1 {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
	}
	if input.Clear {
		input.Rate = nil
	}
	res, err := platform.Upstream[struct {
		Affected int `json:"affected"`
	}](r.Context(), h.s.Upstream, http.MethodPost, affiliatePath+"/users/batch-rate", a.Tokens.Access, "", "", input)
	return platform.Send(w, http.StatusOK, res, err)
}

func validRate(v float64) bool { return v >= 0 && v <= 100 }

func (h *handler) affiliateOverview(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	row, err := platform.Upstream[affiliateOverview](r.Context(), h.s.Upstream, http.MethodGet, fmt.Sprintf("%s/users/%d/overview", affiliatePath, id), a.Tokens.Access, "", "", nil)
	if err == nil {
		err = platform.RequiredID(row.UserID)
	}
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) affiliateWithdraw(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		Amount    float64 `json:"amount"`
		Operation string  `json:"operation_id"`
	}](r)
	if err != nil || input.Amount <= 0 || input.Amount > 1e9 || input.Operation == "" || len(input.Operation) > 100 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	res, err := platform.Upstream[struct {
		LedgerID            int64   `json:"ledger_id"`
		Amount              float64 `json:"amount"`
		AvailableQuotaAfter float64 `json:"available_quota_after"`
	}](r.Context(), h.s.Upstream, http.MethodPost, fmt.Sprintf("%s/users/%d/withdraw", affiliatePath, id), a.Tokens.Access, "", input.Operation,
		struct {
			Amount float64 `json:"amount"`
		}{input.Amount})
	return platform.Send(w, http.StatusOK, res, err)
}

func (h *handler) affiliateUpdate(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		AffCode   *string  `json:"aff_code"`
		Rate      *float64 `json:"aff_rebate_rate_percent"`
		ClearRate bool     `json:"clear_rebate_rate"`
	}](r)
	if err != nil || input.AffCode == nil && input.Rate == nil && !input.ClearRate ||
		input.AffCode != nil && (len(*input.AffCode) < 4 || len(*input.AffCode) > 32) ||
		input.Rate != nil && !validRate(*input.Rate) {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, http.MethodPut, fmt.Sprintf("%s/users/%d", affiliatePath, id), a.Tokens.Access, "", "", input)
	return platform.Send(w, http.StatusOK, struct{}{}, err)
}

func (h *handler) affiliateClear(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, http.MethodDelete, fmt.Sprintf("%s/users/%d", affiliatePath, id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, http.StatusOK, struct{}{}, err)
}
