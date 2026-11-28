package system

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

const promptAuditPath = "/admin/prompt-audit"

// 提示词审计的响应白名单：事件只保留哈希、长度、计数和判定结果，
// 不下发 full_prompt、redacted_preview、scanner_evidence 和 issue 的 evidence 原文。
type promptEndpointView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	BaseURL     string `json:"base_url"`
	Model       string `json:"model"`
	TimeoutMS   int    `json:"timeout_ms"`
	InputLimit  int    `json:"input_limit"`
	Enabled     bool   `json:"enabled"`
	HasToken    bool   `json:"has_token"`
	TokenStatus string `json:"token_status"`
}

type promptConfigView struct {
	Enabled                bool                 `json:"enabled"`
	BlockingEnabled        bool                 `json:"blocking_enabled"`
	BlockingLatestTurnOnly bool                 `json:"blocking_latest_turn_only"`
	StorePassEvents        bool                 `json:"store_pass_events"`
	EffectiveMode          string               `json:"effective_mode"`
	Strategy               string               `json:"strategy"`
	WorkerCount            int                  `json:"worker_count"`
	QueueCapacity          int                  `json:"queue_capacity"`
	Scanners               []string             `json:"scanners"`
	AllGroups              bool                 `json:"all_groups"`
	GroupIDs               []int64              `json:"group_ids"`
	Endpoints              []promptEndpointView `json:"endpoints"`
	ConfigVersion          int64                `json:"config_version"`
	UpdatedAt              time.Time            `json:"updated_at"`
	ChangeSummary          string               `json:"change_summary"`
}

type promptEndpointInput struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	Token      string `json:"token,omitempty"`
	ClearToken bool   `json:"clear_token"`
	TimeoutMS  int    `json:"timeout_ms"`
	InputLimit int    `json:"input_limit"`
	Enabled    bool   `json:"enabled"`
}

type promptConfigInput struct {
	ExpectedConfigVersion  int64                 `json:"expected_config_version"`
	Enabled                bool                  `json:"enabled"`
	BlockingEnabled        bool                  `json:"blocking_enabled"`
	BlockingLatestTurnOnly bool                  `json:"blocking_latest_turn_only"`
	StorePassEvents        bool                  `json:"store_pass_events"`
	Strategy               string                `json:"strategy"`
	WorkerCount            int                   `json:"worker_count"`
	QueueCapacity          int                   `json:"queue_capacity"`
	Scanners               []string              `json:"scanners"`
	AllGroups              bool                  `json:"all_groups"`
	GroupIDs               []int64               `json:"group_ids"`
	Endpoints              []promptEndpointInput `json:"endpoints"`
}

type promptProbeResult struct {
	OK           bool      `json:"ok"`
	Status       string    `json:"status"`
	ErrorCode    string    `json:"error_code,omitempty"`
	Message      string    `json:"message"`
	LatencyMS    int       `json:"latency_ms"`
	HTTPStatus   int       `json:"http_status"`
	Retryable    bool      `json:"retryable"`
	CheckedAt    time.Time `json:"checked_at"`
	TokenApplied bool      `json:"token_applied"`
}

type promptRuntime struct {
	ProcessStatus         string                       `json:"process_status"`
	EffectiveMode         string                       `json:"effective_mode"`
	ExpectedConfigVersion int64                        `json:"expected_config_version"`
	ActiveConfigVersion   int64                        `json:"active_config_version"`
	WorkerTotal           int                          `json:"worker_total"`
	WorkerActive          int64                        `json:"worker_active"`
	WorkerHeartbeatAt     *time.Time                   `json:"worker_heartbeat_at,omitempty"`
	QueueCapacity         int                          `json:"queue_capacity"`
	Queue                 map[string]int64             `json:"queue"`
	ProcessedTotal        int64                        `json:"processed_total"`
	FailedTotal           int64                        `json:"failed_total"`
	EnqueuedTotal         int64                        `json:"enqueued_total"`
	DroppedTotal          int64                        `json:"dropped_total"`
	LastProcessedAt       *time.Time                   `json:"last_processed_at,omitempty"`
	LastErrorCode         string                       `json:"last_error_code,omitempty"`
	DatabaseStatus        string                       `json:"database_status"`
	RedisStatus           string                       `json:"redis_status"`
	Endpoints             map[string]promptProbeResult `json:"endpoints"`
	GuardMetrics          map[string]int64             `json:"guard_metrics"`
}

type promptIssue struct {
	Category      string  `json:"category"`
	ScannerID     string  `json:"scanner_id"`
	Title         string  `json:"title"`
	Severity      string  `json:"severity"`
	SeverityLabel string  `json:"severity_label"`
	Action        string  `json:"action"`
	ActionLabel   string  `json:"action_label"`
	Code          string  `json:"code"`
	Score         float64 `json:"score"`
}

