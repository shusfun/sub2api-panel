package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
)

// 站点设置：把官方 /admin/settings 及其子接口按「管理员要办的事」重组为分区。
// 目录（settings_catalog.go）是唯一的字段白名单：读取时只投影目录里的字段，
// 写入时只接受目录里的字段并按类型校验，再按来源做部分更新。

type settingOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type settingColumn struct {
	Key     string          `json:"key"`
	Kind    string          `json:"kind"`
	Label   string          `json:"label"`
	Options []settingOption `json:"options,omitempty"`
	// Secret 列：值只写不读，Configured 是上游表示「已配置」的布尔字段。
	Configured string `json:"-"`
}

type settingField struct {
	Key     string          `json:"key"`
	Kind    string          `json:"kind"`
	Label   string          `json:"label"`
	Hint    string          `json:"hint,omitempty"`
	Min     *float64        `json:"min,omitempty"`
	Max     *float64        `json:"max,omitempty"`
	Options []settingOption `json:"options,omitempty"`
	When    string          `json:"when,omitempty"`
	Fixed   bool            `json:"read_only,omitempty"`
	Columns []settingColumn `json:"columns,omitempty"`

	Value      json.RawMessage `json:"value,omitempty"`
	Configured *bool           `json:"configured,omitempty"`

	source     string // 空表示 /admin/settings，否则是 settingSources 的键
	name       string // 上游字段名
	configured string // secret：上游的 *_configured 字段名
	wrap       string // rows：上游把数组包在这个键下
}

type settingGroup struct {
	Title  string         `json:"title,omitempty"`
	Hint   string         `json:"hint,omitempty"`
	When   string         `json:"when,omitempty"`
	Fields []settingField `json:"fields"`
}

type settingSection struct {
	ID     string         `json:"id"`
	Title  string         `json:"title"`
	Hint   string         `json:"hint,omitempty"`
	StepUp bool           `json:"step_up"`
	Groups []settingGroup `json:"groups,omitempty"`
}

// 子接口：各自整体读写，面板读出后合并本次改动再写回。
var settingSources = map[string]string{
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

const (
	settingTextLimit = 4000
	settingLongLimit = 400000
	settingListLimit = 20000
	settingRowLimit  = 500
)

func (p *panel) adminSettingsRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api/admin/settings", true, p.adminSettingSections)
	p.route(mux, "GET /api/admin/settings/{section}", true, p.adminSettingSection)
	p.route(mux, "PUT /api/admin/settings/{section}", true, p.updateAdminSettingSection)
	p.adminSettingToolRoutes(mux)
}

func (p *panel) adminSettingSections(w http.ResponseWriter, _ *http.Request, _ sessionAuth) error {
	type entry struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Hint   string `json:"hint,omitempty"`
		StepUp bool   `json:"step_up"`
	}
	list := make([]entry, 0, len(settingCatalog))
	for _, section := range settingCatalog {
		list = append(list, entry{section.ID, section.Title, section.Hint, section.StepUp})
	}
	writeJSON(w, 200, struct {
		Items []entry `json:"items"`
	}{list})
	return nil
}

func findSettingSection(id string) (settingSection, bool) {
	for _, section := range settingCatalog {
		if section.ID == id {
			return section, true
		}
	}
	return settingSection{}, false
}

