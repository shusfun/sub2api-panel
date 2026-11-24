package extplugin

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

var validPluginID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type ProbeSpec struct {
	ID       string   `toml:"id" json:"id"`
	Title    string   `toml:"title" json:"title"`
	Protocol string   `toml:"protocol" json:"protocol"` // gpt-inspector-sse | http-json
	Presets  []string `toml:"presets,omitempty" json:"presets,omitempty"`
}

type PageSpec struct {
	Title string `toml:"title" json:"title"`
	File  string `toml:"file,omitempty" json:"file,omitempty"`
	URL   string `toml:"url,omitempty" json:"url,omitempty"`
	Mount string `toml:"mount" json:"mount"` // home | nav:admin | nav:user | patrol
}

type ActionSpec struct {
	ID     string `toml:"id" json:"id"`
	Target string `toml:"target" json:"target"` // account | group | key | ops-error
	Title  string `toml:"title" json:"title"`
}

type ToolSpec struct {
	Name        string `toml:"name" json:"name"`
	Description string `toml:"description" json:"description"`
}

type OptionSpec struct {
	Key         string `toml:"key" json:"key"`
	Title       string `toml:"title" json:"title"`
	Placeholder string `toml:"placeholder,omitempty" json:"placeholder,omitempty"`
	Default     string `toml:"default,omitempty" json:"default,omitempty"`
}

type CatalogEntry struct {
	ID           string       `toml:"id" json:"id"`
	Name         string       `toml:"name" json:"name"`
	Publisher    string       `toml:"publisher" json:"publisher"`
	Summary      string       `toml:"summary" json:"summary"`
	Category     string       `toml:"category" json:"category"` // verifier | page | ops | tools
	License      string       `toml:"license" json:"license"`
	Homepage     string       `toml:"homepage,omitempty" json:"homepage,omitempty"`
	Repo         string       `toml:"repo,omitempty" json:"repo,omitempty"`
	Version      string       `toml:"version,omitempty" json:"version,omitempty"`
	PackageURL   string       `toml:"package_url,omitempty" json:"package_url,omitempty"`
	PackageSHA   string       `toml:"package_sha256,omitempty" json:"package_sha256,omitempty"`
	Permissions  []string     `toml:"permissions,omitempty" json:"permissions,omitempty"`
	OptionSpecs  []OptionSpec `toml:"option_specs,omitempty" json:"option_specs,omitempty"`
	Probes       []ProbeSpec  `toml:"probes,omitempty" json:"probes,omitempty"`
	Page         *PageSpec    `toml:"page,omitempty" json:"page,omitempty"`
	Actions      []ActionSpec `toml:"actions,omitempty" json:"actions,omitempty"`
	Tools        []ToolSpec   `toml:"tools,omitempty" json:"tools,omitempty"`
	OfficialSeed bool         `toml:"-" json:"official_seed"`
}

type InstalledRecord struct {
	Source   string            `toml:"source" json:"source"`
	LocalDir string            `toml:"local_dir,omitempty" json:"local_dir,omitempty"`
	Enabled  bool              `toml:"enabled" json:"enabled"`
	Options  map[string]string `toml:"options,omitempty" json:"options,omitempty"`
}

type storeFile struct {
	Schema    int                        `toml:"schema"`
	MarketURL string                     `toml:"market_url,omitempty"`
	Installed map[string]InstalledRecord `toml:"installed,omitempty"`
}

type PluginView struct {
	CatalogEntry
	Installed  bool              `json:"installed"`
	Enabled    bool              `json:"enabled"`
	Ready      bool              `json:"ready"`
	StatusNote string            `json:"status_note,omitempty"`
	Source     string            `json:"source,omitempty"`
	Options    map[string]string `json:"options"`
}

type MarketOverview struct {
	MarketURL string       `json:"market_url"`
	Plugins   []PluginView `json:"plugins"`
}

type SandboxedPageBundle struct {
	PluginID    string   `json:"plugin_id"`
	Title       string   `json:"title"`
	Mount       string   `json:"mount"`
	HTML        string   `json:"html"`
	Permissions []string `json:"permissions"`
}

type Manager struct {
	dataDir    string
	seedMarket string
	mu         sync.RWMutex
	remote     []CatalogEntry
}

func NewManager(dataDir, seedMarket string) *Manager {
	if dataDir == "" {
		dataDir = "./data"
	}
	return &Manager{dataDir: dataDir, seedMarket: strings.TrimSpace(seedMarket)}
}

func (m *Manager) configPath() string {
	return filepath.Join(m.dataDir, "managed", "ext-plugins.toml")
}

