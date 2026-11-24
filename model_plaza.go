package main

import "net/http"

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
	namedItem
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

func (p *panel) modelPlaza(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	w.Header().Set("Cache-Control", "no-store")
	p.runtimeMu.RLock()
	client, setup := p.upstream, p.setup
	p.runtimeMu.RUnlock()
	if setup {
		return fail(http.StatusServiceUnavailable, "setup_required")
	}
	row, err := upstream[struct {
		Description string            `json:"description"`
		Groups      []modelPlazaGroup `json:"groups"`
	}](r.Context(), client, http.MethodGet, "/model-plaza", a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	row.Groups, err = projectList(row.Groups, func(group modelPlazaGroup) (modelPlazaGroup, error) {
		if group.Models == nil {
			group.Models = []modelPlazaModel{}
		}
		return group, requiredID(group.ID)
	})
	return send(w, http.StatusOK, row, err)
}