func (section settingSection) sources() []string {
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

func (p *panel) readSettingSource(r *http.Request, token, source string) (map[string]json.RawMessage, error) {
	path := "/admin/settings"
	if source != "" {
		path = settingSources[source]
	}
	data, err := upstream[map[string]json.RawMessage](r.Context(), p.upstream, "GET", path, token, "", "", nil)
	if source != "" {
		var prob *problem
		if errors.As(err, &prob) && prob != nil && prob.Status == http.StatusNotFound {
			return map[string]json.RawMessage{}, nil
		}
	}
	if err == nil && data == nil {
		err = fail(502, "invalid_upstream")
	}
	return data, err
}

func (p *panel) adminSettingSection(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	section, ok := findSettingSection(r.PathValue("section"))
	if !ok {
		return fail(404, "not_found")
	}
	w.Header().Set("Cache-Control", "no-store")
	loaded := map[string]map[string]json.RawMessage{}
	for _, source := range section.sources() {
		data, err := p.readSettingSource(r, a.tokens.Access, source)
		if err != nil {
			return err
		}
		loaded[source] = data
	}
	view := settingSection{ID: section.ID, Title: section.Title, Hint: section.Hint, StepUp: section.StepUp}
	for _, group := range section.Groups {
		out := settingGroup{Title: group.Title, Hint: group.Hint, When: group.When}
		for _, field := range group.Fields {
			projected, err := projectSetting(field, loaded[field.source])
			if err != nil {
				return err
			}
			out.Fields = append(out.Fields, projected)
		}
		view.Groups = append(view.Groups, out)
	}
	writeJSON(w, 200, view)
	return nil
}

// projectSetting 取出一个字段的当前值；密钥只给出是否已配置。
func projectSetting(field settingField, data map[string]json.RawMessage) (settingField, error) {
	if field.Kind == "secret" {
		if field.configured != "" {
			var set bool
			if raw, ok := data[field.configured]; ok && json.Unmarshal(raw, &set) != nil {
				return field, fail(502, "invalid_upstream")
			}
			field.Configured = &set
		}
		return field, nil
	}
	raw, ok := data[field.name]
	if field.wrap != "" && ok {
		var holder map[string]json.RawMessage
		if json.Unmarshal(raw, &holder) != nil {
			return field, fail(502, "invalid_upstream")
		}
		raw, ok = holder[field.wrap]
	}
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		raw = settingZero(field.Kind)
	}
	clean, err := checkSetting(field.Kind, field.Options, field.Columns, nil, nil, raw, true)
	if err != nil {
		return field, fail(502, "invalid_upstream")
	}
	field.Value = clean
	return field, nil
}

func settingZero(kind string) json.RawMessage {
	switch kind {
	case "bool":
		return json.RawMessage("false")
	case "int", "float":
		return json.RawMessage("0")
	case "list", "intlist", "rows":
		return json.RawMessage("[]")
	case "numbers", "quotas":
		return json.RawMessage("{}")
	}
	return json.RawMessage(`""`)
}

// checkSetting 按类型校验并规整一个值。upstream 为真时用于投影上游数据：
// 丢弃未声明的列、密钥列只保留「已配置」标记，不做范围限制。
func checkSetting(kind string, options []settingOption, columns []settingColumn, min, max *float64, raw json.RawMessage, fromUpstream bool) (json.RawMessage, error) {
	bad := fail(400, "invalid_input")
	decode := func(target any) error {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		if decoder.Decode(target) != nil || decoder.More() {
			return bad
		}
		return nil
	}
	switch kind {
	case "bool":
		var value bool
		if err := decode(&value); err != nil {
			return nil, err
		}
	case "int", "float":
		var value float64
		if err := decode(&value); err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, bad
		}
		if kind == "int" && value != math.Trunc(value) {
			return nil, bad
		}
		if !fromUpstream && (min != nil && value < *min || max != nil && value > *max) {
			return nil, bad
		}
	case "optint", "optfloat":
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return json.RawMessage("null"), nil
		}
		return checkSetting(strings.TrimPrefix(kind, "opt"), nil, nil, min, max, raw, fromUpstream)
	case "text", "long", "info":
		var value string
		if err := decode(&value); err != nil {
			if !fromUpstream || kind != "info" {
				return nil, err
			}
			return raw, nil
		}
		limit := settingTextLimit
		if kind == "long" {
			limit = settingLongLimit
		}
		if !fromUpstream && len(value) > limit {
			return nil, bad
		}
	case "select":
		var value string
		if err := decode(&value); err != nil {
			return nil, err
		}
		if fromUpstream {
			break
		}
		known := false
		for _, option := range options {
			known = known || option.Value == value
		}
		if !known {
			return nil, bad
		}
	case "list":
		var value []string
		if err := decode(&value); err != nil || len(value) > settingListLimit {
			return nil, bad
		}
		for _, item := range value {
			if !fromUpstream && (strings.TrimSpace(item) == "" || len(item) > 500) {
				return nil, bad
			}
		}
		if value == nil {
			return json.RawMessage("[]"), nil
		}
	case "intlist":
		var value []float64
		if err := decode(&value); err != nil || len(value) > settingRowLimit {
			return nil, bad
		}
		for _, item := range value {
			if item != math.Trunc(item) {
				return nil, bad
			}
		}
		if value == nil {
			return json.RawMessage("[]"), nil
		}
	case "numbers":
		var value map[string]float64
		if err := decode(&value); err != nil || len(value) > 64 {
			return nil, bad
		}
		for key, number := range value {
			if key == "" || len(key) > 64 || math.IsNaN(number) || number < 0 {
				return nil, bad
			}
		}
	case "quotas":
		var value map[string]struct {
			Daily   *float64 `json:"daily"`
			Weekly  *float64 `json:"weekly"`
			Monthly *float64 `json:"monthly"`
		}
		if err := decode(&value); err != nil || len(value) > 64 {
			return nil, bad
		}
		for key, limits := range value {
			for _, limit := range []*float64{limits.Daily, limits.Weekly, limits.Monthly} {
				if key == "" || len(key) > 64 || limit != nil && (math.IsNaN(*limit) || *limit < 0) {
					return nil, bad
				}
			}
		}
		clean, _ := json.Marshal(value)
		return clean, nil
	case "rows":
		var rows []map[string]json.RawMessage
		if err := decode(&rows); err != nil || len(rows) > settingRowLimit {
			return nil, bad
		}
		clean := make([]map[string]json.RawMessage, 0, len(rows))
		for _, row := range rows {
			out := map[string]json.RawMessage{}
			for _, column := range columns {
				cell, present := row[column.Key]
				if column.Kind == "secret" {
					if fromUpstream {
						if flag, ok := row[column.Configured]; ok {
							out[column.Configured] = flag
						}
						continue
					}
					var secret string
					if present && (json.Unmarshal(cell, &secret) != nil || len(secret) > settingTextLimit) {
						return nil, bad
					}
					encoded, _ := json.Marshal(secret)
					out[column.Key] = encoded
					continue
				}
				if !present || bytes.Equal(bytes.TrimSpace(cell), []byte("null")) {
					cell = settingZero(column.Kind)
					if strings.HasPrefix(column.Kind, "opt") {
						cell = json.RawMessage("null")
					}
				}
				value, err := checkSetting(column.Kind, column.Options, nil, nil, nil, cell, fromUpstream)
				if err != nil {
					return nil, err
				}
				out[column.Key] = value
			}
			if !fromUpstream {
				for key := range row {
					if _, known := out[key]; !known {
						return nil, bad
					}
				}
			}
			clean = append(clean, out)
		}
		encoded, _ := json.Marshal(clean)
		return encoded, nil
	default:
		return nil, bad
	}
	return raw, nil
}