type promptEvent struct {
	ID       int64 `json:"id"`
	Snapshot struct {
		RequestID    string `json:"request_id"`
		UserID       int64  `json:"user_id"`
		UserEmail    string `json:"user_email"`
		APIKeyID     int64  `json:"api_key_id"`
		APIKeyName   string `json:"api_key_name"`
		GroupID      *int64 `json:"group_id,omitempty"`
		GroupName    string `json:"group_name"`
		Provider     string `json:"provider"`
		Endpoint     string `json:"endpoint"`
		Model        string `json:"model"`
		PromptHash   string `json:"prompt_hash"`
		PromptLength int    `json:"prompt_length"`
		MessageCount int    `json:"message_count"`
		Stage        string `json:"stage"`
	} `json:"snapshot"`
	Decision        string             `json:"decision"`
	RiskLevel       string             `json:"risk_level"`
	Action          string             `json:"action"`
	Categories      []string           `json:"categories"`
	MatchedScanners []string           `json:"matched_scanners"`
	ScannerScores   map[string]float64 `json:"scanner_scores"`
	GuardEndpointID string             `json:"guard_endpoint_id"`
	ChunkTotal      int                `json:"chunk_total"`
	LatencyMS       int                `json:"latency_ms"`
	IssueSummaries  []promptIssue      `json:"issue_summaries"`
	CreatedAt       time.Time          `json:"created_at"`
}

type promptEventFilter struct {
	Decision   string     `json:"decision,omitempty"`
	RiskLevel  string     `json:"risk_level,omitempty"`
	Endpoint   string     `json:"endpoint,omitempty"`
	GroupID    *int64     `json:"group_id,omitempty"`
	UserID     *int64     `json:"user_id,omitempty"`
	APIKeyID   *int64     `json:"api_key_id,omitempty"`
	RequestID  string     `json:"request_id,omitempty"`
	PromptHash string     `json:"prompt_hash,omitempty"`
	Keyword    string     `json:"keyword,omitempty"`
	StartAt    *time.Time `json:"start_at,omitempty"`
	EndAt      *time.Time `json:"end_at,omitempty"`
}

type promptDeletePreview struct {
	MatchedCount      int64             `json:"matched_count"`
	FilterSummary     promptEventFilter `json:"filter_summary"`
	SnapshotMaxID     int64             `json:"snapshot_max_id"`
	FilterHash        string            `json:"filter_hash"`
	ConfirmationToken string            `json:"confirmation_token,omitempty"`
	ExpiresAt         time.Time         `json:"expires_at"`
}

type promptDeleteResult struct {
	DeletedEvents int64 `json:"deleted_events"`
	DeletedJobs   int64 `json:"deleted_jobs"`
}

func (h *handler) promptAuditRoutes(mux *http.ServeMux) {
	base := "/api" + promptAuditPath
	h.s.Route(mux, "GET "+base+"/config", true, h.promptConfig)
	h.s.Route(mux, "PUT "+base+"/config", true, h.promptConfig)
	h.s.Route(mux, "POST "+base+"/endpoints/probe", true, h.promptProbe)
	h.s.Route(mux, "GET "+base+"/runtime", true, h.promptRuntime)
	h.s.Route(mux, "GET "+base+"/events", true, h.promptEvents)
	h.s.Route(mux, "GET "+base+"/events/{id}", true, h.promptEvent)
	h.s.Route(mux, "DELETE "+base+"/events/{id}", true, h.promptEventDelete)
	h.s.Route(mux, "POST "+base+"/events/batch-delete", true, h.promptBatchDelete)
	h.s.Route(mux, "POST "+base+"/events/delete-preview", true, h.promptDeletePreview)
	h.s.Route(mux, "POST "+base+"/events/delete-by-filter", true, h.promptDeleteByFilter)
}

func projectPromptConfig(c promptConfigView) promptConfigView {
	if c.Scanners == nil {
		c.Scanners = []string{}
	}
	if c.GroupIDs == nil {
		c.GroupIDs = []int64{}
	}
	if c.Endpoints == nil {
		c.Endpoints = []promptEndpointView{}
	}
	return c
}

func validPromptEndpoint(e promptEndpointInput) bool {
	return strings.TrimSpace(e.ID) != "" && len(e.ID) <= 64 && strings.TrimSpace(e.Name) != "" && len(e.Name) <= 100 &&
		strings.TrimSpace(e.BaseURL) != "" && len(e.BaseURL) <= 500 && len(e.Model) <= 200 && len(e.Token) <= 2000 &&
		e.TimeoutMS >= 0 && e.InputLimit >= 0
}

