package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type opsStreamEvent struct {
	Type       string          `json:"type"`
	Snapshot   json.RawMessage `json:"snapshot,omitempty"`
	RequestID  string          `json:"request_id,omitempty"`
	SampleID   string          `json:"sample_id,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	ServerTime string          `json:"server_time"`
}

type opsStreamSubscriber struct {
	ctx         context.Context
	cancel      context.CancelFunc
	token       string
	query       string
	trace       callMeta
	channels    map[chan []byte]callMeta
	latest      []byte
	latestEvent []byte
}

type opsStreamHub struct {
	mu      sync.Mutex
	entries map[string]*opsStreamSubscriber
	build   func(context.Context, string, string) ([]byte, error)
}

func newOpsStreamHub(build func(context.Context, string, string) ([]byte, error)) *opsStreamHub {
	return &opsStreamHub{entries: make(map[string]*opsStreamSubscriber), build: build}
}

func (h *opsStreamHub) subscribe(ctx context.Context, scope, token, query string, trace callMeta) (<-chan []byte, func()) {
	h.mu.Lock()
	entry := h.entries[scope]
	if entry == nil {
		worker, cancel := context.WithCancel(context.WithoutCancel(ctx))
		entry = &opsStreamSubscriber{ctx: worker, cancel: cancel, query: query, trace: trace, channels: make(map[chan []byte]callMeta)}
		h.entries[scope] = entry
		go h.sample(scope, entry)
	}
	entry.token = token
	ch := make(chan []byte, 1)
	entry.channels[ch] = trace
	if entry.latestEvent != nil {
		var event opsStreamEvent
		if json.Unmarshal(entry.latestEvent, &event) == nil {
			event.RequestID, event.SampleID = trace.Trace, entry.trace.Trace
			if payload, err := json.Marshal(event); err == nil {
				ch <- payload
			}
		}
	}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.entries[scope] != entry {
			return
		}
		delete(entry.channels, ch)
		if len(entry.channels) == 0 {
			entry.cancel()
			delete(h.entries, scope)
		}
	}
}

func (h *opsStreamHub) sample(_ string, entry *opsStreamSubscriber) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	failed := false
	for {
		h.mu.Lock()
		token, trace := entry.token, entry.trace
		h.mu.Unlock()
		ctx := context.WithValue(entry.ctx, metaKey{}, trace)
		payload, err := h.build(ctx, token, entry.query)
		if ctx.Err() != nil {
			return
		}
		changed := !bytes.Equal(payload, entry.latest)
		if err == nil && (changed || failed) {
			entry.latest = payload
			h.broadcast(entry, opsStreamEvent{Type: "snapshot", Snapshot: json.RawMessage(payload)}, trace)
			failed = false
		}
		if err != nil && ctx.Err() == nil {
			reason := "upstream_error"
			var prob *problem
			if errors.As(err, &prob) && (prob.Status == 401 || prob.Status == 403) {
				reason = "not_allowed"
			}
			if !failed || reason == "not_allowed" {
				slog.Info("ops sample unavailable", "trace_id", trace.Trace, "reason", reason, "error", err)
				h.broadcast(entry, opsStreamEvent{Type: "unavailable", Reason: reason}, trace)
			}
			failed = true
		}
		select {
		case <-entry.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *opsStreamHub) broadcast(entry *opsStreamSubscriber, event opsStreamEvent, trace callMeta) {
	event.ServerTime = time.Now().UTC().Format(time.RFC3339Nano)
	event.RequestID = trace.Trace
	event.SampleID = trace.Trace
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	entry.latestEvent = payload
	for ch, subscriber := range entry.channels {
		sub := event
		sub.RequestID, sub.SampleID = subscriber.Trace, trace.Trace
		subPayload, err := json.Marshal(sub)
		if err != nil {
			continue
		}
		select {
		case <-ch:
		default:
		}
		ch <- subPayload
	}
}

type opsRateView struct {
	Current float64 `json:"current"`
	Peak    float64 `json:"peak"`
	Avg     float64 `json:"avg"`
}

type opsPercentilesView struct {
	P50 *float64 `json:"p50_ms"`
	P90 *float64 `json:"p90_ms"`
	P95 *float64 `json:"p95_ms"`
	P99 *float64 `json:"p99_ms"`
	Avg *float64 `json:"avg_ms"`
	Max *float64 `json:"max_ms"`
}

type opsSystemMetricsView struct {
	CPUUsagePercent    *float64 `json:"cpu_usage_percent"`
	MemoryUsedMB       *float64 `json:"memory_used_mb"`
	MemoryTotalMB      *float64 `json:"memory_total_mb"`
	MemoryUsagePercent *float64 `json:"memory_usage_percent"`
	DBOK               *bool    `json:"db_ok"`
	RedisOK            *bool    `json:"redis_ok"`
	GoroutineCount     *int64   `json:"goroutine_count"`
	ConcurrencyQueue   *int64   `json:"concurrency_queue_depth"`
	AccountSwitchCount *int64   `json:"account_switch_count"`
}

type opsOverviewSummary struct {
	StartTime          string                `json:"start_time"`
	EndTime            string                `json:"end_time"`
	Platform           string                `json:"platform"`
	GroupID            *int64                `json:"group_id"`
	HealthScore        *float64              `json:"health_score"`
	SuccessCount       int64                 `json:"success_count"`
	ErrorCountTotal    int64                 `json:"error_count_total"`
	BusinessLimited    int64                 `json:"business_limited_count"`
	ErrorCountSLA      int64                 `json:"error_count_sla"`
	RequestCountTotal  int64                 `json:"request_count_total"`
	RequestCountSLA    int64                 `json:"request_count_sla"`
	TokenConsumed      int64                 `json:"token_consumed"`
	SLA                float64               `json:"sla"`
	ErrorRate          float64               `json:"error_rate"`
	UpstreamErrorRate  float64               `json:"upstream_error_rate"`
	UpstreamExcl429529 int64                 `json:"upstream_error_count_excl_429_529"`
	Upstream429Count   int64                 `json:"upstream_429_count"`
	Upstream529Count   int64                 `json:"upstream_529_count"`
	QPS                opsRateView           `json:"qps"`
	TPS                opsRateView           `json:"tps"`
	Duration           opsPercentilesView    `json:"duration"`
	TTFT               opsPercentilesView    `json:"ttft"`
	SystemMetrics      *opsSystemMetricsView `json:"system_metrics"`
}

type opsThroughputPoint struct {
	BucketStart   string  `json:"bucket_start"`
	RequestCount  int64   `json:"request_count"`
	TokenConsumed int64   `json:"token_consumed"`
	SwitchCount   int64   `json:"switch_count"`
	QPS           float64 `json:"qps"`
	TPS           float64 `json:"tps"`
}

type opsErrorTrendPoint struct {
	BucketStart        string `json:"bucket_start"`
	ErrorCountTotal    int64  `json:"error_count_total"`
	BusinessLimited    int64  `json:"business_limited_count"`
	ErrorCountSLA      int64  `json:"error_count_sla"`
	UpstreamExcl429529 int64  `json:"upstream_error_count_excl_429_529"`
	Upstream429Count   int64  `json:"upstream_429_count"`
	Upstream529Count   int64  `json:"upstream_529_count"`
}

type opsLatencyBucket struct {
	Range string `json:"range"`
	Count int64  `json:"count"`
}

type opsErrorDistributionItem struct {
	StatusCode      int   `json:"status_code"`
	Total           int64 `json:"total"`
	SLA             int64 `json:"sla"`
	BusinessLimited int64 `json:"business_limited"`
}

type opsOverviewView struct {
	GeneratedAt       string                     `json:"generated_at"`
	Overview          opsOverviewSummary         `json:"overview"`
	ThroughputPoints  []opsThroughputPoint       `json:"throughput_points"`
	ErrorPoints       []opsErrorTrendPoint       `json:"error_points"`
	LatencyBuckets    []opsLatencyBucket         `json:"latency_buckets"`
	ErrorDistribution []opsErrorDistributionItem `json:"error_distribution"`
}

type opsPlatformConcurrency struct {
	Platform       string  `json:"platform"`
	CurrentInUse   int64   `json:"current_in_use"`
	MaxCapacity    int64   `json:"max_capacity"`
	LoadPercentage float64 `json:"load_percentage"`
	WaitingInQueue int64   `json:"waiting_in_queue"`
	TotalAccounts  int64   `json:"total_accounts"`
	AvailableCount int64   `json:"available_count"`
	RateLimitCount int64   `json:"rate_limit_count"`
	ErrorCount     int64   `json:"error_count"`
}

type opsGroupConcurrency struct {
	GroupID        int64   `json:"group_id"`
	GroupName      string  `json:"group_name"`
	Platform       string  `json:"platform"`
	CurrentInUse   int64   `json:"current_in_use"`
	MaxCapacity    int64   `json:"max_capacity"`
	LoadPercentage float64 `json:"load_percentage"`
	WaitingInQueue int64   `json:"waiting_in_queue"`
	TotalAccounts  int64   `json:"total_accounts"`
	AvailableCount int64   `json:"available_count"`
	RateLimitCount int64   `json:"rate_limit_count"`
	ErrorCount     int64   `json:"error_count"`
}

type opsAccountRuntime struct {
	AccountID        int64   `json:"account_id"`
	AccountName      string  `json:"account_name"`
	Platform         string  `json:"platform"`
	GroupID          int64   `json:"group_id"`
	GroupName        string  `json:"group_name"`
	Status           string  `json:"status"`
	CurrentInUse     int64   `json:"current_in_use"`
	MaxCapacity      int64   `json:"max_capacity"`
	LoadPercentage   float64 `json:"load_percentage"`
	WaitingInQueue   int64   `json:"waiting_in_queue"`
	IsAvailable      bool    `json:"is_available"`
	IsRateLimited    bool    `json:"is_rate_limited"`
	RateLimitResetAt string  `json:"rate_limit_reset_at,omitempty"`
	IsOverloaded     bool    `json:"is_overloaded"`
	OverloadUntil    string  `json:"overload_until,omitempty"`
	HasError         bool    `json:"has_error"`
}

type opsUserConcurrency struct {
	UserID         int64   `json:"user_id"`
	UserEmail      string  `json:"user_email"`
	Username       string  `json:"username"`
	CurrentInUse   int64   `json:"current_in_use"`
	MaxCapacity    int64   `json:"max_capacity"`
	LoadPercentage float64 `json:"load_percentage"`
	WaitingInQueue int64   `json:"waiting_in_queue"`
}

type opsConcurrencyView struct {
	Enabled   bool                     `json:"enabled"`
	Timestamp string                   `json:"timestamp"`
	QPS       opsRateView              `json:"qps"`
	TPS       opsRateView              `json:"tps"`
	Platforms []opsPlatformConcurrency `json:"platforms"`
	Groups    []opsGroupConcurrency    `json:"groups"`
	Accounts  []opsAccountRuntime      `json:"accounts"`
	Users     []opsUserConcurrency     `json:"users"`
}

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
		return opsErrorView{}, fail(502, "invalid_upstream")
	}
	return opsErrorView(row), nil
}

type opsAlertRule struct {
	ID               int64          `json:"id,omitempty"`
	Name             string         `json:"name"`
	Description      string         `json:"description,omitempty"`
	Enabled          bool           `json:"enabled"`
	MetricType       string         `json:"metric_type"`
	Operator         string         `json:"operator"`
	Threshold        float64        `json:"threshold"`
	WindowMinutes    int            `json:"window_minutes"`
	SustainedMinutes int            `json:"sustained_minutes"`
	Severity         string         `json:"severity"`
	CooldownMinutes  int            `json:"cooldown_minutes"`
	NotifyEmail      bool           `json:"notify_email"`
	Filters          map[string]any `json:"filters,omitempty"`
	LastTriggeredAt  *string        `json:"last_triggered_at,omitempty"`
	CreatedAt        string         `json:"created_at,omitempty"`
	UpdatedAt        string         `json:"updated_at,omitempty"`
}

type opsAlertEvent struct {
	ID             int64          `json:"id"`
	RuleID         int64          `json:"rule_id"`
	Severity       string         `json:"severity"`
	Status         string         `json:"status"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	MetricValue    *float64       `json:"metric_value"`
	ThresholdValue *float64       `json:"threshold_value"`
	Dimensions     map[string]any `json:"dimensions,omitempty"`
	FiredAt        string         `json:"fired_at"`
	ResolvedAt     *string        `json:"resolved_at"`
	EmailSent      bool           `json:"email_sent"`
	CreatedAt      string         `json:"created_at"`
}

