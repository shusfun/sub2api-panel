package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sub2api-panel/internal/extplugin"
	"sub2api-panel/internal/pluginrt"
	"sub2api-panel/plugins/all"
	patrolplugin "sub2api-panel/plugins/patrol"
	"sub2api-panel/plugins/probelmfpd"
)

type mountedPluginView struct {
	PluginID string `json:"plugin_id"`
	Title    string `json:"title"`
	Mount    string `json:"mount"`
}

func (p *panel) ensurePlugins() (*pluginrt.Runtime, *extplugin.Manager) {
	p.runtimeMu.Lock()
	defer p.runtimeMu.Unlock()
	if p.pluginRT == nil {
		dir := strings.TrimSpace(p.dataDir)
		if dir == "" {
			dir = strings.TrimSpace(os.Getenv("PANEL_DATA_DIR"))
		}
		if dir == "" {
			dir = filepath.Join(os.TempDir(), "sub2api-panel-data")
		}
		p.dataDir = dir
		p.pluginRT = pluginrt.New(dir, all.Manifests(), nil)
	}
	if p.extPlugins == nil {
		p.extPlugins = extplugin.NewManager(p.dataDir, os.Getenv("PANEL_PLUGIN_MARKET"))
	}
	return p.pluginRT, p.extPlugins
}

func (p *panel) patrolVisibility() (enabled, homeVisible, userVisible, guestEnabled bool, retentionDays int) {
	rt, _ := p.ensurePlugins()
	on, cfg := rt.ConfigFor(patrolplugin.PatrolPluginID)
	home, _ := cfg["home_visible"].(bool)
	userVis, _ := cfg["user_visible"].(bool)
	ret, _ := cfg["retention_days"].(int)
	if ret < 1 {
		ret = 30
	}
	guestOn, _ := rt.ConfigFor(patrolplugin.GuestPluginID)
	return on, on && home, on && userVis, guestOn, ret
}

func (p *panel) lmfpdService() *probelmfpd.Service {
	rt, _ := p.ensurePlugins()
	svc, _ := rt.Service(probelmfpd.ServiceName).(*probelmfpd.Service)
	return svc
}

func (p *panel) mountedPlugins(includeAdmin bool) []mountedPluginView {
	_, ext := p.ensurePlugins()
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

func (p *panel) pluginRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api/admin/plugins/upstream", true, p.officialPluginList)
	p.route(mux, "POST /api/admin/plugins/upstream/upload/authorize", true, p.officialPluginAuthorizeUpload)
	p.routeWithLimit(mux, "POST /api/admin/plugins/upstream/upload", true, officialPluginUploadLimit, p.officialPluginUpload)
	p.route(mux, "POST /api/admin/plugins/upstream/{id}/{action}", true, p.officialPluginAction)
	p.route(mux, "GET /api/admin/plugins/upstream/{id}/config", true, p.officialPluginConfig)
	p.route(mux, "PUT /api/admin/plugins/upstream/{id}/config", true, p.officialPluginConfig)
	p.route(mux, "DELETE /api/admin/plugins/upstream/{id}", true, p.officialPluginRemove)
	p.public(mux, "GET /api/plugins/mounted", p.publicMountedPlugins)
	p.public(mux, "POST /api/plugins/{id}/public-page", p.loadPublicPluginPage)
	p.public(mux, "GET /api/plugins/frame/{id}", p.servePluginFrame)
	p.route(mux, "GET /api/plugins/user", false, p.userMountedPlugins)
	p.route(mux, "POST /api/plugins/{id}/user-page", false, p.loadUserPluginPage)

	p.route(mux, "GET /api/admin/plugins", true, p.adminPluginsOverview)
	p.route(mux, "PUT /api/admin/plugins/builtin/{id}", true, p.adminUpdateBuiltinPlugin)
	p.route(mux, "POST /api/admin/plugins/market/refresh", true, p.adminRefreshPluginMarket)
	p.route(mux, "PUT /api/admin/plugins/external/{id}", true, p.adminConfigureExtPlugin)
	p.route(mux, "POST /api/admin/plugins/external/folder", true, p.adminMountPluginFolder)
	p.route(mux, "DELETE /api/admin/plugins/external/{id}", true, p.adminRemoveExtPlugin)
	p.route(mux, "POST /api/admin/plugins/external/{id}/page", true, p.adminLoadPluginPage)
	p.route(mux, "POST /api/admin/plugins/{id}/invoke", true, p.adminInvokePluginTool)
}

