package main

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"
)

func (p *panel) accountActivityPage(r *http.Request, token string) (pageResult[accountAdminView], error) {
	page, query, err := listQuery(r, "search", "platform", "type", "status", "group", "proxy_id")
	if err != nil {
		return pageResult[accountAdminView]{}, err
	}
	ctx := r.Context()
	rows, err := p.filteredAccounts(ctx, token, query)
	if err != nil {
		return pageResult[accountAdminView]{}, err
	}
	if len(rows) > 0 {
		activity, err := p.accountConcurrency(ctx, token)
		if err != nil {
			if ctx.Err() != nil {
				return pageResult[accountAdminView]{}, ctx.Err()
			}
			activity = nil
		}
		for i := range rows {
			count, ok := activity[rows[i].ID]
			rows[i].ActivityKnown = &ok
			rows[i].CurrentConcurrency = count
		}
		sortAccountsByActivity(rows)
	}
	start := min((page-1)*pageSize, len(rows))
	end := min(start+pageSize, len(rows))
	items := rows[start:end]
	p.withDayStats(ctx, token, items)
	p.applyBalanceSnapshots(items)
	return pageResult[accountAdminView]{Items: items, More: end < len(rows), Pages: (len(rows) + pageSize - 1) / pageSize}, nil
}

func (p *panel) filteredAccounts(ctx context.Context, token string, query url.Values) ([]accountAdminView, error) {
	proxyID := int64(0)
	if raw := query.Get("proxy_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 {
			return nil, fail(400, "invalid_input")
		}
		proxyID = parsed
		query.Del("proxy_id")
	}
	query.Set("sort_by", "id")
	query.Set("sort_order", "asc")
	const batch = 100
	const maxPages = 100
	query.Set("page_size", strconv.Itoa(batch))
	rows := make([]accountAdminView, 0)
	seen := make(map[int64]bool)
	for cursor := 1; ; cursor++ {
		query.Set("page", strconv.Itoa(cursor))
		raw, err := upstream[wirePage[accountDetailWire]](ctx, p.upstream, "GET", "/admin/accounts?"+query.Encode(), token, "", "", nil)
		if err != nil {
			return nil, err
		}
		pages := -1
		if raw.Pages != nil {
			pages = *raw.Pages
		} else if raw.Total != nil && *raw.Total >= 0 {
			pages = int((*raw.Total + batch - 1) / batch)
		}
		if pages < 0 || pages > maxPages || len(raw.Items) > batch {
			return nil, fail(502, "invalid_upstream")
		}
		for _, item := range raw.Items {
			if seen[item.ID] || proxyID > 0 && (item.ProxyID == nil || *item.ProxyID != proxyID) {
				continue
			}
			row, err := projectAdminAccount(item)
			if err != nil {
				return nil, err
			}
			seen[row.ID] = true
			rows = append(rows, row)
		}
		if cursor >= pages {
			break
		}
	}
	return rows, nil
}

func (p *panel) filteredAccountTargets(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	_, query, err := listQuery(r, "search", "platform", "type", "status", "group", "proxy_id", "sort_by")
	if err != nil {
		return err
	}
	rows, err := p.filteredAccounts(r.Context(), a.tokens.Access, query)
	if err != nil {
		return err
	}
	targets := make([]namedItem, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, namedItem{ID: row.ID, Name: row.Name})
	}
	writeJSON(w, http.StatusOK, targets)
	return nil
}

func (p *panel) accountConcurrency(ctx context.Context, token string) (map[int64]int, error) {
	raw, err := upstream[struct {
		Accounts map[string]struct {
			ID    int64 `json:"account_id"`
			InUse *int  `json:"current_in_use"`
		} `json:"account"`
	}](ctx, p.upstream, "GET", "/admin/ops/concurrency", token, "", "", nil)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]int, len(raw.Accounts))
	for _, item := range raw.Accounts {
		if item.ID > 0 && item.InUse != nil && *item.InUse >= 0 {
			result[item.ID] = *item.InUse
		}
	}
	return result, nil
}

func sortAccountsByActivity(rows []accountAdminView) {
	lastUsed := make(map[int64]time.Time, len(rows))
	for _, row := range rows {
		if row.LastUsedAt != nil {
			lastUsed[row.ID], _ = time.Parse(time.RFC3339Nano, *row.LastUsedAt)
		}
	}
	slices.SortFunc(rows, func(a, b accountAdminView) int {
		if (a.CurrentConcurrency > 0) != (b.CurrentConcurrency > 0) {
			if a.CurrentConcurrency > 0 {
				return -1
			}
			return 1
		}
		return cmp.Or(lastUsed[b.ID].Compare(lastUsed[a.ID]), cmp.Compare(a.Priority, b.Priority), cmp.Compare(a.ID, b.ID))
	})
}

func (p *panel) buildAccountActivitySnapshot(ctx context.Context, token, rawQuery string) ([]byte, error) {
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, fail(400, "invalid_input")
	}
	current, err := currentUser(ctx, p.upstream, token)
	if err != nil {
		return nil, err
	}
	if current.Role != "admin" || strconv.FormatInt(current.ID, 10) != query.Get("actor") {
		return nil, fail(403, "not_allowed")
	}
	req := (&http.Request{URL: &url.URL{RawQuery: rawQuery}}).WithContext(ctx)
	result, err := p.fetchAccountPage(req, token)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func (p *panel) accountActivityStream(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	if p.accountHub == nil {
		return fail(503, "service_unavailable")
	}
	page, query, err := listQuery(r, "search", "platform", "type", "status", "group", "proxy_id", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	query.Set("page", strconv.Itoa(page))
	query.Set("actor", strconv.FormatInt(a.user.ID, 10))
	scope := usageScope(a.user.ID, "accounts|"+p.upstream.base+"|"+query.Encode())
	return serveSnapshotStream(w, r, p.accountHub, scope, a.tokens.Access, query.Encode())
}
