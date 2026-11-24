package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"slices"
	"strings"
)

const officialPluginUploadLimit = 128 << 20

type officialPlugin struct {
	namedItem
	Version       string `json:"version"`
	Description   string `json:"description"`
	State         string `json:"state"`
	Healthy       bool   `json:"runtime_healthy"`
	Compatibility struct {
		Status string `json:"status"`
	} `json:"compatibility"`
}

type officialPluginField struct {
	Key        string          `json:"key"`
	Kind       string          `json:"kind"`
	Value      json.RawMessage `json:"value,omitempty"`
	Configured bool            `json:"configured,omitempty"`
}

type officialPluginConfigView struct {
	Fields []officialPluginField `json:"fields"`
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

func classifyPluginValue(key string, raw json.RawMessage) officialPluginField {
	trimmed := bytes.TrimSpace(raw)
	if isPluginSecretKey(key) {
		var str string
		configured := len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) && !bytes.Equal(trimmed, []byte(`""`))
		if json.Unmarshal(trimmed, &str) == nil {
			configured = strings.TrimSpace(str) != ""
		}
		return officialPluginField{Key: key, Kind: "secret", Configured: configured}
	}
	if bytes.Equal(trimmed, []byte("true")) || bytes.Equal(trimmed, []byte("false")) {
		return officialPluginField{Key: key, Kind: "bool", Value: trimmed}
	}
	var num json.Number
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if dec.Decode(&num) == nil {
		if _, err := num.Float64(); err == nil {
			return officialPluginField{Key: key, Kind: "number", Value: trimmed}
		}
	}
	var text string
	if json.Unmarshal(trimmed, &text) == nil {
		return officialPluginField{Key: key, Kind: "string", Value: trimmed}
	}
	return officialPluginField{Key: key, Kind: "json", Value: trimmed}
}

func (p *panel) readRawOfficialPluginConfig(r *http.Request, id int64, token string) (map[string]json.RawMessage, error) {
	path := fmt.Sprintf("/api/v1/admin/plugins/%d/config", id)
	res, err := p.upstream.open(r.Context(), http.MethodGet, path, token, "", "", nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, upstreamStatus{res.StatusCode}
	}
	var config map[string]json.RawMessage
	decoder := json.NewDecoder(io.LimitReader(res.Body, 4<<20))
	if err := decoder.Decode(&config); err != nil || decoder.Decode(new(any)) != io.EOF || config == nil {
		return nil, fail(http.StatusBadGateway, "invalid_upstream")
	}
	return config, nil
}

func (p *panel) officialPluginAuthorizeUpload(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		TOTP string `json:"totp"`
	}](r)
	if err != nil {
		return err
	}
	return send(w, http.StatusOK, struct{}{}, p.stepUp(r, a, input.TOTP))
}

func (p *panel) officialPluginUpload(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	if r.Header.Get("Content-Type") != "application/octet-stream" || r.ContentLength <= 0 {
		return fail(http.StatusBadRequest, "invalid_input")
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
	res, err := p.upstream.openBody(r.Context(), http.MethodPost, "/api/v1/admin/plugins/upload", a.tokens.Access, "", "", body, writer.FormDataContentType())
	if err != nil {
		return err
	}
	defer res.Body.Close()
	row, err := readUpstream[officialPlugin](res)
	if err == nil {
		err = requiredID(row.ID)
	}
	return send(w, http.StatusCreated, row, err)
}

func (p *panel) officialPluginConfig(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	ctx, end := startFlow(r.Context(), "配置网关插件")
	defer end()
	r = r.WithContext(ctx)
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if r.Method == http.MethodGet {
		config, err := p.readRawOfficialPluginConfig(r, id, a.tokens.Access)
		if err != nil {
			return err
		}
		keys := make([]string, 0, len(config))
		for key := range config {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		fields := make([]officialPluginField, 0, len(keys))
		for _, key := range keys {
			fields = append(fields, classifyPluginValue(key, config[key]))
		}
		w.Header().Set("Cache-Control", "no-store")
		return send(w, http.StatusOK, officialPluginConfigView{Fields: fields}, nil)
	}
	input, err := decodeBody[struct {
		TOTP            string                     `json:"totp"`
		Config          map[string]json.RawMessage `json:"config"`
		PreserveSecrets []string                   `json:"preserve_secrets,omitempty"`
	}](r)
	if err != nil {
		return err
	}
	if input.Config == nil {
		return failField("config", "请提供合法的配置键值")
	}
	if err := p.requireOptionalStepUp(r, a, input.TOTP); err != nil {
		return err
	}
	merged := make(map[string]json.RawMessage, len(input.Config)+len(input.PreserveSecrets))
	for key, raw := range input.Config {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			return failField("config", "配置字段名不能为空")
		}
		var check any
		if json.Unmarshal(raw, &check) != nil {
			return failField(trimmedKey, "字段值不是合法的 JSON")
		}
		merged[trimmedKey] = raw
	}
	if len(input.PreserveSecrets) > 0 {
		existing, err := p.readRawOfficialPluginConfig(r, id, a.tokens.Access)
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
	res, err := p.upstream.open(r.Context(), http.MethodPut, path, a.tokens.Access, "", "", merged)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return upstreamStatus{res.StatusCode}
	}
	return send(w, http.StatusOK, struct{}{}, nil)
}

func (p *panel) officialPluginRemove(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		TOTP string `json:"totp"`
	}](r)
	if err != nil {
		return err
	}
	if err := p.requireOptionalStepUp(r, a, input.TOTP); err != nil {
		return err
	}
	result, err := upstream[struct{}](r.Context(), p.upstream, http.MethodDelete, fmt.Sprintf("/admin/plugins/%d", id), a.tokens.Access, "", "", nil)
	return send(w, http.StatusOK, result, err)
}

func (p *panel) officialPluginList(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	rows, err := upstream[[]officialPlugin](r.Context(), p.upstream, http.MethodGet, "/admin/plugins", a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	rows, err = projectList(rows, func(row officialPlugin) (officialPlugin, error) { return row, requiredID(row.ID) })
	return send(w, http.StatusOK, rows, err)
}

func (p *panel) officialPluginAction(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	action := r.PathValue("action")
	if action != "enable" && action != "disable" && action != "test" {
		return fail(http.StatusNotFound, "not_found")
	}
	input, err := decodeBody[struct {
		TOTP           string `json:"totp"`
		Rollout        int    `json:"rollout_percent"`
		AcceptUntested bool   `json:"accept_untested"`
	}](r)
	if err != nil {
		return err
	}
	if action == "enable" && (input.Rollout < 1 || input.Rollout > 100) {
		return failField("rollout_percent", "启用比例需在 1 到 100 之间")
	}
	if err := p.requireOptionalStepUp(r, a, input.TOTP); err != nil {
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
		row, err := upstream[struct {
			Success bool `json:"success"`
			Latency int  `json:"latency_ms"`
		}](r.Context(), p.upstream, http.MethodPost, path, a.tokens.Access, "", "", payload)
		return send(w, http.StatusOK, row, err)
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, http.MethodPost, path, a.tokens.Access, "", "", payload)
	return send(w, http.StatusOK, struct{}{}, err)
}