type opsMetricThresholds struct {
	SLAPercentMin    *float64 `json:"sla_percent_min"`
	TTFTP99MsMax     *float64 `json:"ttft_p99_ms_max"`
	RequestErrorMax  *float64 `json:"request_error_rate_percent_max"`
	UpstreamErrorMax *float64 `json:"upstream_error_rate_percent_max"`
}

type opsEmailNotification struct {
	Alert struct {
		Enabled               bool     `json:"enabled"`
		Recipients            []string `json:"recipients"`
		MinSeverity           string   `json:"min_severity"`
		RateLimitPerHour      int      `json:"rate_limit_per_hour"`
		BatchingWindowSeconds int      `json:"batching_window_seconds"`
		IncludeResolvedAlerts bool     `json:"include_resolved_alerts"`
	} `json:"alert"`
	Report struct {
		Enabled                     bool     `json:"enabled"`
		Recipients                  []string `json:"recipients"`
		DailySummaryEnabled         bool     `json:"daily_summary_enabled"`
		DailySummarySchedule        string   `json:"daily_summary_schedule"`
		WeeklySummaryEnabled        bool     `json:"weekly_summary_enabled"`
		WeeklySummarySchedule       string   `json:"weekly_summary_schedule"`
		ErrorDigestEnabled          bool     `json:"error_digest_enabled"`
		ErrorDigestSchedule         string   `json:"error_digest_schedule"`
		ErrorDigestMinCount         int      `json:"error_digest_min_count"`
		AccountHealthEnabled        bool     `json:"account_health_enabled"`
		AccountHealthSchedule       string   `json:"account_health_schedule"`
		AccountHealthErrorThreshold float64  `json:"account_health_error_rate_threshold"`
	} `json:"report"`
}

