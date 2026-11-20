package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type callMeta struct {
	Trace string
	Span  string
	UA    string
}

type metaKey struct{}

func randomHex(size int) string { return hex.EncodeToString(randomBytes(size)) }

func randomBytes(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

func randomToken() string { return base64.RawURLEncoding.EncodeToString(randomBytes(32)) }

func traceRequest(r *http.Request, w http.ResponseWriter, name string) (*http.Request, func()) {
	meta := callMeta{Trace: randomHex(16), Span: randomHex(8), UA: r.UserAgent()}
	start := time.Now()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Request-ID", meta.Trace)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	return r.WithContext(context.WithValue(r.Context(), metaKey{}, meta)), func() {
		slog.Info(name, "trace_id", meta.Trace, "span_id", meta.Span, "elapsed_ms", time.Since(start).Milliseconds())
	}
}

func meta(ctx context.Context) callMeta {
	value, _ := ctx.Value(metaKey{}).(callMeta)
	return value
}

type problem struct {
	Status int
	Code   string
}

func (p *problem) Error() string { return p.Code }

func fail(status int, code string) error { return &problem{status, code} }

var errorHints = map[string]string{
	"official_confirmation_required": "请进入官方面板，完成该操作所需的确认。",
	"invalid_proof":                  "浏览器会话签名无效，请重新登录。",
	"sign_in_required":               "会话已结束，请重新登录。",
	"login_rejected":                 "登录失败，请检查账号、密码或验证码。",
	"login_expired":                  "验证已失效，请重新输入账号和密码。",
	"operation_rejected":             "操作未能完成，请检查填写的内容。",
	"not_allowed":                    "你没有此操作的权限，或当前站点未启用该功能。",
	"rate_limited":                   "操作过于频繁，请稍后再试。",
	"grant_exhausted":                "发码权限或剩余额度不足。",
	"issue_unknown":                  "发码结果待核对，请到发码记录查看。",
	"issue_service_unavailable":      "发码服务暂不可用，请先检查发码记录。",
	"duplicate_operation":            "这次发码已提交，请查看发码记录。",
	"invalid_grant":                  "请检查发码额度和允许的面额。",
	"code_mismatch":                  "官方码的类型或面额与发码记录不符。",
	"csrf_rejected":                  "请求校验失败，请刷新页面后重试。",
	"not_found":                      "找不到该记录。",
	"invalid_input":                  "请检查输入内容。",
	"invalid_usage_range":            "请选择有效日期，单次最多查询 90 天。",
	"issue_rejected":                 "Sub2API 拒绝了本次发码，已退回预扣额度。",
	"operation_unavailable":          "当前记录无法核对，请刷新列表。",
	"totp_required":                  "请提供主管理员的有效验证码。",
	"schedule_changed":               "调度状态已变化，请刷新确认。",
	"agreement_required":             "请阅读并同意站点登录协议。",
	"replayed_proof":                 "请求已处理，请重新发起操作。",
	"json_required":                  "此操作需要 JSON 请求。",
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	p := &problem{502, "service_unavailable"}
	var known *problem
	if errors.As(err, &known) {
		p = known
	}
	id := meta(r.Context()).Trace
	slog.Info("operation rejected", "trace_id", id, "status", p.Status, "code", p.Code)
	hint := errorHints[p.Code]
	if hint == "" {
		hint = "服务暂不可用，请稍后再试。"
	}
	writeJSON(w, p.Status, struct {
		Code string `json:"code"`
		Hint string `json:"hint"`
		ID   string `json:"request_id"`
	}{p.Code, hint, id})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func decodeBody[T any](r *http.Request) (T, error) {
	var out T
	if strings.SplitN(r.Header.Get("Content-Type"), ";", 2)[0] != "application/json" {
		return out, fail(415, "json_required")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBody+1))
	dec.DisallowUnknownFields()
	if dec.Decode(&out) != nil || dec.Decode(new(any)) != io.EOF {
		return out, fail(400, "invalid_input")
	}
	return out, nil
}

type serviceClient struct {
	base   string
	client *http.Client
}

func newClient(base string) serviceClient {
	return serviceClient{base: strings.TrimRight(base, "/"), client: &http.Client{
		Timeout: 25 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

func (c serviceClient) open(ctx context.Context, method, path, token, key, operation string, data any) (*http.Response, error) {
	var body io.Reader
	if data != nil {
		value, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(value)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	if operation != "" {
		req.Header.Set("Idempotency-Key", operation)
	}
	info := meta(ctx)
	if info.UA != "" {
		req.Header.Set("User-Agent", info.UA)
	}
	span := randomHex(8)
	if info.Trace != "" {
		req.Header.Set("traceparent", "00-"+info.Trace+"-"+span+"-01")
		req.Header.Set("X-Request-ID", info.Trace)
	}
	start := time.Now()
	res, err := c.client.Do(req)
	status := 0
	if res != nil {
		status = res.StatusCode
	}
	route, _, _ := strings.Cut(path, "?")
	slog.Info("service request", "trace_id", info.Trace, "span_id", span, "parent_span_id", info.Span, "method", method, "route", route, "status", status, "elapsed_ms", time.Since(start).Milliseconds())
	return res, err
}

type upstreamStatus struct{ status int }

func (e upstreamStatus) Error() string { return "upstream request failed" }

func upstream[T any](ctx context.Context, c serviceClient, method, path, token, key, operation string, input any) (T, error) {
	var zero T
	res, err := c.open(ctx, method, "/api/v1"+path, token, key, operation, input)
	if err != nil {
		return zero, err
	}
	defer res.Body.Close()
	var envelope struct {
		Code *int            `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return zero, upstreamStatus{res.StatusCode}
	}
	dec := json.NewDecoder(io.LimitReader(res.Body, 4<<20))
	if dec.Decode(&envelope) != nil || dec.Decode(new(any)) != io.EOF ||
		envelope.Code == nil || *envelope.Code != 0 {
		return zero, fail(502, "invalid_upstream")
	}
	if _, empty := any(zero).(struct{}); empty {
		return zero, nil
	}
	if bytes.Equal(envelope.Data, []byte("null")) || json.Unmarshal(envelope.Data, &zero) != nil {
		return zero, fail(502, "invalid_upstream")
	}
	return zero, nil
}

func translateUpstream(err error) error {
	var e upstreamStatus
	if errors.As(err, &e) {
		switch e.status {
		case 400, 409, 422:
			return fail(e.status, "operation_rejected")
		case 401:
			return fail(401, "sign_in_required")
		case 403:
			return fail(e.status, "not_allowed")
		case 423:
			return fail(e.status, "official_confirmation_required")
		case 404:
			return fail(404, "not_found")
		case 429:
			return fail(429, "rate_limited")
		}
	}
	return err
}

func checkOrigin(raw string, httpsOnly bool) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Hostname() != "" && u.User == nil && u.Path == "" &&
		u.RawQuery == "" && u.Fragment == "" && !u.ForceQuery &&
		(u.Scheme == "https" || !httpsOnly && u.Scheme == "http")
}
