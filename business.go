package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type wirePage[T any] struct {
	Items []T  `json:"items"`
	Pages *int `json:"pages"`
}

func queryPage[T any, V any](ctx context.Context, c serviceClient, token, path string, page int, query url.Values, project func(T) (V, error)) (pageResult[V], error) {
	if query == nil {
		query = url.Values{}
	}
	query.Set("page", strconv.Itoa(page))
	query.Set("page_size", strconv.Itoa(pageSize))
	input, err := upstream[wirePage[T]](ctx, c, "GET", path+"?"+query.Encode(), token, "", "", nil)
	if err != nil {
		return pageResult[V]{}, err
	}
	if input.Pages == nil || *input.Pages < 0 || len(input.Items) > pageSize {
		return pageResult[V]{}, fail(502, "invalid_upstream")
	}
	items, err := projectList(input.Items, project)
	return pageResult[V]{items, page < *input.Pages}, err
}

func projectList[T any, V any](input []T, project func(T) (V, error)) ([]V, error) {
	out := make([]V, 0, len(input))
	for _, item := range input {
		value, err := project(item)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func requiredID(id int64) error {
	if id < 1 {
		return fail(502, "invalid_upstream")
	}
	return nil
}

func send[T any](w http.ResponseWriter, status int, data T, err error) error {
	if err == nil {
		writeJSON(w, status, data)
	}
	return err
}

type balanceView struct {
	Balance *float64 `json:"balance"`
}

type namedItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type subscriptionWire struct {
	ID      int64      `json:"id"`
	GroupID int64      `json:"group_id"`
	Group   *namedItem `json:"group"`
	Status  string     `json:"status"`
	Expires time.Time  `json:"expires_at"`
}

type subscriptionView struct {
	ID      int64     `json:"id"`
	Group   string    `json:"group"`
	Status  string    `json:"status"`
	Expires time.Time `json:"expires_at"`
}

func projectSubscription(s subscriptionWire) (subscriptionView, error) {
	group := fmt.Sprintf("分组 %d", s.GroupID)
	if s.Group != nil && s.Group.Name != "" {
		group = s.Group.Name
	}
	return subscriptionView{s.ID, group, s.Status, s.Expires}, requiredID(s.ID)
}

type usageView struct {
	ID      int64     `json:"id"`
	Model   string    `json:"model"`
	Created time.Time `json:"created_at"`
	Input   int64     `json:"input_tokens"`
	Output  int64     `json:"output_tokens"`
	Cost    *float64  `json:"actual_cost"`
}

type userRow struct {
	ID      int64    `json:"id"`
	Email   string   `json:"email"`
	Status  string   `json:"status"`
	Balance *float64 `json:"balance"`
}

type keyWire struct {
	namedItem
	Key    string `json:"key"`
	Status string `json:"status"`
	Group  *int64 `json:"group_id"`
	Owner  int64  `json:"user_id"`
}

type keyView struct {
	namedItem
	Masked string `json:"masked"`
	Status string `json:"status"`
}

func projectKey(k keyWire, owner int64) (keyView, error) {
	if k.Owner != owner {
		return keyView{}, fail(502, "invalid_upstream")
	}
	masked := "••••••••"
	if len(k.Key) > 12 {
		masked += k.Key[len(k.Key)-4:]
	}
	return keyView{k.namedItem, masked, k.Status}, requiredID(k.ID)
}

type quotaWire struct {
	Usage   *float64   `json:"utilization"`
	Unknown bool       `json:"utilization_unknown"`
	Reset   *time.Time `json:"resets_at"`
}

type quotaView struct {
	Usage *float64   `json:"percent_used"`
	Reset *time.Time `json:"resets_at"`
}

type accountWire struct {
	namedItem
	Platform    string `json:"platform"`
	Status      string `json:"status"`
	Schedulable *bool  `json:"schedulable"`
	Extra       struct {
		Five *float64 `json:"codex_5h_used_percent"`
		Week *float64 `json:"codex_7d_used_percent"`
	} `json:"extra"`
}

type accountRow struct {
	namedItem
	Platform    string   `json:"platform"`
	Status      string   `json:"status"`
	Schedulable bool     `json:"schedulable"`
	Five        *float64 `json:"five_hours_used"`
	Week        *float64 `json:"week_used"`
}

func projectAccount(a accountWire) (accountRow, error) {
	if a.ID < 1 || a.Schedulable == nil {
		return accountRow{}, fail(502, "invalid_upstream")
	}
	return accountRow{a.namedItem, a.Platform, a.Status, *a.Schedulable, a.Extra.Five, a.Extra.Week}, nil
}

func (p *panel) businessRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api/balance", false, func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		row, err := upstream[balanceView](r.Context(), p.upstream, "GET", "/user/profile", a.tokens.Access, "", "", nil)
		return send(w, 200, row, err)
	})
	p.route(mux, "GET /api/subscriptions", false, func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		rows, err := upstream[[]subscriptionWire](r.Context(), p.upstream, "GET", "/subscriptions", a.tokens.Access, "", "", nil)
		if err != nil {
			return err
		}
		data, err := projectList(rows, projectSubscription)
		return send(w, 200, data, err)
	})
	p.route(mux, "GET /api/usage", false, p.usage)
	p.route(mux, "GET /api/usage/summary", false, p.usageSummary)
	p.route(mux, "GET /api/usage/keys", false, p.usageKeys)
	p.route(mux, "GET /api/keys", false, func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		page, err := requestedPage(r)
		if err != nil {
			return err
		}
		rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/keys", page, nil, func(k keyWire) (keyView, error) {
			return projectKey(k, a.user.ID)
		})
		return send(w, 200, rows, err)
	})
	p.route(mux, "GET /api/key-groups", false, func(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
		rows, err := upstream[[]namedItem](r.Context(), p.upstream, "GET", "/groups/available", a.tokens.Access, "", "", nil)
		if rows == nil {
			rows = []namedItem{}
		}
		return send(w, 200, rows, err)
	})
	p.route(mux, "POST /api/keys", false, p.createKey)
	p.route(mux, "PUT /api/keys/{id}", false, p.updateKey)
	p.route(mux, "DELETE /api/keys/{id}", false, p.deleteKey)
	p.route(mux, "POST /api/redeem", false, p.redeem)
	p.route(mux, "GET /api/referral", false, p.referral)
	p.route(mux, "GET /api/admin/users", true, p.users)
	p.route(mux, "GET /api/admin/users/{user}/usage", true, p.adminUsage)
	p.route(mux, "GET /api/admin/users/{user}/subscriptions", true, p.userSubscriptions)
	p.route(mux, "GET /api/admin/accounts", true, p.accounts)
	p.route(mux, "GET /api/admin/accounts/{id}/quota", true, p.accountQuota)
	p.route(mux, "POST /api/admin/accounts/{id}/test", true, p.testAccount)
	p.route(mux, "PUT /api/admin/accounts/{id}/scheduling", true, p.scheduleAccount)
}