type opsRuntimeLogConfig struct {
	Level                string `json:"level"`
	PersistAccessLogs    bool   `json:"persist_access_logs"`
	EnableSampling       bool   `json:"enable_sampling"`
	SamplingInitial      int    `json:"sampling_initial"`
	SamplingThereafter   int    `json:"sampling_thereafter"`
	Caller               bool   `json:"caller"`
	StacktraceLevel      string `json:"stacktrace_level"`
	RetentionDays        int    `json:"retention_days"`
	RequestRetentionDays int    `json:"request_retention_days"`
}

type opsAdvancedConfig struct {
	DataRetention struct {
		CleanupEnabled             bool   `json:"cleanup_enabled"`
		CleanupSchedule            string `json:"cleanup_schedule"`
		ErrorLogRetentionDays      int    `json:"error_log_retention_days"`
		MinuteMetricsRetentionDays int    `json:"minute_metrics_retention_days"`
		HourlyMetricsRetentionDays int    `json:"hourly_metrics_retention_days"`
	} `json:"data_retention"`
	Aggregation struct {
		AggregationEnabled bool `json:"aggregation_enabled"`
	} `json:"aggregation"`
	OpenAIQuotaAutoPause struct {
		DefaultThreshold5h float64 `json:"default_threshold_5h"`
		DefaultThreshold7d float64 `json:"default_threshold_7d"`
	} `json:"openai_account_quota_auto_pause"`
	IgnoreCountTokensErrors    bool `json:"ignore_count_tokens_errors"`
	IgnoreContextCanceled      bool `json:"ignore_context_canceled"`
	IgnoreNoAvailableAccounts  bool `json:"ignore_no_available_accounts"`
	IgnoreInvalidAPIKeyErrors  bool `json:"ignore_invalid_api_key_errors"`
	IgnoreInsufficientBalance  bool `json:"ignore_insufficient_balance_errors"`
	DisplayOpenAITokenStats    bool `json:"display_openai_token_stats"`
	DisplayAlertEvents         bool `json:"display_alert_events"`
	AutoRefreshEnabled         bool `json:"auto_refresh_enabled"`
	AutoRefreshIntervalSeconds int  `json:"auto_refresh_interval_seconds"`
}

