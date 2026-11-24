package observe

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/usagecursor"
	"sub2api-panel/ent/usageobservation"
	"sub2api-panel/ent/usagesnapshot"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type observedUsageDay struct {
	Page       int                `json:"page"`
	Generation int64              `json:"generation"`
	Ready      bool               `json:"ready"`
	Total      UsageTotals        `json:"total"`
	Windows    []UsageBucket      `json:"windows"`
	Models     []UsageModelBucket `json:"models"`
}

func DashboardScope(s *core.Server, actor int64, query url.Values, admin bool) string {
	filter, _ := ParseUsageFilter(query, time.Now().In(s.UsageZone()), !query.Has("model"))
	target, _ := usageTarget(query, actor)
	dates := filter.Start.Format(UsageDateLayout) + "/" + filter.End.Format(UsageDateLayout)
	if filter.Period != "" {
		dates = filter.Period
	}
	extra := ""
	if filter.Group != 0 || filter.Account != 0 {
		extra = fmt.Sprintf("|g:%d|a:%d", filter.Group, filter.Account)
	}
	source := s.Upstream.Base
	if s.UsageSampleVersion != "" {
		source = "preview:" + s.UsageSampleVersion
	}
	return core.UsageScope(actor, fmt.Sprintf("dashboard3|%s|%t|%d|%d|%s|%s|%s|%t%s", source, admin, target, filter.Key, filter.Timezone, filter.Granularity, dates, query.Get("analysis") == "models", extra))
}

func observationScope(scope string, day time.Time) string {
	return scope + "/" + day.Format(UsageDateLayout)
}

func equivalentUsage(first, second UsageTotals) bool {
	return first.Requests == second.Requests && first.UsageTokens == second.UsageTokens && math.Abs(first.Cost-second.Cost) <= max(1e-8, math.Abs(second.Cost)*1e-9)
}

func findObservedDay(ctx context.Context, db *ent.Client, scope string) (observedUsageDay, int, error) {
	row, err := db.UsageCursor.Query().Where(usagecursor.ScopeEQ(scope)).Only(ctx)
	if ent.IsNotFound(err) {
		return observedUsageDay{}, 0, nil
	}
	if err != nil {
		return observedUsageDay{}, 0, err
	}
	var value observedUsageDay
	err = json.Unmarshal([]byte(row.Cursor), &value)
	return value, row.ID, err
}

func readUsageModelSeries(s *core.Server, ctx context.Context, actor int64, admin bool, query url.Values, result *UsageSummary) error {
	if query.Get("analysis") != "models" {
		return nil
	}
	filter, err := ParseUsageFilter(query, time.Now().In(s.UsageZone()), true)
	if err != nil {
		return err
	}
	target := actor
	if admin {
		target, _ = usageTarget(query, actor)
	}
	return populateObservedUsage(s, ctx, DashboardScope(s, actor, query, admin), filter, result, "", target, admin)
}

func populateObservedUsage(s *core.Server, ctx context.Context, scope string, filter UsageFilter, result *UsageSummary, token string, target int64, admin bool) error {
	result.ModelSeries = UsageModelSeries{Buckets: []UsageModelBucket{}}
	if ok, err := officialUsageModelSeries(ctx, s, filter, target, result); ok && err == nil {
		return nil
	}
	maybePruneUsageCache(s, ctx)
	if s.Database == nil {
		return nil
	}
	var pending []time.Time
	states := make(map[string]observedUsageDay)
	partitions := make([]string, 0)
	for day := filter.End; !day.Before(filter.Start); day = day.AddDate(0, 0, -1) {
		partitions = append(partitions, observationScope(scope, day))
	}
	cursors, err := s.Database.UsageCursor.Query().Where(usagecursor.ScopeIn(partitions...)).All(ctx)
	if err != nil {
		return err
	}
	stored := make(map[string]observedUsageDay, len(cursors))
	for _, cursor := range cursors {
		var day observedUsageDay
		if err := json.Unmarshal([]byte(cursor.Cursor), &day); err != nil {
			return err
		}
		if len(day.Windows) != len(day.Models) {
			return platform.Fail(502, "invalid_upstream")
		}
		stored[cursor.Scope] = day
	}
	for day := filter.End; !day.Before(filter.Start); day = day.AddDate(0, 0, -1) {
		result.ModelSeries.TotalDays++
		label := day.Format(UsageDateLayout)
		state := stored[observationScope(scope, day)]
		expected := result.expected[label]
		if !state.Ready || !equivalentUsage(state.Total, expected) {
			if expected.Requests != 0 || state.Ready || state.Generation != 0 {
				pending = append(pending, day)
				continue
			}
			state = observedUsageDay{Ready: true}
		}
		states[label] = state
	}
	if token != "" && len(pending) > 0 {
		day := pending[0]
		// Give closed dates time to catch up even while the newest day is receiving writes.
		if len(pending) > 1 && time.Now().Unix()/5%2 != 0 {
			day = pending[1]
		}
		label := day.Format(UsageDateLayout)
		partition := filter
		partition.Start, partition.End = day, day
		state, err := sampleUsageDay(s, ctx, observationScope(scope, day), token, target, admin, partition, result.expected[label])
		if err != nil {
			return err
		}
		if state.Ready && equivalentUsage(state.Total, result.expected[label]) {
			states[label] = state
		}
	}
	result.ModelSeries.ReadyDays = len(states)
	for index := range result.Buckets {
		window := &result.Buckets[index]
		state, ok := states[window.Start.Format(UsageDateLayout)]
		if !ok {
			continue
		}
		models := []UsageModel{}
		for i, row := range state.Windows {
			if !row.Start.Equal(window.Start) {
				continue
			}
			models = state.Models[i].Models
			if window.Pending {
				window.UsageTotals = row.UsageTotals
			}
			break
		}
		window.Pending = false
		result.ModelSeries.Buckets = append(result.ModelSeries.Buckets, UsageModelBucket{window.Start, models})
	}
	return nil
}

