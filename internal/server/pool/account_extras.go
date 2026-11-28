package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	officialent "sub2api-panel/official"
	"sub2api-panel/official/opserrorlog"
	"sub2api-panel/official/usagelog"
)

type crsAccountInput struct {
	BaseURL string   `json:"base_url"`
	User    string   `json:"username"`
	Pass    string   `json:"password"`
	Proxies *bool    `json:"sync_proxies,omitempty"`
	NewIDs  []string `json:"selected_account_ids,omitempty"`
}

type crsAccountCandidate struct {
	ID       string `json:"crs_account_id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Type     string `json:"type"`
}

func (h *handler) syncCRSAccounts(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[crsAccountInput](r)
	if err != nil {
		return err
	}
	input.BaseURL, input.User = strings.TrimSpace(input.BaseURL), strings.TrimSpace(input.User)
	address, err := url.Parse(input.BaseURL)
	if err != nil || address.Hostname() == "" || address.User != nil || address.Fragment != "" || address.RawQuery != "" ||
		address.ForceQuery || (address.Scheme != "http" && address.Scheme != "https") || input.User == "" || input.Pass == "" || len(input.NewIDs) > 1000 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	for _, id := range input.NewIDs {
		if strings.TrimSpace(id) != id || id == "" || len(id) > 200 {
			return platform.Fail(http.StatusBadRequest, "invalid_input")
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	const base = "/admin/accounts/sync/crs"
	if strings.HasSuffix(r.URL.Path, "/preview") {
		result, err := platform.Upstream[struct {
			New      []crsAccountCandidate `json:"new_accounts"`
			Existing []crsAccountCandidate `json:"existing_accounts"`
		}](r.Context(), h.s.Upstream, http.MethodPost, base+"/preview", a.Tokens.Access, "", "", input)
		return platform.Send(w, http.StatusOK, result, err)
	}
	result, err := platform.Upstream[struct {
		Created int `json:"created"`
		Updated int `json:"updated"`
		Skipped int `json:"skipped"`
		Failed  int `json:"failed"`
		Items   []struct {
			ID     string `json:"crs_account_id"`
			Name   string `json:"name"`
			Action string `json:"action"`
		} `json:"items"`
	}](r.Context(), h.s.Upstream, http.MethodPost, base, a.Tokens.Access, "", "", input)
	return platform.Send(w, http.StatusOK, result, err)
}

type scheduledTestPlanView struct {
	ID          int64      `json:"id"`
	AccountID   int64      `json:"account_id"`
	ModelID     string     `json:"model_id"`
	Cron        string     `json:"cron_expression"`
	Enabled     bool       `json:"enabled"`
	MaxResults  int        `json:"max_results"`
	AutoRecover bool       `json:"auto_recover"`
	LastRun     *time.Time `json:"last_run_at"`
	NextRun     *time.Time `json:"next_run_at"`
}

type scheduledTestResultView struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"`
	LatencyMs int64     `json:"latency_ms"`
	Started   time.Time `json:"started_at"`
}

