package observe

import (
	"fmt"
	"net/http"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/patrolpolicy"
	"sub2api-panel/ent/probetemplate"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func adminListPatrolPolicies(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
		if s.Database == nil {
			platform.WriteJSON(w, 200, []patrolPolicyView{})
			return nil
		}
		rows, err := s.Database.PatrolPolicy.Query().Order(ent.Asc(patrolpolicy.FieldGroupID)).All(r.Context())
		if err != nil {
			return err
		}
		out := make([]patrolPolicyView, 0, len(rows))
		for _, row := range rows {
			out = append(out, projectPatrolPolicy(row))
		}
		platform.WriteJSON(w, 200, out)
		return nil
	}
}

func adminUpsertPatrolPolicy(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
		if s.Database == nil {
			return platform.Fail(503, "service_unavailable")
		}
		groupID, err := platform.PathID(r, "group_id")
		if err != nil {
			return err
		}
		input, err := platform.DecodeBody[struct {
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
			return platform.Fail(400, "invalid_input")
		}
		if input.Engine != "lmfpd" && input.Engine != "gpt-relay-inspector" && input.Engine != "template" && input.Engine != "custom" {
			return platform.Fail(400, "invalid_input")
		}
		now := time.Now().Unix()
		nextAt := now + int64(input.IntervalMinutes)*60
		existing, qErr := s.Database.PatrolPolicy.Query().Where(patrolpolicy.GroupIDEQ(groupID)).Only(r.Context())
		if ent.IsNotFound(qErr) {
			create := s.Database.PatrolPolicy.Create().
				SetGroupID(groupID).
				SetGroupName(input.GroupName).
				SetPlatform(input.Platform).
				SetRateMultiplier(max(0, input.RateMultiplier)).
				SetEnabled(input.Enabled).
				SetPublicVisible(input.PublicVisible).
				SetEngine(input.Engine).
				SetNillableTemplateID(input.TemplateID).
				SetModel(input.Model).
				SetProtocol(platform.FirstNonEmpty(input.Protocol, "responses")).
				SetEffort(input.Effort).
				SetInspectorPreset(platform.FirstNonEmpty(input.InspectorPreset, "fast")).
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
			platform.WriteJSON(w, 200, projectPatrolPolicy(saved))
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
			SetProtocol(platform.FirstNonEmpty(input.Protocol, "responses")).
			SetEffort(input.Effort).
			SetInspectorPreset(platform.FirstNonEmpty(input.InspectorPreset, "fast")).
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
		platform.WriteJSON(w, 200, projectPatrolPolicy(saved))
		return nil
	}
}

func adminDeletePatrolPolicy(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
		if s.Database == nil {
			return platform.Fail(503, "service_unavailable")
		}
		groupID, err := platform.PathID(r, "group_id")
		if err != nil {
			return err
		}
		_, err = s.Database.PatrolPolicy.Delete().Where(patrolpolicy.GroupIDEQ(groupID)).Exec(r.Context())
		return platform.Send(w, 200, struct{}{}, err)
	}
}

func adminMintPatrolPolicyKey(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		if s.Database == nil {
			return platform.Fail(503, "service_unavailable")
		}
		groupID, err := platform.PathID(r, "group_id")
		if err != nil {
			return err
		}
		pol, err := s.Database.PatrolPolicy.Query().Where(patrolpolicy.GroupIDEQ(groupID)).Only(r.Context())
		if err != nil {
			return platform.Fail(404, "not_found")
		}
		dailyUSD := float64(pol.DailyBudgetCents) / 100.0
		if dailyUSD <= 0 {
			dailyUSD = 2.0
		}
		created, err := platform.Upstream[mintedKeyWire](r.Context(), s.Upstream, "POST", "/keys", a.Tokens.Access, "", "", map[string]any{
			"name":          fmt.Sprintf("panel-patrol-group-%d", groupID),
			"group_id":      groupID,
			"rate_limit_1d": dailyUSD,
		})
		if err != nil || created.Key == "" {
			if err == nil {
				err = platform.Fail(502, "invalid_upstream")
			}
			return err
		}
		saved, err := pol.Update().SetPatrolAPIKey(created.Key).SetUpdatedAt(time.Now().Unix()).Save(r.Context())
		if err != nil {
			return err
		}
		platform.WriteJSON(w, 200, projectPatrolPolicy(saved))
		return nil
	}
}

