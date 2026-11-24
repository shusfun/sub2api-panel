package system

import (
	"encoding/json"
	"fmt"
)

type SettingOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type SettingColumn struct {
	Key     string          `json:"key"`
	Kind    string          `json:"kind"`
	Label   string          `json:"label"`
	Options []SettingOption `json:"options,omitempty"`
	// Secret 列：值只写不读，Configured 是上游表示「已配置」的布尔字段。
	Configured string `json:"-"`
}

type SettingField struct {
	Key     string          `json:"key"`
	Kind    string          `json:"kind"`
	Label   string          `json:"label"`
	Hint    string          `json:"hint,omitempty"`
	Min     *float64        `json:"min,omitempty"`
	Max     *float64        `json:"max,omitempty"`
	Options []SettingOption `json:"options,omitempty"`
	When    string          `json:"when,omitempty"`
	Fixed   bool            `json:"read_only,omitempty"`
	Columns []SettingColumn `json:"columns,omitempty"`

	Value      json.RawMessage `json:"value,omitempty"`
	Configured *bool           `json:"configured,omitempty"`

	source     string // 空表示 /admin/settings，否则是 SettingSources 的键
	name       string // 上游字段名
	configured string // secret：上游的 *_configured 字段名
	wrap       string // rows：上游把数组包在这个键下
}

type SettingGroup struct {
	Title  string         `json:"title,omitempty"`
	Hint   string         `json:"hint,omitempty"`
	When   string         `json:"when,omitempty"`
	Fields []SettingField `json:"fields"`
}

type SettingSection struct {
	ID     string         `json:"id"`
	Title  string         `json:"title"`
	Hint   string         `json:"hint,omitempty"`
	StepUp bool           `json:"step_up"`
	Groups []SettingGroup `json:"groups,omitempty"`
}

// 子接口：各自整体读写，面板读出后合并本次改动再写回。
var SettingSources = map[string]string{
	"blacklist":       "/admin/settings/registration-email-blacklist",
	"console_limit":   "/admin/settings/panel-rate-limit",
	"overload":        "/admin/settings/overload-cooldown",
	"cooldown_429":    "/admin/settings/rate-limit-429-cooldown",
	"images_cooldown": "/admin/settings/openai-images-oauth-unavailable-cooldown",
	"stream_timeout":  "/admin/settings/stream-timeout",
	"rectifier":       "/admin/settings/rectifier",
	"beta":            "/admin/settings/beta-policy",
	"web_search":      "/admin/settings/web-search-emulation",
}

func findSettingSection(id string) (SettingSection, bool) {
	for _, section := range SettingCatalog {
		if section.ID == id {
			return section, true
		}
	}
	return SettingSection{}, false
}

func (section SettingSection) sources() []string {
	seen := map[string]bool{}
	var list []string
	for _, group := range section.Groups {
		for _, field := range group.Fields {
			if !seen[field.source] {
				seen[field.source] = true
				list = append(list, field.source)
			}
		}
	}
	return list
}

// 以下是目录的书写辅助。

func limit(value float64) *float64 { return &value }

func (f SettingField) when(key string) SettingField { f.When = key; return f }
func (f SettingField) from(source string) SettingField {
	f.source = source
	f.Key = source + "." + f.name
	return f
}
func (f SettingField) inside(key string) SettingField { f.wrap = key; return f }
func (f SettingField) fixed() SettingField            { f.Fixed = true; return f }
func (f SettingField) between(min, max float64) SettingField {
	f.Min, f.Max = limit(min), limit(max)
	return f
}

func field(kind, key, label, hint string) SettingField {
	return SettingField{Key: key, name: key, Kind: kind, Label: label, Hint: hint}
}
func toggle(key, label, hint string) SettingField { return field("bool", key, label, hint) }
func whole(key, label, hint string) SettingField {
	return field("int", key, label, hint).between(0, 1e12)
}
func number(key, label, hint string) SettingField {
	return field("float", key, label, hint).between(0, 1e12)
}
func text(key, label, hint string) SettingField  { return field("text", key, label, hint) }
func long(key, label, hint string) SettingField  { return field("long", key, label, hint) }
func items(key, label, hint string) SettingField { return field("list", key, label, hint) }
func info(key, label, hint string) SettingField  { return field("info", key, label, hint).fixed() }
func choice(key, label, hint string, pairs ...string) SettingField {
	f := field("select", key, label, hint)
	f.Options = optionPairs(pairs)
	return f
}
func secret(key, label, hint string) SettingField {
	f := field("secret", key, label, hint)
	f.configured = key + "_configured"
	return f
}
func table(key, label, hint string, columns ...SettingColumn) SettingField {
	f := field("rows", key, label, hint)
	f.Columns = columns
	return f
}
func column(kind, key, label string, pairs ...string) SettingColumn {
	return SettingColumn{Key: key, Kind: kind, Label: label, Options: optionPairs(pairs)}
}
func optionPairs(pairs []string) []SettingOption {
	var options []SettingOption
	for i := 0; i+1 < len(pairs); i += 2 {
		options = append(options, SettingOption{pairs[i], pairs[i+1]})
	}
	return options
}
func group(title, hint string, fields ...SettingField) SettingGroup {
	return SettingGroup{Title: title, Hint: hint, Fields: fields}
}
func (g SettingGroup) when(key string) SettingGroup { g.When = key; return g }

// SettingKeys 返回目录覆盖的 /admin/settings 字段名（含密钥对应的 *_configured）。
func SettingKeys() map[string]bool {
	keys := map[string]bool{}
	for _, section := range SettingCatalog {
		for _, group := range section.Groups {
			for _, field := range group.Fields {
				if field.source != "" {
					continue
				}
				keys[field.name] = true
				if field.configured != "" {
					keys[field.configured] = true
				}
			}
		}
	}
	return keys
}

func init() {
	seen := map[string]bool{}
	for _, section := range SettingCatalog {
		for _, group := range section.Groups {
			for _, field := range group.Fields {
				if seen[field.Key] {
					panic(fmt.Sprintf("duplicate setting %s", field.Key))
				}
				seen[field.Key] = true
				if field.source != "" && SettingSources[field.source] == "" {
					panic(fmt.Sprintf("unknown setting source %s", field.source))
				}
			}
		}
	}
}