func (p *panel) adminUsage(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	id, err := pathID(r, "user")
	if err != nil {
		return err
	}
	query := url.Values{"user_id": {strconv.FormatInt(id, 10)}}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/usage", page, query, func(row struct {
		usageView
		UserID int64 `json:"user_id"`
	}) (usageView, error) {
		if row.UserID != id {
			return usageView{}, fail(502, "invalid_upstream")
		}
		return row.usageView, requiredID(row.ID)
	})
	return send(w, 200, rows, err)
}

type keyInput struct {
	Name   string `json:"name"`
	Group  *int64 `json:"group_id,omitempty"`
	Status string `json:"status,omitempty"`
}

func (p *panel) createKey(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[keyInput](r)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.Name) == "" || len(input.Name) > 100 || input.Status != "" || input.Group != nil && *input.Group < 1 {
		return fail(400, "invalid_input")
	}
	result, err := upstream[keyWire](r.Context(), p.upstream, "POST", "/keys", a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	if result.ID < 1 || result.Owner != a.user.ID || result.Key == "" {
		return fail(502, "invalid_upstream")
	}
	writeJSON(w, 201, struct {
		Key string `json:"key"`
	}{result.Key})
	return nil
}

func (p *panel) updateKey(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[keyInput](r)
	if err != nil {
		return err
	}
	if len(input.Name) > 100 || input.Name != "" && strings.TrimSpace(input.Name) == "" || input.Group != nil ||
		input.Name == "" && input.Status == "" || input.Status != "" && input.Status != "active" && input.Status != "inactive" {
		return fail(400, "invalid_input")
	}
	data, err := upstream[keyWire](r.Context(), p.upstream, "PUT", fmt.Sprintf("/keys/%d", id), a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	if data.ID != id || data.Owner != a.user.ID {
		return fail(502, "invalid_upstream")
	}
	writeJSON(w, 200, struct{}{})
	return nil
}

func (p *panel) deleteKey(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "DELETE", fmt.Sprintf("/keys/%d", id), a.tokens.Access, "", "", nil)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) redeem(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Code string `json:"code"`
	}](r)
	if err != nil {
		return err
	}
	input.Code = strings.TrimSpace(input.Code)
	if len(input.Code) < 3 || len(input.Code) > 128 {
		return fail(400, "invalid_input")
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "POST", "/redeem", a.tokens.Access, "", "", input)
	return send(w, 200, struct{}{}, err)
}