func adminTriggerPatrolNow(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		if s.Database == nil {
			return platform.Fail(503, "service_unavailable")
		}
		groupID, err := platform.PathID(r, "group_id")
		if err != nil {
			return err
		}
		pol, err := s.Database.PatrolPolicy.Query().Where(patrolpolicy.GroupIDEQ(groupID)).Only(r.Context())
		if err != nil {
			return platform.Fail(404, "not_found")
		}
		run, err := executePatrolCycle(r.Context(), s, pol, true, a.Tokens.Access)
		if err != nil {
			return err
		}
		platform.WriteJSON(w, 200, projectPatrolRunDetail(run))
		return nil
	}
}

func adminListProbeTemplates(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
		if s.Database == nil {
			platform.WriteJSON(w, 200, []probeTemplateView{})
			return nil
		}
		rows, err := s.Database.ProbeTemplate.Query().Order(ent.Desc(probetemplate.FieldUpdatedAt)).All(r.Context())
		if err != nil {
			return err
		}
		out := make([]probeTemplateView, 0, len(rows))
		for _, row := range rows {
			out = append(out, projectProbeTemplate(row))
		}
		platform.WriteJSON(w, 200, out)
		return nil
	}
}

func adminCreateProbeTemplate(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
		if s.Database == nil {
			return platform.Fail(503, "service_unavailable")
		}
		in, err := platform.DecodeBody[probeTemplateView](r)
		if err != nil || in.Name == "" || in.UserPrompt == "" || len(in.Name) > 100 || len(in.UserPrompt) > 12000 {
			return platform.Fail(400, "invalid_input")
		}
		maxTok := in.MaxTokens
		if maxTok < 32 || maxTok > 16384 {
			maxTok = 2048
		}
		saved, err := s.Database.ProbeTemplate.Create().
			SetName(in.Name).
			SetProtocol(platform.FirstNonEmpty(in.Protocol, "chat_completions")).
			SetSystemPrompt(in.SystemPrompt).
			SetUserPrompt(in.UserPrompt).
			SetEffort(in.Effort).
			SetStream(in.Stream).
			SetMaxTokens(maxTok).
			SetMatchKind(platform.FirstNonEmpty(in.MatchKind, "contains")).
			SetExpected(in.Expected).
			SetMinReasoningTokens(max(0, in.MinReasoningTokens)).
			SetMaxTtftMs(max(0, in.MaxTTFTMs)).
			SetUpdatedAt(time.Now().Unix()).
			Save(r.Context())
		if err != nil {
			return err
		}
		platform.WriteJSON(w, 201, projectProbeTemplate(saved))
		return nil
	}
}

func adminUpdateProbeTemplate(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
		if s.Database == nil {
			return platform.Fail(503, "service_unavailable")
		}
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		in, err := platform.DecodeBody[probeTemplateView](r)
		if err != nil || in.Name == "" || in.UserPrompt == "" {
			return platform.Fail(400, "invalid_input")
		}
		maxTok := in.MaxTokens
		if maxTok < 32 || maxTok > 16384 {
			maxTok = 2048
		}
		saved, err := s.Database.ProbeTemplate.UpdateOneID(int(id)).
			SetName(in.Name).
			SetProtocol(platform.FirstNonEmpty(in.Protocol, "chat_completions")).
			SetSystemPrompt(in.SystemPrompt).
			SetUserPrompt(in.UserPrompt).
			SetEffort(in.Effort).
			SetStream(in.Stream).
			SetMaxTokens(maxTok).
			SetMatchKind(platform.FirstNonEmpty(in.MatchKind, "contains")).
			SetExpected(in.Expected).
			SetMinReasoningTokens(max(0, in.MinReasoningTokens)).
			SetMaxTtftMs(max(0, in.MaxTTFTMs)).
			SetUpdatedAt(time.Now().Unix()).
			Save(r.Context())
		if err != nil {
			return platform.Fail(404, "not_found")
		}
		platform.WriteJSON(w, 200, projectProbeTemplate(saved))
		return nil
	}
}

func adminDeleteProbeTemplate(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
		if s.Database == nil {
			return platform.Fail(503, "service_unavailable")
		}
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		err = s.Database.ProbeTemplate.DeleteOneID(int(id)).Exec(r.Context())
		if err != nil {
			return platform.Fail(404, "not_found")
		}
		platform.WriteJSON(w, 200, struct{}{})
		return nil
	}
}