func (p *panel) publicMountedPlugins(w http.ResponseWriter, _ *http.Request) error {
	items := make([]mountedPluginView, 0)
	for _, m := range p.mountedPlugins(false) {
		if m.Mount == "home" || m.Mount == "patrol" {
			items = append(items, m)
		}
	}
	writeJSON(w, 200, struct {
		Items []mountedPluginView `json:"items"`
	}{items})
	return nil
}

func (p *panel) userMountedPlugins(w http.ResponseWriter, _ *http.Request, a sessionAuth) error {
	writeJSON(w, 200, struct {
		Items []mountedPluginView `json:"items"`
	}{p.mountedPlugins(a.user.Role == "admin")})
	return nil
}

type pluginPageInput struct {
	Theme   string            `json:"theme,omitempty"`
	CSSVars map[string]string `json:"css_vars,omitempty"`
}

func (p *panel) loadPublicPluginPage(w http.ResponseWriter, r *http.Request) error {
	id := strings.TrimSpace(r.PathValue("id"))
	input, err := decodeBody[pluginPageInput](r)
	if err != nil {
		return err
	}
	_, ext := p.ensurePlugins()
	bundle, err := ext.LoadSandboxedPage(id, input.Theme, input.CSSVars)
	if err != nil || (bundle.Mount != "home" && bundle.Mount != "patrol") {
		return fail(404, "not_found")
	}
	writeJSON(w, 200, bundle)
	return nil
}

