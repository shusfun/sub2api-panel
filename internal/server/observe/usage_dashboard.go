package observe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type usageKeyWire struct {
	platform.NamedItem
	Owner int64 `json:"user_id"`
}

func dashboardKeys(s *core.Server, ctx context.Context, token string, target int64, admin bool, page int) (platform.PageResult[platform.NamedItem], error) {
	path := "/keys"
	if admin {
		path = fmt.Sprintf("/admin/users/%d/api-keys", target)
	}
	return core.QueryPage(ctx, s.Upstream, token, path, page, nil, func(row usageKeyWire) (platform.NamedItem, error) {
		if row.Owner != target || row.ID < 1 {
			return platform.NamedItem{}, platform.Fail(502, "invalid_upstream")
		}
		return row.NamedItem, nil
	})
}

func adminDashboardKeys(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		target, err := usageTarget(r.URL.Query(), a.User.ID)
		if err != nil {
			return err
		}
		page, err := platform.RequestedPage(r)
		if err != nil {
			return err
		}
		if target == 0 {
			platform.WriteJSON(w, 200, platform.PageResult[platform.NamedItem]{Items: []platform.NamedItem{}})
			return nil
		}
		result, err := dashboardKeys(s, r.Context(), a.Tokens.Access, target, target != a.User.ID, page)
		return platform.Send(w, 200, result, err)
	}
}

func verifyDashboardKey(s *core.Server, ctx context.Context, token string, target int64, admin bool, key int64) error {
	if key == 0 || admin && target == 0 {
		return nil
	}
	if !admin {
		row, err := platform.Upstream[usageKeyWire](ctx, s.Upstream, "GET", fmt.Sprintf("/keys/%d", key), token, "", "", nil)
		// 官方对他人的 Key 返回 404；面板统一按无权处理，不区分「不存在」和「不是你的」。
		var missing platform.UpstreamStatus
		if errors.As(err, &missing) && missing.Status == 404 {
			return platform.Fail(403, "not_allowed")
		}
		if err != nil {
			return err
		}
		if row.Owner != target || row.ID != key {
			return platform.Fail(403, "not_allowed")
		}
		return nil
	}
	query := url.Values{"page": {"1"}, "page_size": {"1000"}}
	for page := 1; page <= 1000; page++ {
		query.Set("page", strconv.Itoa(page))
		rows, err := platform.Upstream[platform.WirePage[usageKeyWire]](ctx, s.Upstream, "GET",
			fmt.Sprintf("/admin/users/%d/api-keys?%s", target, query.Encode()), token, "", "", nil)
		if err != nil {
			return err
		}
		if rows.Pages == nil || *rows.Pages < 0 || *rows.Pages > 1000 || len(rows.Items) > 1000 {
			return platform.Fail(502, "invalid_upstream")
		}
		for _, row := range rows.Items {
			if row.ID == key && row.Owner == target {
				return nil
			}
			if row.ID < 1 || row.Owner != target {
				return platform.Fail(502, "invalid_upstream")
			}
		}
		if page >= *rows.Pages {
			return platform.Fail(403, "not_allowed")
		}
	}
	return platform.Fail(502, "invalid_upstream")
}

func AdminUsageQuery(s *core.Server, ctx context.Context, a core.SessionAuth, raw url.Values) (url.Values, error) {
	query := make(url.Values, len(raw)+1)
	for k, v := range raw {
		query[k] = append([]string(nil), v...)
	}
	target, err := usageTarget(query, a.User.ID)
	if err != nil {
		return nil, err
	}
	filter, err := ParseUsageFilter(query, time.Now().In(s.UsageZone()), false)
	if err != nil {
		return nil, err
	}
	if err := verifyDashboardKey(s, ctx, a.Tokens.Access, target, target != a.User.ID, filter.Key); err != nil {
		return nil, err
	}
	if target == 0 {
		query.Set("user_id", "all")
	} else {
		query.Set("user_id", strconv.FormatInt(target, 10))
	}
	return query, nil
}

func adminDashboardUsage(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		target, err := usageTarget(r.URL.Query(), a.User.ID)
		if err != nil {
			return err
		}
		return dashboardUsageRecords(s, w, r, a, target, true)
	}
}

