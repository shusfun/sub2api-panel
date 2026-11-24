package system

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
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

type UserAnnouncement struct {
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

func MaskRedeemCode(raw string) string {
	if len(raw) > 8 {
		return "••••••••" + raw[len(raw)-4:]
	}
	return "••••••••"
}

func (h *handler) siteContentRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/announcements", false, h.userAnnouncements)
	h.s.Route(mux, "POST /api/announcements/{id}/read", false, h.markAnnouncementRead)
	h.s.Route(mux, "GET /api/admin/announcements", true, h.adminAnnouncements)
	h.s.Route(mux, "POST /api/admin/announcements", true, h.changeAnnouncement)
	h.s.Route(mux, "GET /api/admin/announcements/{id}", true, h.changeAnnouncement)
	h.s.Route(mux, "PUT /api/admin/announcements/{id}", true, h.changeAnnouncement)
	h.s.Route(mux, "DELETE /api/admin/announcements/{id}", true, h.changeAnnouncement)
	h.s.Route(mux, "GET /api/admin/announcements/{id}/read-status", true, h.announcementReadStatuses)
	h.s.Route(mux, "GET /api/admin/promo-codes", true, h.adminPromoCodes)
	h.s.Route(mux, "POST /api/admin/promo-codes", true, h.changePromoCode)
	h.s.Route(mux, "PUT /api/admin/promo-codes/{id}", true, h.changePromoCode)
	h.s.Route(mux, "DELETE /api/admin/promo-codes/{id}", true, h.changePromoCode)
	h.s.Route(mux, "GET /api/admin/promo-codes/{id}/usages", true, h.promoUsageRows)
}

func (h *handler) userAnnouncements(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	path := "/announcements"
	if r.URL.Query().Get("unread_only") == "true" {
		path += "?unread_only=true"
	}
	rows, err := platform.Upstream[[]UserAnnouncement](r.Context(), h.s.Upstream, http.MethodGet, path, a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	rows, err = platform.ProjectList(rows, func(row UserAnnouncement) (UserAnnouncement, error) { return row, platform.RequiredID(row.ID) })
	w.Header().Set("Cache-Control", "no-store")
	return platform.Send(w, http.StatusOK, rows, err)
}

func (h *handler) markAnnouncementRead(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	if _, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, http.MethodPost, fmt.Sprintf("/announcements/%d/read", id), a.Tokens.Access, "", "", nil); err != nil {
		return err
	}
	return platform.Send(w, http.StatusOK, struct{}{}, nil)
}

func (h *handler) adminAnnouncements(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, query, err := platform.ListQuery(r, "status", "search")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/admin/announcements", page, query,
		func(row announcementSummary) (announcementSummary, error) { return row, platform.RequiredID(row.ID) })
	return platform.Send(w, http.StatusOK, rows, err)
}

func (h *handler) changeAnnouncement(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	path := "/admin/announcements"
	if r.Method != http.MethodPost {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		path += fmt.Sprintf("/%d", id)
	}
	var payload any
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		input, err := platform.DecodeBody[announcementInput](r)
		if err != nil {
			return err
		}
		input.Title = strings.TrimSpace(input.Title)
		if input.Title == "" || strings.TrimSpace(input.Content) == "" || input.StartsAt < 0 || input.EndsAt < 0 ||
			(input.StartsAt > 0 && input.EndsAt > 0 && input.EndsAt <= input.StartsAt) {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
		payload = input
	}
	row, err := platform.Upstream[announcementDetail](r.Context(), h.s.Upstream, r.Method, path, a.Tokens.Access, "", "", payload)
	if err != nil {
		return err
	}
	if r.Method != http.MethodGet {
		return platform.Send(w, http.StatusOK, struct{}{}, nil)
	}
	return platform.Send(w, http.StatusOK, row, platform.RequiredID(row.ID))
}

func (h *handler) announcementReadStatuses(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	page, query, err := platform.ListQuery(r, "search")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, fmt.Sprintf("/admin/announcements/%d/read-status", id), page, query,
		func(row announcementReadStatus) (announcementReadStatus, error) { return row, platform.RequiredID(row.UserID) })
	return platform.Send(w, http.StatusOK, rows, err)
}

func (h *handler) adminPromoCodes(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, query, err := platform.ListQuery(r, "status", "search")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/admin/promo-codes", page, query,
		func(row promoWire) (promoListRow, error) {
			return promoListRow{row.promoSummary, MaskRedeemCode(row.Code)}, platform.RequiredID(row.ID)
		})
	return platform.Send(w, http.StatusOK, rows, err)
}

func (h *handler) changePromoCode(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	path := "/admin/promo-codes"
	if r.Method != http.MethodPost {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		path += fmt.Sprintf("/%d", id)
	}
	var payload any
	if r.Method != http.MethodDelete {
		input, err := platform.DecodeBody[promoInput](r)
		if err != nil {
			return err
		}
		if (r.Method == http.MethodPost && (input.BonusAmount == nil || *input.BonusAmount <= 0 || input.Status != "")) ||
			(input.BonusAmount != nil && *input.BonusAmount < 0) || (input.MaxUses != nil && *input.MaxUses < 0) ||
			(input.ExpiresAt != nil && *input.ExpiresAt < 0) || (input.Status != "" && input.Status != "active" && input.Status != "disabled") {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
		payload = input
	}
	row, err := platform.Upstream[promoWire](r.Context(), h.s.Upstream, r.Method, path, a.Tokens.Access, "", "", payload)
	if err != nil {
		return err
	}
	if r.Method == http.MethodPost {
		if row.Code == "" || row.ID < 1 {
			return platform.Fail(http.StatusBadGateway, "invalid_upstream")
		}
		w.Header().Set("Cache-Control", "no-store")
		return platform.Send(w, http.StatusOK, struct {
			Code string `json:"code"`
		}{row.Code}, nil)
	}
	return platform.Send(w, http.StatusOK, struct{}{}, nil)
}

func (h *handler) promoUsageRows(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	page, query, err := platform.ListQuery(r)
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, fmt.Sprintf("/admin/promo-codes/%d/usages", id), page, query,
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
			}, platform.RequiredID(row.ID)
		})
	if err == nil && len(rows.Items) > 0 {
		var missing []int64
		for _, item := range rows.Items {
			if item.UserEmail == "" && item.UserID > 0 {
				missing = append(missing, item.UserID)
			}
		}
		if len(missing) > 0 {
			labels := h.s.ResolveUserLabels(r.Context(), a.Tokens.Access, missing)
			for i := range rows.Items {
				if rows.Items[i].UserEmail == "" {
					rows.Items[i].UserEmail = labels[rows.Items[i].UserID]
				}
			}
		}
	}
	return platform.Send(w, http.StatusOK, rows, err)
}
