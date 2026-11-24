package observe

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

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

func validOpsAlertRule(input opsAlertRule) bool {
	return strings.TrimSpace(input.Name) != "" && len(input.Name) <= 100 &&
		input.MetricType != "" && len(input.MetricType) <= 64 &&
		(input.Operator == ">" || input.Operator == ">=" || input.Operator == "<" || input.Operator == "<=" || input.Operator == "==" || input.Operator == "!=") &&
		input.WindowMinutes >= 1 && input.WindowMinutes <= 1440 &&
		input.SustainedMinutes >= 0 && input.SustainedMinutes <= 1440 &&
		input.CooldownMinutes >= 0 && input.CooldownMinutes <= 10080 &&
		(input.Severity == "critical" || input.Severity == "warning" || input.Severity == "info")
}

func listOpsAlertRules(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		rules, err := platform.Upstream[[]opsAlertRule](r.Context(), s.Upstream, "GET", "/admin/ops/alert-rules", a.Tokens.Access, "", "", nil)
		if err == nil && rules == nil {
			rules = []opsAlertRule{}
		}
		return platform.Send(w, 200, rules, err)
	}
}

func createOpsAlertRule(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		input, err := platform.DecodeBody[opsAlertRule](r)
		if err != nil || !validOpsAlertRule(input) {
			return platform.Fail(400, "invalid_input")
		}
		created, err := platform.Upstream[opsAlertRule](r.Context(), s.Upstream, "POST", "/admin/ops/alert-rules", a.Tokens.Access, "", "", input)
		return platform.Send(w, 201, created, err)
	}
}

func updateOpsAlertRule(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		input, err := platform.DecodeBody[opsAlertRule](r)
		if err != nil || !validOpsAlertRule(input) {
			return platform.Fail(400, "invalid_input")
		}
		updated, err := platform.Upstream[opsAlertRule](r.Context(), s.Upstream, "PUT", fmt.Sprintf("/admin/ops/alert-rules/%d", id), a.Tokens.Access, "", "", input)
		return platform.Send(w, 200, updated, err)
	}
}

func deleteOpsAlertRule(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		_, err = platform.Upstream[struct{}](r.Context(), s.Upstream, "DELETE", fmt.Sprintf("/admin/ops/alert-rules/%d", id), a.Tokens.Access, "", "", nil)
		return platform.Send(w, 200, struct{}{}, err)
	}
}

func listOpsAlertEvents(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		_, params, err := platform.ListQuery(r, "status", "severity", "time_range", "platform", "group_id")
		if err != nil {
			return err
		}
		params.Set("limit", "50")
		events, err := platform.Upstream[[]opsAlertEvent](r.Context(), s.Upstream, "GET", "/admin/ops/alert-events?"+params.Encode(), a.Tokens.Access, "", "", nil)
		if err == nil && events == nil {
			events = []opsAlertEvent{}
		}
		return platform.Send(w, 200, events, err)
	}
}

func updateOpsAlertEventStatus(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		input, err := platform.DecodeBody[struct {
			Status string `json:"status"`
		}](r)
		if err != nil || input.Status != "resolved" && input.Status != "manual_resolved" {
			return platform.Fail(400, "invalid_input")
		}
		_, err = platform.Upstream[struct{}](r.Context(), s.Upstream, "PUT", fmt.Sprintf("/admin/ops/alert-events/%d/status", id), a.Tokens.Access, "", "", input)
		return platform.Send(w, 200, struct{}{}, err)
	}
}

func createOpsAlertSilence(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		input, err := platform.DecodeBody[struct {
			RuleID   int64  `json:"rule_id"`
			Platform string `json:"platform"`
			GroupID  *int64 `json:"group_id,omitempty"`
			Until    string `json:"until"`
			Reason   string `json:"reason,omitempty"`
		}](r)
		if err != nil || input.RuleID < 1 || input.Until == "" || len(input.Reason) > 200 {
			return platform.Fail(400, "invalid_input")
		}
		if _, err := time.Parse(time.RFC3339, input.Until); err != nil {
			return platform.Fail(400, "invalid_input")
		}
		_, err = platform.Upstream[struct{}](r.Context(), s.Upstream, "POST", "/admin/ops/alert-silences", a.Tokens.Access, "", "", input)
		return platform.Send(w, 200, struct{}{}, err)
	}
}