func (m *Manager) pluginsRoot() string {
	return filepath.Join(m.dataDir, "ext-plugins")
}

func builtinCatalog() []CatalogEntry {
	return []CatalogEntry{
		{
			ID:           "gpt-relay-inspector",
			Name:         "少点套路 · 号池黑盒取证引擎",
			Publisher:    "gpttest.xyz",
			Summary:      "零侵入对接少点套路 19 针原子黑盒取证（SSE 假首字鉴伪、模型跨档偷换识别、System 金丝雀与多轮防偷裁）。调用时由宿主自动签发 10 分钟限额临时 Key 测完即焚。",
			Category:     "verifier",
			License:      "MIT",
			Homepage:     "https://gpttest.xyz",
			Version:      "2.1.0",
			Permissions:  []string{"probe:ephemeral-key", "models:catalog"},
			OfficialSeed: true,
			OptionSpecs: []OptionSpec{
				{Key: "endpoint", Title: "取证服务端点", Placeholder: "https://gpttest.xyz 或 http://127.0.0.1:4399", Default: "https://gpttest.xyz"},
				{Key: "public_gateway_url", Title: "对外回调网关地址", Placeholder: "供外部节点反向探测的公开 /v1 地址（自建本机节点可留空）", Default: ""},
			},
			Probes: []ProbeSpec{
				{ID: "inspect", Title: "少点套路黑盒取证", Protocol: "gpt-inspector-sse", Presets: []string{"fast", "full"}},
			},
			Actions: []ActionSpec{
				{ID: "deep-inspect-account", Target: "account", Title: "用少点套路验号"},
				{ID: "deep-inspect-group", Target: "group", Title: "用少点套路巡检此分组"},
			},
			Tools: []ToolSpec{
				{Name: "inspect", Description: "对指定分组或单账号隔离沙箱执行少点套路黑盒取证"},
			},
		},
		{
			ID:           "lm-detector-companion",
			Name:         "Fingerpoint 外部对照参考台",
			Publisher:    "Ikaleio",
			Summary:      "为账号与分组测试生成一次性限额临时通行证并快速对照外部 Fingerpoint Detector 样本库，零长期密钥暴露。",
			Category:     "verifier",
			License:      "MIT",
			Homepage:     "https://lm.ikale.io",
			Repo:         "Ikaleio/lm-detector",
			Version:      "0.2.0",
			Permissions:  []string{"probe:ephemeral-key", "models:catalog"},
			OfficialSeed: true,
			OptionSpecs: []OptionSpec{
				{Key: "web_url", Title: "对照台地址", Placeholder: "https://lm.ikale.io/#/?mode=api", Default: "https://lm.ikale.io/#/?mode=api"},
			},
			Actions: []ActionSpec{
				{ID: "open-ikale-pass", Target: "account", Title: "签发短效通行证去 LM Detector"},
				{ID: "open-ikale-group", Target: "group", Title: "签发分组通行证去 LM Detector"},
			},
		},
	}
}

func (m *Manager) loadState() storeFile {
	st := storeFile{
		Schema:    1,
		MarketURL: m.seedMarket,
		Installed: map[string]InstalledRecord{
			"gpt-relay-inspector": {
				Source:  "seed:gpt-relay-inspector",
				Enabled: true,
				Options: map[string]string{"endpoint": "https://gpttest.xyz"},
			},
		},
	}
	raw, err := os.ReadFile(m.configPath())
	if err != nil {
		return st
	}
	var parsed storeFile
	dec := toml.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&parsed) == nil && parsed.Schema == 1 {
		if parsed.Installed == nil {
			parsed.Installed = map[string]InstalledRecord{}
		}
		if parsed.MarketURL == "" && m.seedMarket != "" {
			parsed.MarketURL = m.seedMarket
		}
		return parsed
	}
	return st
}

func (m *Manager) saveState(next storeFile) error {
	next.Schema = 1
	raw, err := toml.Marshal(next)
	if err != nil {
		return err
	}
	dst := m.configPath()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func ValidEndpointURL(raw string, allowLoopback bool) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	if allowLoopback && u.Scheme == "http" {
		h := u.Hostname()
		return h == "127.0.0.1" || h == "localhost" || h == "::1"
	}
	return false
}