func (p *panel) referral(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	settings, err := p.settings(r.Context())
	if err != nil {
		return err
	}
	if !settings.Affiliate {
		return fail(403, "not_allowed")
	}
	data, err := upstream[struct {
		Code string `json:"aff_code"`
	}](r.Context(), p.upstream, "GET", "/user/aff", a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if data.Code == "" {
		return fail(502, "invalid_upstream")
	}
	writeJSON(w, 200, struct {
		URL string `json:"url"`
	}{p.official + "/register?aff=" + url.QueryEscape(data.Code)})
	return nil
}

func listQuery(r *http.Request, names ...string) (int, url.Values, error) {
	page, err := requestedPage(r)
	if err != nil {
		return 0, nil, err
	}
	params := url.Values{}
	for _, name := range names {
		value := strings.TrimSpace(r.URL.Query().Get(name))
		if len(value) > 100 {
			return 0, nil, fail(400, "invalid_input")
		}
		if value != "" {
			params.Set(name, value)
		}
	}
	return page, params, nil
}

func (p *panel) users(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, params, err := listQuery(r, "search")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/users", page, params, func(u userRow) (userRow, error) {
		return u, requiredID(u.ID)
	})
	return send(w, 200, rows, err)
}

func (p *panel) userSubscriptions(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "user")
	if err != nil {
		return err
	}
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/subscriptions", page,
		url.Values{"user_id": {strconv.FormatInt(id, 10)}}, projectSubscription)
	return send(w, 200, rows, err)
}

func (p *panel) accounts(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, params, err := listQuery(r, "search", "platform", "status")
	if err != nil {
		return err
	}
	result, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/accounts", page, params, projectAccount)
	return send(w, 200, result, err)
}

func (p *panel) accountQuota(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	raw, err := upstream[struct {
		Five *quotaWire `json:"five_hour"`
		Week *quotaWire `json:"seven_day"`
	}](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/accounts/%d/usage?source=passive", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	project := func(window *quotaWire) *quotaView {
		if window == nil {
			return nil
		}
		if window.Unknown {
			window.Usage = nil
		}
		return &quotaView{window.Usage, window.Reset}
	}
	writeJSON(w, 200, struct {
		Five *quotaView `json:"five_hours"`
		Week *quotaView `json:"week"`
	}{project(raw.Five), project(raw.Week)})
	return nil
}

func (p *panel) scheduleAccount(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		On *bool `json:"schedulable"`
	}](r)
	if err != nil {
		return err
	}
	if input.On == nil {
		return fail(400, "invalid_input")
	}
	path := fmt.Sprintf("/admin/accounts/%d", id)
	_, err = upstream[struct{}](r.Context(), p.upstream, "POST", path+"/schedulable", a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	data, err := upstream[accountWire](r.Context(), p.upstream, "GET", path, a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if data.ID != id || data.Schedulable == nil {
		return fail(502, "invalid_upstream")
	}
	if *data.Schedulable != *input.On {
		return fail(409, "schedule_changed")
	}
	writeJSON(w, 200, struct{}{})
	return nil
}

func (p *panel) testAccount(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		Model string `json:"model_id,omitempty"`
	}](r)
	if err != nil || len(input.Model) > 128 {
		return fail(400, "invalid_input")
	}
	stream := p.upstream
	client := *stream.client
	client.Timeout = 70 * time.Second
	stream.client = &client
	res, err := stream.open(r.Context(), "POST", fmt.Sprintf("/api/v1/admin/accounts/%d/test", id), a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return upstreamStatus{res.StatusCode}
	}
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		return fail(502, "invalid_upstream")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 2048), 256<<10)
	start := time.Now()
	emit(w, "running")
	for scanner.Scan() {
		frame, ok := strings.CutPrefix(scanner.Text(), "data:")
		if !ok {
			continue
		}
		var input struct {
			Type    string `json:"type"`
			Success bool   `json:"success"`
		}
		if json.Unmarshal([]byte(frame), &input) == nil {
			if input.Type == "test_complete" && input.Success {
				emit(w, "passed")
				slogAccountResult(r.Context(), id, true, start)
				return nil
			}
			if input.Type == "error" || input.Type == "test_complete" {
				break
			}
		}
	}
	emit(w, "failed")
	slogAccountResult(r.Context(), id, false, start)
	return nil
}

func emit(w http.ResponseWriter, state string) {
	fmt.Fprintf(w, "data: {\"state\":\"%s\"}\n\n", state)
	_ = http.NewResponseController(w).Flush()
}

func slogAccountResult(ctx context.Context, id int64, passed bool, start time.Time) {
	slog.Info("account test finished", "trace_id", meta(ctx).Trace, "account_id", id,
		"passed", passed, "elapsed_ms", time.Since(start).Milliseconds())
}
