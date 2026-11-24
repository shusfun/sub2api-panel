package main

import (
	"net/http"
	"time"
)

type affiliateInvitee struct {
	UserID      int64      `json:"user_id"`
	Email       string     `json:"email"`
	Username    string     `json:"username"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	TotalRebate float64    `json:"total_rebate"`
}

type affiliateDetail struct {
	AffCode    string             `json:"aff_code"`
	AffCount   int                `json:"aff_count"`
	Available  float64            `json:"aff_quota"`
	Frozen     float64            `json:"aff_frozen_quota"`
	Total      float64            `json:"aff_history_quota"`
	RebateRate float64            `json:"effective_rebate_rate_percent"`
	Invitees   []affiliateInvitee `json:"invitees"`
}

func (p *panel) requireAffiliate(r *http.Request) error {
	settings, err := p.settings(r.Context())
	if err != nil {
		return err
	}
	if !settings.Affiliate {
		return fail(http.StatusForbidden, "not_allowed")
	}
	return nil
}

func (p *panel) affiliateDetail(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	if err := p.requireAffiliate(r); err != nil {
		return err
	}
	row, err := upstream[affiliateDetail](r.Context(), p.upstream, http.MethodGet, "/user/aff", a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if row.AffCode == "" || row.AffCount < 0 || row.RebateRate < 0 || row.RebateRate > 100 {
		return fail(http.StatusBadGateway, "invalid_upstream")
	}
	row.Invitees, err = projectList(row.Invitees, func(row affiliateInvitee) (affiliateInvitee, error) { return row, requiredID(row.UserID) })
	w.Header().Set("Cache-Control", "no-store")
	return send(w, http.StatusOK, row, err)
}

func (p *panel) affiliateTransfer(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	if err := p.requireAffiliate(r); err != nil {
		return err
	}
	row, err := upstream[struct {
		Transferred *float64 `json:"transferred_quota"`
		Balance     *float64 `json:"balance"`
	}](r.Context(), p.upstream, http.MethodPost, "/user/aff/transfer", a.tokens.Access, "", "", nil)
	if err == nil && (row.Transferred == nil || *row.Transferred <= 0 || row.Balance == nil) {
		return fail(http.StatusBadGateway, "invalid_upstream")
	}
	w.Header().Set("Cache-Control", "no-store")
	return send(w, http.StatusOK, row, err)
}
