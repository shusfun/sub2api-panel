package system

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"slices"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

const OfficialPluginUploadLimit = 128 << 20

type OfficialPlugin struct {
	platform.NamedItem
	Version       string `json:"version"`
	Description   string `json:"description"`
	State         string `json:"state"`
	Healthy       bool   `json:"runtime_healthy"`
	Compatibility struct {
		Status string `json:"status"`
	} `json:"compatibility"`
}

type OfficialPluginField struct {
	Key        string          `json:"key"`
	Kind       string          `json:"kind"`
	Value      json.RawMessage `json:"value,omitempty"`
	Configured bool            `json:"configured,omitempty"`
}

type OfficialPluginConfigView struct {
	Fields []OfficialPluginField `json:"fields"`
}

func isPluginSecretKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	for _, marker := range []string{"secret", "token", "password", "passwd", "api_key", "apikey", "private_key", "credential", "signing_key"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func classifyPluginValue(key string, raw json.RawMessage) OfficialPluginField {
	trimmed := bytes.TrimSpace(raw)
	if isPluginSecretKey(key) {
		var str string
		configured := len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) && !bytes.Equal(trimmed, []byte(`""`))
		if json.Unmarshal(trimmed, &str) == nil {
			configured = strings.TrimSpace(str) != ""
		}
		return OfficialPluginField{Key: key, Kind: "secret", Configured: configured}
	}
	if bytes.Equal(trimmed, []byte("true")) || bytes.Equal(trimmed, []byte("false")) {
		return OfficialPluginField{Key: key, Kind: "bool", Value: trimmed}
	}
	var num json.Number
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if dec.Decode(&num) == nil {
		if _, err := num.Float64(); err == nil {
			return OfficialPluginField{Key: key, Kind: "number", Value: trimmed}
		}
	}
	var text string
	if json.Unmarshal(trimmed, &text) == nil {
		return OfficialPluginField{Key: key, Kind: "string", Value: trimmed}
	}
	return OfficialPluginField{Key: key, Kind: "json", Value: trimmed}
}

func readRawOfficialPluginConfig(s *core.Server, r *http.Request, id int64, token string) (map[string]json.RawMessage, error) {
	path := fmt.Sprintf("/api/v1/admin/plugins/%d/config", id)
	res, err := s.Upstream.Open(r.Context(), http.MethodGet, path, token, "", "", nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, platform.UpstreamStatus{Status: res.StatusCode}
	}
	var config map[string]json.RawMessage
	decoder := json.NewDecoder(io.LimitReader(res.Body, 4<<20))
	if err := decoder.Decode(&config); err != nil || decoder.Decode(new(any)) != io.EOF || config == nil {
		return nil, platform.Fail(http.StatusBadGateway, "invalid_upstream")
	}
	return config, nil
}

func (h *handler) officialPluginAuthorizeUpload(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		TOTP string `json:"totp"`
	}](r)
	if err != nil {
		return err
	}
	return platform.Send(w, http.StatusOK, struct{}{}, h.s.StepUp(r, a, input.TOTP))
}

func (h *handler) officialPluginUpload(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	if r.Header.Get("Content-Type") != "application/octet-stream" || r.ContentLength <= 0 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	var envelope bytes.Buffer
	writer := multipart.NewWriter(&envelope)
	if _, err := writer.CreateFormFile("plugin", "upload.s2plugin"); err != nil {
		return err
	}
	prefix := bytes.Clone(envelope.Bytes())
	envelope.Reset()
	if err := writer.Close(); err != nil {
		return err
	}
	body := io.MultiReader(bytes.NewReader(prefix), r.Body, &envelope)
	res, err := h.s.Upstream.OpenBody(r.Context(), http.MethodPost, "/api/v1/admin/plugins/upload", a.Tokens.Access, "", "", body, writer.FormDataContentType())
	if err != nil {
		return err
	}
	defer res.Body.Close()
	row, err := platform.ReadUpstream[OfficialPlugin](res)
	if err == nil {
		err = platform.RequiredID(row.ID)
	}
	return platform.Send(w, http.StatusCreated, row, err)
}