func (p *panel) updateAdminSettingSection(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	section, ok := findSettingSection(r.PathValue("section"))
	if !ok {
		return fail(404, "not_found")
	}
	input, err := decodeBody[struct {
		Values map[string]json.RawMessage `json:"values"`
		TOTP   string                     `json:"totp"`
	}](r)
	if err != nil || len(input.Values) == 0 || len(input.Values) > 400 {
		return fail(400, "invalid_input")
	}
	fields := map[string]settingField{}
	for _, group := range section.Groups {
		for _, field := range group.Fields {
			fields[field.Key] = field
		}
	}
	// 先全部校验，再写上游：任何一项不合法都不产生部分写入。
	changes := map[string]map[string]json.RawMessage{}
	for key, raw := range input.Values {
		field, known := fields[key]
		if !known || field.Fixed || field.Kind == "info" {
			return fail(400, "invalid_input")
		}
		var value json.RawMessage
		if field.Kind == "secret" {
			var secret string
			if json.Unmarshal(raw, &secret) != nil || len(secret) > settingTextLimit {
				return fail(400, "invalid_input")
			}
			if secret == "" {
				continue
			}
			value = raw
		} else if value, err = checkSetting(field.Kind, field.Options, field.Columns, field.Min, field.Max, raw, false); err != nil {
			return err
		}
		if field.wrap != "" {
			value, _ = json.Marshal(map[string]json.RawMessage{field.wrap: value})
		}
		if changes[field.source] == nil {
			changes[field.source] = map[string]json.RawMessage{}
		}
		changes[field.source][field.name] = value
	}
	if len(changes) == 0 {
		return fail(400, "invalid_input")
	}
	if section.StepUp {
		if err := p.stepUp(r, a, input.TOTP); err != nil {
			return err
		}
	}
	for source, values := range changes {
		if source == "" {
			// 官方 PUT /admin/settings 只更新出现的字段，未出现的字段和密钥保持原值。
			if _, err := upstream[json.RawMessage](r.Context(), p.upstream, "PUT", "/admin/settings", a.tokens.Access, "", "", values); err != nil {
				return err
			}
			continue
		}
		current, err := p.readSettingSource(r, a.tokens.Access, source)
		if err != nil {
			return err
		}
		for name, value := range values {
			current[name] = value
		}
		if _, err := upstream[json.RawMessage](r.Context(), p.upstream, "PUT", settingSources[source], a.tokens.Access, "", "", current); err != nil {
			return err
		}
	}
	writeJSON(w, 200, struct{}{})
	return nil
}

