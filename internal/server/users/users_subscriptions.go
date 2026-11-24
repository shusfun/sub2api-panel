package users

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type adminSubscriptionWire struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"user_id"`
	GroupID         int64     `json:"group_id"`
	Status          string    `json:"status"`
	Expires         time.Time `json:"expires_at"`
	CreatedAt       string    `json:"created_at"`
	DailyUsageUSD   float64   `json:"daily_usage_usd"`
	WeeklyUsageUSD  float64   `json:"weekly_usage_usd"`
	MonthlyUsageUSD float64   `json:"monthly_usage_usd"`
	Notes           string    `json:"notes"`
	User            *struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
	Group *struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Platform string `json:"platform"`
	} `json:"group"`
}

type adminSubscriptionView struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"user_id"`
	UserEmail       string    `json:"user_email"`
	GroupID         int64     `json:"group_id"`
	Group           string    `json:"group"`
	Platform        string    `json:"platform,omitempty"`
	Status          string    `json:"status"`
	Expires         time.Time `json:"expires_at"`
	DailyUsageUSD   float64   `json:"daily_usage_usd"`
	WeeklyUsageUSD  float64   `json:"weekly_usage_usd"`
	MonthlyUsageUSD float64   `json:"monthly_usage_usd"`
	Notes           string    `json:"notes,omitempty"`
}

func projectAdminSubscription(s adminSubscriptionWire) (adminSubscriptionView, error) {
	if s.ID < 1 {
		return adminSubscriptionView{}, platform.Fail(502, "invalid_upstream")
	}
	groupName := fmt.Sprintf("分组 %d", s.GroupID)
	plat := ""
	if s.Group != nil {
		if s.Group.ID > 0 && s.GroupID == 0 {
			s.GroupID = s.Group.ID
		}
		if s.Group.Name != "" {
			groupName = s.Group.Name
		}
		plat = s.Group.Platform
	}
	email := ""
	if s.User != nil {
		if s.User.ID > 0 && s.UserID == 0 {
			s.UserID = s.User.ID
		}
		email = s.User.Email
	}
	return adminSubscriptionView{
		ID: s.ID, UserID: s.UserID, UserEmail: email,
		GroupID: s.GroupID, Group: groupName, Platform: plat,
		Status: s.Status, Expires: s.Expires,
		DailyUsageUSD: s.DailyUsageUSD, WeeklyUsageUSD: s.WeeklyUsageUSD, MonthlyUsageUSD: s.MonthlyUsageUSD,
		Notes: s.Notes,
	}, nil
}

func (h *handler) adminSubscriptionsList(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, params, err := platform.ListQuery(r, "status", "user_id", "group_id", "platform", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	for _, field := range []string{"user_id", "group_id"} {
		if raw := params.Get(field); raw != "" {
			if val, pErr := strconv.ParseInt(raw, 10, 64); pErr != nil || val < 1 {
				return platform.Fail(400, "invalid_input")
			}
		}
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/admin/subscriptions", page, params, projectAdminSubscription)
	return platform.Send(w, 200, rows, err)
}

func (h *handler) userSubscriptions(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	page, err := platform.RequestedPage(r)
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/admin/subscriptions", page,
		url.Values{"user_id": {strconv.FormatInt(id, 10)}}, projectAdminSubscription)
	return platform.Send(w, 200, rows, err)
}

func (h *handler) assignAdminSubscription(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		UserID       int64  `json:"user_id"`
		GroupID      int64  `json:"group_id"`
		ValidityDays int    `json:"validity_days"`
		Notes        string `json:"notes,omitempty"`
	}](r)
	if err != nil || input.UserID < 1 || input.GroupID < 1 || input.ValidityDays < 1 || input.ValidityDays > 36500 || len(input.Notes) > 255 {
		return platform.Fail(400, "invalid_input")
	}
	raw, err := platform.Upstream[adminSubscriptionWire](r.Context(), h.s.Upstream, "POST", "/admin/subscriptions/assign", a.Tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := projectAdminSubscription(raw)
	return platform.Send(w, 201, view, err)
}

func (h *handler) bulkAssignAdminSubscriptions(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		UserIDs      []int64 `json:"user_ids"`
		GroupID      int64   `json:"group_id"`
		ValidityDays int     `json:"validity_days"`
		Notes        string  `json:"notes,omitempty"`
	}](r)
	if err != nil || len(input.UserIDs) == 0 || len(input.UserIDs) > 100 || input.GroupID < 1 || input.ValidityDays < 1 || input.ValidityDays > 36500 {
		return platform.Fail(400, "invalid_input")
	}
	for _, uid := range input.UserIDs {
		if uid < 1 {
			return platform.Fail(400, "invalid_input")
		}
	}
	res, err := platform.Upstream[struct {
		SuccessCount int `json:"success_count"`
		FailedCount  int `json:"failed_count"`
	}](r.Context(), h.s.Upstream, "POST", "/admin/subscriptions/bulk-assign", a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, res, err)
}

func (h *handler) bulkActionAdminSubscriptions(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		SubscriptionIDs []int64 `json:"subscription_ids"`
		Action          string  `json:"action"`
		Days            int     `json:"days,omitempty"`
		Daily           bool    `json:"daily,omitempty"`
		Weekly          bool    `json:"weekly,omitempty"`
		Monthly         bool    `json:"monthly,omitempty"`
		Operation       string  `json:"operation_id,omitempty"`
	}](r)
	if err != nil || len(input.SubscriptionIDs) == 0 || len(input.SubscriptionIDs) > 200 ||
		(input.Action != "extend" && input.Action != "reset_quota" && input.Action != "revoke" && input.Action != "restore") ||
		input.Action == "extend" && (input.Days == 0 || input.Days < -36500 || input.Days > 36500) {
		return platform.Fail(400, "invalid_input")
	}
	for _, sid := range input.SubscriptionIDs {
		if sid < 1 {
			return platform.Fail(400, "invalid_input")
		}
	}
	res, err := platform.Upstream[struct {
		SuccessCount int `json:"success_count"`
		FailedCount  int `json:"failed_count"`
	}](r.Context(), h.s.Upstream, "POST", "/admin/subscriptions/bulk-action", a.Tokens.Access, "", input.Operation, map[string]any{
		"subscription_ids": input.SubscriptionIDs,
		"action":           input.Action,
		"days":             input.Days,
		"daily":            input.Daily,
		"weekly":           input.Weekly,
		"monthly":          input.Monthly,
	})
	return platform.Send(w, 200, res, err)
}

func (h *handler) extendAdminSubscription(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		Days int `json:"days"`
	}](r)
	if err != nil || input.Days == 0 || input.Days < -36500 || input.Days > 36500 {
		return platform.Fail(400, "invalid_input")
	}
	raw, err := platform.Upstream[adminSubscriptionWire](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/subscriptions/%d/extend", id), a.Tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := projectAdminSubscription(raw)
	return platform.Send(w, 200, view, err)
}

func (h *handler) subAction(action string) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		_, err = platform.Upstream[map[string]any](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/subscriptions/%d/%s", id, action), a.Tokens.Access, "", "", struct{}{})
		return platform.Send(w, 200, struct{}{}, err)
	}
}