func (m *Manager) Overview() MarketOverview {
	st := m.loadState()
	catalog := builtinCatalog()

	m.mu.RLock()
	extra := append([]CatalogEntry(nil), m.remote...)
	m.mu.RUnlock()

	byID := map[string]CatalogEntry{}
	var order []string
	for _, c := range catalog {
		byID[c.ID] = c
		order = append(order, c.ID)
	}
	for _, c := range extra {
		if !validPluginID.MatchString(c.ID) {
			continue
		}
		if _, exists := byID[c.ID]; !exists {
			order = append(order, c.ID)
		}
		byID[c.ID] = c
	}
	for id, rec := range st.Installed {
		if _, exists := byID[id]; exists || rec.LocalDir == "" {
			continue
		}
		if manifest, err := loadFolderManifest(rec.LocalDir); err == nil {
			byID[id] = manifest
			order = append(order, id)
		}
	}

	views := make([]PluginView, 0, len(order))
	for _, id := range order {
		entry := byID[id]
		rec, hasRec := st.Installed[id]
		opts := map[string]string{}
		for _, spec := range entry.OptionSpecs {
			if spec.Default != "" {
				opts[spec.Key] = spec.Default
			}
		}
		for k, v := range rec.Options {
			opts[k] = v
		}
		ready := rec.Enabled
		note := "未启用"
		if rec.Enabled {
			note = "已启用"
			if ep, hasEP := opts["endpoint"]; hasEP && !ValidEndpointURL(ep, true) {
				ready = false
				note = "端点地址无效"
			} else if entry.Page != nil && entry.Page.File != "" && rec.LocalDir == "" {
				ready = false
				note = "缺少本地沙箱页面目录"
			} else if ep != "" {
				note = "已连接 · " + ep
			} else if entry.Page != nil {
				note = "沙箱视图已挂载"
			}
		}
		if entry.Permissions == nil {
			entry.Permissions = []string{}
		}
		if entry.OptionSpecs == nil {
			entry.OptionSpecs = []OptionSpec{}
		}
		if entry.Probes == nil {
			entry.Probes = []ProbeSpec{}
		}
		if entry.Actions == nil {
			entry.Actions = []ActionSpec{}
		}
		if entry.Tools == nil {
			entry.Tools = []ToolSpec{}
		}
		views = append(views, PluginView{
			CatalogEntry: entry,
			Installed:    hasRec || entry.OfficialSeed,
			Enabled:      rec.Enabled,
			Ready:        ready,
			StatusNote:   note,
			Source:       rec.Source,
			Options:      opts,
		})
	}
	return MarketOverview{
		MarketURL: st.MarketURL,
		Plugins:   views,
	}
}

func (m *Manager) FindReady(id string) (PluginView, bool) {
	for _, p := range m.Overview().Plugins {
		if p.ID == id && p.Enabled && p.Ready {
			return p, true
		}
	}
	return PluginView{}, false
}

