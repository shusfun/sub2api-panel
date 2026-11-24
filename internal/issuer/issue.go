package issuer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"sub2api-panel/ent"
	grantent "sub2api-panel/ent/grant"
	issueent "sub2api-panel/ent/issue"
	"sub2api-panel/internal/platform"
)

func (i *Service) reserve(ctx context.Context, actor platform.Identity, input platform.IssueInput) error {
	tx, err := i.DB.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if exists, err := tx.Issue.Query().Where(issueent.OpEQ(input.Operation)).Exist(ctx); err != nil {
		return err
	} else if exists {
		return platform.Fail(409, "duplicate_operation")
	}
	epoch := int64(0)
	if actor.Role != "admin" {
		row, err := tx.Grant.Query().Where(grantent.UserIDEQ(actor.ID)).Only(ctx)
		if ent.IsNotFound(err) {
			return platform.Fail(403, "grant_exhausted")
		}
		if err != nil {
			return err
		}
		epoch = row.Epoch
		update := tx.Grant.UpdateOne(row)
		switch input.Kind {
		case "balance":
			if row.Budget < input.Amount || !slices.Contains(row.Amounts, input.Amount) {
				return platform.Fail(403, "grant_exhausted")
			}
			update.AddBudget(-input.Amount).Where(grantent.BudgetGTE(input.Amount))
		case "invitation":
			if row.Invites < 1 {
				return platform.Fail(403, "grant_exhausted")
			}
			update.AddInvites(-1).Where(grantent.InvitesGTE(1))
		}
		if _, err = update.Save(ctx); err != nil {
			if ent.IsNotFound(err) {
				return platform.Fail(403, "grant_exhausted")
			}
			return err
		}
	}
	if _, err = tx.Issue.Create().SetOp(input.Operation).SetActor(actor.ID).SetKind(input.Kind).SetAmount(input.Amount).SetGrantEpoch(epoch).SetState("pending").SetCreated(time.Now().Unix()).SetTrace(platform.Meta(ctx).Trace).Save(ctx); err != nil {
		return err
	}
	return tx.Commit()
}

