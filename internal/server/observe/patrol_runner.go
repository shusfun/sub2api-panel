package observe

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/patrolpolicy"
	"sub2api-panel/ent/patrolrun"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func StartPatrolScheduler(ctx context.Context, s *core.Server) {
	if s.Database == nil {
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
				runDuePatrols(ctx, s)
			}
		}
	}()
}

func runDuePatrols(ctx context.Context, s *core.Server) {
	enabled, _, _, _, retentionDays := s.PatrolVisibility()
	if !enabled || s.Database == nil {
		return
	}
	ctx, end := platform.BackgroundFlow(ctx, "执行智能巡检")
	defer end()
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	_, _ = s.Database.PatrolRun.Delete().Where(patrolrun.CreatedAtLT(cutoff)).Exec(ctx)

	now := time.Now().Unix()
	due, err := s.Database.PatrolPolicy.Query().
		Where(patrolpolicy.EnabledEQ(true), patrolpolicy.NextCheckAtLTE(now)).
		Limit(6).
		All(ctx)
	if err != nil {
		return
	}
	for _, pol := range due {
		if _, err := executePatrolCycle(ctx, s, pol, false, ""); err != nil {
			slog.Info("patrol cycle skipped", "trace_id", platform.Meta(ctx).Trace, "group_id", pol.GroupID, "error", err)
		}
	}
}

func executePatrolCycle(ctx context.Context, s *core.Server, pol *ent.PatrolPolicy, manual bool, adminToken string) (*ent.PatrolRun, error) {
	now := time.Now()
	activeUsers, winReqs := ObserveGroupTraffic(ctx, s, pol.GroupID, pol.WindowMinutes)

	if !manual {
		// 1. 单日 Token 与美分预算熔断检查
		loc := s.UsageZone()
		nowLocal := now.In(loc)
		dayStart := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
		todayRuns, _ := s.Database.PatrolRun.Query().
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
			return s.Database.PatrolRun.Create().
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
			return s.Database.PatrolRun.Create().
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
		apiKey, _, cleanup, kErr = mintGroupProbeKey(ctx, s, adminToken, pol.GroupID, 5*time.Minute)
		if kErr != nil {
			return nil, kErr
		}
	}
	defer cleanup()
	if apiKey == "" {
		nextAt := now.Unix() + int64(pol.IntervalMinutes)*60
		_, _ = pol.Update().SetNextCheckAt(nextAt).Save(ctx)
		return nil, platform.Fail(400, "operation_rejected")
	}

	req := probeRequest{
		Engine:   pol.Engine,
		Model:    platform.FirstNonEmpty(pol.Model, defaultModelForPlatform(pol.Platform)),
		Protocol: platform.FirstNonEmpty(pol.Protocol, defaultProtocolForPlatform(pol.Platform)),
		Effort:   pol.Effort,
		Preset:   pol.InspectorPreset,
	}
	if pol.Engine == "template" && pol.TemplateID != nil {
		if tpl, tErr := s.Database.ProbeTemplate.Get(ctx, int(*pol.TemplateID)); tErr == nil {
			req.Engine = "custom"
			req.Protocol = tpl.Protocol
			req.SystemPrompt = tpl.SystemPrompt
			req.Prompt = tpl.UserPrompt
			req.Effort = platform.FirstNonEmpty(pol.Effort, tpl.Effort)
			req.MatchKind = tpl.MatchKind
			req.Expected = tpl.Expected
		}
	}

	report, err := executeGatewayProbe(ctx, s, apiKey, req, nil)
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

	return s.Database.PatrolRun.Create().
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