func (m *Manager) RefreshMarket(ctx context.Context, customURL string) (MarketOverview, error) {
	st := m.loadState()
	trimmed := strings.TrimSpace(customURL)
	if trimmed == "default" {
		st.MarketURL = ""
		if err := m.saveState(st); err != nil {
			return m.Overview(), err
		}
	} else if trimmed != "" {
		if !ValidEndpointURL(trimmed, true) {
			return m.Overview(), errors.New("invalid_market_url")
		}
		st.MarketURL = trimmed
		if err := m.saveState(st); err != nil {
			return m.Overview(), err
		}
	}
	if st.MarketURL != "" {
		cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(cctx, http.MethodGet, st.MarketURL, nil)
		if err != nil {
			return m.Overview(), err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return m.Overview(), err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return m.Overview(), fmt.Errorf("market returned status %d", resp.StatusCode)
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		if err != nil {
			return m.Overview(), err
		}
		var remoteList []CatalogEntry
		if err := json.Unmarshal(raw, &remoteList); err != nil {
			return m.Overview(), errors.New("invalid_market_manifest")
		}
		m.mu.Lock()
		m.remote = remoteList
		m.mu.Unlock()
	}
	return m.Overview(), nil
}

func (m *Manager) Configure(ctx context.Context, idOrFolder string, enabled *bool, options map[string]string) (MarketOverview, error) {
	trimmed := strings.TrimSpace(idOrFolder)
	if trimmed == "" {
		return m.Overview(), errors.New("invalid_plugin_id")
	}
	st := m.loadState()

	// 本地受控目录挂载（必须位于 pluginsRoot 下或显式存在的插件包目录）
	if !validPluginID.MatchString(trimmed) {
		absDir, err := filepath.Abs(trimmed)
		if err != nil {
			return m.Overview(), errors.New("invalid_plugin_dir")
		}
		manifest, err := loadFolderManifest(absDir)
		if err != nil {
			return m.Overview(), err
		}
		st.Installed[manifest.ID] = InstalledRecord{
			Source:   "folder:" + filepath.Base(absDir),
			LocalDir: absDir,
			Enabled:  true,
			Options:  options,
		}
		if err := m.saveState(st); err != nil {
			return m.Overview(), err
		}
		return m.Overview(), nil
	}

	ov := m.Overview()
	var target *PluginView
	for i := range ov.Plugins {
		if ov.Plugins[i].ID == trimmed {
			target = &ov.Plugins[i]
			break
		}
	}
	if target == nil {
		return ov, errors.New("plugin_not_found")
	}

	rec := st.Installed[target.ID]
	if rec.Source == "" {
		rec.Source = "market:" + target.ID
	}
	if enabled != nil {
		rec.Enabled = *enabled
	} else {
		rec.Enabled = true
	}
	if options != nil {
		if rec.Options == nil {
			rec.Options = map[string]string{}
		}
		allowed := map[string]bool{}
		for _, spec := range target.OptionSpecs {
			allowed[spec.Key] = true
		}
		for k, v := range options {
			if !allowed[k] || len(v) > 1024 {
				return ov, errors.New("invalid_plugin_option")
			}
			v = strings.TrimSpace(v)
			if (k == "endpoint" || k == "public_gateway_url" || k == "web_url") && v != "" && !strings.HasPrefix(v, "https://") && !strings.HasPrefix(v, "http://127.0.0.1") && !strings.HasPrefix(v, "http://localhost") {
				return ov, errors.New("invalid_plugin_url")
			}
			rec.Options[k] = v
		}
	}
	if rec.Enabled && target.PackageURL != "" && rec.LocalDir == "" {
		dir, err := m.downloadAndVerifyZip(ctx, target.ID, target.PackageURL, target.PackageSHA)
		if err != nil {
			return ov, err
		}
		rec.LocalDir = dir
	}

	st.Installed[target.ID] = rec
	if err := m.saveState(st); err != nil {
		return ov, err
	}
	return m.Overview(), nil
}

func (m *Manager) Remove(id string) (MarketOverview, error) {
	st := m.loadState()
	delete(st.Installed, strings.TrimSpace(id))
	if err := m.saveState(st); err != nil {
		return m.Overview(), err
	}
	return m.Overview(), nil
}

func (m *Manager) LoadSandboxedPage(pluginID, theme string, cssVars map[string]string) (SandboxedPageBundle, error) {
	st := m.loadState()
	rec, ok := st.Installed[pluginID]
	if !ok || !rec.Enabled || rec.LocalDir == "" {
		return SandboxedPageBundle{}, errors.New("plugin_page_unavailable")
	}
	manifest, err := loadFolderManifest(rec.LocalDir)
	if err != nil || manifest.Page == nil || manifest.Page.File == "" {
		return SandboxedPageBundle{}, errors.New("plugin_page_missing")
	}
	cleanRel := filepath.Clean(filepath.FromSlash(manifest.Page.File))
	if strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
		return SandboxedPageBundle{}, errors.New("plugin_path_escape")
	}
	fullPath := filepath.Join(rec.LocalDir, cleanRel)
	rawHTML, err := os.ReadFile(fullPath)
	if err != nil {
		return SandboxedPageBundle{}, err
	}
	bridge := buildPanelBridgeHead(pluginID, theme, manifest.Permissions, cssVars)
	htmlStr := string(rawHTML)
	if idx := strings.Index(strings.ToLower(htmlStr), "<head>"); idx >= 0 {
		htmlStr = htmlStr[:idx+6] + bridge + htmlStr[idx+6:]
	} else {
		htmlStr = bridge + htmlStr
	}
	return SandboxedPageBundle{
		PluginID:    pluginID,
		Title:       manifest.Page.Title,
		Mount:       manifest.Page.Mount,
		HTML:        htmlStr,
		Permissions: manifest.Permissions,
	}, nil
}