type opsSettingsBundle struct {
	Thresholds opsMetricThresholds  `json:"thresholds"`
	Email      opsEmailNotification `json:"email"`
	Logging    opsRuntimeLogConfig  `json:"logging"`
	Advanced   opsAdvancedConfig    `json:"advanced"`
}

func (p *panel) opsRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api/admin/ops/overview", true, p.opsOverview)
	p.route(mux, "GET /api/admin/ops/concurrency", true, p.opsConcurrency)
	p.route(mux, "GET /api/admin/ops/stream", true, p.opsStream)
	p.route(mux, "GET /api/admin/ops/requests", true, p.opsRequests)
	p.route(mux, "GET /api/admin/ops/request-errors", true, p.opsErrorList("/admin/ops/request-errors"))
	p.route(mux, "GET /api/admin/ops/request-errors/{id}", true, p.opsErrorDetail("/admin/ops/request-errors"))
	p.route(mux, "GET /api/admin/ops/request-errors/{id}/upstream-errors", true, p.opsRequestUpstreamErrors)
	p.route(mux, "PUT /api/admin/ops/request-errors/{id}/resolve", true, p.opsErrorResolve("/admin/ops/request-errors"))
	p.route(mux, "GET /api/admin/ops/upstream-errors", true, p.opsErrorList("/admin/ops/upstream-errors"))
	p.route(mux, "GET /api/admin/ops/upstream-errors/{id}", true, p.opsErrorDetail("/admin/ops/upstream-errors"))
	p.route(mux, "PUT /api/admin/ops/upstream-errors/{id}/resolve", true, p.opsErrorResolve("/admin/ops/upstream-errors"))
	p.route(mux, "GET /api/admin/ops/alert-rules", true, p.listOpsAlertRules)
	p.route(mux, "POST /api/admin/ops/alert-rules", true, p.createOpsAlertRule)
	p.route(mux, "PUT /api/admin/ops/alert-rules/{id}", true, p.updateOpsAlertRule)
	p.route(mux, "DELETE /api/admin/ops/alert-rules/{id}", true, p.deleteOpsAlertRule)
	p.route(mux, "GET /api/admin/ops/alert-events", true, p.listOpsAlertEvents)
	p.route(mux, "PUT /api/admin/ops/alert-events/{id}/status", true, p.updateOpsAlertEventStatus)
	p.route(mux, "POST /api/admin/ops/alert-silences", true, p.createOpsAlertSilence)
	p.route(mux, "GET /api/admin/ops/settings", true, p.getOpsSettings)
	p.route(mux, "PUT /api/admin/ops/settings", true, p.updateOpsSettings)
	p.route(mux, "POST /api/admin/ops/runtime/logging/reset", true, p.resetOpsLogging)
}

func validOpsTimeRange(value string) bool {
	switch value {
	case "", "5m", "30m", "1h", "6h", "24h":
		return true
	default:
		return false
	}
}

func parseOpsScopeQuery(raw url.Values) (url.Values, error) {
	out := url.Values{}
	timeRange := strings.TrimSpace(raw.Get("time_range"))
	if !validOpsTimeRange(timeRange) {
		return nil, fail(400, "invalid_input")
	}
	if timeRange == "" {
		timeRange = "1h"
	}
	out.Set("time_range", timeRange)
	if platform := strings.TrimSpace(raw.Get("platform")); platform != "" {
		if len(platform) > 50 {
			return nil, fail(400, "invalid_input")
		}
		out.Set("platform", platform)
	}
	if groupRaw := strings.TrimSpace(raw.Get("group_id")); groupRaw != "" {
		groupID, err := strconv.ParseInt(groupRaw, 10, 64)
		if err != nil || groupID < 1 {
			return nil, fail(400, "invalid_input")
		}
		out.Set("group_id", strconv.FormatInt(groupID, 10))
	}
	return out, nil
}

func (p *panel) opsOverview(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	params, err := parseOpsScopeQuery(r.URL.Query())
	if err != nil {
		return err
	}
	view, err := p.fetchOpsOverview(r.Context(), a.tokens.Access, params)
	return send(w, 200, view, err)
}

func (p *panel) fetchOpsOverview(ctx context.Context, token string, params url.Values) (opsOverviewView, error) {
	snap, err := upstream[struct {
		GeneratedAt     string             `json:"generated_at"`
		Overview        opsOverviewSummary `json:"overview"`
		ThroughputTrend struct {
			Points []opsThroughputPoint `json:"points"`
		} `json:"throughput_trend"`
		ErrorTrend struct {
			Points []opsErrorTrendPoint `json:"points"`
		} `json:"error_trend"`
	}](ctx, p.upstream, "GET", "/admin/ops/dashboard/snapshot-v2?"+params.Encode(), token, "", "", nil)
	if err != nil {
		return opsOverviewView{}, err
	}
	view := opsOverviewView{
		GeneratedAt:       snap.GeneratedAt,
		Overview:          snap.Overview,
		ThroughputPoints:  snap.ThroughputTrend.Points,
		ErrorPoints:       snap.ErrorTrend.Points,
		LatencyBuckets:    []opsLatencyBucket{},
		ErrorDistribution: []opsErrorDistributionItem{},
	}
	if view.ThroughputPoints == nil {
		view.ThroughputPoints = []opsThroughputPoint{}
	}
	if view.ErrorPoints == nil {
		view.ErrorPoints = []opsErrorTrendPoint{}
	}
	if hist, histErr := upstream[struct {
		Buckets []opsLatencyBucket `json:"buckets"`
	}](ctx, p.upstream, "GET", "/admin/ops/dashboard/latency-histogram?"+params.Encode(), token, "", "", nil); histErr == nil && hist.Buckets != nil {
		view.LatencyBuckets = hist.Buckets
	}
	if dist, distErr := upstream[struct {
		Items []opsErrorDistributionItem `json:"items"`
	}](ctx, p.upstream, "GET", "/admin/ops/dashboard/error-distribution?"+params.Encode(), token, "", "", nil); distErr == nil && dist.Items != nil {
		view.ErrorDistribution = dist.Items
	}
	return view, nil
}

