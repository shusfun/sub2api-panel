package pool

import (
	"fmt"
	"net/http"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
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

type availableChannel struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Platforms   []struct {
		Platform string `json:"platform"`
		Groups   []struct {
			platform.NamedItem
			Rate         float64 `json:"rate_multiplier"`
			Subscription string  `json:"subscription_type"`
		} `json:"groups"`
		Models []struct {
			Name    string `json:"name"`
			Pricing *struct {
				Input   *float64 `json:"input_price"`
				Output  *float64 `json:"output_price"`
				Request *float64 `json:"per_request_price"`
			} `json:"pricing"`
		} `json:"supported_models"`
	} `json:"platforms"`
}

type modelPlazaPrice struct {
	BillingMode string                   `json:"billing_mode,omitempty"`
	Input       *float64                 `json:"input_price,omitempty"`
	Output      *float64                 `json:"output_price,omitempty"`
	CacheRead   *float64                 `json:"cache_read_price,omitempty"`
	CacheWrite  *float64                 `json:"cache_write_price,omitempty"`
	ImageOutput *float64                 `json:"image_output_price,omitempty"`
	PerRequest  *float64                 `json:"per_request_price,omitempty"`
	Intervals   []channelPricingInterval `json:"intervals,omitempty"`
}

type modelPlazaModel struct {
	Name             string              `json:"name"`
	Pricing          *modelPlazaPrice    `json:"pricing"`
	Official         *modelPlazaPrice    `json:"official_pricing"`
	LongContextBasis string              `json:"long_context_basis,omitempty"`
	TimePricing      *channelTimePricing `json:"time_pricing,omitempty"`
}

type modelPlazaGroup struct {
	platform.NamedItem
	Description      string            `json:"description"`
	Platform         string            `json:"platform"`
	Subscription     string            `json:"subscription_type"`
	Rate             float64           `json:"rate_multiplier"`
	CustomRate       *float64          `json:"user_rate_multiplier,omitempty"`
	PeakEnabled      bool              `json:"peak_rate_enabled"`
	PeakStart        string            `json:"peak_start,omitempty"`
	PeakEnd          string            `json:"peak_end,omitempty"`
	PeakRate         float64           `json:"peak_rate_multiplier"`
	Private          bool              `json:"is_exclusive"`
	ImageIndependent bool              `json:"image_rate_independent"`
	ImageRate        float64           `json:"image_rate_multiplier"`
	VideoIndependent bool              `json:"video_rate_independent"`
	VideoRate        float64           `json:"video_rate_multiplier"`
	LongContext      bool              `json:"long_context_pricing_enabled"`
	Models           []modelPlazaModel `json:"models"`
}

func (h *handler) registerChannelAndPlazaRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/channels/available", false, h.availableChannels)
	h.s.Route(mux, "GET /api/channel-monitor", false, h.channelMonitor)
	h.s.Route(mux, "GET /api"+monitorRulesPath, true, h.channelRules)
	h.s.Route(mux, "POST /api"+monitorRulesPath, true, h.channelRuleSave)
	h.s.Route(mux, "GET /api"+monitorRulesPath+"/{id}", true, h.channelRuleRead)
	h.s.Route(mux, "PUT /api"+monitorRulesPath+"/{id}", true, h.channelRuleSave)
	h.s.Route(mux, "DELETE /api"+monitorRulesPath+"/{id}", true, h.channelRuleDelete)
	h.s.Public(mux, "GET /api/public/model-plaza", func(w http.ResponseWriter, r *http.Request) error {
		return h.modelPlaza(w, r, core.SessionAuth{})
	})
	h.s.Route(mux, "GET /api/model-plaza", false, h.modelPlaza)
}

func (h *handler) availableChannels(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	channels, err := platform.Upstream[[]availableChannel](r.Context(), h.s.Upstream, "GET", "/channels/available", a.Tokens.Access, "", "", nil)
	if channels == nil {
		channels = []availableChannel{}
	}
	return platform.Send(w, http.StatusOK, channels, err)
}

