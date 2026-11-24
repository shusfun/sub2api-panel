package observe

import (
	"fmt"
	"net/http"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type opsRequestRow struct {
	Kind         string `json:"kind"`
	CreatedAt    string `json:"created_at"`
	RequestID    string `json:"request_id"`
	Platform     string `json:"platform"`
	Model        string `json:"model"`
	DurationMs   *int64 `json:"duration_ms"`
	FirstTokenMs *int64 `json:"first_token_ms"`
	StatusCode   *int   `json:"status_code"`
	ErrorID      *int64 `json:"error_id"`
	Phase        string `json:"phase"`
	Severity     string `json:"severity"`
	UserID       *int64 `json:"user_id"`
	APIKeyID     *int64 `json:"api_key_id"`
	AccountID    *int64 `json:"account_id"`
	GroupID      *int64 `json:"group_id"`
	Stream       bool   `json:"stream"`
}

type opsErrorRowWire struct {
	ID                 int64   `json:"id"`
	CreatedAt          string  `json:"created_at"`
	Phase              string  `json:"phase"`
	Type               string  `json:"type"`
	ErrorOwner         string  `json:"error_owner"`
	ErrorSource        string  `json:"error_source"`
	Severity           string  `json:"severity"`
	StatusCode         int     `json:"status_code"`
	UpstreamStatusCode *int    `json:"upstream_status_code"`
	Platform           string  `json:"platform"`
	Model              string  `json:"model"`
	RequestedModel     string  `json:"requested_model"`
	UpstreamModel      string  `json:"upstream_model"`
	InboundEndpoint    string  `json:"inbound_endpoint"`
	UpstreamEndpoint   string  `json:"upstream_endpoint"`
	Resolved           bool    `json:"resolved"`
	ResolvedAt         *string `json:"resolved_at"`
	IsBusinessLimited  bool    `json:"is_business_limited"`
	ClientRequestID    string  `json:"client_request_id"`
	RequestID          string  `json:"request_id"`
	UserID             *int64  `json:"user_id"`
	UserEmail          string  `json:"user_email"`
	APIKeyID           *int64  `json:"api_key_id"`
	APIKeyName         string  `json:"api_key_name"`
	AccountID          *int64  `json:"account_id"`
	AccountName        string  `json:"account_name"`
	GroupID            *int64  `json:"group_id"`
	GroupName          string  `json:"group_name"`
	Stream             bool    `json:"stream"`
	AuthLatencyMs      *int64  `json:"auth_latency_ms"`
	RoutingLatencyMs   *int64  `json:"routing_latency_ms"`
	UpstreamLatencyMs  *int64  `json:"upstream_latency_ms"`
	ResponseLatencyMs  *int64  `json:"response_latency_ms"`
	TTFTMs             *int64  `json:"time_to_first_token_ms"`
}

type opsErrorView struct {
	ID                 int64   `json:"id"`
	CreatedAt          string  `json:"created_at"`
	Phase              string  `json:"phase"`
	Type               string  `json:"type"`
	ErrorOwner         string  `json:"error_owner"`
	ErrorSource        string  `json:"error_source"`
	Severity           string  `json:"severity"`
	StatusCode         int     `json:"status_code"`
	UpstreamStatusCode *int    `json:"upstream_status_code,omitempty"`
	Platform           string  `json:"platform"`
	Model              string  `json:"model"`
	RequestedModel     string  `json:"requested_model,omitempty"`
	UpstreamModel      string  `json:"upstream_model,omitempty"`
	InboundEndpoint    string  `json:"inbound_endpoint,omitempty"`
	UpstreamEndpoint   string  `json:"upstream_endpoint,omitempty"`
	Resolved           bool    `json:"resolved"`
	ResolvedAt         *string `json:"resolved_at,omitempty"`
	IsBusinessLimited  bool    `json:"is_business_limited"`
	ClientRequestID    string  `json:"client_request_id,omitempty"`
	RequestID          string  `json:"request_id"`
	UserID             *int64  `json:"user_id"`
	UserEmail          string  `json:"user_email"`
	APIKeyID           *int64  `json:"api_key_id"`
	APIKeyName         string  `json:"api_key_name"`
	AccountID          *int64  `json:"account_id"`
	AccountName        string  `json:"account_name"`
	GroupID            *int64  `json:"group_id"`
	GroupName          string  `json:"group_name"`
	Stream             bool    `json:"stream"`
	AuthLatencyMs      *int64  `json:"auth_latency_ms"`
	RoutingLatencyMs   *int64  `json:"routing_latency_ms"`
	UpstreamLatencyMs  *int64  `json:"upstream_latency_ms"`
	ResponseLatencyMs  *int64  `json:"response_latency_ms"`
	TTFTMs             *int64  `json:"time_to_first_token_ms"`
}

func projectOpsError(row opsErrorRowWire) (opsErrorView, error) {
	if row.ID < 1 || row.CreatedAt == "" {
		return opsErrorView{}, platform.Fail(502, "invalid_upstream")
	}
	return opsErrorView(row), nil
}

func opsRequests(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		page, params, err := platform.ListQuery(r, "time_range", "kind", "platform", "group_id", "user_id", "api_key_id", "account_id", "model", "request_id", "q", "sort")
		if err != nil {
			return err
		}
		if !validOpsTimeRange(params.Get("time_range")) {
			return platform.Fail(400, "invalid_input")
		}
		rows, err := core.QueryPage(r.Context(), s.Upstream, a.Tokens.Access, "/admin/ops/requests", page, params, func(item opsRequestRow) (opsRequestRow, error) {
			if item.CreatedAt == "" {
				return opsRequestRow{}, platform.Fail(502, "invalid_upstream")
			}
			return item, nil
		})
		return platform.Send(w, 200, rows, err)
	}
}

func opsErrorList(s *core.Server, upstreamPath string) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		page, params, err := platform.ListQuery(r, "time_range", "platform", "group_id", "account_id", "user_id", "api_key_id", "model", "phase", "category", "error_owner", "resolved", "view", "q", "status_codes", "sort_by", "sort_order")
		if err != nil {
			return err
		}
		rows, err := core.QueryPage(r.Context(), s.Upstream, a.Tokens.Access, upstreamPath, page, params, projectOpsError)
		return platform.Send(w, 200, rows, err)
	}
}

func opsErrorDetail(s *core.Server, upstreamPath string) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		row, err := platform.Upstream[opsErrorRowWire](r.Context(), s.Upstream, "GET", fmt.Sprintf("%s/%d", upstreamPath, id), a.Tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		view, err := projectOpsError(row)
		return platform.Send(w, 200, view, err)
	}
}

func opsRequestUpstreamErrors(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		page, err := platform.RequestedPage(r)
		if err != nil {
			return err
		}
		rows, err := core.QueryPage(r.Context(), s.Upstream, a.Tokens.Access, fmt.Sprintf("/admin/ops/request-errors/%d/upstream-errors", id), page, nil, projectOpsError)
		return platform.Send(w, 200, rows, err)
	}
}

func opsErrorResolve(s *core.Server, upstreamPath string) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		input, err := platform.DecodeBody[struct {
			Resolved *bool `json:"resolved"`
		}](r)
		if err != nil || input.Resolved == nil {
			return platform.Fail(400, "invalid_input")
		}
		_, err = platform.Upstream[struct{}](r.Context(), s.Upstream, "PUT", fmt.Sprintf("%s/%d/resolve", upstreamPath, id), a.Tokens.Access, "", "", input)
		return platform.Send(w, 200, struct{}{}, err)
	}
}
