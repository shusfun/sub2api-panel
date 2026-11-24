package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type adminRedeemCodeWire struct {
	ID           int64   `json:"id"`
	Code         string  `json:"code"`
	Type         string  `json:"type"`
	Value        float64 `json:"value"`
	Status       string  `json:"status"`
	UsedBy       *int64  `json:"used_by"`
	UsedAt       *string `json:"used_at"`
	CreatedAt    string  `json:"created_at"`
	ExpiresAt    *string `json:"expires_at"`
	Notes        string  `json:"notes"`
	GroupID      *int64  `json:"group_id"`
	ValidityDays int     `json:"validity_days"`
	User         *struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
	Group *struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"group"`
}

type adminRedeemCodeView struct {
	ID           int64   `json:"id"`
	Masked       string  `json:"masked"`
	Type         string  `json:"type"`
	Value        float64 `json:"value"`
	Status       string  `json:"status"`
	UsedBy       *int64  `json:"used_by"`
	UsedEmail    string  `json:"used_email,omitempty"`
	UsedAt       *string `json:"used_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
	ExpiresAt    *string `json:"expires_at,omitempty"`
	Notes        string  `json:"notes,omitempty"`
	GroupID      *int64  `json:"group_id,omitempty"`
	GroupName    string  `json:"group_name,omitempty"`
	ValidityDays int     `json:"validity_days,omitempty"`
}

func maskRedeemCode(raw string) string {
	if len(raw) > 8 {
		return "••••••••" + raw[len(raw)-4:]
	}
	return "••••••••"
}

func projectAdminRedeemCode(row adminRedeemCodeWire) (adminRedeemCodeView, error) {
	if row.ID < 1 || row.Type == "" {
		return adminRedeemCodeView{}, fail(502, "invalid_upstream")
	}
	email := ""
	if row.User != nil {
		if row.UsedBy == nil && row.User.ID > 0 {
			uid := row.User.ID
			row.UsedBy = &uid
		}
		email = row.User.Email
	}
	groupName := ""
	if row.Group != nil {
		if row.GroupID == nil && row.Group.ID > 0 {
			gid := row.Group.ID
			row.GroupID = &gid
		}
		groupName = row.Group.Name
	}
	return adminRedeemCodeView{
		ID: row.ID, Masked: maskRedeemCode(row.Code), Type: row.Type, Value: row.Value,
		Status: row.Status, UsedBy: row.UsedBy, UsedEmail: email, UsedAt: row.UsedAt,
		CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, Notes: row.Notes,
		GroupID: row.GroupID, GroupName: groupName, ValidityDays: row.ValidityDays,
	}, nil
}

type createdRedeemCodeView struct {
	ID           int64   `json:"id"`
	Code         string  `json:"code"`
	Type         string  `json:"type"`
	Value        float64 `json:"value"`
	GroupID      *int64  `json:"group_id,omitempty"`
	ValidityDays int     `json:"validity_days,omitempty"`
	ExpiresAt    *string `json:"expires_at,omitempty"`
}

func (p *panel) redeemAdminRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api/admin/redeem-codes", true, p.listAdminRedeemCodes)
	p.route(mux, "GET /api/admin/redeem-codes/stats", true, p.adminRedeemCodeStats)
	p.route(mux, "GET /api/admin/redeem-codes/export", true, p.exportMaskedRedeemCodes)
	p.route(mux, "POST /api/admin/redeem-codes/generate", true, p.generateAdminRedeemCodes)
	p.route(mux, "POST /api/admin/redeem-codes/create-and-redeem", true, p.createAndRedeemForUser)
	p.route(mux, "POST /api/admin/redeem-codes/{id}/expire", true, p.expireAdminRedeemCode)
	p.route(mux, "DELETE /api/admin/redeem-codes/{id}", true, p.deleteAdminRedeemCode)
	p.route(mux, "POST /api/admin/redeem-codes/batch-delete", true, p.batchDeleteAdminRedeemCodes)
	p.route(mux, "POST /api/admin/redeem-codes/batch-update", true, p.batchUpdateAdminRedeemCodes)
}