func OfficialPluginConfig(s *core.Server, w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	ctx, end := platform.StartFlow(r.Context(), "配置网关插件")
	defer end()
	r = r.WithContext(ctx)
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	if r.Method == http.MethodGet {
		config, err := readRawOfficialPluginConfig(s, r, id, a.Tokens.Access)
		if err != nil {
			return err
		}
		keys := make([]string, 0, len(config))
		for key := range config {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		fields := make([]OfficialPluginField, 0, len(keys))
		for _, key := range keys {
			fields = append(fields, classifyPluginValue(key, config[key]))
		}
		w.Header().Set("Cache-Control", "no-store")
		return platform.Send(w, http.StatusOK, OfficialPluginConfigView{Fields: fields}, nil)
	}
	input, err := platform.DecodeBody[struct {
		TOTP            string                     `json:"totp"`
		Config          map[string]json.RawMessage `json:"config"`
		PreserveSecrets []string                   `json:"preserve_secrets,omitempty"`
	}](r)
	if err != nil {
		return err
	}
	if input.Config == nil {
		return platform.FailField("config", "请提供合法的配置键值")
	}
	if err := s.RequireOptionalStepUp(r, a, input.TOTP); err != nil {
		return err
	}
	merged := make(map[string]json.RawMessage, len(input.Config)+len(input.PreserveSecrets))
	for key, raw := range input.Config {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			return platform.FailField("config", "配置字段名不能为空")
		}
		var check any
		if json.Unmarshal(raw, &check) != nil {
			return platform.FailField(trimmedKey, "字段值不是合法的 JSON")
		}
		merged[trimmedKey] = raw
	}
	if len(input.PreserveSecrets) > 0 {
		existing, err := readRawOfficialPluginConfig(s, r, id, a.Tokens.Access)
		if err != nil {
			return err
		}
		for _, secretKey := range input.PreserveSecrets {
			if _, overridden := merged[secretKey]; !overridden {
				if prev, ok := existing[secretKey]; ok {
					merged[secretKey] = prev
				}
			}
		}
	}
	path := fmt.Sprintf("/api/v1/admin/plugins/%d/config", id)
	res, err := s.Upstream.Open(r.Context(), http.MethodPut, path, a.Tokens.Access, "", "", merged)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return platform.UpstreamStatus{Status: res.StatusCode}
	}
	return platform.Send(w, http.StatusOK, struct{}{}, nil)
}

func (h *handler) officialPluginConfig(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	return OfficialPluginConfig(h.s, w, r, a)
}

func (h *handler) officialPluginRemove(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		TOTP string `json:"totp"`
	}](r)
	if err != nil {
		return err
	}
	if err := h.s.RequireOptionalStepUp(r, a, input.TOTP); err != nil {
		return err
	}
	result, err := platform.Upstream[struct{}](r.Context(), h.s.Upstream, http.MethodDelete, fmt.Sprintf("/admin/plugins/%d", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, http.StatusOK, result, err)
}

func (h *handler) officialPluginList(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	rows, err := platform.Upstream[[]OfficialPlugin](r.Context(), h.s.Upstream, http.MethodGet, "/admin/plugins", a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	rows, err = platform.ProjectList(rows, func(row OfficialPlugin) (OfficialPlugin, error) { return row, platform.RequiredID(row.ID) })
	return platform.Send(w, http.StatusOK, rows, err)
}

func (h *handler) officialPluginAction(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	action := r.PathValue("action")
	if action != "enable" && action != "disable" && action != "test" {
		return platform.Fail(http.StatusNotFound, "not_found")
	}
	input, err := platform.DecodeBody[struct {
		TOTP           string `json:"totp"`
		Rollout        int    `json:"rollout_percent"`
		AcceptUntested bool   `json:"accept_untested"`
	}](r)
	if err != nil {
		return err
	}
	if action == "enable" && (input.Rollout < 1 || input.Rollout > 100) {
		return platform.FailField("rollout_percent", "启用比例需在 1 到 100 之间")
	}
	if err := h.s.RequireOptionalStepUp(r, a, input.TOTP); err != nil {
		return err
	}
	var payload any = struct{}{}
	if action == "enable" {
		payload = struct {
			Rollout        int  `json:"rollout_percent"`
			AcceptUntested bool `json:"accept_untested"`
		}{input.Rollout, input.AcceptUntested}
	}
	path := fmt.Sprintf("/admin/plugins/%d/%s", id, action)
	if action == "test" {
		row, err := platform.Upstream[struct {
			Success bool `json:"success"`
			Latency int  `json:"latency_ms"`
		}](r.Context(), h.s.Upstream, http.MethodPost, path, a.Tokens.Access, "", "", payload)
		return platform.Send(w, http.StatusOK, row, err)
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, http.MethodPost, path, a.Tokens.Access, "", "", payload)
	return platform.Send(w, http.StatusOK, struct{}{}, err)
}