func (h *handler) promptConfig(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	var payload any
	if r.Method == http.MethodPut {
		input, err := platform.DecodeBody[promptConfigInput](r)
		if err != nil {
			return err
		}
		if input.ExpectedConfigVersion < 1 || len(input.Scanners) > 50 || len(input.GroupIDs) > 1000 || len(input.Endpoints) > 20 {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
		for _, e := range input.Endpoints {
			if !validPromptEndpoint(e) {
				return platform.Fail(http.StatusBadRequest, "invalid_input")
			}
		}
		if input.Scanners == nil {
			input.Scanners = []string{}
		}
		if input.GroupIDs == nil {
			input.GroupIDs = []int64{}
		}
		if input.Endpoints == nil {
			input.Endpoints = []promptEndpointInput{}
		}
		payload = input
	}
	row, err := platform.Upstream[promptConfigView](r.Context(), h.s.Upstream, r.Method, promptAuditPath+"/config", a.Tokens.Access, "", "", payload)
	return platform.Send(w, http.StatusOK, projectPromptConfig(row), err)
}

func (h *handler) promptProbe(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Endpoint promptEndpointInput `json:"endpoint"`
	}](r)
	if err != nil || !validPromptEndpoint(input.Endpoint) {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	row, err := platform.Upstream[promptProbeResult](r.Context(), h.s.Upstream, http.MethodPost, promptAuditPath+"/endpoints/probe", a.Tokens.Access, "", "", input)
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) promptRuntime(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	row, err := platform.Upstream[promptRuntime](r.Context(), h.s.Upstream, http.MethodGet, promptAuditPath+"/runtime", a.Tokens.Access, "", "", nil)
	if err == nil {
		if row.Endpoints == nil {
			row.Endpoints = map[string]promptProbeResult{}
		}
		if row.Queue == nil {
			row.Queue = map[string]int64{}
		}
		if row.GuardMetrics == nil {
			row.GuardMetrics = map[string]int64{}
		}
	}
	return platform.Send(w, http.StatusOK, row, err)
}

func projectPromptEvent(e promptEvent) (promptEvent, error) {
	if e.Categories == nil {
		e.Categories = []string{}
	}
	if e.MatchedScanners == nil {
		e.MatchedScanners = []string{}
	}
	if e.ScannerScores == nil {
		e.ScannerScores = map[string]float64{}
	}
	if e.IssueSummaries == nil {
		e.IssueSummaries = []promptIssue{}
	}
	return e, platform.RequiredID(e.ID)
}

func (h *handler) promptEvents(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, query, err := platform.ListQuery(r, "decision", "risk_level", "endpoint", "group_id", "user_id", "api_key_id",
		"request_id", "prompt_hash", "keyword", "start_at", "end_at")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, promptAuditPath+"/events", page, query, projectPromptEvent)
	return platform.Send(w, http.StatusOK, rows, err)
}

func (h *handler) promptEvent(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	row, err := platform.Upstream[promptEvent](r.Context(), h.s.Upstream, http.MethodGet, fmt.Sprintf("%s/events/%d", promptAuditPath, id), a.Tokens.Access, "", "", nil)
	if err == nil {
		row, err = projectPromptEvent(row)
	}
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) promptEventDelete(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	row, err := platform.Upstream[promptDeleteResult](r.Context(), h.s.Upstream, http.MethodDelete, fmt.Sprintf("%s/events/%d", promptAuditPath, id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) promptBatchDelete(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		IDs []int64 `json:"ids"`
	}](r)
	if err != nil || len(input.IDs) == 0 || len(input.IDs) > 500 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	for _, id := range input.IDs {
		if id < 1 {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
	}
	row, err := platform.Upstream[promptDeleteResult](r.Context(), h.s.Upstream, http.MethodPost, promptAuditPath+"/events/batch-delete", a.Tokens.Access, "", "", input)
	return platform.Send(w, http.StatusOK, row, err)
}

func validPromptFilter(f promptEventFilter) bool {
	return len(f.Decision) <= 32 && len(f.RiskLevel) <= 32 && len(f.Endpoint) <= 200 && len(f.RequestID) <= 200 &&
		len(f.PromptHash) <= 128 && len(f.Keyword) <= 200
}

func (h *handler) promptDeletePreview(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	filter, err := platform.DecodeBody[promptEventFilter](r)
	if err != nil || !validPromptFilter(filter) {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	row, err := platform.Upstream[promptDeletePreview](r.Context(), h.s.Upstream, http.MethodPost, promptAuditPath+"/events/delete-preview", a.Tokens.Access, "", "", filter)
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) promptDeleteByFilter(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Filter            promptEventFilter `json:"filter"`
		SnapshotMaxID     int64             `json:"snapshot_max_id"`
		FilterHash        string            `json:"filter_hash"`
		ConfirmationToken string            `json:"confirmation_token"`
		Confirm           bool              `json:"confirm"`
	}](r)
	if err != nil || !input.Confirm || !validPromptFilter(input.Filter) || input.SnapshotMaxID < 0 ||
		input.FilterHash == "" || len(input.FilterHash) > 128 || input.ConfirmationToken == "" || len(input.ConfirmationToken) > 2000 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	row, err := platform.Upstream[promptDeleteResult](r.Context(), h.s.Upstream, http.MethodPost, promptAuditPath+"/events/delete-by-filter", a.Tokens.Access, "", "", input)
	return platform.Send(w, http.StatusOK, row, err)
}
