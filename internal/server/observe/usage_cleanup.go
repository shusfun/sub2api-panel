package observe

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

const usageCleanupPath = "/admin/usage/cleanup-tasks"

// 用量清理任务的响应白名单：只保留任务状态、筛选条件和删除行数，不下发创建人之外的操作者信息和原始错误细节。
type usageCleanupFilters struct {
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	UserID      *int64    `json:"user_id,omitempty"`
	APIKeyID    *int64    `json:"api_key_id,omitempty"`
	AccountID   *int64    `json:"account_id,omitempty"`
	GroupID     *int64    `json:"group_id,omitempty"`
	Model       *string   `json:"model,omitempty"`
	Stream      *bool     `json:"stream,omitempty"`
	BillingType *int8     `json:"billing_type,omitempty"`
}

type usageCleanupTask struct {
	ID          int64               `json:"id"`
	Status      string              `json:"status"`
	Filters     usageCleanupFilters `json:"filters"`
	DeletedRows int64               `json:"deleted_rows"`
	StartedAt   *time.Time          `json:"started_at,omitempty"`
	FinishedAt  *time.Time          `json:"finished_at,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
}

type usageCleanupInput struct {
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	UserID      *int64  `json:"user_id,omitempty"`
	APIKeyID    *int64  `json:"api_key_id,omitempty"`
	AccountID   *int64  `json:"account_id,omitempty"`
	GroupID     *int64  `json:"group_id,omitempty"`
	Model       *string `json:"model,omitempty"`
	Stream      *bool   `json:"stream,omitempty"`
	BillingType *int8   `json:"billing_type,omitempty"`
	Timezone    string  `json:"timezone,omitempty"`
	OperationID string  `json:"operation_id"`
}

// 上游请求体不带 operation_id，幂等键走请求头。
type usageCleanupBody struct {
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	UserID      *int64  `json:"user_id,omitempty"`
	APIKeyID    *int64  `json:"api_key_id,omitempty"`
	AccountID   *int64  `json:"account_id,omitempty"`
	GroupID     *int64  `json:"group_id,omitempty"`
	Model       *string `json:"model,omitempty"`
	Stream      *bool   `json:"stream,omitempty"`
	BillingType *int8   `json:"billing_type,omitempty"`
	Timezone    string  `json:"timezone,omitempty"`
}

func positive(v *int64) bool { return v == nil || *v > 0 }

func validUsageCleanup(in usageCleanupInput) bool {
	start, err1 := time.Parse("2006-01-02", strings.TrimSpace(in.StartDate))
	end, err2 := time.Parse("2006-01-02", strings.TrimSpace(in.EndDate))
	if err1 != nil || err2 != nil || end.Before(start) {
		return false
	}
	if in.OperationID == "" || len(in.OperationID) > 100 || len(in.Timezone) > 64 {
		return false
	}
	if in.Model != nil && len(*in.Model) > 200 {
		return false
	}
	return positive(in.UserID) && positive(in.APIKeyID) && positive(in.AccountID) && positive(in.GroupID)
}

func projectCleanupTask(t usageCleanupTask) (usageCleanupTask, error) {
	return t, platform.RequiredID(t.ID)
}

func listUsageCleanupTasks(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		page, params, err := platform.ListQuery(r)
		if err != nil {
			return err
		}
		rows, err := core.QueryPage(r.Context(), s.Upstream, a.Tokens.Access, usageCleanupPath, page, params, projectCleanupTask)
		return platform.Send(w, http.StatusOK, rows, err)
	}
}

func createUsageCleanupTask(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		input, err := platform.DecodeBody[usageCleanupInput](r)
		if err != nil || !validUsageCleanup(input) {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
		body := usageCleanupBody{
			StartDate: strings.TrimSpace(input.StartDate), EndDate: strings.TrimSpace(input.EndDate), UserID: input.UserID,
			APIKeyID: input.APIKeyID, AccountID: input.AccountID, GroupID: input.GroupID, Model: input.Model,
			Stream: input.Stream, BillingType: input.BillingType, Timezone: input.Timezone,
		}
		row, err := platform.Upstream[usageCleanupTask](r.Context(), s.Upstream, http.MethodPost, usageCleanupPath, a.Tokens.Access, "", input.OperationID, body)
		if err == nil {
			row, err = projectCleanupTask(row)
		}
		return platform.Send(w, http.StatusCreated, row, err)
	}
}

func cancelUsageCleanupTask(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		row, err := platform.Upstream[struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		}](r.Context(), s.Upstream, http.MethodPost, fmt.Sprintf("%s/%d/cancel", usageCleanupPath, id), a.Tokens.Access, "", "", struct{}{})
		return platform.Send(w, http.StatusOK, row, err)
	}
}