func (p *panel) servePluginFrame(w http.ResponseWriter, r *http.Request) error {
	id := strings.TrimSpace(r.PathValue("id"))
	theme := r.URL.Query().Get("theme")
	_, ext := p.ensurePlugins()
	bundle, err := ext.LoadSandboxedPage(id, theme, nil)
	if err != nil {
		return fail(404, "not_found")
	}
	if bundle.Mount == "nav:admin" {
		active, aErr := p.sessions.authenticate(r, p.origin, time.Now())
		if aErr != nil {
			return aErr
		}
		user, _, pErr := p.sessions.principal(r.Context(), p.upstream, active)
		if pErr != nil || user.Role != "admin" {
			return fail(403, "not_allowed")
		}
	} else if bundle.Mount == "nav:user" {
		if _, aErr := p.sessions.authenticate(r, p.origin, time.Now()); aErr != nil {
			return aErr
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	_, _ = w.Write([]byte(bundle.HTML))
	return nil
}

func (p *panel) loadUserPluginPage(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id := strings.TrimSpace(r.PathValue("id"))
	input, err := decodeBody[pluginPageInput](r)
	if err != nil {
		return err
	}
	_, ext := p.ensurePlugins()
	bundle, err := ext.LoadSandboxedPage(id, input.Theme, input.CSSVars)
	if err != nil {
		return fail(404, "not_found")
	}
	if bundle.Mount == "nav:admin" && a.user.Role != "admin" {
		return fail(403, "not_allowed")
	}
	writeJSON(w, 200, bundle)
	return nil
}

func (p *panel) adminPluginsOverview(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	rt, ext := p.ensurePlugins()
	upstreamEnabled := false
	if s, err := p.settings(r.Context()); err == nil {
		upstreamEnabled = s.PluginManagement
	}
	writeJSON(w, 200, struct {
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

func (p *panel) adminUpdateBuiltinPlugin(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	id := strings.TrimSpace(r.PathValue("id"))
	input, err := decodeBody[struct {
		Enabled *bool          `json:"enabled,omitempty"`
		Config  map[string]any `json:"config,omitempty"`
	}](r)
	if err != nil || id == "" {
		return fail(400, "invalid_input")
	}
	rt, _ := p.ensurePlugins()
	states, uErr := rt.Update(id, input.Enabled, input.Config)
	if uErr != nil {
		return fail(400, "invalid_input")
	}
	slog.Info("builtin plugin updated", "trace_id", meta(r.Context()).Trace, "plugin_id", id)
	writeJSON(w, 200, struct {
		Builtin []pluginrt.State `json:"builtin"`
	}{states})
	return nil
}

func (p *panel) adminRefreshPluginMarket(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	input, err := decodeBody[struct {
		MarketURL string `json:"market_url,omitempty"`
	}](r)
	if err != nil {
		return err
	}
	_, ext := p.ensurePlugins()
	ov, rErr := ext.RefreshMarket(r.Context(), input.MarketURL)
	if rErr != nil {
		return fail(400, "invalid_input")
	}
	writeJSON(w, 200, ov)
	return nil
}

func (p *panel) adminConfigureExtPlugin(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	id := strings.TrimSpace(r.PathValue("id"))
	input, err := decodeBody[struct {
		Enabled *bool             `json:"enabled,omitempty"`
		Options map[string]string `json:"options,omitempty"`
	}](r)
	if err != nil || id == "" {
		return fail(400, "invalid_input")
	}
	_, ext := p.ensurePlugins()
	ov, cErr := ext.Configure(r.Context(), id, input.Enabled, input.Options)
	if cErr != nil {
		return fail(400, "invalid_input")
	}
	slog.Info("external plugin configured", "trace_id", meta(r.Context()).Trace, "plugin_id", id)
	writeJSON(w, 200, ov)
	return nil
}

func (p *panel) adminMountPluginFolder(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	input, err := decodeBody[struct {
		Folder string `json:"folder"`
	}](r)
	if err != nil || strings.TrimSpace(input.Folder) == "" || len(input.Folder) > 512 {
		return fail(400, "invalid_input")
	}
	_, ext := p.ensurePlugins()
	ov, mErr := ext.Configure(r.Context(), input.Folder, nil, nil)
	if mErr != nil {
		return fail(400, "invalid_input")
	}
	slog.Info("local plugin folder mounted", "trace_id", meta(r.Context()).Trace, "folder", input.Folder)
	writeJSON(w, 200, ov)
	return nil
}

func (p *panel) adminRemoveExtPlugin(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		return fail(400, "invalid_input")
	}
	_, ext := p.ensurePlugins()
	ov, err := ext.Remove(id)
	if err != nil {
		return fail(400, "invalid_input")
	}
	writeJSON(w, 200, ov)
	return nil
}

func (p *panel) adminLoadPluginPage(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	id := strings.TrimSpace(r.PathValue("id"))
	input, err := decodeBody[pluginPageInput](r)
	if err != nil || id == "" {
		return fail(400, "invalid_input")
	}
	_, ext := p.ensurePlugins()
	bundle, lErr := ext.LoadSandboxedPage(id, input.Theme, input.CSSVars)
	if lErr != nil {
		return fail(404, "not_found")
	}
	writeJSON(w, 200, bundle)
	return nil
}

func (p *panel) adminInvokePluginTool(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	id := strings.TrimSpace(r.PathValue("id"))
	input, err := decodeBody[struct {
		Tool   string            `json:"tool"`
		Params map[string]string `json:"params,omitempty"`
	}](r)
	if err != nil || id == "" || strings.TrimSpace(input.Tool) == "" {
		return fail(400, "invalid_input")
	}
	_, ext := p.ensurePlugins()
	target, ok := ext.FindReady(id)
	if !ok {
		return fail(400, "not_allowed")
	}
	slog.Info("plugin tool invoked", "trace_id", meta(r.Context()).Trace, "plugin_id", target.ID, "tool", input.Tool)
	writeJSON(w, 200, struct {
		PluginID string `json:"plugin_id"`
		Tool     string `json:"tool"`
		Status   string `json:"status"`
	}{target.ID, input.Tool, "accepted"})
	return nil
}
