package system

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"sub2api-panel/internal/extplugin"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/pluginrt"
	"sub2api-panel/internal/server/core"
)

type mountedPluginView struct {
	PluginID string `json:"plugin_id"`
	Title    string `json:"title"`
	Mount    string `json:"mount"`
}

func (h *handler) mountedPlugins(includeAdmin bool) []mountedPluginView {
	_, ext := h.s.EnsurePlugins()
	out := make([]mountedPluginView, 0)
	for _, item := range ext.Overview().Plugins {
		if !item.Enabled || !item.Ready || item.Page == nil {
			continue
		}
		mount := item.Page.Mount
		if !includeAdmin && mount == "nav:admin" {
			continue
		}
		if mount == "home" || mount == "patrol" || mount == "nav:user" || (includeAdmin && mount == "nav:admin") {
			out = append(out, mountedPluginView{
				PluginID: item.ID,
				Title:    item.Page.Title,
				Mount:    mount,
			})
		}
	}
	return out
}

func (h *handler) pluginRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/admin/plugins/upstream", true, h.officialPluginList)
	h.s.Route(mux, "POST /api/admin/plugins/upstream/upload/authorize", true, h.officialPluginAuthorizeUpload)
	h.s.RouteWithLimit(mux, "POST /api/admin/plugins/upstream/upload", true, OfficialPluginUploadLimit, h.officialPluginUpload)
	h.s.Route(mux, "POST /api/admin/plugins/upstream/{id}/{action}", true, h.officialPluginAction)
	h.s.Route(mux, "GET /api/admin/plugins/upstream/{id}/config", true, h.officialPluginConfig)
	h.s.Route(mux, "PUT /api/admin/plugins/upstream/{id}/config", true, h.officialPluginConfig)
	h.s.Route(mux, "DELETE /api/admin/plugins/upstream/{id}", true, h.officialPluginRemove)
	h.s.Public(mux, "GET /api/plugins/mounted", h.publicMountedPlugins)
	h.s.Public(mux, "POST /api/plugins/{id}/public-page", h.loadPublicPluginPage)
	h.s.Public(mux, "GET /api/plugins/frame/{id}", h.servePluginFrame)
	h.s.Route(mux, "GET /api/plugins/user", false, h.userMountedPlugins)
	h.s.Route(mux, "POST /api/plugins/{id}/user-page", false, h.loadUserPluginPage)

	h.s.Route(mux, "GET /api/admin/plugins", true, h.adminPluginsOverview)
	h.s.Route(mux, "PUT /api/admin/plugins/builtin/{id}", true, h.adminUpdateBuiltinPlugin)
	h.s.Route(mux, "POST /api/admin/plugins/market/refresh", true, h.adminRefreshPluginMarket)
	h.s.Route(mux, "PUT /api/admin/plugins/external/{id}", true, h.adminConfigureExtPlugin)
	h.s.Route(mux, "POST /api/admin/plugins/external/folder", true, h.adminMountPluginFolder)
	h.s.Route(mux, "DELETE /api/admin/plugins/external/{id}", true, h.adminRemoveExtPlugin)
	h.s.Route(mux, "POST /api/admin/plugins/external/{id}/page", true, h.adminLoadPluginPage)
	h.s.Route(mux, "POST /api/admin/plugins/{id}/invoke", true, h.adminInvokePluginTool)
}

func (h *handler) publicMountedPlugins(w http.ResponseWriter, _ *http.Request) error {
	items := make([]mountedPluginView, 0)
	for _, m := range h.mountedPlugins(false) {
		if m.Mount == "home" || m.Mount == "patrol" {
			items = append(items, m)
		}
	}
	platform.WriteJSON(w, 200, struct {
		Items []mountedPluginView `json:"items"`
	}{items})
	return nil
}

func (h *handler) userMountedPlugins(w http.ResponseWriter, _ *http.Request, a core.SessionAuth) error {
	platform.WriteJSON(w, 200, struct {
		Items []mountedPluginView `json:"items"`
	}{h.mountedPlugins(a.User.Role == "admin")})
	return nil
}

