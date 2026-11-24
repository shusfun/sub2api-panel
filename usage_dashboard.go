package main

import (
	"errors"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const usageHourLayout = "2006-01-02 15:04"

type usageModelBucket struct {
	Start  time.Time    `json:"start"`
	Models []usageModel `json:"models"`
}

type usageModelSeries struct {
	ReadyDays   int                `json:"ready_days"`
	TotalDays   int                `json:"total_days"`
	Buckets     []usageModelBucket `json:"buckets"`
	Unavailable bool               `json:"unavailable,omitempty"`
}

func usageWindows(filter usageFilter) []usageBucket {
	windows := []usageBucket{}
	for day := filter.start; !day.IsZero() && !day.After(filter.end); day = day.AddDate(0, 0, 1) {
		end := day.AddDate(0, 0, 1)
		for start := day; start.Before(end); {
			next := end
			if filter.granularity == "hour" && start.Add(time.Hour).Before(end) {
				next = start.Add(time.Hour)
			}
			windows = append(windows, usageBucket{Start: start, End: next})
			start = next
		}
	}
	return windows
}

func shiftedUsageDay(at time.Time) bool {
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, at.Location())
	return day.AddDate(0, 0, 1).Sub(day) != 24*time.Hour
}

func (total *usageTotals) add(row usageTotals) {
	total.Requests += row.Requests
	total.Cost += row.Cost
	total.Input += row.Input
	total.Output += row.Output
	total.CacheRead += row.CacheRead
	total.CacheWrite += row.CacheWrite
}

func usageTarget(query url.Values, actor int64) (int64, error) {
	if !query.Has("user_id") {
		return actor, nil
	}
	if len(query["user_id"]) != 1 {
		return 0, fail(400, "invalid_input")
	}
	if query.Get("user_id") == "all" {
		return 0, nil
	}
	id, err := strconv.ParseInt(query.Get("user_id"), 10, 64)
	if err != nil || id < 1 {
		return 0, fail(400, "invalid_input")
	}
	return id, nil
}

type usageKeyWire struct {
	namedItem
	Owner int64 `json:"user_id"`
}

func (p *panel) dashboardKeys(ctx context.Context, token string, target int64, admin bool, page int) (pageResult[namedItem], error) {
	path := "/keys"
	if admin {
		path = fmt.Sprintf("/admin/users/%d/api-keys", target)
	}
	return queryPage(ctx, p.upstream, token, path, page, nil, func(row usageKeyWire) (namedItem, error) {
		if row.Owner != target || row.ID < 1 {
			return namedItem{}, fail(502, "invalid_upstream")
		}
		return row.namedItem, nil
	})
}

func (p *panel) adminDashboardKeys(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	target, err := usageTarget(r.URL.Query(), a.user.ID)
	if err != nil {
		return err
	}
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	if target == 0 {
		writeJSON(w, 200, pageResult[namedItem]{Items: []namedItem{}})
		return nil
	}
	result, err := p.dashboardKeys(r.Context(), a.tokens.Access, target, target != a.user.ID, page)
	return send(w, 200, result, err)
}

func (p *panel) verifyDashboardKey(ctx context.Context, token string, target int64, admin bool, key int64) error {
	if key == 0 || admin && target == 0 {
		return nil
	}
	if !admin {
		row, err := upstream[usageKeyWire](ctx, p.upstream, "GET", fmt.Sprintf("/keys/%d", key), token, "", "", nil)
		// 官方对他人的 Key 返回 404；面板统一按无权处理，不区分「不存在」和「不是你的」。
		var missing upstreamStatus
		if errors.As(err, &missing) && missing.status == 404 {
			return fail(403, "not_allowed")
		}
		if err != nil {
			return err
		}
		if row.Owner != target || row.ID != key {
			return fail(403, "not_allowed")
		}
		return nil
	}
	query := url.Values{"page": {"1"}, "page_size": {"1000"}}
	for page := 1; page <= 1000; page++ {
		query.Set("page", strconv.Itoa(page))
		rows, err := upstream[wirePage[usageKeyWire]](ctx, p.upstream, "GET",
			fmt.Sprintf("/admin/users/%d/api-keys?%s", target, query.Encode()), token, "", "", nil)
		if err != nil {
			return err
		}
		if rows.Pages == nil || *rows.Pages < 0 || *rows.Pages > 1000 || len(rows.Items) > 1000 {
			return fail(502, "invalid_upstream")
		}
		for _, row := range rows.Items {
			if row.ID == key && row.Owner == target {
				return nil
			}
			if row.ID < 1 || row.Owner != target {
				return fail(502, "invalid_upstream")
			}
		}
		if page >= *rows.Pages {
			return fail(403, "not_allowed")
		}
	}
	return fail(502, "invalid_upstream")
}

func (p *panel) adminUsageQuery(ctx context.Context, a sessionAuth, query url.Values) (url.Values, error) {
	target, err := usageTarget(query, a.user.ID)
	if err != nil {
		return nil, err
	}
	filter, err := parseUsageFilter(query, time.Now().In(p.usageZone()), false)
	if err != nil {
		return nil, err
	}
	if err := p.verifyDashboardKey(ctx, a.tokens.Access, target, target != a.user.ID, filter.key); err != nil {
		return nil, err
	}
	if target == 0 {
		query.Set("user_id", "all")
	} else {
		query.Set("user_id", strconv.FormatInt(target, 10))
	}
	return query, nil
}

