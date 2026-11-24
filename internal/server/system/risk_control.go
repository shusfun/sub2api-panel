package system

import (
	"fmt"
	"net/http"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type RiskConfigView struct {
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
	OpenAI         RiskEngineView `json:"openai"`
}

type RiskEngineView struct {
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	KeysConfigured bool   `json:"api_key_configured"`
	Timeout        int    `json:"timeout_ms"`
}

type RiskLogRow struct {
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

func (h *handler) riskRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/admin/risk-control/config", true, h.riskConfiguration)
	h.s.Route(mux, "PUT /api/admin/risk-control/config", true, h.riskConfiguration)
	h.s.Route(mux, "GET /api/admin/risk-control/logs", true, h.riskEntries)
	h.s.Route(mux, "POST /api/admin/risk-control/users/{id}/unban", true, h.riskUnban)
}

func (h *handler) riskConfiguration(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	var payload any
	if r.Method == http.MethodPut {
		input, err := platform.DecodeBody[struct {
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
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
		payload = input
	}
	data, err := platform.Upstream[struct {
		RiskConfigView
		Profiles map[string]RiskEngineView `json:"engine_configs"`
	}](r.Context(), h.s.Upstream, r.Method, "/admin/risk-control/config", a.Tokens.Access, "", "", payload)
	row := data.RiskConfigView
	if profile, ok := data.Profiles["openai"]; ok {
		row.OpenAI = profile
	} else if row.Engine == "openai" {
		row.OpenAI = RiskEngineView{row.BaseURL, row.Model, row.KeysConfigured, row.Timeout}
	}
	if row.GroupIDs == nil {
		row.GroupIDs = []int64{}
	}
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) riskEntries(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, query, err := platform.ListQuery(r, "search", "group_id", "result")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/admin/risk-control/logs", page, query,
		func(row RiskLogRow) (RiskLogRow, error) { return row, platform.RequiredID(row.ID) })
	return platform.Send(w, http.StatusOK, rows, err)
}

func (h *handler) riskUnban(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	result, err := platform.Upstream[struct{}](r.Context(), h.s.Upstream, http.MethodPost, fmt.Sprintf("/admin/risk-control/users/%d/unban", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, http.StatusOK, result, err)
}