type pluginPageInput struct {
	Theme   string            `json:"theme,omitempty"`
	CSSVars map[string]string `json:"css_vars,omitempty"`
}

func (h *handler) loadPublicPluginPage(w http.ResponseWriter, r *http.Request) error {
	id := strings.TrimSpace(r.PathValue("id"))
	input, err := platform.DecodeBody[pluginPageInput](r)
	if err != nil {
		return err
	}
	_, ext := h.s.EnsurePlugins()
	bundle, err := ext.LoadSandboxedPage(id, input.Theme, input.CSSVars)
	if err != nil || (bundle.Mount != "home" && bundle.Mount != "patrol") {
		return platform.Fail(404, "not_found")
	}
	platform.WriteJSON(w, 200, bundle)
	return nil
}

func (h *handler) servePluginFrame(w http.ResponseWriter, r *http.Request) error {
	id := strings.TrimSpace(r.PathValue("id"))
	theme := r.URL.Query().Get("theme")
	_, ext := h.s.EnsurePlugins()
	bundle, err := ext.LoadSandboxedPage(id, theme, nil)
	if err != nil {
		return platform.Fail(404, "not_found")
	}
	if bundle.Mount == "nav:admin" {
		active, aErr := h.s.Sessions.Authenticate(r, h.s.Origin, time.Now())
		if aErr != nil {
			return aErr
		}
		user, _, pErr := h.s.Sessions.Principal(r.Context(), h.s.Upstream, active)
		if pErr != nil || user.Role != "admin" {
			return platform.Fail(403, "not_allowed")
		}
	} else if bundle.Mount == "nav:user" {
		if _, aErr := h.s.Sessions.Authenticate(r, h.s.Origin, time.Now()); aErr != nil {
			return aErr
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	_, _ = w.Write([]byte(bundle.HTML))
	return nil
}

func (h *handler) loadUserPluginPage(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id := strings.TrimSpace(r.PathValue("id"))
	input, err := platform.DecodeBody[pluginPageInput](r)
	if err != nil {
		return err
	}
	_, ext := h.s.EnsurePlugins()
	bundle, err := ext.LoadSandboxedPage(id, input.Theme, input.CSSVars)
	if err != nil {
		return platform.Fail(404, "not_found")
	}
	if bundle.Mount == "nav:admin" && a.User.Role != "admin" {
		return platform.Fail(403, "not_allowed")
	}
	platform.WriteJSON(w, 200, bundle)
	return nil
}

func (h *handler) adminPluginsOverview(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
	rt, ext := h.s.EnsurePlugins()
	upstreamEnabled := false
	if s, err := h.s.Settings(r.Context()); err == nil {
		upstreamEnabled = s.PluginManagement
	}
	platform.WriteJSON(w, 200, struct {
		Builtin         []pluginrt.State         `json:"builtin"`
		Market          extplugin.MarketOverview `json:"market"`
		UpstreamEnabled bool                     `json:"upstream_enabled"`
	}{
		Builtin:         rt.States(),
		Market:          ext.Overview(),
		UpstreamEnabled: upstreamEnabled,
	})
	return nil
}

func (h *handler) adminUpdateBuiltinPlugin(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
	id := strings.TrimSpace(r.PathValue("id"))
	input, err := platform.DecodeBody[struct {
		Enabled *bool          `json:"enabled,omitempty"`
		Config  map[string]any `json:"config,omitempty"`
	}](r)
	if err != nil || id == "" {
		return platform.Fail(400, "invalid_input")
	}
	rt, _ := h.s.EnsurePlugins()
	states, uErr := rt.Update(id, input.Enabled, input.Config)
	if uErr != nil {
		return platform.Fail(400, "invalid_input")
	}
	slog.Info("builtin plugin updated", "trace_id", platform.Meta(r.Context()).Trace, "plugin_id", id)
	platform.WriteJSON(w, 200, struct {
		Builtin []pluginrt.State `json:"builtin"`
	}{states})
	return nil
}

func (h *handler) adminRefreshPluginMarket(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		MarketURL string `json:"market_url,omitempty"`
	}](r)
	if err != nil {
		return err
	}
	_, ext := h.s.EnsurePlugins()
	ov, rErr := ext.RefreshMarket(r.Context(), input.MarketURL)
	if rErr != nil {
		return platform.Fail(400, "invalid_input")
	}
	platform.WriteJSON(w, 200, ov)
	return nil
}

