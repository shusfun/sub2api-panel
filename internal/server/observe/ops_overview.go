package observe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

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
		return nil, platform.Fail(400, "invalid_input")
	}
	if timeRange == "" {
		timeRange = "1h"
	}
	out.Set("time_range", timeRange)
	if plat := strings.TrimSpace(raw.Get("platform")); plat != "" {
		if len(plat) > 50 {
			return nil, platform.Fail(400, "invalid_input")
		}
		out.Set("platform", plat)
	}
	if groupRaw := strings.TrimSpace(raw.Get("group_id")); groupRaw != "" {
		groupID, err := strconv.ParseInt(groupRaw, 10, 64)
		if err != nil || groupID < 1 {
			return nil, platform.Fail(400, "invalid_input")
		}
		out.Set("group_id", strconv.FormatInt(groupID, 10))
	}
	return out, nil
}

func opsOverview(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		params, err := parseOpsScopeQuery(r.URL.Query())
		if err != nil {
			return err
		}
		view, err := fetchOpsOverview(s, r.Context(), a.Tokens.Access, params)
		return platform.Send(w, 200, view, err)
	}
}

func fetchOpsOverview(s *core.Server, ctx context.Context, token string, params url.Values) (opsOverviewView, error) {
	snap, err := platform.Upstream[struct {
		GeneratedAt     string             `json:"generated_at"`
		Overview        opsOverviewSummary `json:"overview"`
		ThroughputTrend struct {
			Points []opsThroughputPoint `json:"points"`
		} `json:"throughput_trend"`
		ErrorTrend struct {
			Points []opsErrorTrendPoint `json:"points"`
		} `json:"error_trend"`
	}](ctx, s.Upstream, "GET", "/admin/ops/dashboard/snapshot-v2?"+params.Encode(), token, "", "", nil)
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
	if hist, histErr := platform.Upstream[struct {
		Buckets []opsLatencyBucket `json:"buckets"`
	}](ctx, s.Upstream, "GET", "/admin/ops/dashboard/latency-histogram?"+params.Encode(), token, "", "", nil); histErr == nil && hist.Buckets != nil {
		view.LatencyBuckets = hist.Buckets
	}
	if dist, distErr := platform.Upstream[struct {
		Items []opsErrorDistributionItem `json:"items"`
	}](ctx, s.Upstream, "GET", "/admin/ops/dashboard/error-distribution?"+params.Encode(), token, "", "", nil); distErr == nil && dist.Items != nil {
		view.ErrorDistribution = dist.Items
	}
	return view, nil
}

func BuildOpsStreamSnapshot(s *core.Server) func(context.Context, string, string) ([]byte, error) {
	return func(ctx context.Context, token, rawQuery string) ([]byte, error) {
		query, err := url.ParseQuery(rawQuery)
		if err != nil {
			return nil, platform.Fail(400, "invalid_input")
		}
		actor, err := strconv.ParseInt(query.Get("actor"), 10, 64)
		if err != nil || actor < 1 {
			return nil, platform.Fail(400, "invalid_input")
		}
		current, err := platform.CurrentUser(ctx, s.Upstream, token)
		if err != nil {
			return nil, err
		}
		if current.Role != "admin" || current.ID != actor {
			return nil, platform.Fail(403, "not_allowed")
		}
		query.Del("actor")
		result, err := fetchOpsConcurrency(s, ctx, token, query)
		if err != nil {
			return nil, err
		}
		result.Timestamp = ""
		charts, err := fetchOpsOverview(s, ctx, token, query)
		if err != nil {
			return nil, err
		}
		charts.GeneratedAt = ""
		return json.Marshal(struct {
			opsConcurrencyView
			Charts opsOverviewView `json:"charts"`
		}{result, charts})
	}
}

func opsStream(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		if s.OpsHub == nil {
			return platform.Fail(503, "service_unavailable")
		}
		params, err := parseOpsScopeQuery(r.URL.Query())
		if err != nil {
			return err
		}
		params.Set("actor", strconv.FormatInt(a.User.ID, 10))
		scope := core.UsageScope(a.User.ID, "ops|"+s.Upstream.Base+"|"+params.Encode())
		return core.ServeSnapshotStream(w, r, s.OpsHub, scope, a.Tokens.Access, params.Encode())
	}
}