func sampleUsageDay(s *core.Server, ctx context.Context, scope, token string, target int64, admin bool, filter UsageFilter, expected UsageTotals) (observedUsageDay, error) {
	var zero observedUsageDay
	state, id, err := findObservedDay(ctx, s.Database, scope)
	if err != nil {
		return zero, err
	}
	if state.Page < 1 {
		state.Page = 1
	}
	existingState := state
	wasReady := state.Ready
	if state.Generation == 0 || state.Ready {
		state.Generation = time.Now().UnixNano()
	}
	state.Ready = false
	path, query := usageRecordsQuery(filter, target, admin)
	query.Set("page", strconv.Itoa(state.Page))
	query.Set("page_size", "100")
	query.Set("sort_by", "created_at")
	query.Set("sort_order", "desc")
	input, err := platform.Upstream[platform.WirePage[usageRecordWire]](ctx, s.Upstream, "GET", path+"?"+query.Encode(), token, "", "", nil)
	if err != nil {
		return zero, err
	}
	if input.Pages == nil || *input.Pages < 0 || len(input.Items) > 100 {
		return zero, platform.Fail(502, "invalid_upstream")
	}
	rows, err := platform.ProjectList(input.Items, func(row usageRecordWire) (UsageRecord, error) {
		if row.CacheRead == nil || row.CacheWrite == nil || row.Cost == nil {
			return UsageRecord{}, platform.Fail(502, "invalid_upstream")
		}
		return projectUsageRecord(row, filter, target)
	})
	if err != nil {
		return zero, err
	}
	unique := make([]UsageRecord, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if _, duplicate := seen[row.ID]; duplicate {
			continue
		}
		seen[row.ID] = struct{}{}
		unique = append(unique, row)
	}
	rows = unique
	tx, err := s.Database.Tx(ctx)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback()
	started := time.Now()
	unchanged, err := saveObservations(ctx, tx, scope, state.Generation, rows)
	if err != nil {
		return zero, err
	}
	slog.Info("usage observations sampled", "trace_id", platform.Meta(ctx).Trace, "scope", scope, "page", state.Page,
		"pages", *input.Pages, "records", len(rows), "ready", state.Ready, "elapsed_ms", time.Since(started).Milliseconds())
	if state.Page == 1 && id != 0 {
		previous, err := readObservationTotals(ctx, tx, scope, 0)
		if err != nil {
			return zero, err
		}
		if equivalentUsage(previous, expected) {
			if wasReady && unchanged {
				if err := tx.Commit(); err != nil {
					return zero, err
				}
				return existingState, nil
			}
		}
	}
	if state.Page >= *input.Pages {
		current, err := readObservationTotals(ctx, tx, scope, state.Generation)
		if err != nil {
			return zero, err
		}
		all, err := readObservationTotals(ctx, tx, scope, 0)
		if err != nil {
			return zero, err
		}
		if equivalentUsage(current, expected) {
			_, err = tx.UsageObservation.Delete().Where(usageobservation.ScopeEQ(scope), usageobservation.LastSeenNEQ(state.Generation)).Exec(ctx)
			if err != nil {
				return zero, err
			}
			all = current
		}
		if equivalentUsage(all, expected) {
			state.Windows, state.Models, err = aggregateObservations(ctx, tx, scope, filter)
			if err != nil {
				return zero, err
			}
			state.Ready, state.Total = true, all
		}
		state.Page = 1
		state.Generation = 0
	} else {
		state.Page++
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return zero, err
	}
	now := time.Now().UnixMilli()
	if id == 0 {
		_, err = tx.UsageCursor.Create().SetScope(scope).SetCursor(string(encoded)).SetSampledAt(now).SetUpdated(now).Save(ctx)
	} else {
		_, err = tx.UsageCursor.UpdateOneID(id).SetCursor(string(encoded)).SetSampledAt(now).SetUpdated(now).Save(ctx)
	}
	if err != nil {
		return zero, err
	}
	return state, tx.Commit()
}

