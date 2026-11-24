package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type auditEntry struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ActorID   *int64    `json:"actor_user_id,omitempty"`
	Email     string    `json:"actor_email"`
	Role      string    `json:"actor_role"`
	Auth      string    `json:"auth_method"`
	Action    string    `json:"action"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	RequestID string    `json:"request_id"`
	IP        string    `json:"client_ip"`
	Status    int       `json:"status_code"`
	Latency   int64     `json:"latency_ms"`
}

func projectAuditEntry(row auditEntry) (auditEntry, error) {
	row.Path, _, _ = strings.Cut(row.Path, "?")
	return row, requiredID(row.ID)
}

func (p *panel) auditLogList(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, query, err := listQuery(r, "q", "actor_email", "method", "success", "start_time", "end_time")
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/audit-logs", page, query, projectAuditEntry)
	return send(w, http.StatusOK, rows, err)
}

func (p *panel) auditLogDetail(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	row, err := upstream[auditEntry](r.Context(), p.upstream, http.MethodGet, fmt.Sprintf("/admin/audit-logs/%d", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	row, err = projectAuditEntry(row)
	return send(w, http.StatusOK, row, err)
}

func (p *panel) auditLogClear(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		TOTP string `json:"totp"`
	}](r)
	if err != nil {
		return err
	}
	if !otpPattern.MatchString(input.TOTP) {
		return failField("totp", "请输入 6 位数字验证码")
	}
	result, err := upstream[struct {
		Deleted int64 `json:"deleted"`
	}](r.Context(), p.upstream, http.MethodPost, "/admin/audit-logs/clear", a.tokens.Access, "", "", struct {
		Code string `json:"totp_code"`
	}{input.TOTP})
	return send(w, http.StatusOK, result, err)
}