func (p *panel) listAdminRedeemCodes(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, params, err := listQuery(r, "type", "status", "search", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/redeem-codes", page, params, projectAdminRedeemCode)
	return send(w, 200, rows, err)
}

func (p *panel) adminRedeemCodeStats(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	stats, err := upstream[struct {
		Total   int64 `json:"total_codes"`
		Active  int64 `json:"active_codes"`
		Used    int64 `json:"used_codes"`
		Expired int64 `json:"expired_codes"`
	}](r.Context(), p.upstream, "GET", "/admin/redeem-codes/stats", a.tokens.Access, "", "", nil)
	return send(w, 200, stats, err)
}

func validRedeemType(kind string) bool {
	return kind == "balance" || kind == "subscription" || kind == "concurrency" || kind == "invitation"
}

func (p *panel) generateAdminRedeemCodes(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Count         int     `json:"count"`
		Type          string  `json:"type"`
		Value         float64 `json:"value"`
		GroupID       *int64  `json:"group_id,omitempty"`
		ValidityDays  int     `json:"validity_days,omitempty"`
		ExpiresInDays *int    `json:"expires_in_days,omitempty"`
		Operation     string  `json:"operation_id,omitempty"`
	}](r)
	if err != nil || input.Count < 1 || input.Count > 100 || !validRedeemType(input.Type) ||
		input.Value < 0 || input.Value > 1_000_000 ||
		(input.Type == "balance" || input.Type == "concurrency") && input.Value <= 0 ||
		input.Type == "subscription" && (input.GroupID == nil || *input.GroupID < 1 || input.ValidityDays < 1 || input.ValidityDays > 36500) ||
		input.ExpiresInDays != nil && (*input.ExpiresInDays < 1 || *input.ExpiresInDays > 3650) ||
		len(input.Operation) > 128 {
		return fail(400, "invalid_input")
	}
	body := map[string]any{
		"count": input.Count,
		"type":  input.Type,
		"value": input.Value,
	}
	if input.Type == "subscription" && input.GroupID != nil {
		body["group_id"] = *input.GroupID
		body["validity_days"] = input.ValidityDays
	}
	if input.ExpiresInDays != nil {
		body["expires_in_days"] = *input.ExpiresInDays
	}
	created, err := upstream[[]adminRedeemCodeWire](r.Context(), p.upstream, "POST", "/admin/redeem-codes/generate", a.tokens.Access, "", input.Operation, body)
	if err != nil {
		return err
	}
	out := make([]createdRedeemCodeView, 0, len(created))
	for _, item := range created {
		if item.ID < 1 || item.Code == "" {
			return fail(502, "invalid_upstream")
		}
		out = append(out, createdRedeemCodeView{
			ID: item.ID, Code: item.Code, Type: item.Type, Value: item.Value,
			GroupID: item.GroupID, ValidityDays: item.ValidityDays, ExpiresAt: item.ExpiresAt,
		})
	}
	writeJSON(w, 201, struct {
		Codes []createdRedeemCodeView `json:"codes"`
	}{out})
	return nil
}

