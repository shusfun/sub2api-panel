package observe

import (
	"context"
	"database/sql"
	"sort"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	officialent "sub2api-panel/official"
	"sub2api-panel/official/usagelog"
)

// 只读直连官方库：补官方接口没有提供的统计。
// 连接使用只有 SELECT 权限的角色，所有写入仍走官方接口。

func OpenOfficialDatabase(ctx context.Context, raw string) (*officialent.Client, error) {
	if err := platform.ValidateDatabaseURL(raw); err != nil {
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

// ObserveGroupTraffic 聚合指定分组在最近 windowMinutes 内的去重活跃用户数与请求数：
// 优先查询只读官方库 usage_logs；未配置直连库时返回 0。
func ObserveGroupTraffic(ctx context.Context, s *core.Server, groupID int64, windowMinutes int) (activeUsers int, requests int64) {
	if windowMinutes < 1 {
		windowMinutes = 15
	}
	if s.OfficialDB != nil {
		since := time.Now().Add(-time.Duration(windowMinutes) * time.Minute)
		var rows []struct {
			UserID *int64 `json:"user_id"`
			Count  int64  `json:"count"`
		}
		if err := s.OfficialDB.UsageLog.Query().
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

// officialUsageModelSeries 在配置了官方只读库时直接按时间桶与模型聚合 usage_logs，免去 HTTP 分页搬运。
func officialUsageModelSeries(ctx context.Context, s *core.Server, filter UsageFilter, target int64, result *UsageSummary) (bool, error) {
	if s.OfficialDB == nil || filter.Start.IsZero() || filter.End.Before(filter.Start) {
		return false, nil
	}
	ctx, end := platform.StartFlow(ctx, "查询模型分时用量", "granularity", filter.Granularity)
	defer end()
	endExclusive := filter.End.AddDate(0, 0, 1)
	q := s.OfficialDB.UsageLog.Query().Where(
		usagelog.CreatedAtGTE(filter.Start),
		usagelog.CreatedAtLT(endExclusive),
	)
	if target > 0 {
		q = q.Where(usagelog.UserIDEQ(target))
	}
	if filter.Key > 0 {
		q = q.Where(usagelog.APIKeyIDEQ(filter.Key))
	}
	if filter.Group > 0 {
		q = q.Where(usagelog.GroupIDEQ(filter.Group))
	}
	if filter.Account > 0 {
		q = q.Where(usagelog.AccountIDEQ(filter.Account))
	}
	rows, err := q.All(ctx)
	if err != nil {
		return false, err
	}
	loc := timezoneLocation(filter.Timezone)
	totalDays := 0
	for day := filter.Start; !day.After(filter.End); day = day.AddDate(0, 0, 1) {
		totalDays++
	}
	type modelAcc struct {
		requests int64
		tokens   int64
		cost     float64
	}
	bucketTotals := make([]UsageTotals, len(result.Buckets))
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
		tot := UsageTotals{
			UsageTokens: UsageTokens{
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
	series := make([]UsageModelBucket, 0, len(result.Buckets))
	for i := range result.Buckets {
		window := &result.Buckets[i]
		if window.Pending {
			window.UsageTotals = bucketTotals[i]
		}
		window.Pending = false
		models := make([]UsageModel, 0, len(bucketModels[i]))
		for name, acc := range bucketModels[i] {
			models = append(models, UsageModel{
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
		series = append(series, UsageModelBucket{Start: window.Start, Models: models})
	}
	result.ModelSeries = UsageModelSeries{
		ReadyDays: totalDays,
		TotalDays: totalDays,
		Buckets:   series,
	}
	return true, nil
}
