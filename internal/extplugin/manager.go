package extplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

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