func saveObservations(ctx context.Context, tx *ent.Tx, scope string, generation int64, rows []UsageRecord) (bool, error) {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	previous := []*ent.UsageObservation{}
	var err error
	if len(ids) > 0 {
		previous, err = tx.UsageObservation.Query().Where(usageobservation.ScopeEQ(scope), usageobservation.UpstreamIDIn(ids...)).All(ctx)
	}
	if err != nil {
		return false, err
	}
	known := make(map[int64]int, len(previous))
	saved := make(map[int64]*ent.UsageObservation, len(previous))
	for _, row := range previous {
		known[row.UpstreamID] = row.ID
		saved[row.UpstreamID] = row
	}
	creates := make([]*ent.UsageObservationCreate, 0, len(rows))
	unchanged := len(previous) == len(rows)
	for _, row := range rows {
		if id, ok := known[row.ID]; ok {
			old := saved[row.ID]
			cost := row.Cost != nil && old.ActualCost != nil && math.Abs(*row.Cost-*old.ActualCost) <= 1e-10
			read, write := int64(0), int64(0)
			if row.CacheRead != nil {
				read = *row.CacheRead
			}
			if row.CacheWrite != nil {
				write = *row.CacheWrite
			}
			if old.Model != row.Model || old.KeyName != row.KeyName || !old.CreatedAt.Equal(row.Created) ||
				old.InputTokens != row.Input || old.OutputTokens != row.Output || old.CacheReadTokens != read ||
				old.CacheCreationTokens != write || !cost {
				unchanged = false
			}
			err = tx.UsageObservation.UpdateOneID(id).SetLastSeen(generation).SetModel(row.Model).SetKeyName(row.KeyName).
				SetCreatedAt(row.Created).SetInputTokens(row.Input).SetOutputTokens(row.Output).SetCacheReadTokens(read).
				SetCacheCreationTokens(write).SetActualCost(*row.Cost).Exec(ctx)
			if err != nil {
				return false, err
			}
			continue
		}
		creates = append(creates, tx.UsageObservation.Create().SetScope(scope).SetUpstreamID(row.ID).SetModel(row.Model).SetKeyName(row.KeyName).
			SetCreatedAt(row.Created).SetInputTokens(row.Input).SetOutputTokens(row.Output).SetCacheReadTokens(*row.CacheRead).
			SetCacheCreationTokens(*row.CacheWrite).SetActualCost(*row.Cost).SetFirstSeen(generation).SetLastSeen(generation).SetFingerprint("usage"))
	}
	if len(creates) > 0 {
		if err := tx.UsageObservation.CreateBulk(creates...).Exec(ctx); err != nil {
			return false, err
		}
	}
	return unchanged, nil
}

func readObservationTotals(ctx context.Context, tx *ent.Tx, scope string, generation int64) (UsageTotals, error) {
	q := tx.UsageObservation.Query().Where(usageobservation.ScopeEQ(scope))
	if generation != 0 {
		q.Where(usageobservation.LastSeenEQ(generation))
	}
	var rows []struct {
		Scope    string  `json:"scope"`
		Requests int64   `json:"requests"`
		Input    int64   `json:"input"`
		Output   int64   `json:"output"`
		Read     int64   `json:"read"`
		Write    int64   `json:"write"`
		Cost     float64 `json:"cost"`
	}
	err := q.GroupBy(usageobservation.FieldScope).Aggregate(ent.As(ent.Count(), "requests"), ent.As(ent.Sum(usageobservation.FieldInputTokens), "input"),
		ent.As(ent.Sum(usageobservation.FieldOutputTokens), "output"), ent.As(ent.Sum(usageobservation.FieldCacheReadTokens), "read"),
		ent.As(ent.Sum(usageobservation.FieldCacheCreationTokens), "write"), ent.As(ent.Sum(usageobservation.FieldActualCost), "cost")).Scan(ctx, &rows)
	if err != nil || len(rows) == 0 {
		return UsageTotals{}, err
	}
	row := rows[0]
	return UsageTotals{UsageTokens{row.Input, row.Output, row.Read, row.Write}, row.Requests, row.Cost}, nil
}