func (p *panel) fetchOpsConcurrency(ctx context.Context, token string, params url.Values) (opsConcurrencyView, error) {
	scope := url.Values{}
	if platform := params.Get("platform"); platform != "" {
		scope.Set("platform", platform)
	}
	if groupID := params.Get("group_id"); groupID != "" {
		scope.Set("group_id", groupID)
	}
	suffix := ""
	if len(scope) > 0 {
		suffix = "?" + scope.Encode()
	}
	concurrency, err := upstream[struct {
		Enabled  bool `json:"enabled"`
		Platform map[string]struct {
			Platform       string  `json:"platform"`
			CurrentInUse   int64   `json:"current_in_use"`
			MaxCapacity    int64   `json:"max_capacity"`
			LoadPercentage float64 `json:"load_percentage"`
			WaitingInQueue int64   `json:"waiting_in_queue"`
		} `json:"platform"`
		Group map[string]struct {
			GroupID        int64   `json:"group_id"`
			GroupName      string  `json:"group_name"`
			Platform       string  `json:"platform"`
			CurrentInUse   int64   `json:"current_in_use"`
			MaxCapacity    int64   `json:"max_capacity"`
			LoadPercentage float64 `json:"load_percentage"`
			WaitingInQueue int64   `json:"waiting_in_queue"`
		} `json:"group"`
		Account map[string]struct {
			AccountID      int64   `json:"account_id"`
			AccountName    string  `json:"account_name"`
			Platform       string  `json:"platform"`
			GroupID        int64   `json:"group_id"`
			GroupName      string  `json:"group_name"`
			CurrentInUse   int64   `json:"current_in_use"`
			MaxCapacity    int64   `json:"max_capacity"`
			LoadPercentage float64 `json:"load_percentage"`
			WaitingInQueue int64   `json:"waiting_in_queue"`
		} `json:"account"`
		Timestamp string `json:"timestamp"`
	}](ctx, p.upstream, "GET", "/admin/ops/concurrency"+suffix, token, "", "", nil)
	if err != nil {
		return opsConcurrencyView{}, err
	}

	availability, _ := upstream[struct {
		Platform map[string]struct {
			TotalAccounts  int64 `json:"total_accounts"`
			AvailableCount int64 `json:"available_count"`
			RateLimitCount int64 `json:"rate_limit_count"`
			ErrorCount     int64 `json:"error_count"`
		} `json:"platform"`
		Group map[string]struct {
			GroupID        int64  `json:"group_id"`
			GroupName      string `json:"group_name"`
			Platform       string `json:"platform"`
			TotalAccounts  int64  `json:"total_accounts"`
			AvailableCount int64  `json:"available_count"`
			RateLimitCount int64  `json:"rate_limit_count"`
			ErrorCount     int64  `json:"error_count"`
		} `json:"group"`
		Account map[string]struct {
			AccountID        int64  `json:"account_id"`
			AccountName      string `json:"account_name"`
			Platform         string `json:"platform"`
			GroupID          int64  `json:"group_id"`
			GroupName        string `json:"group_name"`
			Status           string `json:"status"`
			IsAvailable      bool   `json:"is_available"`
			IsRateLimited    bool   `json:"is_rate_limited"`
			RateLimitResetAt string `json:"rate_limit_reset_at"`
			IsOverloaded     bool   `json:"is_overloaded"`
			OverloadUntil    string `json:"overload_until"`
			HasError         bool   `json:"has_error"`
		} `json:"account"`
	}](ctx, p.upstream, "GET", "/admin/ops/account-availability"+suffix, token, "", "", nil)

	userConc, _ := upstream[struct {
		User map[string]opsUserConcurrency `json:"user"`
	}](ctx, p.upstream, "GET", "/admin/ops/user-concurrency", token, "", "", nil)

	trafficParams := url.Values{"window": {"5m"}}
	for k, v := range scope {
		trafficParams[k] = v
	}
	traffic, _ := upstream[struct {
		Summary *struct {
			QPS opsRateView `json:"qps"`
			TPS opsRateView `json:"tps"`
		} `json:"summary"`
	}](ctx, p.upstream, "GET", "/admin/ops/realtime-traffic?"+trafficParams.Encode(), token, "", "", nil)

	result := opsConcurrencyView{
		Enabled:   concurrency.Enabled,
		Timestamp: concurrency.Timestamp,
		Platforms: []opsPlatformConcurrency{},
		Groups:    []opsGroupConcurrency{},
		Accounts:  []opsAccountRuntime{},
		Users:     []opsUserConcurrency{},
	}
	if traffic.Summary != nil {
		result.QPS = traffic.Summary.QPS
		result.TPS = traffic.Summary.TPS
	}

	platformMap := make(map[string]opsPlatformConcurrency)
	for key, item := range concurrency.Platform {
		name := item.Platform
		if name == "" {
			name = key
		}
		platformMap[name] = opsPlatformConcurrency{
			Platform:       name,
			CurrentInUse:   item.CurrentInUse,
			MaxCapacity:    item.MaxCapacity,
			LoadPercentage: item.LoadPercentage,
			WaitingInQueue: item.WaitingInQueue,
		}
	}
	for key, avail := range availability.Platform {
		row := platformMap[key]
		row.Platform = key
		row.TotalAccounts = avail.TotalAccounts
		row.AvailableCount = avail.AvailableCount
		row.RateLimitCount = avail.RateLimitCount
		row.ErrorCount = avail.ErrorCount
		platformMap[key] = row
	}
	for _, item := range platformMap {
		result.Platforms = append(result.Platforms, item)
	}
	sort.Slice(result.Platforms, func(i, j int) bool { return result.Platforms[i].Platform < result.Platforms[j].Platform })

	groupMap := make(map[int64]opsGroupConcurrency)
	for _, item := range concurrency.Group {
		if item.GroupID > 0 {
			groupMap[item.GroupID] = opsGroupConcurrency{
				GroupID:        item.GroupID,
				GroupName:      item.GroupName,
				Platform:       item.Platform,
				CurrentInUse:   item.CurrentInUse,
				MaxCapacity:    item.MaxCapacity,
				LoadPercentage: item.LoadPercentage,
				WaitingInQueue: item.WaitingInQueue,
			}
		}
	}
	for _, avail := range availability.Group {
		if avail.GroupID > 0 {
			row := groupMap[avail.GroupID]
			row.GroupID = avail.GroupID
			if row.GroupName == "" {
				row.GroupName = avail.GroupName
			}
			if row.Platform == "" {
				row.Platform = avail.Platform
			}
			row.TotalAccounts = avail.TotalAccounts
			row.AvailableCount = avail.AvailableCount
			row.RateLimitCount = avail.RateLimitCount
			row.ErrorCount = avail.ErrorCount
			groupMap[avail.GroupID] = row
		}
	}
	for _, item := range groupMap {
		result.Groups = append(result.Groups, item)
	}
	sort.Slice(result.Groups, func(i, j int) bool { return result.Groups[i].GroupID < result.Groups[j].GroupID })

	accountMap := make(map[int64]opsAccountRuntime)
	for _, item := range concurrency.Account {
		if item.AccountID > 0 {
			accountMap[item.AccountID] = opsAccountRuntime{
				AccountID:      item.AccountID,
				AccountName:    item.AccountName,
				Platform:       item.Platform,
				GroupID:        item.GroupID,
				GroupName:      item.GroupName,
				Status:         "active",
				CurrentInUse:   item.CurrentInUse,
				MaxCapacity:    item.MaxCapacity,
				LoadPercentage: item.LoadPercentage,
				WaitingInQueue: item.WaitingInQueue,
				IsAvailable:    true,
			}
		}
	}
	for _, avail := range availability.Account {
		if avail.AccountID > 0 {
			row := accountMap[avail.AccountID]
			row.AccountID = avail.AccountID
			if row.AccountName == "" {
				row.AccountName = avail.AccountName
			}
			if row.Platform == "" {
				row.Platform = avail.Platform
			}
			if row.GroupID == 0 {
				row.GroupID = avail.GroupID
			}
			if row.GroupName == "" {
				row.GroupName = avail.GroupName
			}
			row.Status = avail.Status
			row.IsAvailable = avail.IsAvailable
			row.IsRateLimited = avail.IsRateLimited
			row.RateLimitResetAt = avail.RateLimitResetAt
			row.IsOverloaded = avail.IsOverloaded
			row.OverloadUntil = avail.OverloadUntil
			row.HasError = avail.HasError
			accountMap[avail.AccountID] = row
		}
	}
	for _, item := range accountMap {
		result.Accounts = append(result.Accounts, item)
	}
	sort.Slice(result.Accounts, func(i, j int) bool {
		if result.Accounts[i].HasError != result.Accounts[j].HasError {
			return result.Accounts[i].HasError
		}
		if result.Accounts[i].IsRateLimited != result.Accounts[j].IsRateLimited {
			return result.Accounts[i].IsRateLimited
		}
		return result.Accounts[i].AccountID < result.Accounts[j].AccountID
	})

	for _, item := range userConc.User {
		if item.UserID > 0 {
			result.Users = append(result.Users, item)
		}
	}
	sort.Slice(result.Users, func(i, j int) bool { return result.Users[i].CurrentInUse > result.Users[j].CurrentInUse })

	return result, nil
}

