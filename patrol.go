package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/patrolpolicy"
	"sub2api-panel/ent/patrolrun"
	"sub2api-panel/ent/probetemplate"
)

type patrolBarItem struct {
	ID         int64   `json:"id"`
	Verdict    string  `json:"verdict"` // ok | degraded | failed | skipped_idle
	DurationMs int64   `json:"duration_ms"`
	Tokens     int64   `json:"tokens"`
	CreatedAt  string  `json:"created_at"`
	Summary    string  `json:"summary,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`
}

type patrolGroupBoardRow struct {
	GroupID        int64           `json:"group_id"`
	GroupName      string          `json:"group_name"`
	Platform       string          `json:"platform"`
	RateMultiplier float64         `json:"rate_multiplier"`
	Engine         string          `json:"engine"`
	Model          string          `json:"model"`
	Effort         string          `json:"effort,omitempty"`
	Enabled        bool            `json:"enabled"`
	PublicVisible  bool            `json:"public_visible"`
	LatestVerdict  string          `json:"latest_verdict"`
	HealthyRate    *float64        `json:"healthy_rate"`
	ValidCount     int             `json:"valid_count"`
	HealthyCount   int             `json:"healthy_count"`
	LatestMs       *int64          `json:"latest_duration_ms"`
	LatestTokens   *int64          `json:"latest_tokens"`
	LatestAt       string          `json:"latest_at,omitempty"`
	LatestRunID    *int64          `json:"latest_run_id,omitempty"`
	NextCheckAt    int64           `json:"next_check_at"`
	MinActiveUsers int             `json:"min_active_users"`
	WindowMinutes  int             `json:"window_minutes"`
	IntervalMin    int             `json:"interval_minutes"`
	Bars           []patrolBarItem `json:"bars"`
}

type patrolBoardOverview struct {
	Enabled             bool                  `json:"enabled"`
	GuestCheckEnabled   bool                  `json:"guest_check_enabled"`
	GuestDefaultModel   string                `json:"guest_default_model,omitempty"`
	SyncedAt            string                `json:"synced_at"`
	NextCountdownSec    int64                 `json:"next_countdown_sec"`
	GroupCount          int                   `json:"group_count"`
	HealthyGroups       int                   `json:"healthy_groups"`
	DegradedGroups      int                   `json:"degraded_groups"`
	IdleGroups          int                   `json:"idle_groups"`
	OverallHealthyRate  *float64              `json:"overall_healthy_rate"`
	TotalValidRuns      int                   `json:"total_valid_runs"`
	TotalHealthyRuns    int                   `json:"total_healthy_runs"`
	TotalFailedRuns     int                   `json:"total_failed_runs"`
	AvgDurationMs       *int64                `json:"avg_duration_ms"`
	RecentTokens        int64                 `json:"recent_tokens"`
	IdleSkippedToday    int                   `json:"idle_skipped_today"`
	EstimatedSavedToken int64                 `json:"estimated_saved_tokens"`
	Groups              []patrolGroupBoardRow `json:"groups"`
}

type patrolRunDetailView struct {
	ID              int64    `json:"id"`
	GroupName       string   `json:"group_name"`
	Trigger         string   `json:"trigger"`
	Verdict         string   `json:"verdict"`
	Engine          string   `json:"engine"`
	Model           string   `json:"model"`
	Effort          string   `json:"effort,omitempty"`
	ActiveUsers     int      `json:"active_users"`
	WindowRequests  int64    `json:"window_requests"`
	DurationMs      int64    `json:"duration_ms"`
	TTFTMs          int64    `json:"ttft_ms"`
	InputTokens     int64    `json:"input_tokens"`
	OutputTokens    int64    `json:"output_tokens"`
	ReasoningTokens int64    `json:"reasoning_tokens"`
	CachedTokens    int64    `json:"cached_tokens"`
	EstimatedMicros int64    `json:"estimated_micros"`
	Confidence      *float64 `json:"confidence,omitempty"`
	PredictedModel  string   `json:"predicted_model,omitempty"`
	Summary         string   `json:"summary"`
	ReplyExcerpt    string   `json:"reply_excerpt,omitempty"`
	CreatedAt       string   `json:"created_at"`
}