func aggregateObservations(ctx context.Context, tx *ent.Tx, scope string, filter UsageFilter) ([]UsageBucket, []UsageModelBucket, error) {
	windows := UsageWindows(filter)
	models := make([]UsageModelBucket, len(windows))
	for i := range windows {
		models[i] = UsageModelBucket{windows[i].Start, []UsageModel{}}
	}
	// Bounded ID pages keep memory independent of the number of calls in the date.
	groups := make([]map[string]UsageModel, len(windows))
	last := int64(0)
	for {
		rows, err := tx.UsageObservation.Query().Where(usageobservation.ScopeEQ(scope), usageobservation.UpstreamIDGT(last)).
			Order(usageobservation.ByUpstreamID()).Limit(1000).All(ctx)
		if err != nil {
			return nil, nil, err
		}
		for _, row := range rows {
			last = row.UpstreamID
			i := sort.Search(len(windows), func(i int) bool { return row.CreatedAt.Before(windows[i].End) })
			if i == len(windows) || row.CreatedAt.Before(windows[i].Start) {
				return nil, nil, platform.Fail(502, "invalid_upstream")
			}
			cost := *row.ActualCost
			windows[i].UsageTotals.add(UsageTotals{UsageTokens{row.InputTokens, row.OutputTokens, row.CacheReadTokens, row.CacheCreationTokens}, 1, cost})
			if groups[i] == nil {
				groups[i] = make(map[string]UsageModel)
			}
			model := groups[i][row.Model]
			model.Model = row.Model
			model.Requests++
			model.Cost += cost
			model.Tokens += row.InputTokens + row.OutputTokens + row.CacheReadTokens + row.CacheCreationTokens
			groups[i][row.Model] = model
		}
		if len(rows) < 1000 {
			break
		}
	}
	for i, group := range groups {
		for _, model := range group {
			models[i].Models = append(models[i].Models, model)
		}
		sort.Slice(models[i].Models, func(a, b int) bool { return models[i].Models[a].Model < models[i].Models[b].Model })
	}
	return windows, models, nil
}

func readHourlyUsage(s *core.Server, ctx context.Context, scope string, filter UsageFilter, bucket UsageBucket, page int) (platform.PageResult[UsageRecord], error) {
	result := platform.PageResult[UsageRecord]{Items: []UsageRecord{}}
	if s.Database == nil {
		return result, platform.Fail(http.StatusConflict, "usage_sync_pending")
	}
	scope = observationScope(scope, bucket.Start)
	state, _, err := findObservedDay(ctx, s.Database, scope)
	if err != nil {
		return result, err
	}
	if !state.Ready {
		latest, snapshotErr := s.Database.UsageSnapshot.Query().Where(usagesnapshot.ScopeEQ(strings.TrimSuffix(scope, "/"+bucket.Start.Format(UsageDateLayout)))).Only(ctx)
		if snapshotErr == nil {
			payload, marshalErr := json.Marshal(latest.Payload)
			var snapshot UsageSummary
			if marshalErr == nil && json.Unmarshal(payload, &snapshot) == nil {
				for _, ready := range snapshot.ModelSeries.Buckets {
					if ready.Start.Equal(bucket.Start) && len(ready.Models) == 0 {
						return result, nil
					}
				}
			}
		}
		return result, platform.Fail(http.StatusConflict, "usage_sync_pending")
	}
	query := s.Database.UsageObservation.Query().Where(usageobservation.ScopeEQ(scope), usageobservation.CreatedAtGTE(bucket.Start), usageobservation.CreatedAtLT(bucket.End))
	if filter.Model != "" {
		query.Where(usageobservation.ModelEQ(filter.Model))
	}
	count, err := query.Clone().Count(ctx)
	if err != nil {
		return result, err
	}
	rows, err := query.Order(ent.Desc(usageobservation.FieldCreatedAt), ent.Desc(usageobservation.FieldUpstreamID)).Offset((page - 1) * platform.PageSize).Limit(platform.PageSize).All(ctx)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		read, write := row.CacheReadTokens, row.CacheCreationTokens
		result.Items = append(result.Items, UsageRecord{UsageView: UsageView{row.UpstreamID, row.Model, row.CreatedAt.In(bucket.Start.Location()), row.InputTokens, row.OutputTokens, row.ActualCost}, CacheRead: &read, CacheWrite: &write, KeyName: row.KeyName})
	}
	result.Pages = (count + platform.PageSize - 1) / platform.PageSize
	result.More = page < result.Pages
	return result, nil
}