func getOpsSettings(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		var bundle opsSettingsBundle
		thresholds, err := platform.Upstream[opsMetricThresholds](r.Context(), s.Upstream, "GET", "/admin/ops/settings/metric-thresholds", a.Tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		bundle.Thresholds = thresholds
		if emailCfg, emailErr := platform.Upstream[opsEmailNotification](r.Context(), s.Upstream, "GET", "/admin/ops/email-notification/config", a.Tokens.Access, "", "", nil); emailErr == nil {
			if emailCfg.Alert.Recipients == nil {
				emailCfg.Alert.Recipients = []string{}
			}
			if emailCfg.Report.Recipients == nil {
				emailCfg.Report.Recipients = []string{}
			}
			bundle.Email = emailCfg
		}
		if logCfg, logErr := platform.Upstream[opsRuntimeLogConfig](r.Context(), s.Upstream, "GET", "/admin/ops/runtime/logging", a.Tokens.Access, "", "", nil); logErr == nil {
			bundle.Logging = logCfg
		}
		if advCfg, advErr := platform.Upstream[opsAdvancedConfig](r.Context(), s.Upstream, "GET", "/admin/ops/advanced-settings", a.Tokens.Access, "", "", nil); advErr == nil {
			bundle.Advanced = advCfg
		}
		platform.WriteJSON(w, 200, bundle)
		return nil
	}
}

func updateOpsSettings(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		input, err := platform.DecodeBody[struct {
			Thresholds *opsMetricThresholds  `json:"thresholds,omitempty"`
			Email      *opsEmailNotification `json:"email,omitempty"`
			Logging    *opsRuntimeLogConfig  `json:"logging,omitempty"`
			Advanced   *opsAdvancedConfig    `json:"advanced,omitempty"`
		}](r)
		if err != nil || input.Thresholds == nil && input.Email == nil && input.Logging == nil && input.Advanced == nil {
			return platform.Fail(400, "invalid_input")
		}
		if input.Thresholds != nil {
			if _, err := platform.Upstream[struct{}](r.Context(), s.Upstream, "PUT", "/admin/ops/settings/metric-thresholds", a.Tokens.Access, "", "", input.Thresholds); err != nil {
				return err
			}
		}
		if input.Email != nil {
			if _, err := platform.Upstream[opsEmailNotification](r.Context(), s.Upstream, "PUT", "/admin/ops/email-notification/config", a.Tokens.Access, "", "", input.Email); err != nil {
				return err
			}
		}
		if input.Logging != nil {
			if input.Logging.Level != "debug" && input.Logging.Level != "info" && input.Logging.Level != "warn" && input.Logging.Level != "error" {
				return platform.Fail(400, "invalid_input")
			}
			if _, err := platform.Upstream[opsRuntimeLogConfig](r.Context(), s.Upstream, "PUT", "/admin/ops/runtime/logging", a.Tokens.Access, "", "", input.Logging); err != nil {
				return err
			}
		}
		if input.Advanced != nil {
			if _, err := platform.Upstream[opsAdvancedConfig](r.Context(), s.Upstream, "PUT", "/admin/ops/advanced-settings", a.Tokens.Access, "", "", input.Advanced); err != nil {
				return err
			}
		}
		platform.WriteJSON(w, 200, struct{}{})
		return nil
	}
}

func resetOpsLogging(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		cfg, err := platform.Upstream[opsRuntimeLogConfig](r.Context(), s.Upstream, "POST", "/admin/ops/runtime/logging/reset", a.Tokens.Access, "", "", struct{}{})
		return platform.Send(w, 200, cfg, err)
	}
}