type patrolPolicyView struct {
	GroupID             int64   `json:"group_id"`
	GroupName           string  `json:"group_name"`
	Platform            string  `json:"platform"`
	RateMultiplier      float64 `json:"rate_multiplier"`
	Enabled             bool    `json:"enabled"`
	PublicVisible       bool    `json:"public_visible"`
	Engine              string  `json:"engine"`
	TemplateID          *int64  `json:"template_id,omitempty"`
	Model               string  `json:"model"`
	Protocol            string  `json:"protocol"`
	Effort              string  `json:"effort"`
	InspectorPreset     string  `json:"inspector_preset"`
	HasPatrolKey        bool    `json:"has_patrol_key"`
	WindowMinutes       int     `json:"window_minutes"`
	MinActiveUsers      int     `json:"min_active_users"`
	MinWindowRequests   int     `json:"min_window_requests"`
	PeakActiveUsers     int     `json:"peak_active_users"`
	IntervalMinutes     int     `json:"interval_minutes"`
	PeakIntervalMinutes int     `json:"peak_interval_minutes"`
	RecheckMinutes      int     `json:"recheck_minutes"`
	DailyTokenLimit     int64   `json:"daily_token_limit"`
	DailyBudgetCents    int64   `json:"daily_budget_cents"`
	NextCheckAt         int64   `json:"next_check_at"`
}

type probeTemplateView struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Protocol           string `json:"protocol"`
	SystemPrompt       string `json:"system_prompt"`
	UserPrompt         string `json:"user_prompt"`
	Effort             string `json:"effort"`
	Stream             bool   `json:"stream"`
	MaxTokens          int    `json:"max_tokens"`
	MatchKind          string `json:"match_kind"`
	Expected           string `json:"expected"`
	MinReasoningTokens int64  `json:"min_reasoning_tokens"`
	MaxTTFTMs          int64  `json:"max_ttft_ms"`
}

func (p *panel) patrolRoutes(mux *http.ServeMux) {
	p.public(mux, "GET /api/patrol/public", p.publicPatrolBoard)
	p.public(mux, "GET /api/patrol/public/runs/{id}", p.publicPatrolRunDetail)
	p.route(mux, "GET /api/patrol", false, p.userPatrolBoard)
	p.route(mux, "GET /api/patrol/runs/{id}", false, p.userPatrolRunDetail)

	p.route(mux, "GET /api/admin/patrol/policies", true, p.adminListPatrolPolicies)
	p.route(mux, "PUT /api/admin/patrol/policies/{group_id}", true, p.adminUpsertPatrolPolicy)
	p.route(mux, "DELETE /api/admin/patrol/policies/{group_id}", true, p.adminDeletePatrolPolicy)
	p.route(mux, "POST /api/admin/patrol/policies/{group_id}/mint-key", true, p.adminMintPatrolPolicyKey)
	p.route(mux, "POST /api/admin/patrol/policies/{group_id}/run", true, p.adminTriggerPatrolNow)

	p.route(mux, "GET /api/admin/patrol/templates", true, p.adminListProbeTemplates)
	p.route(mux, "POST /api/admin/patrol/templates", true, p.adminCreateProbeTemplate)
	p.route(mux, "PUT /api/admin/patrol/templates/{id}", true, p.adminUpdateProbeTemplate)
	p.route(mux, "DELETE /api/admin/patrol/templates/{id}", true, p.adminDeleteProbeTemplate)
}

func projectPatrolPolicy(row *ent.PatrolPolicy) patrolPolicyView {
	return patrolPolicyView{
		GroupID:             row.GroupID,
		GroupName:           row.GroupName,
		Platform:            row.Platform,
		RateMultiplier:      row.RateMultiplier,
		Enabled:             row.Enabled,
		PublicVisible:       row.PublicVisible,
		Engine:              row.Engine,
		TemplateID:          row.TemplateID,
		Model:               row.Model,
		Protocol:            row.Protocol,
		Effort:              row.Effort,
		InspectorPreset:     row.InspectorPreset,
		HasPatrolKey:        row.PatrolAPIKey != "",
		WindowMinutes:       row.WindowMinutes,
		MinActiveUsers:      row.MinActiveUsers,
		MinWindowRequests:   row.MinWindowRequests,
		PeakActiveUsers:     row.PeakActiveUsers,
		IntervalMinutes:     row.IntervalMinutes,
		PeakIntervalMinutes: row.PeakIntervalMinutes,
		RecheckMinutes:      row.RecheckMinutes,
		DailyTokenLimit:     row.DailyTokenLimit,
		DailyBudgetCents:    row.DailyBudgetCents,
		NextCheckAt:         row.NextCheckAt,
	}
}

func projectProbeTemplate(row *ent.ProbeTemplate) probeTemplateView {
	return probeTemplateView{
		ID:                 int64(row.ID),
		Name:               row.Name,
		Protocol:           row.Protocol,
		SystemPrompt:       row.SystemPrompt,
		UserPrompt:         row.UserPrompt,
		Effort:             row.Effort,
		Stream:             row.Stream,
		MaxTokens:          row.MaxTokens,
		MatchKind:          row.MatchKind,
		Expected:           row.Expected,
		MinReasoningTokens: row.MinReasoningTokens,
		MaxTTFTMs:          row.MaxTtftMs,
	}
}