func (h *handler) adminConfigureExtPlugin(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
	id := strings.TrimSpace(r.PathValue("id"))
	input, err := platform.DecodeBody[struct {
		Enabled *bool             `json:"enabled,omitempty"`
		Options map[string]string `json:"options,omitempty"`
	}](r)
	if err != nil || id == "" {
		return platform.Fail(400, "invalid_input")
	}
	_, ext := h.s.EnsurePlugins()
	ov, cErr := ext.Configure(r.Context(), id, input.Enabled, input.Options)
	if cErr != nil {
		return platform.Fail(400, "invalid_input")
	}
	slog.Info("external plugin configured", "trace_id", platform.Meta(r.Context()).Trace, "plugin_id", id)
	platform.WriteJSON(w, 200, ov)
	return nil
}

func (h *handler) adminMountPluginFolder(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Folder string `json:"folder"`
	}](r)
	if err != nil || strings.TrimSpace(input.Folder) == "" || len(input.Folder) > 512 {
		return platform.Fail(400, "invalid_input")
	}
	_, ext := h.s.EnsurePlugins()
	ov, mErr := ext.Configure(r.Context(), input.Folder, nil, nil)
	if mErr != nil {
		return platform.Fail(400, "invalid_input")
	}
	slog.Info("local plugin folder mounted", "trace_id", platform.Meta(r.Context()).Trace, "folder", input.Folder)
	platform.WriteJSON(w, 200, ov)
	return nil
}

func (h *handler) adminRemoveExtPlugin(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		return platform.Fail(400, "invalid_input")
	}
	_, ext := h.s.EnsurePlugins()
	ov, err := ext.Remove(id)
	if err != nil {
		return platform.Fail(400, "invalid_input")
	}
	platform.WriteJSON(w, 200, ov)
	return nil
}

func (h *handler) adminLoadPluginPage(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
	id := strings.TrimSpace(r.PathValue("id"))
	input, err := platform.DecodeBody[pluginPageInput](r)
	if err != nil || id == "" {
		return platform.Fail(400, "invalid_input")
	}
	_, ext := h.s.EnsurePlugins()
	bundle, lErr := ext.LoadSandboxedPage(id, input.Theme, input.CSSVars)
	if lErr != nil {
		return platform.Fail(404, "not_found")
	}
	platform.WriteJSON(w, 200, bundle)
	return nil
}

func (h *handler) adminInvokePluginTool(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
	id := strings.TrimSpace(r.PathValue("id"))
	input, err := platform.DecodeBody[struct {
		Tool   string            `json:"tool"`
		Params map[string]string `json:"params,omitempty"`
	}](r)
	if err != nil || id == "" || strings.TrimSpace(input.Tool) == "" {
		return platform.Fail(400, "invalid_input")
	}
	_, ext := h.s.EnsurePlugins()
	target, ok := ext.FindReady(id)
	if !ok {
		return platform.Fail(400, "not_allowed")
	}
	slog.Info("plugin tool invoked", "trace_id", platform.Meta(r.Context()).Trace, "plugin_id", target.ID, "tool", input.Tool)
	platform.WriteJSON(w, 200, struct {
		PluginID string `json:"plugin_id"`
		Tool     string `json:"tool"`
		Status   string `json:"status"`
	}{target.ID, input.Tool, "accepted"})
	return nil
}