func buildPanelBridgeHead(pluginID, theme string, perms []string, cssVars map[string]string) string {
	permJSON, _ := json.Marshal(perms)
	safeTheme := "light"
	if theme == "dark" {
		safeTheme = "dark"
	}
	var varLines strings.Builder
	for k, v := range cssVars {
		if strings.HasPrefix(k, "--weir-") && !strings.ContainsAny(v, ";{}<>") {
			varLines.WriteString(k)
			varLines.WriteByte(':')
			varLines.WriteString(v)
			varLines.WriteByte(';')
		}
	}
	fg := "#232323"
	if safeTheme == "dark" {
		fg = "#ececef"
	}
	return fmt.Sprintf(`<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:; script-src 'unsafe-inline';">
<style>:root{%s}:where(html,body){margin:0;padding:13px;font-family:var(--weir-font,system-ui,sans-serif);background:transparent;color:%s;}</style>
<script>
window.sub2apiPanel = Object.freeze({
  pluginId: %q,
  theme: %q,
  permissions: %s,
  ready: Promise.resolve({ dark: %t }),
  read: function(scope, params) {
    return new Promise(function(resolve, reject) {
      var reqId = Math.random().toString(36).slice(2);
      function onMsg(ev) {
        if (!ev.data || ev.data.panelReply !== reqId) return;
        window.removeEventListener("message", onMsg);
        if (ev.data.error) reject(new Error(ev.data.error));
        else resolve(ev.data.result);
      }
      window.addEventListener("message", onMsg);
      parent.postMessage({ panelPluginRead: scope, pluginId: %q, reqId: reqId, params: params || {} }, "*");
    });
  }
});
</script>`, varLines.String(), fg, pluginID, safeTheme, string(permJSON), safeTheme == "dark", pluginID)
}

func loadFolderManifest(dir string) (CatalogEntry, error) {
	if raw, err := os.ReadFile(filepath.Join(dir, "panel-plugin.json")); err == nil {
		var entry CatalogEntry
		if err := json.Unmarshal(raw, &entry); err != nil || !validPluginID.MatchString(entry.ID) {
			return CatalogEntry{}, errors.New("invalid_panel_plugin_json")
		}
		if entry.Name == "" {
			entry.Name = entry.ID
		}
		return entry, nil
	}
	rawPkg, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return CatalogEntry{}, errors.New("missing_plugin_manifest")
	}
	var pkg struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
		License     string `json:"license"`
		Panel       struct {
			ID          string       `json:"id"`
			Title       string       `json:"title"`
			Category    string       `json:"category"`
			Permissions []string     `json:"permissions"`
			OptionSpecs []OptionSpec `json:"option_specs"`
			Probes      []ProbeSpec  `json:"probes"`
			Page        *PageSpec    `json:"page"`
			Actions     []ActionSpec `json:"actions"`
			Tools       []ToolSpec   `json:"tools"`
		} `json:"sub2apiPanel"`
	}
	if err := json.Unmarshal(rawPkg, &pkg); err != nil {
		return CatalogEntry{}, errors.New("invalid_package_json")
	}
	id := pkg.Panel.ID
	if id == "" {
		id = strings.TrimPrefix(pkg.Name, "@sub2api-panel/")
	}
	if !validPluginID.MatchString(id) {
		return CatalogEntry{}, errors.New("invalid_plugin_id")
	}
	title := pkg.Panel.Title
	if title == "" {
		title = id
	}
	cat := pkg.Panel.Category
	if cat == "" {
		cat = "page"
	}
	return CatalogEntry{
		ID:          id,
		Name:        title,
		Publisher:   "local",
		Summary:     pkg.Description,
		Category:    cat,
		License:     pkg.License,
		Version:     pkg.Version,
		Permissions: pkg.Panel.Permissions,
		OptionSpecs: pkg.Panel.OptionSpecs,
		Probes:      pkg.Panel.Probes,
		Page:        pkg.Panel.Page,
		Actions:     pkg.Panel.Actions,
		Tools:       pkg.Panel.Tools,
	}, nil
}

func (m *Manager) downloadAndVerifyZip(ctx context.Context, pluginID, zipURL, expectedSHA string) (string, error) {
	if !ValidEndpointURL(zipURL, true) || len(expectedSHA) != 64 {
		return "", errors.New("plugin_sha256_required")
	}
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, zipURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("package download status %d", resp.StatusCode)
	}
	zipBytes, err := io.ReadAll(io.LimitReader(resp.Body, 40<<20))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(zipBytes)
	if hex.EncodeToString(sum[:]) != strings.ToLower(expectedSHA) {
		return "", errors.New("plugin_sha256_mismatch")
	}
	destDir := filepath.Join(m.pluginsRoot(), pluginID)
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return "", err
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return "", err
	}
	for _, f := range zr.File {
		clean := filepath.Clean(filepath.FromSlash(f.Name))
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			continue
		}
		outPath := filepath.Join(destDir, clean)
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(outPath, 0o700)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(outPath), 0o700)
		rc, oErr := f.Open()
		if oErr != nil {
			continue
		}
		data, rErr := io.ReadAll(io.LimitReader(rc, 16<<20))
		_ = rc.Close()
		if rErr == nil {
			_ = os.WriteFile(outPath, data, 0o600)
		}
	}
	return destDir, nil
}
