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
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