func (p *panel) createAndRedeemForUser(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		UserID       int64   `json:"user_id"`
		Type         string  `json:"type"`
		Value        float64 `json:"value"`
		GroupID      *int64  `json:"group_id,omitempty"`
		ValidityDays int     `json:"validity_days,omitempty"`
		Notes        string  `json:"notes,omitempty"`
		Operation    string  `json:"operation_id,omitempty"`
	}](r)
	if err != nil || input.UserID < 1 || !validRedeemType(input.Type) || input.Type == "invitation" ||
		input.Value <= 0 || input.Value > 1_000_000 || len(input.Notes) > 255 || len(input.Operation) > 128 ||
		input.Type == "subscription" && (input.GroupID == nil || *input.GroupID < 1 || input.ValidityDays < 1 || input.ValidityDays > 36500) {
		return fail(400, "invalid_input")
	}
	codeToken := "DIRECT-" + strings.ToUpper(randomHex(8))
	body := map[string]any{
		"code":    codeToken,
		"user_id": input.UserID,
		"type":    input.Type,
		"value":   input.Value,
		"notes":   input.Notes,
	}
	if input.Type == "subscription" && input.GroupID != nil {
		body["group_id"] = *input.GroupID
		body["validity_days"] = input.ValidityDays
	}
	_, err = upstream[map[string]any](r.Context(), p.upstream, "POST", "/admin/redeem-codes/create-and-redeem", a.tokens.Access, "", input.Operation, body)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) expireAdminRedeemCode(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	raw, err := upstream[adminRedeemCodeWire](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/redeem-codes/%d/expire", id), a.tokens.Access, "", "", struct{}{})
	if err != nil {
		return err
	}
	view, err := projectAdminRedeemCode(raw)
	return send(w, 200, view, err)
}

func (p *panel) deleteAdminRedeemCode(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "DELETE", fmt.Sprintf("/admin/redeem-codes/%d", id), a.tokens.Access, "", "", nil)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) batchDeleteAdminRedeemCodes(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		IDs []int64 `json:"ids"`
	}](r)
	if err != nil || len(input.IDs) == 0 || len(input.IDs) > 200 {
		return fail(400, "invalid_input")
	}
	for _, id := range input.IDs {
		if id < 1 {
			return fail(400, "invalid_input")
		}
	}
	res, err := upstream[struct {
		Deleted int `json:"deleted"`
	}](r.Context(), p.upstream, "POST", "/admin/redeem-codes/batch-delete", a.tokens.Access, "", "", input)
	return send(w, 200, res, err)
}

func (p *panel) batchUpdateAdminRedeemCodes(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		IDs    []int64 `json:"ids"`
		Fields struct {
			Status    string          `json:"status,omitempty"`
			Notes     *string         `json:"notes,omitempty"`
			GroupID   json.RawMessage `json:"group_id,omitempty"`
			ExpiresAt json.RawMessage `json:"expires_at,omitempty"`
		} `json:"fields"`
	}](r)
	if err != nil || len(input.IDs) == 0 || len(input.IDs) > 200 ||
		input.Fields.Status != "" && input.Fields.Status != "unused" && input.Fields.Status != "disabled" ||
		input.Fields.Status == "" && input.Fields.Notes == nil && input.Fields.GroupID == nil && input.Fields.ExpiresAt == nil {
		return fail(400, "invalid_input")
	}
	if input.Fields.GroupID != nil && string(input.Fields.GroupID) != "null" {
		var group int64
		if json.Unmarshal(input.Fields.GroupID, &group) != nil || group <= 0 {
			return fail(400, "invalid_input")
		}
	}
	if input.Fields.ExpiresAt != nil && string(input.Fields.ExpiresAt) != "null" {
		var until time.Time
		if json.Unmarshal(input.Fields.ExpiresAt, &until) != nil || until.IsZero() {
			return fail(400, "invalid_input")
		}
	}
	for _, id := range input.IDs {
		if id < 1 {
			return fail(400, "invalid_input")
		}
	}
	res, err := upstream[struct {
		Updated int `json:"updated"`
	}](r.Context(), p.upstream, "POST", "/admin/redeem-codes/batch-update", a.tokens.Access, "", "", input)
	return send(w, 200, res, err)
}

func (p *panel) exportMaskedRedeemCodes(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	_, params, err := listQuery(r, "type", "status", "search", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	items := make([]adminRedeemCodeView, 0)
	for offset := 1; offset <= 100; offset++ {
		rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/redeem-codes", offset, params, projectAdminRedeemCode)
		if err != nil {
			return err
		}
		items = append(items, rows.Items...)
		if !rows.More {
			writeJSON(w, 200, items)
			return nil
		}
	}
	return fail(400, "invalid_input")
}