func (p *panel) opsConcurrency(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	params, err := parseOpsScopeQuery(r.URL.Query())
	if err != nil {
		return err
	}
	result, err := p.fetchOpsConcurrency(r.Context(), a.tokens.Access, params)
	return send(w, 200, result, err)
}

func (p *panel) buildOpsStreamSnapshot(ctx context.Context, token, rawQuery string) ([]byte, error) {
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, fail(400, "invalid_input")
	}
	actor, err := strconv.ParseInt(query.Get("actor"), 10, 64)
	if err != nil || actor < 1 {
		return nil, fail(400, "invalid_input")
	}
	current, err := currentUser(ctx, p.upstream, token)
	if err != nil {
		return nil, err
	}
	if current.Role != "admin" || current.ID != actor {
		return nil, fail(403, "not_allowed")
	}
	query.Del("actor")
	result, err := p.fetchOpsConcurrency(ctx, token, query)
	if err != nil {
		return nil, err
	}
	result.Timestamp = ""
	charts, err := p.fetchOpsOverview(ctx, token, query)
	if err != nil {
		return nil, err
	}
	charts.GeneratedAt = ""
	return json.Marshal(struct {
		opsConcurrencyView
		Charts opsOverviewView `json:"charts"`
	}{result, charts})
}

func (p *panel) opsStream(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	if p.opsHub == nil {
		return fail(503, "service_unavailable")
	}
	params, err := parseOpsScopeQuery(r.URL.Query())
	if err != nil {
		return err
	}
	params.Set("actor", strconv.FormatInt(a.user.ID, 10))
	scope := usageScope(a.user.ID, "ops|"+p.upstream.base+"|"+params.Encode())
	return serveSnapshotStream(w, r, p.opsHub, scope, a.tokens.Access, params.Encode())
}

