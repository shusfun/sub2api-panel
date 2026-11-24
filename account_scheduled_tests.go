package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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

// The official scheduler returns a bare JSON document, unlike most admin routes.
func schedulerResponse[T any](ctx context.Context, client serviceClient, method, path, token string, body any) (T, error) {
	var value T
	response, err := client.open(ctx, method, "/api/v1"+path, token, "", "", body)
	if err != nil {
		return value, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return value, upstreamStatus{response.StatusCode}
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF {
		return value, fail(502, "invalid_upstream")
	}
	return value, nil
}

func (p *panel) scheduledAccountTests(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	account, err := pathID(r, "id")
	if err != nil {
		return err
	}
	plans, err := schedulerResponse[[]scheduledTestPlanView](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/accounts/%d/scheduled-test-plans", account), a.tokens.Access, nil)
	if err != nil {
		return err
	}
	if plans == nil {
		plans = []scheduledTestPlanView{}
	}
	for _, plan := range plans {
		if plan.ID < 1 || plan.AccountID != account {
			return fail(502, "invalid_upstream")
		}
	}
	writeJSON(w, 200, plans)
	return nil
}

func (p *panel) editAccountTestPlan(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	account, err := pathID(r, "id")
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
		input, err := decodeBody[inputView](r)
		if err != nil || strings.TrimSpace(input.Cron) == "" || len(input.Cron) > 100 || len(input.Model) > 200 || input.MaxResults < 0 || input.MaxResults > 1000 {
			return fail(400, "invalid_input")
		}
		result, err := schedulerResponse[scheduledTestPlanView](r.Context(), p.upstream, "POST", base, a.tokens.Access, struct {
			AccountID int64 `json:"account_id"`
			inputView
		}{account, input})
		if err == nil && (result.ID < 1 || result.AccountID != account) {
			return fail(502, "invalid_upstream")
		}
		return send(w, 201, result, err)
	}
	plan, err := pathID(r, "plan")
	if err != nil {
		return err
	}
	list, err := schedulerResponse[[]scheduledTestPlanView](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/accounts/%d/scheduled-test-plans", account), a.tokens.Access, nil)
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
		return fail(404, "not_found")
	}
	path := fmt.Sprintf("%s/%d", base, plan)
	switch r.Method {
	case "GET":
		items, err := schedulerResponse[[]scheduledTestResultView](r.Context(), p.upstream, "GET", path+"/results?limit=25", a.tokens.Access, nil)
		if items == nil {
			items = []scheduledTestResultView{}
		}
		return send(w, 200, items, err)
	case "PUT":
		input, err := decodeBody[inputView](r)
		if err != nil || len(input.Cron) > 100 || len(input.Model) > 200 || input.MaxResults < 0 || input.MaxResults > 1000 {
			return fail(400, "invalid_input")
		}
		result, err := schedulerResponse[scheduledTestPlanView](r.Context(), p.upstream, "PUT", path, a.tokens.Access, input)
		if err == nil && (result.ID != plan || result.AccountID != account) {
			return fail(502, "invalid_upstream")
		}
		return send(w, 200, result, err)
	default:
		_, err := schedulerResponse[struct{}](r.Context(), p.upstream, "DELETE", path, a.tokens.Access, nil)
		return send(w, 200, struct{}{}, err)
	}
}

func (p *panel) revertAccountProxy(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/accounts/%d/revert-proxy-fallback", id), a.tokens.Access, "", "", struct{}{})
	return send(w, 200, struct{}{}, err)
}