func (h *handler) channelRules(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, query, err := platform.ListQuery(r, "search", "provider")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, monitorRulesPath, page, query, func(row officialMonitor) (officialMonitor, error) {
		return row, platform.RequiredID(row.ID)
	})
	return platform.Send(w, http.StatusOK, rows, err)
}

func (h *handler) channelRuleRead(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	row, err := platform.Upstream[channelRule](r.Context(), h.s.Upstream, r.Method, fmt.Sprintf("%s/%d", monitorRulesPath, id), a.Tokens.Access, "", "", nil)
	if err == nil && row.ID != id {
		err = platform.Fail(http.StatusBadGateway, "invalid_upstream")
	}
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) channelRuleSave(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[channelRuleInput](r)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.Name) == "" || input.Provider == "" || input.Interval < 15 || input.Interval > 3600 ||
		input.Jitter < 0 || input.Jitter >= input.Interval || input.Check != "probe" && input.Check != "quota" && input.Check != "quota_probe" ||
		input.Check != "probe" && (input.Account == nil || *input.Account < 1) {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	path, status := monitorRulesPath, http.StatusCreated
	if r.Method == http.MethodPut {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		path, status = fmt.Sprintf("%s/%d", path, id), http.StatusOK
	}
	row, err := platform.Upstream[channelRule](r.Context(), h.s.Upstream, r.Method, path, a.Tokens.Access, "", "", input)
	if err == nil {
		err = platform.RequiredID(row.ID)
	}
	return platform.Send(w, status, row, err)
}

func (h *handler) channelRuleDelete(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	row, err := platform.Upstream[struct{}](r.Context(), h.s.Upstream, r.Method, fmt.Sprintf("%s/%d", monitorRulesPath, id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) channelMonitor(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	settings, err := h.s.Settings(r.Context())
	if err != nil {
		return err
	}
	if !settings.ChannelMonitor {
		return platform.Fail(http.StatusNotFound, "not_found")
	}
	view := struct {
		Mode   string            `json:"mode"`
		Active []officialMonitor `json:"active"`
		Health []monitorHealth   `json:"health"`
	}{Mode: "v1", Active: []officialMonitor{}, Health: []monitorHealth{}}
	if settings.MonitorMode == "v2" {
		view.Mode = "v2"
		data, err := platform.Upstream[struct {
			Items []monitorHealth `json:"items"`
		}](r.Context(), h.s.Upstream, http.MethodGet, "/channel-monitor-v2/matrix?group_by=group", a.Tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		view.Health = append(view.Health, data.Items...)
	} else {
		data, err := platform.Upstream[struct {
			Items []officialMonitor `json:"items"`
		}](r.Context(), h.s.Upstream, http.MethodGet, "/channel-monitors", a.Tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		view.Active = append(view.Active, data.Items...)
	}
	return platform.Send(w, http.StatusOK, view, nil)
}

func (h *handler) modelPlaza(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	w.Header().Set("Cache-Control", "no-store")
	h.s.RuntimeMu.RLock()
	client, setup := h.s.Upstream, h.s.Setup
	h.s.RuntimeMu.RUnlock()
	if setup {
		return platform.Fail(http.StatusServiceUnavailable, "setup_required")
	}
	row, err := platform.Upstream[struct {
		Description string            `json:"description"`
		Groups      []modelPlazaGroup `json:"groups"`
	}](r.Context(), client, http.MethodGet, "/model-plaza", a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	row.Groups, err = platform.ProjectList(row.Groups, func(group modelPlazaGroup) (modelPlazaGroup, error) {
		if group.Models == nil {
			group.Models = []modelPlazaModel{}
		}
		return group, platform.RequiredID(group.ID)
	})
	return platform.Send(w, http.StatusOK, row, err)
}
