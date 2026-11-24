package main

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	officialent "sub2api-panel/official"
	"sub2api-panel/official/officialuser"
	"sub2api-panel/official/opserrorlog"
	"sub2api-panel/official/usagelog"
)

// 只读直连官方库：补官方接口没有提供的每账号当日统计。
// 连接使用只有 SELECT 权限的角色，所有写入仍走官方接口。

func openOfficialDatabase(ctx context.Context, raw string) (*officialent.Client, error) {
	if err := validateDatabaseURL(raw); err != nil {
		return nil, err
	}
	db, err := sql.Open("pgx", raw)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return officialent.NewClient(officialent.Driver(entsql.OpenDB(dialect.Postgres, db))), nil
}

type accountDayStat struct {
	Requests int64
	Cost     float64
	Errors   int64
}

// accountDayStats 返回这些账号在用量时区当天的请求数、扣费和失败数。
func (p *panel) accountDayStats(ctx context.Context, ids []int64) (map[int64]accountDayStat, error) {
	stats := map[int64]accountDayStat{}
	if p.officialDB == nil || len(ids) == 0 {
		return stats, nil
	}
	location := p.usageLocation
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
	if err := p.officialDB.UsageLog.Query().
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
	if err := p.officialDB.OpsErrorLog.Query().
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

// withDayStats 给一页账号补上当日统计：优先走只读官方库，未配直连时回退官方批量今日统计接口补齐请求数、Token 与扣费。
func (p *panel) withDayStats(ctx context.Context, token string, items []accountAdminView) {
	if len(items) == 0 {
		return
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	fromDB := false
	if p.officialDB != nil {
		if stats, err := p.accountDayStats(ctx, ids); err == nil {
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
	batch, err := upstream[struct {
		Stats map[string]struct {
			Requests int64   `json:"requests"`
			Tokens   int64   `json:"tokens"`
			Cost     float64 `json:"cost"`
			UserCost float64 `json:"user_cost"`
		} `json:"stats"`
	}](ctx, p.upstream, "POST", "/admin/accounts/today-stats/batch", token, "", "", map[string]any{"account_ids": ids})
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

// observeGroupTraffic 聚合指定分组在最近 windowMinutes 内的去重活跃用户数与请求数：
// 优先查询只读官方库 usage_logs；未配置直连库时回退读取官方实时在途并发指标。
func (p *panel) observeGroupTraffic(ctx context.Context, groupID int64, windowMinutes int) (activeUsers int, requests int64) {
	if windowMinutes < 1 {
		windowMinutes = 15
	}
	if p.officialDB != nil {
		since := time.Now().Add(-time.Duration(windowMinutes) * time.Minute)
		var rows []struct {
			UserID *int64 `json:"user_id"`
			Count  int64  `json:"count"`
		}
		if err := p.officialDB.UsageLog.Query().
			Where(usagelog.GroupIDEQ(groupID), usagelog.CreatedAtGTE(since)).
			GroupBy(usagelog.FieldUserID).
			Aggregate(officialent.Count()).
			Scan(ctx, &rows); err == nil {
			users := 0
			var total int64
			for _, r := range rows {
				total += r.Count
				if r.UserID != nil && *r.UserID > 0 {
					users++
				}
			}
			return users, total
		}
	}
	return 0, 0
}

// resolveUserLabels 优先通过同机只读官方库批量补全用户显示标识（邮箱或用户名），未配置只读库时按需回退上游查询。
func (p *panel) resolveUserLabels(ctx context.Context, token string, ids []int64) map[int64]string {
	ctx, end := startFlow(ctx, "查询优惠码使用记录", "user_count", len(ids))
	defer end()
	out := make(map[int64]string, len(ids))
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return out
	}
	if p.officialDB != nil {
		if rows, err := p.officialDB.OfficialUser.Query().Where(officialuser.IDIn(unique...)).All(ctx); err == nil {
			for _, u := range rows {
				label := u.Email
				if label == "" {
					label = u.Username
				}
				if label != "" {
					out[u.ID] = label
				}
			}
		}
	}
	if token == "" || len(unique) > 25 {
		return out
	}
	for _, id := range unique {
		if out[id] != "" {
			continue
		}
		row, err := upstream[struct {
			Email    string `json:"email"`
			Username string `json:"username"`
		}](ctx, p.upstream, "GET", fmt.Sprintf("/admin/users/%d", id), token, "", "", nil)
		if err == nil {
			if row.Email != "" {
				out[id] = row.Email
			} else if row.Username != "" {
				out[id] = row.Username
			}
		}
	}
	return out
}

// officialUsageModelSeries 在配置了官方只读库时直接按时间桶与模型聚合 usage_logs，免去 HTTP 分页搬运。
func (p *panel) officialUsageModelSeries(ctx context.Context, filter usageFilter, target int64, result *usageSummary) (bool, error) {
	if p.officialDB == nil || filter.start.IsZero() || filter.end.Before(filter.start) {
		return false, nil
	}
	ctx, end := startFlow(ctx, "查询模型分时用量", "granularity", filter.granularity)
	defer end()
	endExclusive := filter.end.AddDate(0, 0, 1)
	q := p.officialDB.UsageLog.Query().Where(
		usagelog.CreatedAtGTE(filter.start),
		usagelog.CreatedAtLT(endExclusive),
	)
	if target > 0 {
		q = q.Where(usagelog.UserIDEQ(target))
	}
	if filter.key > 0 {
		q = q.Where(usagelog.APIKeyIDEQ(filter.key))
	}
	if filter.group > 0 {
		q = q.Where(usagelog.GroupIDEQ(filter.group))
	}
	if filter.account > 0 {
		q = q.Where(usagelog.AccountIDEQ(filter.account))
	}
	rows, err := q.All(ctx)
	if err != nil {
		return false, err
	}
	loc := timezoneLocation(filter.timezone)
	totalDays := 0
	for day := filter.start; !day.After(filter.end); day = day.AddDate(0, 0, 1) {
		totalDays++
	}
	type modelAcc struct {
		requests int64
		tokens   int64
		cost     float64
	}
	bucketTotals := make([]usageTotals, len(result.Buckets))
	bucketModels := make([]map[string]*modelAcc, len(result.Buckets))
	for i := range bucketModels {
		bucketModels[i] = make(map[string]*modelAcc)
	}
	for _, row := range rows {
		at := row.CreatedAt.In(loc)
		idx := sort.Search(len(result.Buckets), func(i int) bool {
			return !result.Buckets[i].End.Before(at) && !result.Buckets[i].End.Equal(at)
		})
		if idx >= len(result.Buckets) || at.Before(result.Buckets[idx].Start) {
			continue
		}
		tot := usageTotals{
			usageTokens: usageTokens{
				Input:      row.InputTokens,
				Output:     row.OutputTokens,
				CacheRead:  row.CacheReadTokens,
				CacheWrite: row.CacheCreationTokens,
			},
			Requests: 1,
			Cost:     row.ActualCost,
		}
		bucketTotals[idx].add(tot)
		name := row.Model
		if name == "" {
			name = "unknown"
		}
		acc := bucketModels[idx][name]
		if acc == nil {
			acc = &modelAcc{}
			bucketModels[idx][name] = acc
		}
		acc.requests++
		acc.tokens += row.InputTokens + row.OutputTokens + row.CacheReadTokens + row.CacheCreationTokens
		acc.cost += row.ActualCost
	}
	series := make([]usageModelBucket, 0, len(result.Buckets))
	for i := range result.Buckets {
		window := &result.Buckets[i]
		if window.Pending {
			window.usageTotals = bucketTotals[i]
		}
		window.Pending = false
		models := make([]usageModel, 0, len(bucketModels[i]))
		for name, acc := range bucketModels[i] {
			models = append(models, usageModel{
				Model:    name,
				Requests: acc.requests,
				Tokens:   acc.tokens,
				Cost:     acc.cost,
			})
		}
		sort.Slice(models, func(a, b int) bool {
			if models[a].Cost != models[b].Cost {
				return models[a].Cost > models[b].Cost
			}
			if models[a].Tokens != models[b].Tokens {
				return models[a].Tokens > models[b].Tokens
			}
			return models[a].Model < models[b].Model
		})
		series = append(series, usageModelBucket{Start: window.Start, Models: models})
	}
	result.ModelSeries = usageModelSeries{
		ReadyDays: totalDays,
		TotalDays: totalDays,
		Buckets:   series,
	}
	return true, nil
}