func dashboardUsageRecords(s *core.Server, w http.ResponseWriter, r *http.Request, a core.SessionAuth, target int64, admin bool) error {
	page, err := platform.RequestedPage(r)
	if err != nil {
		return err
	}
	query := r.URL.Query()
	filter, err := ParseUsageFilter(query, time.Now().In(s.UsageZone()), false)
	if err != nil {
		return err
	}
	if !admin && (query.Has("user_id") || query.Has("group_id") || query.Has("account_id")) {
		return platform.Fail(400, "invalid_input")
	}
	if err := verifyDashboardKey(s, r.Context(), a.Tokens.Access, target, admin && target != a.User.ID, filter.Key); err != nil {
		return err
	}
	if query.Has("bucket_start") {
		if filter.Start.IsZero() || len(query["bucket_start"]) != 1 {
			return platform.Fail(400, "invalid_input")
		}
		start, err := time.Parse(time.RFC3339, query.Get("bucket_start"))
		if err != nil {
			return platform.Fail(400, "invalid_input")
		}
		for _, bucket := range UsageWindows(filter) {
			if !bucket.Start.Equal(start) {
				continue
			}
			if filter.Granularity == "hour" {
				result, err := readHourlyUsage(s, r.Context(), DashboardScope(s, a.User.ID, query, admin), filter, bucket, page)
				return platform.Send(w, 200, result, err)
			}
			filter.Start, filter.End = bucket.Start, bucket.Start
			return sendUsageRecords(s, w, r, a, target, admin, filter, page)
		}
		return platform.Fail(400, "invalid_input")
	}
	return sendUsageRecords(s, w, r, a, target, admin, filter, page)
}

func sendUsageRecords(s *core.Server, w http.ResponseWriter, r *http.Request, a core.SessionAuth, target int64, admin bool, filter UsageFilter, page int) error {
	path, query := usageRecordsQuery(filter, target, admin)
	result, err := core.QueryPage(r.Context(), s.Upstream, a.Tokens.Access, path, page, query, func(row usageRecordWire) (UsageRecord, error) {
		return projectUsageRecordWithRole(row, filter, target, admin)
	})
	return platform.Send(w, 200, result, err)
}

func usageRecordsQuery(filter UsageFilter, target int64, admin bool) (string, url.Values) {
	query := filter.Query()
	if admin {
		if target > 0 {
			query.Set("user_id", strconv.FormatInt(target, 10))
		}
		return "/admin/usage", query
	}
	return "/usage", query
}

func projectUsageRecord(row usageRecordWire, filter UsageFilter, target int64) (UsageRecord, error) {
	return projectUsageRecordWithRole(row, filter, target, false)
}

func projectUsageRecordWithRole(row usageRecordWire, filter UsageFilter, target int64, admin bool) (UsageRecord, error) {
	if row.ID < 1 || row.User < 1 || target > 0 && row.User != target || row.KeyID < 1 || filter.Key != 0 && row.KeyID != filter.Key ||
		filter.Group != 0 && (row.GroupID == nil || *row.GroupID != filter.Group) ||
		filter.Account != 0 && (row.AccountID == nil || *row.AccountID != filter.Account) ||
		row.Model == "" || filter.Model != "" && row.Model != filter.Model || row.Created.IsZero() || row.Input < 0 || row.Output < 0 ||
		row.CacheRead != nil && *row.CacheRead < 0 || row.CacheWrite != nil && *row.CacheWrite < 0 ||
		!filter.Start.IsZero() && (row.Created.Before(filter.Start) || !row.Created.Before(filter.End.AddDate(0, 0, 1))) {
		return UsageRecord{}, platform.Fail(502, "invalid_upstream")
	}
	name := fmt.Sprintf("Key #%d", row.KeyID)
	if row.Key != nil && row.Key.ID == row.KeyID && row.Key.Name != "" {
		name = row.Key.Name
	}
	row.Created = row.Created.In(timezoneLocation(filter.Timezone))
	rec := UsageRecord{UsageView: row.UsageView, CacheRead: row.CacheRead, CacheWrite: row.CacheWrite, KeyName: name}
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
