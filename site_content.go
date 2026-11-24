package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type announcementSummary struct {
	ID         int64      `json:"id"`
	Title      string     `json:"title"`
	Status     string     `json:"status"`
	NotifyMode string     `json:"notify_mode"`
	StartsAt   *time.Time `json:"starts_at,omitempty"`
	EndsAt     *time.Time `json:"ends_at,omitempty"`
}

type announcementCondition struct {
	Type     string  `json:"type"`
	Operator string  `json:"operator"`
	GroupIDs []int64 `json:"group_ids,omitempty"`
	Value    float64 `json:"value,omitempty"`
}

type announcementTargeting struct {
	AnyOf []struct {
		AllOf []announcementCondition `json:"all_of"`
	} `json:"any_of,omitempty"`
}

type announcementDetail struct {
	announcementSummary
	Content   string                `json:"content"`
	Targeting announcementTargeting `json:"targeting"`
}

type announcementInput struct {
	Title      string                `json:"title"`
	Content    string                `json:"content"`
	Status     string                `json:"status"`
	NotifyMode string                `json:"notify_mode"`
	Targeting  announcementTargeting `json:"targeting"`
	StartsAt   int64                 `json:"starts_at"`
	EndsAt     int64                 `json:"ends_at"`
}

type userAnnouncement struct {
	ID         int64      `json:"id"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	NotifyMode string     `json:"notify_mode"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type announcementReadStatus struct {
	UserID   int64      `json:"user_id"`
	Email    string     `json:"email"`
	Username string     `json:"username"`
	Eligible bool       `json:"eligible"`
	ReadAt   *time.Time `json:"read_at,omitempty"`
}

type promoSummary struct {
	ID          int64      `json:"id"`
	BonusAmount float64    `json:"bonus_amount"`
	MaxUses     int        `json:"max_uses"`
	UsedCount   int        `json:"used_count"`
	Status      string     `json:"status"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Notes       string     `json:"notes"`
}

type promoWire struct {
	promoSummary
	Code string `json:"code"`
}

type promoListRow struct {
	promoSummary
	Masked string `json:"masked"`
}

type promoInput struct {
	Code        *string  `json:"code,omitempty"`
	BonusAmount *float64 `json:"bonus_amount,omitempty"`
	MaxUses     *int     `json:"max_uses,omitempty"`
	Status      string   `json:"status,omitempty"`
	ExpiresAt   *int64   `json:"expires_at,omitempty"`
	Notes       *string  `json:"notes,omitempty"`
}

type promoUsageWire struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	UserEmail   string    `json:"user_email"`
	BonusAmount float64   `json:"bonus_amount"`
	UsedAt      time.Time `json:"used_at"`
	User        *struct {
		Email    string `json:"email"`
		Username string `json:"username"`
	} `json:"user"`
}

type promoUsage struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	UserEmail   string    `json:"user_email,omitempty"`
	BonusAmount float64   `json:"bonus_amount"`
	UsedAt      time.Time `json:"used_at"`
}

func (p *panel) siteContentRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api/affiliate", false, p.affiliateDetail)
	p.route(mux, "POST /api/affiliate/transfer", false, p.affiliateTransfer)
	p.route(mux, "GET /api/announcements", false, p.userAnnouncements)
	p.route(mux, "POST /api/announcements/{id}/read", false, p.markAnnouncementRead)
	p.route(mux, "GET /api/admin/announcements", true, p.adminAnnouncements)
	p.route(mux, "POST /api/admin/announcements", true, p.changeAnnouncement)
	p.route(mux, "GET /api/admin/announcements/{id}", true, p.changeAnnouncement)
	p.route(mux, "PUT /api/admin/announcements/{id}", true, p.changeAnnouncement)
	p.route(mux, "DELETE /api/admin/announcements/{id}", true, p.changeAnnouncement)
	p.route(mux, "GET /api/admin/announcements/{id}/read-status", true, p.announcementReadStatuses)
	p.route(mux, "GET /api/admin/promo-codes", true, p.adminPromoCodes)
	p.route(mux, "POST /api/admin/promo-codes", true, p.changePromoCode)
	p.route(mux, "PUT /api/admin/promo-codes/{id}", true, p.changePromoCode)
	p.route(mux, "DELETE /api/admin/promo-codes/{id}", true, p.changePromoCode)
	p.route(mux, "GET /api/admin/promo-codes/{id}/usages", true, p.promoUsageRows)
}

func (p *panel) userAnnouncements(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	path := "/announcements"
	if r.URL.Query().Get("unread_only") == "true" {
		path += "?unread_only=true"
	}
	rows, err := upstream[[]userAnnouncement](r.Context(), p.upstream, http.MethodGet, path, a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	rows, err = projectList(rows, func(row userAnnouncement) (userAnnouncement, error) { return row, requiredID(row.ID) })
	w.Header().Set("Cache-Control", "no-store")
	return send(w, http.StatusOK, rows, err)
}

func (p *panel) markAnnouncementRead(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if _, err = upstream[struct{}](r.Context(), p.upstream, http.MethodPost, fmt.Sprintf("/announcements/%d/read", id), a.tokens.Access, "", "", nil); err != nil {
		return err
	}
	return send(w, http.StatusOK, struct{}{}, nil)
}

func (p *panel) adminAnnouncements(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, query, err := listQuery(r, "status", "search")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/announcements", page, query,
		func(row announcementSummary) (announcementSummary, error) { return row, requiredID(row.ID) })
	return send(w, http.StatusOK, rows, err)
}

func (p *panel) changeAnnouncement(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	path := "/admin/announcements"
	if r.Method != http.MethodPost {
		id, err := pathID(r, "id")
		if err != nil {
			return err
		}
		path += fmt.Sprintf("/%d", id)
	}
	var payload any
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		input, err := decodeBody[announcementInput](r)
		if err != nil {
			return err
		}
		input.Title = strings.TrimSpace(input.Title)
		if input.Title == "" || strings.TrimSpace(input.Content) == "" || input.StartsAt < 0 || input.EndsAt < 0 ||
			(input.StartsAt > 0 && input.EndsAt > 0 && input.EndsAt <= input.StartsAt) {
			return fail(http.StatusBadRequest, "invalid_input")
		}
		payload = input
	}
	row, err := upstream[announcementDetail](r.Context(), p.upstream, r.Method, path, a.tokens.Access, "", "", payload)
	if err != nil {
		return err
	}
	if r.Method != http.MethodGet {
		return send(w, http.StatusOK, struct{}{}, nil)
	}
	return send(w, http.StatusOK, row, requiredID(row.ID))
}

func (p *panel) announcementReadStatuses(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	page, query, err := listQuery(r, "search")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, fmt.Sprintf("/admin/announcements/%d/read-status", id), page, query,
		func(row announcementReadStatus) (announcementReadStatus, error) { return row, requiredID(row.UserID) })
	return send(w, http.StatusOK, rows, err)
}

func (p *panel) adminPromoCodes(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, query, err := listQuery(r, "status", "search")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/promo-codes", page, query,
		func(row promoWire) (promoListRow, error) {
			return promoListRow{row.promoSummary, maskRedeemCode(row.Code)}, requiredID(row.ID)
		})
	return send(w, http.StatusOK, rows, err)
}

func (p *panel) changePromoCode(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	path := "/admin/promo-codes"
	if r.Method != http.MethodPost {
		id, err := pathID(r, "id")
		if err != nil {
			return err
		}
		path += fmt.Sprintf("/%d", id)
	}
	var payload any
	if r.Method != http.MethodDelete {
		input, err := decodeBody[promoInput](r)
		if err != nil {
			return err
		}
		if (r.Method == http.MethodPost && (input.BonusAmount == nil || *input.BonusAmount <= 0 || input.Status != "")) ||
			(input.BonusAmount != nil && *input.BonusAmount < 0) || (input.MaxUses != nil && *input.MaxUses < 0) ||
			(input.ExpiresAt != nil && *input.ExpiresAt < 0) || (input.Status != "" && input.Status != "active" && input.Status != "disabled") {
			return fail(http.StatusBadRequest, "invalid_input")
		}
		payload = input
	}
	row, err := upstream[promoWire](r.Context(), p.upstream, r.Method, path, a.tokens.Access, "", "", payload)
	if err != nil {
		return err
	}
	if r.Method == http.MethodPost {
		if row.Code == "" || row.ID < 1 {
			return fail(http.StatusBadGateway, "invalid_upstream")
		}
		w.Header().Set("Cache-Control", "no-store")
		return send(w, http.StatusOK, struct {
			Code string `json:"code"`
		}{row.Code}, nil)
	}
	return send(w, http.StatusOK, struct{}{}, nil)
}

func (p *panel) promoUsageRows(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	page, query, err := listQuery(r)
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, fmt.Sprintf("/admin/promo-codes/%d/usages", id), page, query,
		func(row promoUsageWire) (promoUsage, error) {
			email := strings.TrimSpace(row.UserEmail)
			if email == "" && row.User != nil {
				email = strings.TrimSpace(row.User.Email)
				if email == "" {
					email = strings.TrimSpace(row.User.Username)
				}
			}
			return promoUsage{
				ID:          row.ID,
				UserID:      row.UserID,
				UserEmail:   email,
				BonusAmount: row.BonusAmount,
				UsedAt:      row.UsedAt,
			}, requiredID(row.ID)
		})
	if err == nil && len(rows.Items) > 0 {
		var missing []int64
		for _, item := range rows.Items {
			if item.UserEmail == "" && item.UserID > 0 {
				missing = append(missing, item.UserID)
			}
		}
		if len(missing) > 0 {
			labels := p.resolveUserLabels(r.Context(), a.tokens.Access, missing)
			for i := range rows.Items {
				if rows.Items[i].UserEmail == "" {
					rows.Items[i].UserEmail = labels[rows.Items[i].UserID]
				}
			}
		}
	}
	return send(w, http.StatusOK, rows, err)
}