func (p *panel) adminDashboardUsage(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	target, err := usageTarget(r.URL.Query(), a.user.ID)
	if err != nil {
		return err
	}
	return p.dashboardUsageRecords(w, r, a, target, true)
}

func (p *panel) dashboardUsageRecords(w http.ResponseWriter, r *http.Request, a sessionAuth, target int64, admin bool) error {
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	query := r.URL.Query()
	filter, err := parseUsageFilter(query, time.Now().In(p.usageZone()), false)
	if err != nil {
		return err
	}
	if !admin && (query.Has("user_id") || query.Has("group_id") || query.Has("account_id")) {
		return fail(400, "invalid_input")
	}
	if err := p.verifyDashboardKey(r.Context(), a.tokens.Access, target, admin && target != a.user.ID, filter.key); err != nil {
		return err
	}
	if query.Has("bucket_start") {
		if filter.start.IsZero() || len(query["bucket_start"]) != 1 {
			return fail(400, "invalid_input")
		}
		start, err := time.Parse(time.RFC3339, query.Get("bucket_start"))
		if err != nil {
			return fail(400, "invalid_input")
		}
		for _, bucket := range usageWindows(filter) {
			if !bucket.Start.Equal(start) {
				continue
			}
			if filter.granularity == "hour" {
				result, err := p.readHourlyUsage(r.Context(), p.dashboardScope(a.user.ID, query, admin), filter, bucket, page)
				return send(w, 200, result, err)
			}
			filter.start, filter.end = bucket.Start, bucket.Start
			return p.sendUsageRecords(w, r, a, target, admin, filter, page)
		}
		return fail(400, "invalid_input")
	}
	return p.sendUsageRecords(w, r, a, target, admin, filter, page)
}

func (p *panel) sendUsageRecords(w http.ResponseWriter, r *http.Request, a sessionAuth, target int64, admin bool, filter usageFilter, page int) error {
	path, query := usageRecordsQuery(filter, target, admin)
	result, err := queryPage(r.Context(), p.upstream, a.tokens.Access, path, page, query, func(row usageRecordWire) (usageRecord, error) {
		return projectUsageRecordWithRole(row, filter, target, admin)
	})
	return send(w, 200, result, err)
}

func usageRecordsQuery(filter usageFilter, target int64, admin bool) (string, url.Values) {
	query := filter.query()
	if admin {
		if target > 0 {
			query.Set("user_id", strconv.FormatInt(target, 10))
		}
		return "/admin/usage", query
	}
	return "/usage", query
}

func projectUsageRecord(row usageRecordWire, filter usageFilter, target int64) (usageRecord, error) {
	return projectUsageRecordWithRole(row, filter, target, false)
}

func projectUsageRecordWithRole(row usageRecordWire, filter usageFilter, target int64, admin bool) (usageRecord, error) {
	if row.ID < 1 || row.User < 1 || target > 0 && row.User != target || row.KeyID < 1 || filter.key != 0 && row.KeyID != filter.key ||
		filter.group != 0 && (row.GroupID == nil || *row.GroupID != filter.group) ||
		filter.account != 0 && (row.AccountID == nil || *row.AccountID != filter.account) ||
		row.Model == "" || filter.model != "" && row.Model != filter.model || row.Created.IsZero() || row.Input < 0 || row.Output < 0 ||
		row.CacheRead != nil && *row.CacheRead < 0 || row.CacheWrite != nil && *row.CacheWrite < 0 ||
		!filter.start.IsZero() && (row.Created.Before(filter.start) || !row.Created.Before(filter.end.AddDate(0, 0, 1))) {
		return usageRecord{}, fail(502, "invalid_upstream")
	}
	name := fmt.Sprintf("Key #%d", row.KeyID)
	if row.Key != nil && row.Key.ID == row.KeyID && row.Key.Name != "" {
		name = row.Key.Name
	}
	row.Created = row.Created.In(timezoneLocation(filter.timezone))
	rec := usageRecord{usageView: row.usageView, CacheRead: row.CacheRead, CacheWrite: row.CacheWrite, KeyName: name}
	if admin {
		rec.KeyID = row.KeyID
		rec.UserID = row.User
		if row.UserObj != nil {
			rec.UserEmail = row.UserObj.Email
		}
		rec.GroupID = row.GroupID
		if row.Group != nil {
			rec.GroupName = row.Group.Name
		}
		rec.AccountID = row.AccountID
		if row.Account != nil {
			rec.AccountName = row.Account.Name
		}
		rec.DurationMs = row.DurationMs
		rec.FirstTokenMs = row.FirstTokenMs
		rec.Stream = row.Stream
	}
	return rec, nil
}

func timezoneLocation(name string) *time.Location {
	zone, err := loadUsageLocation(name)
	if err != nil {
		return time.UTC
	}
	return zone
}
