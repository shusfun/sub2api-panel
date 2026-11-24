package main

import (
	"fmt"
	"net/http"
	"time"
)

type riskConfigView struct {
	Enabled        bool           `json:"enabled"`
	Engine         string         `json:"engine"`
	Mode           string         `json:"mode"`
	Model          string         `json:"model"`
	SampleRate     int            `json:"sample_rate"`
	AllGroups      bool           `json:"all_groups"`
	GroupIDs       []int64        `json:"group_ids"`
	AutoBan        bool           `json:"auto_ban_enabled"`
	BanThreshold   int            `json:"ban_threshold"`
	KeysConfigured bool           `json:"api_key_configured"`
	BaseURL        string         `json:"base_url"`
	Timeout        int            `json:"timeout_ms"`
	RecordNonHits  bool           `json:"record_non_hits"`
	BlockStatus    int            `json:"block_status"`
	BlockMessage   string         `json:"block_message"`
	HitRetention   int            `json:"hit_retention_days"`
	OtherRetention int            `json:"non_hit_retention_days"`
	OpenAI         riskEngineView `json:"openai"`
}

type riskEngineView struct {
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	KeysConfigured bool   `json:"api_key_configured"`
	Timeout        int    `json:"timeout_ms"`
}

type riskLogRow struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id"`
	Email     string    `json:"user_email"`
	RequestID string    `json:"request_id"`
	Model     string    `json:"model"`
	Group     string    `json:"group_name"`
	Action    string    `json:"action"`
	Flagged   bool      `json:"flagged"`
	Category  string    `json:"highest_category"`
	Score     float64   `json:"highest_score"`
	Banned    bool      `json:"auto_banned"`
	UserState string    `json:"user_status"`
	Created   time.Time `json:"created_at"`
}

func (p *panel) riskRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api/admin/risk-control/config", true, p.riskConfiguration)
	p.route(mux, "PUT /api/admin/risk-control/config", true, p.riskConfiguration)
	p.route(mux, "GET /api/admin/risk-control/logs", true, p.riskEntries)
	p.route(mux, "POST /api/admin/risk-control/users/{id}/unban", true, p.riskUnban)
}

func (p *panel) riskConfiguration(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	var payload any
	if r.Method == http.MethodPut {
		input, err := decodeBody[struct {
			Enabled    *bool    `json:"enabled"`
			SampleRate *int     `json:"sample_rate,omitempty"`
			AllGroups  *bool    `json:"all_groups,omitempty"`
			GroupIDs   *[]int64 `json:"group_ids,omitempty"`
			AutoBan    *bool    `json:"auto_ban_enabled,omitempty"`
			Threshold  *int     `json:"ban_threshold,omitempty"`
			Engine     string   `json:"engine,omitempty"`
			Mode       string   `json:"mode,omitempty"`
			BaseURL    *string  `json:"base_url,omitempty"`
			Model      *string  `json:"model,omitempty"`
			Key        string   `json:"api_key,omitempty"`
			Timeout    *int     `json:"timeout_ms,omitempty"`
			NonHits    *bool    `json:"record_non_hits,omitempty"`
			Status     *int     `json:"block_status,omitempty"`
			Message    *string  `json:"block_message,omitempty"`
			HitDays    *int     `json:"hit_retention_days,omitempty"`
			OtherDays  *int     `json:"non_hit_retention_days,omitempty"`
		}](r)
		if err != nil {
			return err
		}
		if input.Enabled == nil || input.SampleRate != nil && (*input.SampleRate < 0 || *input.SampleRate > 100) ||
			input.Threshold != nil && *input.Threshold < 1 || input.GroupIDs != nil && len(*input.GroupIDs) > 1000 ||
			input.Engine != "" && input.Engine != "openai" && input.Engine != "typesafe" ||
			input.Mode != "" && input.Mode != "off" && input.Mode != "observe" && input.Mode != "pre_block" ||
			input.Timeout != nil && *input.Timeout < 1 || input.Status != nil && (*input.Status < 400 || *input.Status > 599) ||
			input.HitDays != nil && *input.HitDays < 0 || input.OtherDays != nil && *input.OtherDays < 0 {
			return fail(http.StatusBadRequest, "invalid_input")
		}
		payload = input
	}
	data, err := upstream[struct {
		riskConfigView
		Profiles map[string]riskEngineView `json:"engine_configs"`
	}](r.Context(), p.upstream, r.Method, "/admin/risk-control/config", a.tokens.Access, "", "", payload)
	row := data.riskConfigView
	if profile, ok := data.Profiles["openai"]; ok {
		row.OpenAI = profile
	} else if row.Engine == "openai" {
		row.OpenAI = riskEngineView{row.BaseURL, row.Model, row.KeysConfigured, row.Timeout}
	}
	if row.GroupIDs == nil {
		row.GroupIDs = []int64{}
	}
	return send(w, http.StatusOK, row, err)
}

func (p *panel) riskEntries(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, query, err := listQuery(r, "search", "group_id", "result")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/risk-control/logs", page, query,
		func(row riskLogRow) (riskLogRow, error) { return row, requiredID(row.ID) })
	return send(w, http.StatusOK, rows, err)
}

func (p *panel) riskUnban(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	result, err := upstream[struct{}](r.Context(), p.upstream, http.MethodPost, fmt.Sprintf("/admin/risk-control/users/%d/unban", id), a.tokens.Access, "", "", nil)
	return send(w, http.StatusOK, result, err)
}
