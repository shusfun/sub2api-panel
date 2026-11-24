package main

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
)

type observedUsageDay struct {
	Page       int                `json:"page"`
	Generation int64              `json:"generation"`
	Ready      bool               `json:"ready"`
	Total      usageTotals        `json:"total"`
	Windows    []usageBucket      `json:"windows"`
	Models     []usageModelBucket `json:"models"`
}

func (p *panel) dashboardScope(actor int64, query url.Values, admin bool) string {
	filter, _ := parseUsageFilter(query, time.Now().In(p.usageZone()), !query.Has("model"))
	target, _ := usageTarget(query, actor)
	dates := filter.start.Format(usageDateLayout) + "/" + filter.end.Format(usageDateLayout)
	if filter.period != "" {
		dates = filter.period
	}
	extra := ""
	if filter.group != 0 || filter.account != 0 {
		extra = fmt.Sprintf("|g:%d|a:%d", filter.group, filter.account)
	}
	source := p.upstream.base
	if p.usageSampleVersion != "" {
		source = "preview:" + p.usageSampleVersion
	}
	return usageScope(actor, fmt.Sprintf("dashboard3|%s|%t|%d|%d|%s|%s|%s|%t%s", source, admin, target, filter.key, filter.timezone, filter.granularity, dates, query.Get("analysis") == "models", extra))
}

func observationScope(scope string, day time.Time) string {
	return scope + "/" + day.Format(usageDateLayout)
}

func equivalentUsage(first, second usageTotals) bool {
	return first.Requests == second.Requests && first.usageTokens == second.usageTokens && math.Abs(first.Cost-second.Cost) <= max(1e-8, math.Abs(second.Cost)*1e-9)
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

func (p *panel) readUsageModelSeries(ctx context.Context, actor int64, admin bool, query url.Values, result *usageSummary) error {
	if query.Get("analysis") != "models" {
		return nil
	}
	filter, err := parseUsageFilter(query, time.Now().In(p.usageZone()), true)
	if err != nil {
		return err
	}
	target := actor
	if admin {
		target, _ = usageTarget(query, actor)
	}
	return p.populateObservedUsage(ctx, p.dashboardScope(actor, query, admin), filter, result, "", target, admin)
}

func (p *panel) populateObservedUsage(ctx context.Context, scope string, filter usageFilter, result *usageSummary, token string, target int64, admin bool) error {
	result.ModelSeries = usageModelSeries{Buckets: []usageModelBucket{}}
	if ok, err := p.officialUsageModelSeries(ctx, filter, target, result); ok && err == nil {
		return nil
	}
	p.maybePruneUsageCache(ctx)
	if p.database == nil {
		return nil
	}
	var pending []time.Time
	states := make(map[string]observedUsageDay)
	partitions := make([]string, 0)
	for day := filter.end; !day.Before(filter.start); day = day.AddDate(0, 0, -1) {
		partitions = append(partitions, observationScope(scope, day))
	}
	cursors, err := p.database.UsageCursor.Query().Where(usagecursor.ScopeIn(partitions...)).All(ctx)
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
			return fail(502, "invalid_upstream")
		}
		stored[cursor.Scope] = day
	}
	for day := filter.end; !day.Before(filter.start); day = day.AddDate(0, 0, -1) {
		result.ModelSeries.TotalDays++
		label := day.Format(usageDateLayout)
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
		label := day.Format(usageDateLayout)
		partition := filter
		partition.start, partition.end = day, day
		state, err := p.sampleUsageDay(ctx, observationScope(scope, day), token, target, admin, partition, result.expected[label])
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
		state, ok := states[window.Start.Format(usageDateLayout)]
		if !ok {
			continue
		}
		models := []usageModel{}
		for i, row := range state.Windows {
			if !row.Start.Equal(window.Start) {
				continue
			}
			models = state.Models[i].Models
			if window.Pending {
				window.usageTotals = row.usageTotals
			}
			break
		}
		window.Pending = false
		result.ModelSeries.Buckets = append(result.ModelSeries.Buckets, usageModelBucket{window.Start, models})
	}
	return nil
}

