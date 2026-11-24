package billing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
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
		return adminRedeemCodeView{}, platform.Fail(502, "invalid_upstream")
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

func (h *handler) registerRedeemAdminRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/admin/redeem-codes", true, h.listAdminRedeemCodes)
	h.s.Route(mux, "GET /api/admin/redeem-codes/stats", true, h.adminRedeemCodeStats)
	h.s.Route(mux, "GET /api/admin/redeem-codes/export", true, h.exportMaskedRedeemCodes)
	h.s.Route(mux, "POST /api/admin/redeem-codes/generate", true, h.generateAdminRedeemCodes)
	h.s.Route(mux, "POST /api/admin/redeem-codes/create-and-redeem", true, h.createAndRedeemForUser)
	h.s.Route(mux, "POST /api/admin/redeem-codes/{id}/expire", true, h.expireAdminRedeemCode)
	h.s.Route(mux, "DELETE /api/admin/redeem-codes/{id}", true, h.deleteAdminRedeemCode)
	h.s.Route(mux, "POST /api/admin/redeem-codes/batch-delete", true, h.batchDeleteAdminRedeemCodes)
	h.s.Route(mux, "POST /api/admin/redeem-codes/batch-update", true, h.batchUpdateAdminRedeemCodes)
}

func (h *handler) listAdminRedeemCodes(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, params, err := platform.ListQuery(r, "type", "status", "search", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/admin/redeem-codes", page, params, projectAdminRedeemCode)
	return platform.Send(w, 200, rows, err)
}

func (h *handler) adminRedeemCodeStats(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	stats, err := platform.Upstream[struct {
		Total   int64 `json:"total_codes"`
		Active  int64 `json:"active_codes"`
		Used    int64 `json:"used_codes"`
		Expired int64 `json:"expired_codes"`
	}](r.Context(), h.s.Upstream, "GET", "/admin/redeem-codes/stats", a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, stats, err)
}

func validRedeemType(kind string) bool {
	return kind == "balance" || kind == "subscription" || kind == "concurrency" || kind == "invitation"
}

func (h *handler) generateAdminRedeemCodes(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
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
		return platform.Fail(400, "invalid_input")
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
	created, err := platform.Upstream[[]adminRedeemCodeWire](r.Context(), h.s.Upstream, "POST", "/admin/redeem-codes/generate", a.Tokens.Access, "", input.Operation, body)
	if err != nil {
		return err
	}
	out := make([]createdRedeemCodeView, 0, len(created))
	for _, item := range created {
		if item.ID < 1 || item.Code == "" {
			return platform.Fail(502, "invalid_upstream")
		}
		out = append(out, createdRedeemCodeView{
			ID: item.ID, Code: item.Code, Type: item.Type, Value: item.Value,
			GroupID: item.GroupID, ValidityDays: item.ValidityDays, ExpiresAt: item.ExpiresAt,
		})
	}
	platform.WriteJSON(w, 201, struct {
		Codes []createdRedeemCodeView `json:"codes"`
	}{out})
	return nil
}

func (h *handler) createAndRedeemForUser(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
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
		return platform.Fail(400, "invalid_input")
	}
	codeToken := "DIRECT-" + strings.ToUpper(platform.RandomHex(8))
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
	_, err = platform.Upstream[map[string]any](r.Context(), h.s.Upstream, "POST", "/admin/redeem-codes/create-and-redeem", a.Tokens.Access, "", input.Operation, body)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) expireAdminRedeemCode(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	raw, err := platform.Upstream[adminRedeemCodeWire](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/redeem-codes/%d/expire", id), a.Tokens.Access, "", "", struct{}{})
	if err != nil {
		return err
	}
	view, err := projectAdminRedeemCode(raw)
	return platform.Send(w, 200, view, err)
}

func (h *handler) deleteAdminRedeemCode(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "DELETE", fmt.Sprintf("/admin/redeem-codes/%d", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) batchDeleteAdminRedeemCodes(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		IDs []int64 `json:"ids"`
	}](r)
	if err != nil || len(input.IDs) == 0 || len(input.IDs) > 200 {
		return platform.Fail(400, "invalid_input")
	}
	for _, id := range input.IDs {
		if id < 1 {
			return platform.Fail(400, "invalid_input")
		}
	}
	res, err := platform.Upstream[struct {
		Deleted int `json:"deleted"`
	}](r.Context(), h.s.Upstream, "POST", "/admin/redeem-codes/batch-delete", a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, res, err)
}

func (h *handler) batchUpdateAdminRedeemCodes(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
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
		return platform.Fail(400, "invalid_input")
	}
	if input.Fields.GroupID != nil && string(input.Fields.GroupID) != "null" {
		var group int64
		if json.Unmarshal(input.Fields.GroupID, &group) != nil || group <= 0 {
			return platform.Fail(400, "invalid_input")
		}
	}
	if input.Fields.ExpiresAt != nil && string(input.Fields.ExpiresAt) != "null" {
		var until time.Time
		if json.Unmarshal(input.Fields.ExpiresAt, &until) != nil || until.IsZero() {
			return platform.Fail(400, "invalid_input")
		}
	}
	for _, id := range input.IDs {
		if id < 1 {
			return platform.Fail(400, "invalid_input")
		}
	}
	res, err := platform.Upstream[struct {
		Updated int `json:"updated"`
	}](r.Context(), h.s.Upstream, "POST", "/admin/redeem-codes/batch-update", a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, res, err)
}

func (h *handler) exportMaskedRedeemCodes(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	_, params, err := platform.ListQuery(r, "type", "status", "search", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	items := make([]adminRedeemCodeView, 0)
	for offset := 1; offset <= 100; offset++ {
		rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/admin/redeem-codes", offset, params, projectAdminRedeemCode)
		if err != nil {
			return err
		}
		items = append(items, rows.Items...)
		if !rows.More {
			platform.WriteJSON(w, 200, items)
			return nil
		}
	}
	return platform.Fail(400, "invalid_input")
}
