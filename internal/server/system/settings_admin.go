package system

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

// 站点设置：把官方 /admin/settings 及其子接口按「管理员要办的事」重组为分区。
// 目录（settings_catalog.go）是唯一的字段白名单：读取时只投影目录里的字段，
// 写入时只接受目录里的字段并按类型校验，再按来源做部分更新。

const (
	settingTextLimit = 4000
	settingLongLimit = 400000
	settingListLimit = 20000
	settingRowLimit  = 500
)

func (h *handler) adminSettingsRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/admin/settings", true, h.adminSettingSections)
	h.s.Route(mux, "GET /api/admin/settings/{section}", true, h.adminSettingSection)
	h.s.Route(mux, "PUT /api/admin/settings/{section}", true, h.updateAdminSettingSection)
	h.adminSettingToolRoutes(mux)
}

func (h *handler) adminSettingSections(w http.ResponseWriter, _ *http.Request, _ core.SessionAuth) error {
	type entry struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Hint   string `json:"hint,omitempty"`
		StepUp bool   `json:"step_up"`
	}
	list := make([]entry, 0, len(SettingCatalog))
	for _, section := range SettingCatalog {
		list = append(list, entry{section.ID, section.Title, section.Hint, section.StepUp})
	}
	platform.WriteJSON(w, 200, struct {
		Items []entry `json:"items"`
	}{list})
	return nil
}

func (h *handler) readSettingSource(r *http.Request, token, source string) (map[string]json.RawMessage, error) {
	path := "/admin/settings"
	if source != "" {
		path = SettingSources[source]
	}
	data, err := platform.Upstream[map[string]json.RawMessage](r.Context(), h.s.Upstream, "GET", path, token, "", "", nil)
	if source != "" {
		var prob *platform.Problem
		if errors.As(err, &prob) && prob != nil && prob.Status == http.StatusNotFound {
			return map[string]json.RawMessage{}, nil
		}
	}
	if err == nil && data == nil {
		err = platform.Fail(502, "invalid_upstream")
	}
	return data, err
}

func (h *handler) adminSettingSection(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	section, ok := findSettingSection(r.PathValue("section"))
	if !ok {
		return platform.Fail(404, "not_found")
	}
	w.Header().Set("Cache-Control", "no-store")
	loaded := map[string]map[string]json.RawMessage{}
	for _, source := range section.sources() {
		data, err := h.readSettingSource(r, a.Tokens.Access, source)
		if err != nil {
			return err
		}
		loaded[source] = data
	}
	view := SettingSection{ID: section.ID, Title: section.Title, Hint: section.Hint, StepUp: section.StepUp}
	for _, group := range section.Groups {
		out := SettingGroup{Title: group.Title, Hint: group.Hint, When: group.When}
		for _, field := range group.Fields {
			projected, err := projectSetting(field, loaded[field.source])
			if err != nil {
				return err
			}
			out.Fields = append(out.Fields, projected)
		}
		view.Groups = append(view.Groups, out)
	}
	platform.WriteJSON(w, 200, view)
	return nil
}

// projectSetting 取出一个字段的当前值；密钥只给出是否已配置。
func projectSetting(field SettingField, data map[string]json.RawMessage) (SettingField, error) {
	if field.Kind == "secret" {
		if field.configured != "" {
			var set bool
			if raw, ok := data[field.configured]; ok && json.Unmarshal(raw, &set) != nil {
				return field, platform.Fail(502, "invalid_upstream")
			}
			field.Configured = &set
		}
		return field, nil
	}
	raw, ok := data[field.name]
	if field.wrap != "" && ok {
		var holder map[string]json.RawMessage
		if json.Unmarshal(raw, &holder) != nil {
			return field, platform.Fail(502, "invalid_upstream")
		}
		raw, ok = holder[field.wrap]
	}
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		raw = settingZero(field.Kind)
	}
	clean, err := checkSetting(field.Kind, field.Options, field.Columns, nil, nil, raw, true)
	if err != nil {
		return field, platform.Fail(502, "invalid_upstream")
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

// checkSetting 按类型校验并规整一个值。fromUpstream 为真时用于投影上游数据：
// 丢弃未声明的列、密钥列只保留「已配置」标记，不做范围限制。
func checkSetting(kind string, options []SettingOption, columns []SettingColumn, min, max *float64, raw json.RawMessage, fromUpstream bool) (json.RawMessage, error) {
	bad := platform.Fail(400, "invalid_input")
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
		return checkSettingRows(columns, raw, fromUpstream)
	default:
		return nil, bad
	}
	return raw, nil
}

func checkSettingRows(columns []SettingColumn, raw json.RawMessage, fromUpstream bool) (json.RawMessage, error) {
	bad := platform.Fail(400, "invalid_input")
	var rows []map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if decoder.Decode(&rows) != nil || decoder.More() || len(rows) > settingRowLimit {
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
}

func (h *handler) updateAdminSettingSection(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	section, ok := findSettingSection(r.PathValue("section"))
	if !ok {
		return platform.Fail(404, "not_found")
	}
	input, err := platform.DecodeBody[struct {
		Values map[string]json.RawMessage `json:"values"`
		TOTP   string                     `json:"totp"`
	}](r)
	if err != nil || len(input.Values) == 0 || len(input.Values) > 400 {
		return platform.Fail(400, "invalid_input")
	}
	fields := map[string]SettingField{}
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
			return platform.Fail(400, "invalid_input")
		}
		var value json.RawMessage
		if field.Kind == "secret" {
			var secret string
			if json.Unmarshal(raw, &secret) != nil || len(secret) > settingTextLimit {
				return platform.Fail(400, "invalid_input")
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
		return platform.Fail(400, "invalid_input")
	}
	if section.StepUp {
		if err := h.s.StepUp(r, a, input.TOTP); err != nil {
			return err
		}
	}
	for source, values := range changes {
		if source == "" {
			// 官方 PUT /admin/settings 只更新出现的字段，未出现的字段和密钥保持原值。
			if _, err := platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "PUT", "/admin/settings", a.Tokens.Access, "", "", values); err != nil {
				return err
			}
			continue
		}
		current, err := h.readSettingSource(r, a.Tokens.Access, source)
		if err != nil {
			return err
		}
		for name, value := range values {
			current[name] = value
		}
		if _, err := platform.Upstream[json.RawMessage](r.Context(), h.s.Upstream, "PUT", SettingSources[source], a.Tokens.Access, "", "", current); err != nil {
			return err
		}
	}
	platform.WriteJSON(w, 200, struct{}{})
	return nil
}