func serveSnapshotStream(w http.ResponseWriter, r *http.Request, hub *opsStreamHub, scope, token, query string) error {
	ch, unsubscribe := hub.subscribe(r.Context(), scope, token, query, meta(r.Context()))
	defer unsubscribe()
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fail(500, "service_unavailable")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	response := http.NewResponseController(w)
	_ = response.SetWriteDeadline(time.Time{})
	flusher.Flush()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		var payload []byte
		var eventType = "snapshot"
		select {
		case <-r.Context().Done():
			return nil
		case <-heartbeat.C:
			eventType = "heartbeat"
			payload, _ = json.Marshal(struct {
				Type       string `json:"type"`
				ServerTime string `json:"server_time"`
			}{"heartbeat", time.Now().UTC().Format(time.RFC3339Nano)})
		case raw := <-ch:
			var envelope struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
			}
			if json.Unmarshal(raw, &envelope) == nil && envelope.Type != "" {
				eventType = envelope.Type
			}
			payload = raw
			if envelope.Reason == "not_allowed" {
				_ = response.SetWriteDeadline(time.Now().Add(10 * time.Second))
				_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload)
				flusher.Flush()
				return nil
			}
		}
		_ = response.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload); err != nil {
			return nil
		}
		flusher.Flush()
		_ = response.SetWriteDeadline(time.Time{})
	}
}

func (p *panel) opsRequests(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, params, err := listQuery(r, "time_range", "kind", "platform", "group_id", "user_id", "api_key_id", "account_id", "model", "request_id", "q", "sort")
	if err != nil {
		return err
	}
	if !validOpsTimeRange(params.Get("time_range")) {
		return fail(400, "invalid_input")
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/ops/requests", page, params, func(item opsRequestRow) (opsRequestRow, error) {
		if item.CreatedAt == "" {
			return opsRequestRow{}, fail(502, "invalid_upstream")
		}
		return item, nil
	})
	return send(w, 200, rows, err)
}

func (p *panel) opsErrorList(upstreamPath string) task {
	return func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		page, params, err := listQuery(r, "time_range", "platform", "group_id", "account_id", "user_id", "api_key_id", "model", "phase", "category", "error_owner", "resolved", "view", "q", "status_codes", "sort_by", "sort_order")
		if err != nil {
			return err
		}
		rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, upstreamPath, page, params, projectOpsError)
		return send(w, 200, rows, err)
	}
}

func (p *panel) opsErrorDetail(upstreamPath string) task {
	return func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		id, err := pathID(r, "id")
		if err != nil {
			return err
		}
		row, err := upstream[opsErrorRowWire](r.Context(), p.upstream, "GET", fmt.Sprintf("%s/%d", upstreamPath, id), a.tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		view, err := projectOpsError(row)
		return send(w, 200, view, err)
	}
}

func (p *panel) opsRequestUpstreamErrors(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, fmt.Sprintf("/admin/ops/request-errors/%d/upstream-errors", id), page, nil, projectOpsError)
	return send(w, 200, rows, err)
}

func (p *panel) opsErrorResolve(upstreamPath string) task {
	return func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		id, err := pathID(r, "id")
		if err != nil {
			return err
		}
		input, err := decodeBody[struct {
			Resolved *bool `json:"resolved"`
		}](r)
		if err != nil || input.Resolved == nil {
			return fail(400, "invalid_input")
		}
		_, err = upstream[struct{}](r.Context(), p.upstream, "PUT", fmt.Sprintf("%s/%d/resolve", upstreamPath, id), a.tokens.Access, "", "", input)
		return send(w, 200, struct{}{}, err)
	}
}

func validOpsAlertRule(input opsAlertRule) bool {
	return strings.TrimSpace(input.Name) != "" && len(input.Name) <= 100 &&
		input.MetricType != "" && len(input.MetricType) <= 64 &&
		(input.Operator == ">" || input.Operator == ">=" || input.Operator == "<" || input.Operator == "<=" || input.Operator == "==" || input.Operator == "!=") &&
		input.WindowMinutes >= 1 && input.WindowMinutes <= 1440 &&
		input.SustainedMinutes >= 0 && input.SustainedMinutes <= 1440 &&
		input.CooldownMinutes >= 0 && input.CooldownMinutes <= 10080 &&
		(input.Severity == "critical" || input.Severity == "warning" || input.Severity == "info")
}

func (p *panel) listOpsAlertRules(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	rules, err := upstream[[]opsAlertRule](r.Context(), p.upstream, "GET", "/admin/ops/alert-rules", a.tokens.Access, "", "", nil)
	if err == nil && rules == nil {
		rules = []opsAlertRule{}
	}
	return send(w, 200, rules, err)
}

func (p *panel) createOpsAlertRule(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[opsAlertRule](r)
	if err != nil || !validOpsAlertRule(input) {
		return fail(400, "invalid_input")
	}
	created, err := upstream[opsAlertRule](r.Context(), p.upstream, "POST", "/admin/ops/alert-rules", a.tokens.Access, "", "", input)
	return send(w, 201, created, err)
}