func (i *Service) finishIssue(ctx context.Context, operation, state string, codeID *int64, reviewer int64) error {
	tx, err := i.DB.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	event, err := tx.Issue.Query().Where(issueent.OpEQ(operation)).Only(ctx)
	if ent.IsNotFound(err) {
		return platform.Fail(404, "not_found")
	}
	if err != nil {
		return err
	}
	if event.State != "pending" && event.State != "unknown" || reviewer != 0 && event.State == "pending" && time.Now().Unix()-event.Created < 60 {
		return platform.Fail(409, "operation_unavailable")
	}
	refunded := false
	if state == "failed" && event.GrantEpoch != 0 {
		update := tx.Grant.Update().Where(grantent.UserIDEQ(event.Actor), grantent.EpochEQ(event.GrantEpoch))
		if event.Kind == "invitation" {
			update.AddInvites(1)
		} else {
			update.AddBudget(event.Amount)
		}
		count, err := update.Save(ctx)
		if err != nil {
			return err
		}
		refunded = count == 1
	}
	mut := tx.Issue.UpdateOne(event).SetState(state).SetRefunded(refunded)
	if codeID == nil {
		mut.ClearCodeID()
	} else {
		mut.SetCodeID(*codeID)
	}
	if _, err = mut.Save(ctx); err != nil {
		return err
	}
	if reviewer != 0 {
		if err = auditGrant(ctx, tx, reviewer, event.Actor, 0, 0, "reconcile:"+operation); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func codeIDValue(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}

type issuedCode struct {
	ID    int64   `json:"id"`
	Code  string  `json:"code"`
	Kind  string  `json:"type"`
	Value float64 `json:"value"`
}

func (i *Service) issue(w http.ResponseWriter, r *http.Request, actor platform.Identity, _ string) error {
	input, err := platform.DecodeBody[platform.IssueInput](r)
	if err != nil {
		return err
	}
	if !platform.OperationPattern.MatchString(input.Operation) || !(input.Kind == "invitation" && input.Amount == 0 || input.Kind == "balance" && input.Amount > 0 && input.Amount <= maxCents) {
		return platform.Fail(400, "invalid_input")
	}
	days := 30
	if input.Kind == "invitation" {
		days = 7
		settings, err := platform.Upstream[struct {
			Enabled *bool `json:"invitation_code_enabled"`
		}](r.Context(), i.Upstream, "GET", "/settings/public", "", "", "", nil)
		if err != nil {
			return err
		}
		if settings.Enabled == nil {
			return platform.Fail(502, "invalid_upstream")
		}
		if !*settings.Enabled {
			return platform.Fail(403, "not_allowed")
		}
	}
	ctx := context.WithoutCancel(r.Context())
	if err := i.reserve(ctx, actor, input); err != nil {
		return err
	}
	slog.Info("issue quota reserved", "trace_id", platform.Meta(ctx).Trace, "actor_id", actor.ID, "operation_id", input.Operation)
	codes, err := platform.Upstream[[]issuedCode](ctx, i.Upstream, "POST", "/admin/redeem-codes/generate", "", i.AdminKey, input.Operation, struct {
		Count int     `json:"count"`
		Kind  string  `json:"type"`
		Value float64 `json:"value"`
		Days  int     `json:"expires_in_days"`
	}{1, input.Kind, float64(input.Amount) / 100, days})
	state := "unknown"
	var id *int64
	var refused platform.UpstreamStatus
	switch {
	case err == nil && len(codes) == 1 && codes[0].ID > 0 && codes[0].Code != "" && codes[0].Kind == input.Kind && codes[0].Value == float64(input.Amount)/100:
		state, id = "issued", &codes[0].ID
	case errors.As(err, &refused) && (refused.Status == 400 || refused.Status == 401 || refused.Status == 403 || refused.Status == 404 || refused.Status == 423 || refused.Status == 429):
		state = "failed"
	}
	if err := i.finishIssue(ctx, input.Operation, state, id, 0); err != nil {
		return platform.Fail(503, "issue_unknown")
	}
	slog.Info("issue completed", "trace_id", platform.Meta(ctx).Trace, "actor_id", actor.ID, "operation_id", input.Operation, "state", state, "code_id", codeIDValue(id))
	switch state {
	case "issued":
		platform.WriteJSON(w, 201, platform.IssueResult{Code: codes[0].Code})
		return nil
	case "failed":
		return platform.Fail(502, "issue_rejected")
	default:
		return platform.Fail(503, "issue_unknown")
	}
}

func (i *Service) history(w http.ResponseWriter, r *http.Request, actor platform.Identity, _ string) error {
	page, err := platform.RequestedPage(r)
	if err != nil {
		return err
	}
	query := i.DB.Issue.Query()
	if actor.Role != "admin" {
		query.Where(issueent.ActorEQ(actor.ID))
	}
	rows, err := query.Order(issueent.ByCreated(entsql.OrderDesc())).Limit(platform.PageSize + 1).Offset((page - 1) * platform.PageSize).All(r.Context())
	if err != nil {
		return err
	}
	items := make([]platform.IssueEvent, 0, len(rows))
	for _, row := range rows {
		delta := int64(0)
		if row.GrantEpoch != 0 && !row.Refunded {
			if row.Kind == "invitation" {
				delta = -1
			} else {
				delta = -row.Amount
			}
		}
		item := platform.IssueEvent{Operation: row.Op, Actor: row.Actor, Kind: row.Kind, Amount: row.Amount, State: row.State, CodeID: row.CodeID, Created: row.Created, RequestID: row.Trace, QuotaDelta: delta}
		if actor.Role != "admin" {
			item.Actor, item.RequestID = 0, ""
		}
		items = append(items, item)
	}
	platform.WriteJSON(w, 200, platform.WindowPage(items))
	return nil
}

func (i *Service) reconcile(w http.ResponseWriter, r *http.Request, actor platform.Identity, token string) error {
	operation := r.PathValue("operation")
	if !platform.OperationPattern.MatchString(operation) {
		return platform.Fail(400, "invalid_input")
	}
	input, err := platform.DecodeBody[platform.Reconciliation](r)
	if err != nil {
		return err
	}
	state := "failed"
	if input.CodeID != nil {
		if *input.CodeID <= 0 {
			return platform.Fail(400, "invalid_input")
		}
		row, err := i.DB.Issue.Query().Where(issueent.OpEQ(operation)).Only(r.Context())
		if ent.IsNotFound(err) {
			return platform.Fail(404, "not_found")
		}
		if err != nil {
			return err
		}
		code, err := platform.Upstream[issuedCode](r.Context(), i.Upstream, "GET", fmt.Sprintf("/admin/redeem-codes/%d", *input.CodeID), token, "", "", nil)
		if err != nil {
			return err
		}
		if code.ID != *input.CodeID || code.Kind != row.Kind || code.Value != float64(row.Amount)/100 {
			return platform.Fail(400, "code_mismatch")
		}
		state = "issued"
	}
	if err := i.finishIssue(r.Context(), operation, state, input.CodeID, actor.ID); err != nil {
		return err
	}
	platform.WriteJSON(w, 200, struct{}{})
	return nil
}
