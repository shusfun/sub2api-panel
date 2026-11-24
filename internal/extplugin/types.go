package extplugin

import (
	"regexp"
	"sync"
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