func (p *panel) updateOpsAlertRule(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[opsAlertRule](r)
	if err != nil || !validOpsAlertRule(input) {
		return fail(400, "invalid_input")
	}
	updated, err := upstream[opsAlertRule](r.Context(), p.upstream, "PUT", fmt.Sprintf("/admin/ops/alert-rules/%d", id), a.tokens.Access, "", "", input)
	return send(w, 200, updated, err)
}

func (p *panel) deleteOpsAlertRule(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "DELETE", fmt.Sprintf("/admin/ops/alert-rules/%d", id), a.tokens.Access, "", "", nil)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) listOpsAlertEvents(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	_, params, err := listQuery(r, "status", "severity", "time_range", "platform", "group_id")
	if err != nil {
		return err
	}
	params.Set("limit", "50")
	events, err := upstream[[]opsAlertEvent](r.Context(), p.upstream, "GET", "/admin/ops/alert-events?"+params.Encode(), a.tokens.Access, "", "", nil)
	if err == nil && events == nil {
		events = []opsAlertEvent{}
	}
	return send(w, 200, events, err)
}

func (p *panel) updateOpsAlertEventStatus(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		Status string `json:"status"`
	}](r)
	if err != nil || input.Status != "resolved" && input.Status != "manual_resolved" {
		return fail(400, "invalid_input")
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "PUT", fmt.Sprintf("/admin/ops/alert-events/%d/status", id), a.tokens.Access, "", "", input)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) createOpsAlertSilence(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		RuleID   int64  `json:"rule_id"`
		Platform string `json:"platform"`
		GroupID  *int64 `json:"group_id,omitempty"`
		Until    string `json:"until"`
		Reason   string `json:"reason,omitempty"`
	}](r)
	if err != nil || input.RuleID < 1 || input.Until == "" || len(input.Reason) > 200 {
		return fail(400, "invalid_input")
	}
	if _, err := time.Parse(time.RFC3339, input.Until); err != nil {
		return fail(400, "invalid_input")
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "POST", "/admin/ops/alert-silences", a.tokens.Access, "", "", input)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) getOpsSettings(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	var bundle opsSettingsBundle
	thresholds, err := upstream[opsMetricThresholds](r.Context(), p.upstream, "GET", "/admin/ops/settings/metric-thresholds", a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	bundle.Thresholds = thresholds
	if emailCfg, emailErr := upstream[opsEmailNotification](r.Context(), p.upstream, "GET", "/admin/ops/email-notification/config", a.tokens.Access, "", "", nil); emailErr == nil {
		if emailCfg.Alert.Recipients == nil {
			emailCfg.Alert.Recipients = []string{}
		}
		if emailCfg.Report.Recipients == nil {
			emailCfg.Report.Recipients = []string{}
		}
		bundle.Email = emailCfg
	}
	if logCfg, logErr := upstream[opsRuntimeLogConfig](r.Context(), p.upstream, "GET", "/admin/ops/runtime/logging", a.tokens.Access, "", "", nil); logErr == nil {
		bundle.Logging = logCfg
	}
	if advCfg, advErr := upstream[opsAdvancedConfig](r.Context(), p.upstream, "GET", "/admin/ops/advanced-settings", a.tokens.Access, "", "", nil); advErr == nil {
		bundle.Advanced = advCfg
	}
	writeJSON(w, 200, bundle)
	return nil
}

func (p *panel) updateOpsSettings(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Thresholds *opsMetricThresholds  `json:"thresholds,omitempty"`
		Email      *opsEmailNotification `json:"email,omitempty"`
		Logging    *opsRuntimeLogConfig  `json:"logging,omitempty"`
		Advanced   *opsAdvancedConfig    `json:"advanced,omitempty"`
	}](r)
	if err != nil || input.Thresholds == nil && input.Email == nil && input.Logging == nil && input.Advanced == nil {
		return fail(400, "invalid_input")
	}
	if input.Thresholds != nil {
		if _, err := upstream[struct{}](r.Context(), p.upstream, "PUT", "/admin/ops/settings/metric-thresholds", a.tokens.Access, "", "", input.Thresholds); err != nil {
			return err
		}
	}
	if input.Email != nil {
		if _, err := upstream[opsEmailNotification](r.Context(), p.upstream, "PUT", "/admin/ops/email-notification/config", a.tokens.Access, "", "", input.Email); err != nil {
			return err
		}
	}
	if input.Logging != nil {
		if input.Logging.Level != "debug" && input.Logging.Level != "info" && input.Logging.Level != "warn" && input.Logging.Level != "error" {
			return fail(400, "invalid_input")
		}
		if _, err := upstream[opsRuntimeLogConfig](r.Context(), p.upstream, "PUT", "/admin/ops/runtime/logging", a.tokens.Access, "", "", input.Logging); err != nil {
			return err
		}
	}
	if input.Advanced != nil {
		if _, err := upstream[opsAdvancedConfig](r.Context(), p.upstream, "PUT", "/admin/ops/advanced-settings", a.tokens.Access, "", "", input.Advanced); err != nil {
			return err
		}
	}
	writeJSON(w, 200, struct{}{})
	return nil
}

func (p *panel) resetOpsLogging(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	cfg, err := upstream[opsRuntimeLogConfig](r.Context(), p.upstream, "POST", "/admin/ops/runtime/logging/reset", a.tokens.Access, "", "", struct{}{})
	return send(w, 200, cfg, err)
}
