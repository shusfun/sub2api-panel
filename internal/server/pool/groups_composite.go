package pool

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func validCompositeRouteInput(in compositeRouteInput) bool {
	return strings.TrimSpace(in.PublicModel) != "" && len(in.PublicModel) <= 128 &&
		(in.MatchType == "exact" || in.MatchType == "prefix") &&
		in.TargetPlatform != "" && in.TargetPlatform != "composite" && len(in.TargetPlatform) <= 50 &&
		len(in.UpstreamModel) <= 128 && len(in.Notes) <= 255 &&
		(in.Endpoint == "any" || in.Endpoint == "messages" || in.Endpoint == "count_tokens" || in.Endpoint == "responses" || in.Endpoint == "chat_completions" || in.Endpoint == "embeddings" || in.Endpoint == "images" || in.Endpoint == "gemini")
}

func (h *handler) listCompositeRoutes(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	rows, err := platform.Upstream[[]compositeRouteView](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/groups/%d/composite-routes", id), a.Tokens.Access, "", "", nil)
	if err == nil && rows == nil {
		rows = []compositeRouteView{}
	}
	return platform.Send(w, 200, rows, err)
}

func (h *handler) createCompositeRoute(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[compositeRouteInput](r)
	if err != nil || !validCompositeRouteInput(input) {
		return platform.Fail(400, "invalid_input")
	}
	created, err := platform.Upstream[compositeRouteView](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/groups/%d/composite-routes", id), a.Tokens.Access, "", "", input)
	return platform.Send(w, 201, created, err)
}

func (h *handler) updateCompositeRoute(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	routeID, err := platform.PathID(r, "route")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[compositeRouteInput](r)
	if err != nil || !validCompositeRouteInput(input) {
		return platform.Fail(400, "invalid_input")
	}
	updated, err := platform.Upstream[compositeRouteView](r.Context(), h.s.Upstream, "PUT", fmt.Sprintf("/admin/groups/%d/composite-routes/%d", id, routeID), a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, updated, err)
}

func (h *handler) deleteCompositeRoute(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	routeID, err := platform.PathID(r, "route")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "DELETE", fmt.Sprintf("/admin/groups/%d/composite-routes/%d", id, routeID), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) previewCompositeRoute(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		Model    string `json:"model"`
		Endpoint string `json:"endpoint"`
	}](r)
	if err != nil || strings.TrimSpace(input.Model) == "" || len(input.Model) > 128 {
		return platform.Fail(400, "invalid_input")
	}
	if input.Endpoint == "" {
		input.Endpoint = "any"
	}
	decision, err := platform.Upstream[struct {
		Matched        bool                `json:"matched"`
		Source         string              `json:"source"`
		GroupID        int64               `json:"group_id"`
		PublicModel    string              `json:"public_model"`
		TargetPlatform string              `json:"target_platform"`
		UpstreamModel  string              `json:"upstream_model"`
		Endpoint       string              `json:"endpoint"`
		Route          *compositeRouteView `json:"route,omitempty"`
		Reason         string              `json:"reason,omitempty"`
	}](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/groups/%d/composite-routes/preview", id), a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, decision, err)
}

func projectChannel(ch channelView) (channelView, error) {
	if ch.ID < 1 || ch.Name == "" {
		return channelView{}, platform.Fail(502, "invalid_upstream")
	}
	if ch.GroupIDs == nil {
		ch.GroupIDs = []int64{}
	}
	if ch.ModelPricing == nil {
		ch.ModelPricing = []channelModelPricing{}
	}
	if ch.ModelMapping == nil {
		ch.ModelMapping = map[string]map[string]string{}
	}
	return ch, nil
}

func (h *handler) listChannels(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, params, err := platform.ListQuery(r, "search", "status", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/admin/channels", page, params, projectChannel)
	return platform.Send(w, 200, rows, err)
}

func (h *handler) getChannel(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	ch, err := platform.Upstream[channelView](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/channels/%d", id), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	view, err := projectChannel(ch)
	return platform.Send(w, 200, view, err)
}

func (h *handler) createChannel(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[channelInput](r)
	if err != nil || strings.TrimSpace(input.Name) == "" || len(input.Name) > 100 {
		return platform.Fail(400, "invalid_input")
	}
	ch, err := platform.Upstream[channelView](r.Context(), h.s.Upstream, "POST", "/admin/channels", a.Tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := projectChannel(ch)
	return platform.Send(w, 201, view, err)
}

func (h *handler) updateChannel(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[channelInput](r)
	if err != nil || input.Name != "" && (strings.TrimSpace(input.Name) == "" || len(input.Name) > 100) ||
		input.Status != "" && input.Status != "active" && input.Status != "disabled" {
		return platform.Fail(400, "invalid_input")
	}
	ch, err := platform.Upstream[channelView](r.Context(), h.s.Upstream, "PUT", fmt.Sprintf("/admin/channels/%d", id), a.Tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := projectChannel(ch)
	return platform.Send(w, 200, view, err)
}

func (h *handler) deleteChannel(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "DELETE", fmt.Sprintf("/admin/channels/%d", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) getChannelModelDefaultPricing(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	model := strings.TrimSpace(r.URL.Query().Get("model"))
	if model == "" || len(model) > 128 {
		return platform.Fail(400, "invalid_input")
	}
	res, err := platform.Upstream[struct {
		Found           bool     `json:"found"`
		InputPrice      *float64 `json:"input_price,omitempty"`
		OutputPrice     *float64 `json:"output_price,omitempty"`
		CacheWritePrice *float64 `json:"cache_write_price,omitempty"`
		CacheReadPrice  *float64 `json:"cache_read_price,omitempty"`
	}](r.Context(), h.s.Upstream, "GET", "/admin/channels/model-pricing?model="+url.QueryEscape(model), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, res, err)
}

func (h *handler) syncChannelPricingModels(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	plat := strings.TrimSpace(r.URL.Query().Get("platform"))
	if plat == "" || len(plat) > 50 {
		return platform.Fail(400, "invalid_input")
	}
	res, err := platform.Upstream[struct {
		Models []string `json:"models"`
	}](r.Context(), h.s.Upstream, "GET", "/admin/channels/pricing/sync-models?platform="+url.QueryEscape(plat), a.Tokens.Access, "", "", nil)
	if err == nil && res.Models == nil {
		res.Models = []string{}
	}
	return platform.Send(w, 200, res, err)
}