func projectPatrolRunDetail(row *ent.PatrolRun) patrolRunDetailView {
	return patrolRunDetailView{
		ID:              int64(row.ID),
		GroupName:       row.GroupName,
		Trigger:         row.Trigger,
		Verdict:         row.Verdict,
		Engine:          row.Engine,
		Model:           row.Model,
		Effort:          row.Effort,
		ActiveUsers:     row.ActiveUsers,
		WindowRequests:  row.WindowRequests,
		DurationMs:      row.DurationMs,
		TTFTMs:          row.TtftMs,
		InputTokens:     row.InputTokens,
		OutputTokens:    row.OutputTokens,
		ReasoningTokens: row.ReasoningTokens,
		CachedTokens:    row.CachedTokens,
		EstimatedMicros: row.EstimatedMicros,
		Confidence:      row.Confidence,
		PredictedModel:  row.PredictedModel,
		Summary:         row.Summary,
		ReplyExcerpt:    row.ReplyExcerpt,
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (p *panel) buildPatrolBoard(ctx context.Context, publicOnly bool) (patrolBoardOverview, error) {
	enabled, _, _, guestOn, _ := p.patrolVisibility()
	rt, _ := p.ensurePlugins()
	_, gCfg := rt.ConfigFor("guest-self-check")
	guestModel, _ := gCfg["default_model"].(string)

	now := time.Now().UTC()
	board := patrolBoardOverview{
		Enabled:           enabled,
		GuestCheckEnabled: guestOn,
		GuestDefaultModel: guestModel,
		SyncedAt:          now.Format(time.RFC3339),
		Groups:            []patrolGroupBoardRow{},
	}
	if p.database == nil {
		return board, nil
	}
	q := p.database.PatrolPolicy.Query()
	if publicOnly {
		q = q.Where(patrolpolicy.EnabledEQ(true), patrolpolicy.PublicVisibleEQ(true))
	}
	policies, err := q.Order(ent.Asc(patrolpolicy.FieldGroupID)).All(ctx)
	if err != nil {
		return board, err
	}

	loc := p.usageZone()
	nowLocal := now.In(loc)
	dayStart := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
	if skippedToday, cErr := p.database.PatrolRun.Query().
		Where(patrolrun.CreatedAtGTE(dayStart), patrolrun.VerdictEQ("skipped_idle")).
		Count(ctx); cErr == nil {
		board.IdleSkippedToday = skippedToday
		board.EstimatedSavedToken = int64(skippedToday) * 4200
	}

	var durSum int64
	var durCount int64
	var minNext int64
	for _, pol := range policies {
		runs, rErr := p.database.PatrolRun.Query().
			Where(patrolrun.GroupIDEQ(pol.GroupID)).
			Order(ent.Desc(patrolrun.FieldCreatedAt)).
			Limit(60).
			All(ctx)
		if rErr != nil {
			continue
		}
		bars := make([]patrolBarItem, 0, len(runs))
		validCnt, okCnt, failCnt := 0, 0, 0
		latestVerdict := "unknown"
		var latestMs, latestTok *int64
		var latestRunID *int64
		latestAt := ""
		for i := len(runs) - 1; i >= 0; i-- {
			r := runs[i]
			tokTotal := r.InputTokens + r.OutputTokens
			bars = append(bars, patrolBarItem{
				ID:         int64(r.ID),
				Verdict:    r.Verdict,
				DurationMs: r.DurationMs,
				Tokens:     tokTotal,
				CreatedAt:  r.CreatedAt.UTC().Format(time.RFC3339),
				Summary:    r.Summary,
				Confidence: r.Confidence,
			})
			board.RecentTokens += tokTotal
			switch r.Verdict {
			case "ok":
				validCnt++
				okCnt++
			case "degraded":
				validCnt++
			case "failed":
				failCnt++
			}
		}
		if len(runs) > 0 {
			head := runs[0]
			latestVerdict = head.Verdict
			latestAt = head.CreatedAt.UTC().Format(time.RFC3339)
			idCopy := int64(head.ID)
			latestRunID = &idCopy
			for _, r := range runs {
				if r.Verdict == "ok" || r.Verdict == "degraded" {
					latestVerdict = r.Verdict
					ms := r.DurationMs
					tok := r.InputTokens + r.OutputTokens
					latestMs = &ms
					latestTok = &tok
					durSum += ms
					durCount++
					break
				}
			}
		}
		var rate *float64
		if validCnt > 0 {
			val := (float64(okCnt) / float64(validCnt)) * 100
			rate = &val
		}
		board.TotalValidRuns += validCnt
		board.TotalHealthyRuns += okCnt
		board.TotalFailedRuns += failCnt
		switch latestVerdict {
		case "ok":
			board.HealthyGroups++
		case "degraded":
			board.DegradedGroups++
		default:
			board.IdleGroups++
		}
		if pol.Enabled && pol.NextCheckAt > now.Unix() && (minNext == 0 || pol.NextCheckAt < minNext) {
			minNext = pol.NextCheckAt
		}
		board.Groups = append(board.Groups, patrolGroupBoardRow{
			GroupID:        pol.GroupID,
			GroupName:      pol.GroupName,
			Platform:       pol.Platform,
			RateMultiplier: pol.RateMultiplier,
			Engine:         pol.Engine,
			Model:          pol.Model,
			Effort:         pol.Effort,
			Enabled:        pol.Enabled,
			PublicVisible:  pol.PublicVisible,
			LatestVerdict:  latestVerdict,
			HealthyRate:    rate,
			ValidCount:     validCnt,
			HealthyCount:   okCnt,
			LatestMs:       latestMs,
			LatestTokens:   latestTok,
			LatestAt:       latestAt,
			LatestRunID:    latestRunID,
			NextCheckAt:    pol.NextCheckAt,
			MinActiveUsers: pol.MinActiveUsers,
			WindowMinutes:  pol.WindowMinutes,
			IntervalMin:    pol.IntervalMinutes,
			Bars:           bars,
		})
	}
	board.GroupCount = len(board.Groups)
	if board.TotalValidRuns > 0 {
		v := (float64(board.TotalHealthyRuns) / float64(board.TotalValidRuns)) * 100
		board.OverallHealthyRate = &v
	}
	if durCount > 0 {
		avg := durSum / durCount
		board.AvgDurationMs = &avg
	}
	if minNext > now.Unix() {
		board.NextCountdownSec = minNext - now.Unix()
	}
	return board, nil
}

func (p *panel) publicPatrolBoard(w http.ResponseWriter, r *http.Request) error {
	_, homeVisible, _, _, _ := p.patrolVisibility()
	if !homeVisible {
		return fail(403, "not_allowed")
	}
	board, err := p.buildPatrolBoard(r.Context(), true)
	return send(w, 200, board, err)
}

func (p *panel) userPatrolBoard(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	_, _, userVisible, _, _ := p.patrolVisibility()
	if a.user.Role != "admin" && !userVisible {
		return fail(403, "not_allowed")
	}
	board, err := p.buildPatrolBoard(r.Context(), a.user.Role != "admin")
	return send(w, 200, board, err)
}

func (p *panel) publicPatrolRunDetail(w http.ResponseWriter, r *http.Request) error {
	_, homeVisible, _, _, _ := p.patrolVisibility()
	if !homeVisible || p.database == nil {
		return fail(403, "not_allowed")
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	run, err := p.database.PatrolRun.Get(r.Context(), int(id))
	if err != nil {
		return fail(404, "not_found")
	}
	pol, err := p.database.PatrolPolicy.Query().Where(patrolpolicy.GroupIDEQ(run.GroupID)).Only(r.Context())
	if err != nil || !pol.PublicVisible || !pol.Enabled {
		return fail(404, "not_found")
	}
	writeJSON(w, 200, projectPatrolRunDetail(run))
	return nil
}

func (p *panel) userPatrolRunDetail(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	_, _, userVisible, _, _ := p.patrolVisibility()
	if a.user.Role != "admin" && !userVisible {
		return fail(403, "not_allowed")
	}
	if p.database == nil {
		return fail(404, "not_found")
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	run, err := p.database.PatrolRun.Get(r.Context(), int(id))
	if err != nil {
		return fail(404, "not_found")
	}
	if a.user.Role != "admin" {
		pol, pErr := p.database.PatrolPolicy.Query().Where(patrolpolicy.GroupIDEQ(run.GroupID)).Only(r.Context())
		if pErr != nil || !pol.PublicVisible {
			return fail(404, "not_found")
		}
	}
	writeJSON(w, 200, projectPatrolRunDetail(run))
	return nil
}

func (p *panel) adminListPatrolPolicies(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	if p.database == nil {
		writeJSON(w, 200, []patrolPolicyView{})
		return nil
	}
	rows, err := p.database.PatrolPolicy.Query().Order(ent.Asc(patrolpolicy.FieldGroupID)).All(r.Context())
	if err != nil {
		return err
	}
	out := make([]patrolPolicyView, 0, len(rows))
	for _, row := range rows {
		out = append(out, projectPatrolPolicy(row))
	}
	writeJSON(w, 200, out)
	return nil
}

func (p *panel) adminUpsertPatrolPolicy(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	if p.database == nil {
		return fail(503, "service_unavailable")
	}
	groupID, err := pathID(r, "group_id")
	if err != nil {
		return err
	}
	input, err := decodeBody[struct {
		GroupName           string  `json:"group_name"`
		Platform            string  `json:"platform"`
		RateMultiplier      float64 `json:"rate_multiplier"`
		Enabled             bool    `json:"enabled"`
		PublicVisible       bool    `json:"public_visible"`
		Engine              string  `json:"engine"`
		TemplateID          *int64  `json:"template_id,omitempty"`
		Model               string  `json:"model"`
		Protocol            string  `json:"protocol"`
		Effort              string  `json:"effort,omitempty"`
		InspectorPreset     string  `json:"inspector_preset,omitempty"`
		PatrolAPIKey        *string `json:"patrol_api_key,omitempty"`
		WindowMinutes       int     `json:"window_minutes"`
		MinActiveUsers      int     `json:"min_active_users"`
		MinWindowRequests   int     `json:"min_window_requests"`
		PeakActiveUsers     int     `json:"peak_active_users"`
		IntervalMinutes     int     `json:"interval_minutes"`
		PeakIntervalMinutes int     `json:"peak_interval_minutes"`
		RecheckMinutes      int     `json:"recheck_minutes"`
		DailyTokenLimit     int64   `json:"daily_token_limit"`
		DailyBudgetCents    int64   `json:"daily_budget_cents"`
	}](r)
	if err != nil || input.WindowMinutes < 1 || input.WindowMinutes > 1440 ||
		input.IntervalMinutes < 1 || input.IntervalMinutes > 1440 ||
		input.PeakIntervalMinutes < 1 || input.RecheckMinutes < 1 ||
		input.MinActiveUsers < 0 || input.MinWindowRequests < 0 ||
		input.DailyTokenLimit < 0 || input.DailyBudgetCents < 0 {
		return fail(400, "invalid_input")
	}
	if input.Engine != "lmfpd" && input.Engine != "gpt-relay-inspector" && input.Engine != "template" && input.Engine != "custom" {
		return fail(400, "invalid_input")
	}
	now := time.Now().Unix()
	nextAt := now + int64(input.IntervalMinutes)*60
	existing, qErr := p.database.PatrolPolicy.Query().Where(patrolpolicy.GroupIDEQ(groupID)).Only(r.Context())
	if ent.IsNotFound(qErr) {
		create := p.database.PatrolPolicy.Create().
			SetGroupID(groupID).
			SetGroupName(input.GroupName).
			SetPlatform(input.Platform).
			SetRateMultiplier(max(0, input.RateMultiplier)).
			SetEnabled(input.Enabled).
			SetPublicVisible(input.PublicVisible).
			SetEngine(input.Engine).
			SetNillableTemplateID(input.TemplateID).
			SetModel(input.Model).
			SetProtocol(firstNonEmpty(input.Protocol, "responses")).
			SetEffort(input.Effort).
			SetInspectorPreset(firstNonEmpty(input.InspectorPreset, "fast")).
			SetWindowMinutes(input.WindowMinutes).
			SetMinActiveUsers(input.MinActiveUsers).
			SetMinWindowRequests(input.MinWindowRequests).
			SetPeakActiveUsers(max(input.MinActiveUsers, input.PeakActiveUsers)).
			SetIntervalMinutes(input.IntervalMinutes).
			SetPeakIntervalMinutes(input.PeakIntervalMinutes).
			SetRecheckMinutes(input.RecheckMinutes).
			SetDailyTokenLimit(input.DailyTokenLimit).
			SetDailyBudgetCents(input.DailyBudgetCents).
			SetNextCheckAt(nextAt).
			SetUpdatedAt(now)
		if input.PatrolAPIKey != nil {
			create.SetPatrolAPIKey(*input.PatrolAPIKey)
		}
		saved, sErr := create.Save(r.Context())
		if sErr != nil {
			return sErr
		}
		writeJSON(w, 200, projectPatrolPolicy(saved))
		return nil
	}
	if qErr != nil {
		return qErr
	}
	upd := existing.Update().
		SetGroupName(input.GroupName).
		SetPlatform(input.Platform).
		SetRateMultiplier(max(0, input.RateMultiplier)).
		SetEnabled(input.Enabled).
		SetPublicVisible(input.PublicVisible).
		SetEngine(input.Engine).
		ClearTemplateID().
		SetNillableTemplateID(input.TemplateID).
		SetModel(input.Model).
		SetProtocol(firstNonEmpty(input.Protocol, "responses")).
		SetEffort(input.Effort).
		SetInspectorPreset(firstNonEmpty(input.InspectorPreset, "fast")).
		SetWindowMinutes(input.WindowMinutes).
		SetMinActiveUsers(input.MinActiveUsers).
		SetMinWindowRequests(input.MinWindowRequests).
		SetPeakActiveUsers(max(input.MinActiveUsers, input.PeakActiveUsers)).
		SetIntervalMinutes(input.IntervalMinutes).
		SetPeakIntervalMinutes(input.PeakIntervalMinutes).
		SetRecheckMinutes(input.RecheckMinutes).
		SetDailyTokenLimit(input.DailyTokenLimit).
		SetDailyBudgetCents(input.DailyBudgetCents).
		SetUpdatedAt(now)
	if input.PatrolAPIKey != nil {
		upd.SetPatrolAPIKey(*input.PatrolAPIKey)
	}
	saved, sErr := upd.Save(r.Context())
	if sErr != nil {
		return sErr
	}
	writeJSON(w, 200, projectPatrolPolicy(saved))
	return nil
}

func (p *panel) adminDeletePatrolPolicy(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	if p.database == nil {
		return fail(503, "service_unavailable")
	}
	groupID, err := pathID(r, "group_id")
	if err != nil {
		return err
	}
	_, err = p.database.PatrolPolicy.Delete().Where(patrolpolicy.GroupIDEQ(groupID)).Exec(r.Context())
	return send(w, 200, struct{}{}, err)
}

func (p *panel) adminMintPatrolPolicyKey(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	if p.database == nil {
		return fail(503, "service_unavailable")
	}
	groupID, err := pathID(r, "group_id")
	if err != nil {
		return err
	}
	pol, err := p.database.PatrolPolicy.Query().Where(patrolpolicy.GroupIDEQ(groupID)).Only(r.Context())
	if err != nil {
		return fail(404, "not_found")
	}
	dailyUSD := float64(pol.DailyBudgetCents) / 100.0
	if dailyUSD <= 0 {
		dailyUSD = 2.0
	}
	created, err := upstream[keyWire](r.Context(), p.upstream, "POST", "/keys", a.tokens.Access, "", "", map[string]any{
		"name":          fmt.Sprintf("panel-patrol-group-%d", groupID),
		"group_id":      groupID,
		"rate_limit_1d": dailyUSD,
	})
	if err != nil || created.Key == "" {
		if err == nil {
			err = fail(502, "invalid_upstream")
		}
		return err
	}
	saved, err := pol.Update().SetPatrolAPIKey(created.Key).SetUpdatedAt(time.Now().Unix()).Save(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, 200, projectPatrolPolicy(saved))
	return nil
}

func (p *panel) adminTriggerPatrolNow(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	if p.database == nil {
		return fail(503, "service_unavailable")
	}
	groupID, err := pathID(r, "group_id")
	if err != nil {
		return err
	}
	pol, err := p.database.PatrolPolicy.Query().Where(patrolpolicy.GroupIDEQ(groupID)).Only(r.Context())
	if err != nil {
		return fail(404, "not_found")
	}
	run, err := p.executePatrolCycle(r.Context(), pol, true, a.tokens.Access)
	if err != nil {
		return err
	}
	writeJSON(w, 200, projectPatrolRunDetail(run))
	return nil
}

func (p *panel) adminListProbeTemplates(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	if p.database == nil {
		writeJSON(w, 200, []probeTemplateView{})
		return nil
	}
	rows, err := p.database.ProbeTemplate.Query().Order(ent.Desc(probetemplate.FieldUpdatedAt)).All(r.Context())
	if err != nil {
		return err
	}
	out := make([]probeTemplateView, 0, len(rows))
	for _, row := range rows {
		out = append(out, projectProbeTemplate(row))
	}
	writeJSON(w, 200, out)
	return nil
}

func (p *panel) adminCreateProbeTemplate(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	if p.database == nil {
		return fail(503, "service_unavailable")
	}
	in, err := decodeBody[probeTemplateView](r)
	if err != nil || in.Name == "" || in.UserPrompt == "" || len(in.Name) > 100 || len(in.UserPrompt) > 12000 {
		return fail(400, "invalid_input")
	}
	maxTok := in.MaxTokens
	if maxTok < 32 || maxTok > 16384 {
		maxTok = 2048
	}
	saved, err := p.database.ProbeTemplate.Create().
		SetName(in.Name).
		SetProtocol(firstNonEmpty(in.Protocol, "chat_completions")).
		SetSystemPrompt(in.SystemPrompt).
		SetUserPrompt(in.UserPrompt).
		SetEffort(in.Effort).
		SetStream(in.Stream).
		SetMaxTokens(maxTok).
		SetMatchKind(firstNonEmpty(in.MatchKind, "contains")).
		SetExpected(in.Expected).
		SetMinReasoningTokens(max(0, in.MinReasoningTokens)).
		SetMaxTtftMs(max(0, in.MaxTTFTMs)).
		SetUpdatedAt(time.Now().Unix()).
		Save(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, 201, projectProbeTemplate(saved))
	return nil
}

func (p *panel) adminUpdateProbeTemplate(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	if p.database == nil {
		return fail(503, "service_unavailable")
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	in, err := decodeBody[probeTemplateView](r)
	if err != nil || in.Name == "" || in.UserPrompt == "" {
		return fail(400, "invalid_input")
	}
	maxTok := in.MaxTokens
	if maxTok < 32 || maxTok > 16384 {
		maxTok = 2048
	}
	saved, err := p.database.ProbeTemplate.UpdateOneID(int(id)).
		SetName(in.Name).
		SetProtocol(firstNonEmpty(in.Protocol, "chat_completions")).
		SetSystemPrompt(in.SystemPrompt).
		SetUserPrompt(in.UserPrompt).
		SetEffort(in.Effort).
		SetStream(in.Stream).
		SetMaxTokens(maxTok).
		SetMatchKind(firstNonEmpty(in.MatchKind, "contains")).
		SetExpected(in.Expected).
		SetMinReasoningTokens(max(0, in.MinReasoningTokens)).
		SetMaxTtftMs(max(0, in.MaxTTFTMs)).
		SetUpdatedAt(time.Now().Unix()).
		Save(r.Context())
	if err != nil {
		return fail(404, "not_found")
	}
	writeJSON(w, 200, projectProbeTemplate(saved))
	return nil
}

func (p *panel) adminDeleteProbeTemplate(w http.ResponseWriter, r *http.Request, _ sessionAuth) error {
	if p.database == nil {
		return fail(503, "service_unavailable")
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	err = p.database.ProbeTemplate.DeleteOneID(int(id)).Exec(r.Context())
	if err != nil {
		return fail(404, "not_found")
	}
	writeJSON(w, 200, struct{}{})
	return nil
}

func (p *panel) startPatrolScheduler(ctx context.Context) {
	if p.database == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.runDuePatrols(ctx)
			}
		}
	}()
}

func (p *panel) runDuePatrols(ctx context.Context) {
	enabled, _, _, _, retentionDays := p.patrolVisibility()
	if !enabled || p.database == nil {
		return
	}
	ctx, end := backgroundContext(ctx, "执行智能巡检")
	defer end()
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	_, _ = p.database.PatrolRun.Delete().Where(patrolrun.CreatedAtLT(cutoff)).Exec(ctx)

	now := time.Now().Unix()
	due, err := p.database.PatrolPolicy.Query().
		Where(patrolpolicy.EnabledEQ(true), patrolpolicy.NextCheckAtLTE(now)).
		Limit(6).
		All(ctx)
	if err != nil {
		return
	}
	for _, pol := range due {
		if _, err := p.executePatrolCycle(ctx, pol, false, ""); err != nil {
			slog.Info("patrol cycle skipped", "trace_id", meta(ctx).Trace, "group_id", pol.GroupID, "error", err)
		}
	}
}

func (p *panel) executePatrolCycle(ctx context.Context, pol *ent.PatrolPolicy, manual bool, adminToken string) (*ent.PatrolRun, error) {
	now := time.Now()
	activeUsers, winReqs := p.observeGroupTraffic(ctx, pol.GroupID, pol.WindowMinutes)

	if !manual {
		// 1. 单日 Token 与美分预算熔断检查
		loc := p.usageZone()
		nowLocal := now.In(loc)
		dayStart := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
		todayRuns, _ := p.database.PatrolRun.Query().
			Where(patrolrun.GroupIDEQ(pol.GroupID), patrolrun.CreatedAtGTE(dayStart)).
			All(ctx)
		var usedTokens, usedMicros int64
		for _, r := range todayRuns {
			usedTokens += r.InputTokens + r.OutputTokens
			usedMicros += r.EstimatedMicros
		}
		if (pol.DailyTokenLimit > 0 && usedTokens >= pol.DailyTokenLimit) ||
			(pol.DailyBudgetCents > 0 && usedMicros >= pol.DailyBudgetCents*10000) {
			nextDay := dayStart.AddDate(0, 0, 1).Unix()
			_, _ = pol.Update().SetNextCheckAt(nextDay).Save(ctx)
			return p.database.PatrolRun.Create().
				SetGroupID(pol.GroupID).
				SetGroupName(pol.GroupName).
				SetTrigger("budget_fuse").
				SetVerdict("skipped_idle").
				SetEngine(pol.Engine).
				SetModel(pol.Model).
				SetActiveUsers(activeUsers).
				SetWindowRequests(winReqs).
				SetSummary("已达当日巡检预算上限，自动熔断免检").
				SetCreatedAt(now).
				Save(ctx)
		}

		// 2. 低峰活跃用户数门控（0 Token 消耗）
		if activeUsers < pol.MinActiveUsers && winReqs < int64(pol.MinWindowRequests) {
			nextAt := now.Unix() + int64(pol.IntervalMinutes)*60
			_, _ = pol.Update().SetNextCheckAt(nextAt).Save(ctx)
			return p.database.PatrolRun.Create().
				SetGroupID(pol.GroupID).
				SetGroupName(pol.GroupName).
				SetTrigger("idle_skip").
				SetVerdict("skipped_idle").
				SetEngine(pol.Engine).
				SetModel(pol.Model).
				SetActiveUsers(activeUsers).
				SetWindowRequests(winReqs).
				SetSummary(fmt.Sprintf("近 %d 分钟仅 %d 名活跃用户（阈值 %d），自动静默免检省 Token", pol.WindowMinutes, activeUsers, pol.MinActiveUsers)).
				SetCreatedAt(now).
				Save(ctx)
		}
	}

	apiKey := pol.PatrolAPIKey
	cleanup := func() {}
	if apiKey == "" && adminToken != "" {
		var kErr error
		apiKey, _, cleanup, kErr = p.mintGroupProbeKey(ctx, adminToken, pol.GroupID, 5*time.Minute)
		if kErr != nil {
			return nil, kErr
		}
	}
	defer cleanup()
	if apiKey == "" {
		nextAt := now.Unix() + int64(pol.IntervalMinutes)*60
		_, _ = pol.Update().SetNextCheckAt(nextAt).Save(ctx)
		return nil, fail(400, "operation_rejected")
	}

	req := probeRequest{
		Engine:   pol.Engine,
		Model:    firstNonEmpty(pol.Model, defaultModelForPlatform(pol.Platform)),
		Protocol: firstNonEmpty(pol.Protocol, defaultProtocolForPlatform(pol.Platform)),
		Effort:   pol.Effort,
		Preset:   pol.InspectorPreset,
	}
	if pol.Engine == "template" && pol.TemplateID != nil {
		if tpl, tErr := p.database.ProbeTemplate.Get(ctx, int(*pol.TemplateID)); tErr == nil {
			req.Engine = "custom"
			req.Protocol = tpl.Protocol
			req.SystemPrompt = tpl.SystemPrompt
			req.Prompt = tpl.UserPrompt
			req.Effort = firstNonEmpty(pol.Effort, tpl.Effort)
			req.MatchKind = tpl.MatchKind
			req.Expected = tpl.Expected
		}
	}

	report, err := p.executeGatewayProbe(ctx, apiKey, req, nil)
	if err != nil {
		report = probeReportView{
			Verdict: "failed",
			Engine:  pol.Engine,
			Model:   req.Model,
			Effort:  req.Effort,
			Summary: scrubText(err.Error(), 220),
		}
	}

	trigger := "scheduled"
	nextInterval := pol.IntervalMinutes
	if manual {
		trigger = "manual"
	} else if report.Verdict == "degraded" || report.Verdict == "failed" {
		trigger = "recheck"
		nextInterval = pol.RecheckMinutes
	} else if activeUsers >= pol.PeakActiveUsers {
		trigger = "peak"
		nextInterval = pol.PeakIntervalMinutes
	}
	_, _ = pol.Update().SetNextCheckAt(now.Unix() + int64(nextInterval)*60).Save(ctx)

	return p.database.PatrolRun.Create().
		SetGroupID(pol.GroupID).
		SetGroupName(pol.GroupName).
		SetTrigger(trigger).
		SetVerdict(report.Verdict).
		SetEngine(pol.Engine).
		SetModel(report.Model).
		SetEffort(report.Effort).
		SetActiveUsers(activeUsers).
		SetWindowRequests(winReqs).
		SetDurationMs(report.DurationMs).
		SetTtftMs(report.TTFTMs).
		SetInputTokens(report.InputTokens).
		SetOutputTokens(report.OutputTokens).
		SetReasoningTokens(report.ReasoningTokens).
		SetCachedTokens(report.CachedTokens).
		SetEstimatedMicros(report.EstimatedMicros).
		SetNillableConfidence(report.Confidence).
		SetPredictedModel(report.PredictedModel).
		SetSummary(report.Summary).
		SetReplyExcerpt(scrubText(report.Reply, 1500)).
		SetCreatedAt(now).
		Save(ctx)
}
