package observe

import (
	"context"
	"net/http"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/patrolpolicy"
	"sub2api-panel/ent/patrolrun"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type patrolBarItem struct {
	ID         int64    `json:"id"`
	Verdict    string   `json:"verdict"` // ok | degraded | failed | skipped_idle
	DurationMs int64    `json:"duration_ms"`
	Tokens     int64    `json:"tokens"`
	CreatedAt  string   `json:"created_at"`
	Summary    string   `json:"summary,omitempty"`
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

func buildPatrolBoard(s *core.Server, ctx context.Context, publicOnly bool) (patrolBoardOverview, error) {
	enabled, _, _, guestOn, _ := s.PatrolVisibility()
	rt, _ := s.EnsurePlugins()
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
	if s.Database == nil {
		return board, nil
	}
	q := s.Database.PatrolPolicy.Query()
	if publicOnly {
		q = q.Where(patrolpolicy.EnabledEQ(true), patrolpolicy.PublicVisibleEQ(true))
	}
	policies, err := q.Order(ent.Asc(patrolpolicy.FieldGroupID)).All(ctx)
	if err != nil {
		return board, err
	}

	loc := s.UsageZone()
	nowLocal := now.In(loc)
	dayStart := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
	if skippedToday, cErr := s.Database.PatrolRun.Query().
		Where(patrolrun.CreatedAtGTE(dayStart), patrolrun.VerdictEQ("skipped_idle")).
		Count(ctx); cErr == nil {
		board.IdleSkippedToday = skippedToday
		board.EstimatedSavedToken = int64(skippedToday) * 4200
	}

	var durSum int64
	var durCount int64
	var minNext int64
	for _, pol := range policies {
		runs, rErr := s.Database.PatrolRun.Query().
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

func publicPatrolBoard(s *core.Server) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		_, homeVisible, _, _, _ := s.PatrolVisibility()
		if !homeVisible {
			return platform.Fail(403, "not_allowed")
		}
		board, err := buildPatrolBoard(s, r.Context(), true)
		return platform.Send(w, 200, board, err)
	}
}

func userPatrolBoard(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		_, _, userVisible, _, _ := s.PatrolVisibility()
		if a.User.Role != "admin" && !userVisible {
			return platform.Fail(403, "not_allowed")
		}
		board, err := buildPatrolBoard(s, r.Context(), a.User.Role != "admin")
		return platform.Send(w, 200, board, err)
	}
}

func publicPatrolRunDetail(s *core.Server) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		_, homeVisible, _, _, _ := s.PatrolVisibility()
		if !homeVisible || s.Database == nil {
			return platform.Fail(403, "not_allowed")
		}
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		run, err := s.Database.PatrolRun.Get(r.Context(), int(id))
		if err != nil {
			return platform.Fail(404, "not_found")
		}
		pol, err := s.Database.PatrolPolicy.Query().Where(patrolpolicy.GroupIDEQ(run.GroupID)).Only(r.Context())
		if err != nil || !pol.PublicVisible || !pol.Enabled {
			return platform.Fail(404, "not_found")
		}
		platform.WriteJSON(w, 200, projectPatrolRunDetail(run))
		return nil
	}
}

func userPatrolRunDetail(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		_, _, userVisible, _, _ := s.PatrolVisibility()
		if a.User.Role != "admin" && !userVisible {
			return platform.Fail(403, "not_allowed")
		}
		if s.Database == nil {
			return platform.Fail(404, "not_found")
		}
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		run, err := s.Database.PatrolRun.Get(r.Context(), int(id))
		if err != nil {
			return platform.Fail(404, "not_found")
		}
		if a.User.Role != "admin" {
			pol, pErr := s.Database.PatrolPolicy.Query().Where(patrolpolicy.GroupIDEQ(run.GroupID)).Only(r.Context())
			if pErr != nil || !pol.PublicVisible {
				return platform.Fail(404, "not_found")
			}
		}
		platform.WriteJSON(w, 200, projectPatrolRunDetail(run))
		return nil
	}
}
