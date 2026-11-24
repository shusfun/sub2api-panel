package main

import (
	"fmt"
	"net/http"
	"strings"
)

const monitorRulesPath = "/admin/channel-monitors"

type officialMonitor struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Provider     string  `json:"provider"`
	GroupName    string  `json:"group_name"`
	PrimaryModel string  `json:"primary_model"`
	Status       string  `json:"primary_status"`
	Latency      *int    `json:"primary_latency_ms"`
	Uptime       float64 `json:"availability_7d"`
}

type monitorHealth struct {
	Platform string `json:"platform"`
	GroupID  *int64 `json:"group_id,omitempty"`
	Group    string `json:"group_name"`
	Health   struct {
		Overall string   `json:"overall"`
		Score   *float64 `json:"score"`
	} `json:"health"`
}

type channelRule struct {
	officialMonitor
	APIMode  string   `json:"api_mode"`
	Endpoint string   `json:"endpoint"`
	Key      string   `json:"api_key_masked"`
	Models   []string `json:"extra_models"`
	Enabled  bool     `json:"enabled"`
	Interval int      `json:"interval_seconds"`
	Jitter   int      `json:"jitter_seconds"`
	Check    string   `json:"check_mode"`
	Account  *int64   `json:"account_id"`
}

type channelRuleInput struct {
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
	APIMode  string   `json:"api_mode,omitempty"`
	Endpoint *string  `json:"endpoint,omitempty"`
	Key      string   `json:"api_key,omitempty"`
	Model    *string  `json:"primary_model,omitempty"`
	Models   []string `json:"extra_models"`
	Group    string   `json:"group_name"`
	Enabled  bool     `json:"enabled"`
	Interval int      `json:"interval_seconds"`
	Jitter   int      `json:"jitter_seconds"`
	Check    string   `json:"check_mode"`
	Account  *int64   `json:"account_id,omitempty"`
}

func (p *panel) channelRuleRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api"+monitorRulesPath, true, p.channelRules)
	p.route(mux, "POST /api"+monitorRulesPath, true, p.channelRuleSave)
	p.route(mux, "GET /api"+monitorRulesPath+"/{id}", true, p.channelRuleRead)
	p.route(mux, "PUT /api"+monitorRulesPath+"/{id}", true, p.channelRuleSave)
	p.route(mux, "DELETE /api"+monitorRulesPath+"/{id}", true, p.channelRuleDelete)
}

func (p *panel) channelRules(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, query, err := listQuery(r, "search", "provider")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, monitorRulesPath, page, query, func(row officialMonitor) (officialMonitor, error) {
		return row, requiredID(row.ID)
	})
	return send(w, http.StatusOK, rows, err)
}

func (p *panel) channelRuleRead(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	row, err := upstream[channelRule](r.Context(), p.upstream, r.Method, fmt.Sprintf("%s/%d", monitorRulesPath, id), a.tokens.Access, "", "", nil)
	if err == nil && row.ID != id {
		err = fail(http.StatusBadGateway, "invalid_upstream")
	}
	return send(w, http.StatusOK, row, err)
}

func (p *panel) channelRuleSave(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[channelRuleInput](r)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.Name) == "" || input.Provider == "" || input.Interval < 15 || input.Interval > 3600 ||
		input.Jitter < 0 || input.Jitter >= input.Interval || input.Check != "probe" && input.Check != "quota" && input.Check != "quota_probe" ||
		input.Check != "probe" && (input.Account == nil || *input.Account < 1) {
		return fail(http.StatusBadRequest, "invalid_input")
	}
	path, status := monitorRulesPath, http.StatusCreated
	if r.Method == http.MethodPut {
		id, err := pathID(r, "id")
		if err != nil {
			return err
		}
		path, status = fmt.Sprintf("%s/%d", path, id), http.StatusOK
	}
	row, err := upstream[channelRule](r.Context(), p.upstream, r.Method, path, a.tokens.Access, "", "", input)
	if err == nil {
		err = requiredID(row.ID)
	}
	return send(w, status, row, err)
}

func (p *panel) channelRuleDelete(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	row, err := upstream[struct{}](r.Context(), p.upstream, r.Method, fmt.Sprintf("%s/%d", monitorRulesPath, id), a.tokens.Access, "", "", nil)
	return send(w, http.StatusOK, row, err)
}

func (p *panel) channelMonitor(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	settings, err := p.settings(r.Context())
	if err != nil {
		return err
	}
	if !settings.ChannelMonitor {
		return fail(http.StatusNotFound, "not_found")
	}
	view := struct {
		Mode   string            `json:"mode"`
		Active []officialMonitor `json:"active"`
		Health []monitorHealth   `json:"health"`
	}{Mode: "v1", Active: []officialMonitor{}, Health: []monitorHealth{}}
	if settings.MonitorMode == "v2" {
		view.Mode = "v2"
		data, err := upstream[struct {
			Items []monitorHealth `json:"items"`
		}](r.Context(), p.upstream, http.MethodGet, "/channel-monitor-v2/matrix?group_by=group", a.tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		view.Health = append(view.Health, data.Items...)
	} else {
		data, err := upstream[struct {
			Items []officialMonitor `json:"items"`
		}](r.Context(), p.upstream, http.MethodGet, "/channel-monitors", a.tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		view.Active = append(view.Active, data.Items...)
	}
	return send(w, http.StatusOK, view, nil)
}