func (p *panel) sampleUsageDay(ctx context.Context, scope, token string, target int64, admin bool, filter usageFilter, expected usageTotals) (observedUsageDay, error) {
	var zero observedUsageDay
	state, id, err := findObservedDay(ctx, p.database, scope)
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
	input, err := upstream[wirePage[usageRecordWire]](ctx, p.upstream, "GET", path+"?"+query.Encode(), token, "", "", nil)
	if err != nil {
		return zero, err
	}
	if input.Pages == nil || *input.Pages < 0 || len(input.Items) > 100 {
		return zero, fail(502, "invalid_upstream")
	}
	rows, err := projectList(input.Items, func(row usageRecordWire) (usageRecord, error) {
		if row.CacheRead == nil || row.CacheWrite == nil || row.Cost == nil {
			return usageRecord{}, fail(502, "invalid_upstream")
		}
		return projectUsageRecord(row, filter, target)
	})
	if err != nil {
		return zero, err
	}
	unique := make([]usageRecord, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if _, duplicate := seen[row.ID]; duplicate {
			continue
		}
		seen[row.ID] = struct{}{}
		unique = append(unique, row)
	}
	rows = unique
	tx, err := p.database.Tx(ctx)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback()
	started := time.Now()
	unchanged, err := saveObservations(ctx, tx, scope, state.Generation, rows)
	if err != nil {
		return zero, err
	}
	slog.Info("usage observations sampled", "trace_id", meta(ctx).Trace, "scope", scope, "page", state.Page,
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

func saveObservations(ctx context.Context, tx *ent.Tx, scope string, generation int64, rows []usageRecord) (bool, error) {
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

func readObservationTotals(ctx context.Context, tx *ent.Tx, scope string, generation int64) (usageTotals, error) {
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
		return usageTotals{}, err
	}
	row := rows[0]
	return usageTotals{usageTokens{row.Input, row.Output, row.Read, row.Write}, row.Requests, row.Cost}, nil
}

func aggregateObservations(ctx context.Context, tx *ent.Tx, scope string, filter usageFilter) ([]usageBucket, []usageModelBucket, error) {
	windows := usageWindows(filter)
	models := make([]usageModelBucket, len(windows))
	for i := range windows {
		models[i] = usageModelBucket{windows[i].Start, []usageModel{}}
	}
	// Bounded ID pages keep memory independent of the number of calls in the date.
	groups := make([]map[string]usageModel, len(windows))
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
				return nil, nil, fail(502, "invalid_upstream")
			}
			cost := *row.ActualCost
			windows[i].usageTotals.add(usageTotals{usageTokens{row.InputTokens, row.OutputTokens, row.CacheReadTokens, row.CacheCreationTokens}, 1, cost})
			if groups[i] == nil {
				groups[i] = make(map[string]usageModel)
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

func (p *panel) readHourlyUsage(ctx context.Context, scope string, filter usageFilter, bucket usageBucket, page int) (pageResult[usageRecord], error) {
	result := pageResult[usageRecord]{Items: []usageRecord{}}
	if p.database == nil {
		return result, fail(http.StatusConflict, "usage_sync_pending")
	}
	scope = observationScope(scope, bucket.Start)
	state, _, err := findObservedDay(ctx, p.database, scope)
	if err != nil {
		return result, err
	}
	if !state.Ready {
		latest, snapshotErr := p.database.UsageSnapshot.Query().Where(usagesnapshot.ScopeEQ(strings.TrimSuffix(scope, "/"+bucket.Start.Format(usageDateLayout)))).Only(ctx)
		if snapshotErr == nil {
			payload, marshalErr := json.Marshal(latest.Payload)
			var snapshot usageSummary
			if marshalErr == nil && json.Unmarshal(payload, &snapshot) == nil {
				for _, ready := range snapshot.ModelSeries.Buckets {
					if ready.Start.Equal(bucket.Start) && len(ready.Models) == 0 {
						return result, nil
					}
				}
			}
		}
		return result, fail(http.StatusConflict, "usage_sync_pending")
	}
	query := p.database.UsageObservation.Query().Where(usageobservation.ScopeEQ(scope), usageobservation.CreatedAtGTE(bucket.Start), usageobservation.CreatedAtLT(bucket.End))
	if filter.model != "" {
		query.Where(usageobservation.ModelEQ(filter.model))
	}
	count, err := query.Clone().Count(ctx)
	if err != nil {
		return result, err
	}
	rows, err := query.Order(ent.Desc(usageobservation.FieldCreatedAt), ent.Desc(usageobservation.FieldUpstreamID)).Offset((page - 1) * pageSize).Limit(pageSize).All(ctx)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		read, write := row.CacheReadTokens, row.CacheCreationTokens
		result.Items = append(result.Items, usageRecord{usageView: usageView{row.UpstreamID, row.Model, row.CreatedAt.In(bucket.Start.Location()), row.InputTokens, row.OutputTokens, row.ActualCost}, CacheRead: &read, CacheWrite: &write, KeyName: row.KeyName})
	}
	result.Pages = (count + pageSize - 1) / pageSize
	result.More = page < result.Pages
	return result, nil
}
