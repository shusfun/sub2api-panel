package platform

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type Problem struct {
	Status int
	Code   string
	Fields map[string]string
}

func (p *Problem) Error() string { return p.Code }

func Fail(status int, code string) error { return &Problem{Status: status, Code: code} }

func FailField(field, hint string) error {
	return &Problem{Status: http.StatusBadRequest, Code: "invalid_input", Fields: map[string]string{field: hint}}
}

func FailFields(fields map[string]string) error {
	if len(fields) == 0 {
		return Fail(http.StatusBadRequest, "invalid_input")
	}
	return &Problem{Status: http.StatusBadRequest, Code: "invalid_input", Fields: fields}
}

var errorHints = map[string]string{
	"official_confirmation_required": "请刷新管理工作区并完成管理员首次确认。",
	"invalid_proof":                  "浏览器会话签名无效，请重新登录。",
	"sign_in_required":               "会话已结束，请重新登录。",
	"login_rejected":                 "登录失败，请检查账号、密码或验证码。",
	"login_expired":                  "验证已失效，请重新输入账号和密码。",
	"operation_rejected":             "操作未能完成，请检查填写的内容。",
	"conflict":                       "名称或编码已被占用，或当前记录状态已变化，请刷新确认。",
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
	"invalid_input":                  "请检查标红或提示的输入项。",
	"invalid_usage_range":            "请选择有效日期，单次最多查询 90 天。",
	"invalid_hour_range":             "按小时最多可查询 7 天。",
	"usage_sync_pending":             "这个小时的逐笔记录正在同步。",
	"issue_rejected":                 "Sub2API 拒绝了本次发码，已退回预扣额度。",
	"operation_unavailable":          "当前记录无法核对，请刷新列表。",
	"totp_required":                  "此敏感操作需要有效的 6 位验证器动态码。",
	"schedule_changed":               "调度状态已变化，请刷新确认。",
	"agreement_required":             "请阅读并同意站点登录协议。",
	"replayed_proof":                 "请求已处理，请重新发起操作。",
	"json_required":                  "此操作需要 JSON 请求。",
	"setup_required":                 "请先完成 Sub2API 连接向导。",
}

type ProblemResponse struct {
	Code   string            `json:"code"`
	Hint   string            `json:"hint"`
	ID     string            `json:"request_id"`
	Fields map[string]string `json:"fields,omitempty"`
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	p := &Problem{Status: 502, Code: "service_unavailable"}
	var known *Problem
	if errors.As(err, &known) {
		p = known
	}
	id := Meta(r.Context()).Trace
	slog.Info("operation rejected", "trace_id", id, "status", p.Status, "code", p.Code)
	hint := errorHints[p.Code]
	if hint == "" {
		hint = "服务暂不可用，请稍后再试。"
	}
	WriteJSON(w, p.Status, ProblemResponse{Code: p.Code, Hint: hint, ID: id, Fields: p.Fields})
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func Send[T any](w http.ResponseWriter, status int, data T, err error) error {
	if err == nil {
		WriteJSON(w, status, data)
	}
	return err
}

func DecodeBody[T any](r *http.Request) (T, error) {
	var out T
	if strings.SplitN(r.Header.Get("Content-Type"), ";", 2)[0] != "application/json" {
		return out, Fail(415, "json_required")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, MaxBody+1))
	dec.DisallowUnknownFields()
	if dec.Decode(&out) != nil || dec.Decode(new(any)) != io.EOF {
		return out, Fail(400, "invalid_input")
	}
	return out, nil
}

func DecodeBodyInto[T any](r *http.Request, output *T) error {
	value, err := DecodeBody[T](r)
	if err == nil {
		*output = value
	}
	return err
}

type UpstreamStatus struct{ Status int }

func (e UpstreamStatus) Error() string { return "upstream request failed" }

func TranslateUpstream(err error) error {
	var e UpstreamStatus
	if errors.As(err, &e) {
		switch e.Status {
		case 400, 422:
			return Fail(e.Status, "operation_rejected")
		case 409:
			return Fail(409, "conflict")
		case 401:
			return Fail(401, "sign_in_required")
		case 403:
			return Fail(e.Status, "not_allowed")
		case 423:
			return Fail(423, "official_confirmation_required")
		case 404:
			return Fail(404, "not_found")
		case 429:
			return Fail(429, "rate_limited")
		}
	}
	return err
}