func schedulerResponse[T any](ctx context.Context, client platform.ServiceClient, method, path, token string, body any) (T, error) {
	var value T
	response, err := client.Open(ctx, method, "/api/v1"+path, token, "", "", body)
	if err != nil {
		return value, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return value, platform.UpstreamStatus{Status: response.StatusCode}
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF {
		return value, platform.Fail(502, "invalid_upstream")
	}
	return value, nil
}

func (h *handler) scheduledAccountTests(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	account, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	plans, err := schedulerResponse[[]scheduledTestPlanView](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/accounts/%d/scheduled-test-plans", account), a.Tokens.Access, nil)
	if err != nil {
		return err
	}
	if plans == nil {
		plans = []scheduledTestPlanView{}
	}
	for _, plan := range plans {
		if plan.ID < 1 || plan.AccountID != account {
			return platform.Fail(502, "invalid_upstream")
		}
	}
	platform.WriteJSON(w, 200, plans)
	return nil
}

func (h *handler) editAccountTestPlan(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	account, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	type inputView struct {
		Model       string `json:"model_id,omitempty"`
		Cron        string `json:"cron_expression,omitempty"`
		Enabled     *bool  `json:"enabled,omitempty"`
		MaxResults  int    `json:"max_results,omitempty"`
		AutoRecover *bool  `json:"auto_recover,omitempty"`
	}
	const base = "/admin/scheduled-test-plans"
	if r.Method == "POST" {
		input, err := platform.DecodeBody[inputView](r)
		if err != nil || strings.TrimSpace(input.Cron) == "" || len(input.Cron) > 100 || len(input.Model) > 200 || input.MaxResults < 0 || input.MaxResults > 1000 {
			return platform.Fail(400, "invalid_input")
		}
		result, err := schedulerResponse[scheduledTestPlanView](r.Context(), h.s.Upstream, "POST", base, a.Tokens.Access, struct {
			AccountID int64 `json:"account_id"`
			inputView
		}{account, input})
		if err == nil && (result.ID < 1 || result.AccountID != account) {
			return platform.Fail(502, "invalid_upstream")
		}
		return platform.Send(w, 201, result, err)
	}
	plan, err := platform.PathID(r, "plan")
	if err != nil {
		return err
	}
	list, err := schedulerResponse[[]scheduledTestPlanView](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/accounts/%d/scheduled-test-plans", account), a.Tokens.Access, nil)
	if err != nil {
		return err
	}
	found := false
	for _, item := range list {
		if item.ID == plan && item.AccountID == account {
			found = true
			break
		}
	}
	if !found {
		return platform.Fail(404, "not_found")
	}
	path := fmt.Sprintf("%s/%d", base, plan)
	switch r.Method {
	case "GET":
		items, err := schedulerResponse[[]scheduledTestResultView](r.Context(), h.s.Upstream, "GET", path+"/results?limit=25", a.Tokens.Access, nil)
		if items == nil {
			items = []scheduledTestResultView{}
		}
		return platform.Send(w, 200, items, err)
	case "PUT":
		input, err := platform.DecodeBody[inputView](r)
		if err != nil || len(input.Cron) > 100 || len(input.Model) > 200 || input.MaxResults < 0 || input.MaxResults > 1000 {
			return platform.Fail(400, "invalid_input")
		}
		result, err := schedulerResponse[scheduledTestPlanView](r.Context(), h.s.Upstream, "PUT", path, a.Tokens.Access, input)
		if err == nil && (result.ID != plan || result.AccountID != account) {
			return platform.Fail(502, "invalid_upstream")
		}
		return platform.Send(w, 200, result, err)
	default:
		_, err := schedulerResponse[struct{}](r.Context(), h.s.Upstream, "DELETE", path, a.Tokens.Access, nil)
		return platform.Send(w, 200, struct{}{}, err)
	}
}

func (h *handler) revertAccountProxy(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/accounts/%d/revert-proxy-fallback", id), a.Tokens.Access, "", "", struct{}{})
	return platform.Send(w, 200, struct{}{}, err)
}

type accountDayStat struct {
	Requests int64
	Cost     float64
	Errors   int64
}

func (h *handler) accountDayStats(ctx context.Context, ids []int64) (map[int64]accountDayStat, error) {
	stats := map[int64]accountDayStat{}
	if h.s.OfficialDB == nil || len(ids) == 0 {
		return stats, nil
	}
	location := h.s.UsageLocation
	if location == nil {
		location = time.Local
	}
	now := time.Now().In(location)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	var usage []struct {
		Account int64   `json:"account_id"`
		Count   int64   `json:"count"`
		Sum     float64 `json:"sum"`
	}
	if err := h.s.OfficialDB.UsageLog.Query().
		Where(usagelog.AccountIDIn(ids...), usagelog.CreatedAtGTE(start)).
		GroupBy(usagelog.FieldAccountID).
		Aggregate(officialent.Count(), officialent.Sum(usagelog.FieldActualCost)).
		Scan(ctx, &usage); err != nil {
		return nil, err
	}
	for _, row := range usage {
		stats[row.Account] = accountDayStat{Requests: row.Count, Cost: row.Sum}
	}
	var failures []struct {
		Account int64 `json:"account_id"`
		Count   int64 `json:"count"`
	}
	if err := h.s.OfficialDB.OpsErrorLog.Query().
		Where(opserrorlog.AccountIDIn(ids...), opserrorlog.CreatedAtGTE(start)).
		GroupBy(opserrorlog.FieldAccountID).
		Aggregate(officialent.Count()).
		Scan(ctx, &failures); err != nil {
		return nil, err
	}
	for _, row := range failures {
		stat := stats[row.Account]
		stat.Errors = row.Count
		stats[row.Account] = stat
	}
	return stats, nil
}

func (h *handler) withDayStats(ctx context.Context, token string, items []AccountAdminView) {
	if len(items) == 0 {
		return
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	h.withLedgerExtras(ctx, items, ids)
	fromDB := false
	if h.s.OfficialDB != nil {
		if stats, err := h.accountDayStats(ctx, ids); err == nil {
			fromDB = true
			for i := range items {
				stat := stats[items[i].ID]
				items[i].TodayRequests, items[i].TodayCost, items[i].TodayErrors = &stat.Requests, &stat.Cost, &stat.Errors
			}
		}
	}
	if token == "" {
		return
	}
	batch, err := platform.Upstream[struct {
		Stats map[string]struct {
			Requests int64   `json:"requests"`
			Tokens   int64   `json:"tokens"`
			Cost     float64 `json:"cost"`
			UserCost float64 `json:"user_cost"`
		} `json:"stats"`
	}](ctx, h.s.Upstream, "POST", "/admin/accounts/today-stats/batch", token, "", "", map[string]any{"account_ids": ids})
	if err != nil || batch.Stats == nil {
		return
	}
	for i := range items {
		stat, ok := batch.Stats[strconv.FormatInt(items[i].ID, 10)]
		if !ok {
			if !fromDB {
				var zeroReq, zeroTok int64
				var zeroCost float64
				items[i].TodayRequests, items[i].TodayTokens, items[i].TodayCost = &zeroReq, &zeroTok, &zeroCost
			}
			continue
		}
		tokens := stat.Tokens
		items[i].TodayTokens = &tokens
		if !fromDB {
			requests := stat.Requests
			cost := stat.UserCost
			if cost == 0 && stat.Cost > 0 {
				cost = stat.Cost
			}
			items[i].TodayRequests = &requests
			items[i].TodayCost = &cost
		}
	}
}

type accountRecentStat struct {
	RPM5m        float64
	AvgLatencyMS *float64
}

// accountRecentStats 用官方只读库 usage_logs 近 5 分钟的 COUNT 与 AVG(duration_ms)，按批量账号一次查询。
// 没接只读库时返回空表，调用方据此不输出 rpm_5m 与 avg_latency_ms。
func (h *handler) accountRecentStats(ctx context.Context, ids []int64) (map[int64]accountRecentStat, error) {
	stats := map[int64]accountRecentStat{}
	if h.s.OfficialDB == nil || len(ids) == 0 {
		return stats, nil
	}
	const window = 5
	since := time.Now().Add(-window * time.Minute)
	var rows []struct {
		Account int64    `json:"account_id"`
		Count   int64    `json:"count"`
		Mean    *float64 `json:"mean"`
	}
	if err := h.s.OfficialDB.UsageLog.Query().
		Where(usagelog.AccountIDIn(ids...), usagelog.CreatedAtGTE(since)).
		GroupBy(usagelog.FieldAccountID).
		Aggregate(officialent.Count(), officialent.Mean(usagelog.FieldDurationMs)).
		Scan(ctx, &rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		stats[row.Account] = accountRecentStat{RPM5m: float64(row.Count) / window, AvgLatencyMS: row.Mean}
	}
	return stats, nil
}