// stepUpEnabled 读取官方站点是否开启管理员敏感操作二次验证。
func (p *panel) stepUpEnabled(ctx context.Context, token string) bool {
	settings, err := upstream[struct {
		StepUp bool `json:"step_up_enabled"`
	}](ctx, p.upstream, http.MethodGet, "/admin/settings", token, "", "", nil)
	return err == nil && settings.StepUp
}

// requireOptionalStepUp 在站点开启敏感操作验证或请求主动携带验证码时强制校验验证器。
func (p *panel) requireOptionalStepUp(r *http.Request, a sessionAuth, code string) error {
	if code != "" || p.stepUpEnabled(r.Context(), a.tokens.Access) {
		return p.stepUp(r, a, code)
	}
	return nil
}

// stepUp 用当前管理员的验证器代码向官方换取敏感操作窗口。
func (p *panel) stepUp(r *http.Request, a sessionAuth, code string) error {
	ctx, end := startFlow(r.Context(), "敏感操作二次验证", "actor_id", a.user.ID)
	defer end()
	if !otpPattern.MatchString(code) {
		return fail(403, "totp_required")
	}
	result, err := upstream[struct {
		Verified bool `json:"verified"`
	}](ctx, p.upstream, "POST", "/user/totp/step-up", a.tokens.Access, "", "", struct {
		Code string `json:"code"`
	}{code})
	var rejected upstreamStatus
	if errors.As(err, &rejected) && (rejected.status == 400 || rejected.status == 403) || err == nil && !result.Verified {
		return fail(403, "totp_required")
	}
	return err
}

// 以下是目录的书写辅助。

func limit(value float64) *float64 { return &value }

func (f settingField) when(key string) settingField { f.When = key; return f }
func (f settingField) from(source string) settingField {
	f.source = source
	f.Key = source + "." + f.name
	return f
}
func (f settingField) inside(key string) settingField { f.wrap = key; return f }
func (f settingField) fixed() settingField            { f.Fixed = true; return f }
func (f settingField) between(min, max float64) settingField {
	f.Min, f.Max = limit(min), limit(max)
	return f
}

func field(kind, key, label, hint string) settingField {
	return settingField{Key: key, name: key, Kind: kind, Label: label, Hint: hint}
}
func toggle(key, label, hint string) settingField { return field("bool", key, label, hint) }
func whole(key, label, hint string) settingField {
	return field("int", key, label, hint).between(0, 1e12)
}
func number(key, label, hint string) settingField {
	return field("float", key, label, hint).between(0, 1e12)
}
func text(key, label, hint string) settingField  { return field("text", key, label, hint) }
func long(key, label, hint string) settingField  { return field("long", key, label, hint) }
func items(key, label, hint string) settingField { return field("list", key, label, hint) }
func info(key, label, hint string) settingField  { return field("info", key, label, hint).fixed() }
func choice(key, label, hint string, pairs ...string) settingField {
	f := field("select", key, label, hint)
	f.Options = optionPairs(pairs)
	return f
}
func secret(key, label, hint string) settingField {
	f := field("secret", key, label, hint)
	f.configured = key + "_configured"
	return f
}
func table(key, label, hint string, columns ...settingColumn) settingField {
	f := field("rows", key, label, hint)
	f.Columns = columns
	return f
}
func column(kind, key, label string, pairs ...string) settingColumn {
	return settingColumn{Key: key, Kind: kind, Label: label, Options: optionPairs(pairs)}
}
func optionPairs(pairs []string) []settingOption {
	var options []settingOption
	for i := 0; i+1 < len(pairs); i += 2 {
		options = append(options, settingOption{pairs[i], pairs[i+1]})
	}
	return options
}
func group(title, hint string, fields ...settingField) settingGroup {
	return settingGroup{Title: title, Hint: hint, Fields: fields}
}
func (g settingGroup) when(key string) settingGroup { g.When = key; return g }

// settingKeys 返回目录覆盖的 /admin/settings 字段名（含密钥对应的 *_configured）。
func settingKeys() map[string]bool {
	keys := map[string]bool{}
	for _, section := range settingCatalog {
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
	for _, section := range settingCatalog {
		for _, group := range section.Groups {
			for _, field := range group.Fields {
				if seen[field.Key] {
					panic(fmt.Sprintf("duplicate setting %s", field.Key))
				}
				seen[field.Key] = true
				if field.source != "" && settingSources[field.source] == "" {
					panic(fmt.Sprintf("unknown setting source %s", field.source))
				}
			}
		}
	}
}
