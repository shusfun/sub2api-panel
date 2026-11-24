package pool

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func AccountActivityPage(s *core.Server, r *http.Request, token string) (platform.PageResult[AccountAdminView], error) {
	return (&handler{s: s}).accountActivityPage(r, token)
}

func (h *handler) accountActivityPage(r *http.Request, token string) (platform.PageResult[AccountAdminView], error) {
	page, query, err := platform.ListQuery(r, "search", "platform", "type", "status", "group", "proxy_id")
	if err != nil {
		return platform.PageResult[AccountAdminView]{}, err
	}
	ctx := r.Context()
	rows, err := h.filteredAccounts(ctx, token, query)
	if err != nil {
		return platform.PageResult[AccountAdminView]{}, err
	}
	if len(rows) > 0 {
		activity, err := h.accountConcurrency(ctx, token)
		if err != nil {
			if ctx.Err() != nil {
				return platform.PageResult[AccountAdminView]{}, ctx.Err()
			}
			activity = nil
		}
		for i := range rows {
			count, ok := activity[rows[i].ID]
			rows[i].ActivityKnown = &ok
			rows[i].CurrentConcurrency = count
		}
		SortAccountsByActivity(rows)
	}
	start := min((page-1)*platform.PageSize, len(rows))
	end := min(start+platform.PageSize, len(rows))
	items := rows[start:end]
	h.withDayStats(ctx, token, items)
	h.applyBalanceSnapshots(items)
	return platform.PageResult[AccountAdminView]{Items: items, More: end < len(rows), Pages: (len(rows) + platform.PageSize - 1) / platform.PageSize}, nil
}

func (h *handler) filteredAccounts(ctx context.Context, token string, query url.Values) ([]AccountAdminView, error) {
	proxyID := int64(0)
	if raw := query.Get("proxy_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 {
			return nil, platform.Fail(400, "invalid_input")
		}
		proxyID = parsed
		query.Del("proxy_id")
	}
	query.Set("sort_by", "id")
	query.Set("sort_order", "asc")
	const batch = 100
	const maxPages = 100
	query.Set("page_size", strconv.Itoa(batch))
	rows := make([]AccountAdminView, 0)
	seen := make(map[int64]bool)
	for cursor := 1; ; cursor++ {
		query.Set("page", strconv.Itoa(cursor))
		raw, err := platform.Upstream[platform.WirePage[AccountDetailWire]](ctx, h.s.Upstream, "GET", "/admin/accounts?"+query.Encode(), token, "", "", nil)
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
			return nil, platform.Fail(502, "invalid_upstream")
		}
		for _, item := range raw.Items {
			if seen[item.ID] || proxyID > 0 && (item.ProxyID == nil || *item.ProxyID != proxyID) {
				continue
			}
			row, err := ProjectAdminAccount(item)
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

func (h *handler) filteredAccountTargets(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	_, query, err := platform.ListQuery(r, "search", "platform", "type", "status", "group", "proxy_id", "sort_by")
	if err != nil {
		return err
	}
	rows, err := h.filteredAccounts(r.Context(), a.Tokens.Access, query)
	if err != nil {
		return err
	}
	targets := make([]platform.NamedItem, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, platform.NamedItem{ID: row.ID, Name: row.Name})
	}
	platform.WriteJSON(w, http.StatusOK, targets)
	return nil
}

func (h *handler) accountConcurrency(ctx context.Context, token string) (map[int64]int, error) {
	raw, err := platform.Upstream[struct {
		Accounts map[string]struct {
			ID    int64 `json:"account_id"`
			InUse *int  `json:"current_in_use"`
		} `json:"account"`
	}](ctx, h.s.Upstream, "GET", "/admin/ops/concurrency", token, "", "", nil)
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

func SortAccountsByActivity(rows []AccountAdminView) {
	lastUsed := make(map[int64]time.Time, len(rows))
	for _, row := range rows {
		if row.LastUsedAt != nil {
			lastUsed[row.ID], _ = time.Parse(time.RFC3339Nano, *row.LastUsedAt)
		}
	}
	slices.SortFunc(rows, func(a, b AccountAdminView) int {
		if (a.CurrentConcurrency > 0) != (b.CurrentConcurrency > 0) {
			if a.CurrentConcurrency > 0 {
				return -1
			}
			return 1
		}
		return cmp.Or(lastUsed[b.ID].Compare(lastUsed[a.ID]), cmp.Compare(a.Priority, b.Priority), cmp.Compare(a.ID, b.ID))
	})
}

func BuildAccountActivitySnapshot(s *core.Server) func(context.Context, string, string) ([]byte, error) {
	h := &handler{s: s}
	return func(ctx context.Context, token, rawQuery string) ([]byte, error) {
		query, err := url.ParseQuery(rawQuery)
		if err != nil {
			return nil, platform.Fail(400, "invalid_input")
		}
		current, err := platform.CurrentUser(ctx, s.Upstream, token)
		if err != nil {
			return nil, err
		}
		if current.Role != "admin" || strconv.FormatInt(current.ID, 10) != query.Get("actor") {
			return nil, platform.Fail(403, "not_allowed")
		}
		req := (&http.Request{URL: &url.URL{RawQuery: rawQuery}}).WithContext(ctx)
		result, err := h.fetchAccountPage(req, token)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	}
}

func (h *handler) accountActivityStream(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	if h.s.AccountHub == nil {
		return platform.Fail(503, "service_unavailable")
	}
	page, query, err := platform.ListQuery(r, "search", "platform", "type", "status", "group", "proxy_id", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	query.Set("page", strconv.Itoa(page))
	query.Set("actor", strconv.FormatInt(a.User.ID, 10))
	scope := core.UsageScope(a.User.ID, "accounts|"+h.s.Upstream.Base+"|"+query.Encode())
	return core.ServeSnapshotStream(w, r, h.s.AccountHub, scope, a.Tokens.Access, query.Encode())
}
