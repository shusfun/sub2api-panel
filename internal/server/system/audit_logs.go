package system

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type AuditEntry struct {
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

func projectAuditEntry(row AuditEntry) (AuditEntry, error) {
	row.Path, _, _ = strings.Cut(row.Path, "?")
	return row, platform.RequiredID(row.ID)
}

func (h *handler) auditLogList(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, query, err := platform.ListQuery(r, "q", "actor_email", "method", "success", "start_time", "end_time")
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/admin/audit-logs", page, query, projectAuditEntry)
	return platform.Send(w, http.StatusOK, rows, err)
}

func (h *handler) auditLogDetail(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	row, err := platform.Upstream[AuditEntry](r.Context(), h.s.Upstream, http.MethodGet, fmt.Sprintf("/admin/audit-logs/%d", id), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	row, err = projectAuditEntry(row)
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) auditLogClear(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		TOTP string `json:"totp"`
	}](r)
	if err != nil {
		return err
	}
	if !platform.OTPPattern.MatchString(input.TOTP) {
		return platform.FailField("totp", "请输入 6 位数字验证码")
	}
	result, err := platform.Upstream[struct {
		Deleted int64 `json:"deleted"`
	}](r.Context(), h.s.Upstream, http.MethodPost, "/admin/audit-logs/clear", a.Tokens.Access, "", "", struct {
		Code string `json:"totp_code"`
	}{input.TOTP})
	return platform.Send(w, http.StatusOK, result, err)
}
