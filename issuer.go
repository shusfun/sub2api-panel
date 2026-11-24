package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"sub2api-panel/ent"
	grantent "sub2api-panel/ent/grant"
	issueent "sub2api-panel/ent/issue"
)

const maxCents = 100_000_000

type grantWire struct {
	Invites int64   `json:"invites"`
	Budget  int64   `json:"budget_cents"`
	Amounts []int64 `json:"amounts_cents"`
}

// grant is kept as the panel-facing alias used by the existing HTTP contract.
type grant = grantWire

type grantChange struct {
	Invites int64   `json:"invites"`
	Budget  int64   `json:"budget_cents"`
	Amounts []int64 `json:"amounts_cents"`
	TOTP    string  `json:"totp"`
}

type issueInput struct {
	Operation string `json:"operation_id"`
	Kind      string `json:"kind"`
	Amount    int64  `json:"amount_cents"`
}

type issueResult struct {
	Code string `json:"code"`
}

type issueEvent struct {
	Operation  string `json:"operation_id"`
	Actor      int64  `json:"actor_id,omitempty"`
	Kind       string `json:"kind"`
	Amount     int64  `json:"amount_cents"`
	State      string `json:"state"`
	CodeID     *int64 `json:"code_id"`
	Created    int64  `json:"created_at"`
	QuotaDelta int64  `json:"quota_delta"`
	RequestID  string `json:"request_id,omitempty"`
}

type reconciliation struct {
	CodeID *int64 `json:"code_id"`
}

type issuer struct {
	db       *ent.Client
	upstream serviceClient
	secret   string
	adminKey string
}

func grantFromEntity(row *ent.Grant) grant {
	amounts := append([]int64(nil), row.Amounts...)
	if amounts == nil {
		amounts = []int64{}
	}
	return grant{Invites: row.Invites, Budget: row.Budget, Amounts: amounts}
}

func readGrant(ctx context.Context, query func() *ent.GrantQuery, user int64) (grant, error) {
	row, err := query().Where(grantent.UserIDEQ(user)).Only(ctx)
	if ent.IsNotFound(err) {
		return grant{Amounts: []int64{}}, nil
	}
	if err != nil {
		return grant{}, err
	}
	return grantFromEntity(row), nil
}

func (i *issuer) routes() http.Handler {
	mux := http.NewServeMux()
	i.route(mux, "GET /internal/grant", false, func(w http.ResponseWriter, r *http.Request, actor identity, _ string) error {
		g, err := readGrant(r.Context(), i.db.Grant.Query, actor.ID)
		if err == nil {
			writeJSON(w, 200, g)
		}
		return err
	})
	i.route(mux, "GET /internal/grants/{user}", true, func(w http.ResponseWriter, r *http.Request, _ identity, _ string) error {
		id, err := pathID(r, "user")
		if err != nil {
			return err
		}
		g, err := readGrant(r.Context(), i.db.Grant.Query, id)
		if err == nil {
			writeJSON(w, 200, g)
		}
		return err
	})
	i.route(mux, "PUT /internal/grants/{user}", true, i.changeGrant)
	i.route(mux, "DELETE /internal/grants/{user}", true, i.revokeGrant)
	i.route(mux, "POST /internal/issues", false, i.issue)
	i.route(mux, "GET /internal/issues", false, i.history)
	i.route(mux, "POST /internal/issues/{operation}/resolve", true, i.reconcile)
	return mux
}

var traceHex = regexp.MustCompile(`^[a-f0-9]{32}$`)
var spanHex = regexp.MustCompile(`^00-([a-f0-9]{32})-([a-f0-9]{16})-01$`)
var operationPattern = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)
var otpPattern = regexp.MustCompile(`^[0-9]{6}$`)

func (i *issuer) route(mux *http.ServeMux, pattern string, admin bool, run func(http.ResponseWriter, *http.Request, identity, string) error) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-API-Key")), []byte(i.secret)) != 1 {
			http.Error(w, "Forbidden", 403)
			return
		}
		ctx := r.Context()
		info := callMeta{Trace: r.Header.Get("X-Request-ID"), Span: randomHex(8), UA: r.UserAgent()}
		parent := spanHex.FindStringSubmatch(r.Header.Get("traceparent"))
		if !traceHex.MatchString(info.Trace) {
			info.Trace = randomHex(16)
		}
		if len(parent) == 3 && parent[1] == info.Trace {
			slog.Info("issuer received request", "trace_id", info.Trace, "span_id", info.Span, "parent_span_id", parent[2])
		}
		w.Header().Set("X-Request-ID", info.Trace)
		w.Header().Set("Cache-Control", "no-store")
		r = r.WithContext(context.WithValue(ctx, metaKey{}, info))
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			writeError(w, r, fail(401, "sign_in_required"))
			return
		}
		actor, err := currentUser(r.Context(), i.upstream, token)
		if err == nil {
			if admin && actor.Role != "admin" {
				err = fail(403, "not_allowed")
			} else {
				err = run(w, r, actor, token)
			}
		}
		if err != nil {
			writeError(w, r, translateUpstream(err))
		}
	})
}

func pathID(r *http.Request, key string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(key), 10, 64)
	if err != nil || id < 1 {
		return 0, fail(400, "invalid_input")
	}
	return id, nil
}

func (i *issuer) changeGrant(w http.ResponseWriter, r *http.Request, actor identity, token string) error {
	target, err := pathID(r, "user")
	if err != nil {
		return err
	}
	input, err := decodeBody[grantChange](r)
	if err != nil {
		return err
	}
	if !otpPattern.MatchString(input.TOTP) || input.Invites < 0 || input.Invites > 10000 ||
		input.Budget < 0 || input.Budget > maxCents || len(input.Amounts) > 20 ||
		input.Budget > 0 && len(input.Amounts) == 0 {
		return fail(400, "invalid_grant")
	}
	slices.Sort(input.Amounts)
	input.Amounts = slices.Compact(input.Amounts)
	for _, cents := range input.Amounts {
		if cents < 1 || cents > maxCents {
			return fail(400, "invalid_grant")
		}
	}
	ctx := r.Context()
	targetUser, err := upstream[identity](ctx, i.upstream, "GET", fmt.Sprintf("/admin/users/%d", target), token, "", "", nil)
	if err != nil {
		return err
	}
	if targetUser.ID != target || targetUser.Role != "user" {
		return fail(400, "invalid_grant")
	}
	verification, err := upstream[struct {
		Verified bool `json:"verified"`
	}](ctx, i.upstream, "POST", "/user/totp/step-up", token, "", "", struct {
		Code string `json:"code"`
	}{input.TOTP})
	if err != nil {
		return err
	}
	if !verification.Verified {
		return fail(403, "totp_required")
	}
	tx, err := i.db.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	old, err := tx.Grant.Query().Where(grantent.UserIDEQ(target)).Only(ctx)
	if ent.IsNotFound(err) {
		old = nil
	} else if err != nil {
		return err
	}
	oldEpoch := int64(1)
	if old != nil {
		oldEpoch = old.Epoch
	}
	reservedInvites, reservedBudget := int64(0), int64(0)
	reserved, err := tx.Issue.Query().Where(issueent.ActorEQ(target), issueent.GrantEpochEQ(oldEpoch), issueent.StateIn("pending", "unknown")).All(ctx)
	if err != nil {
		return err
	}
	for _, row := range reserved {
		if row.Kind == "invitation" {
			reservedInvites++
		} else {
			reservedBudget += row.Amount
		}
	}
	if input.Invites+reservedInvites > 10000 || input.Budget+reservedBudget > maxCents || input.Budget+reservedBudget > 0 && len(input.Amounts) == 0 {
		return fail(400, "invalid_grant")
	}
	if old == nil {
		if _, err = tx.Grant.Create().SetUserID(target).SetInvites(input.Invites).SetBudget(input.Budget).SetAmounts(input.Amounts).Save(ctx); err != nil {
			return err
		}
	} else if _, err = tx.Grant.UpdateOne(old).SetInvites(input.Invites).SetBudget(input.Budget).SetAmounts(input.Amounts).Save(ctx); err != nil {
		return err
	}
	if err = auditGrant(ctx, tx, actor.ID, target, input.Invites, input.Budget, "grant"); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	writeJSON(w, 200, struct{}{})
	return nil
}

func auditGrant(ctx context.Context, tx *ent.Tx, actor, target, invites, budget int64, action string) error {
	_, err := tx.GrantAudit.Create().SetActor(actor).SetTarget(target).SetInviteDelta(invites).SetBudgetDelta(budget).SetAction(action).SetTrace(meta(ctx).Trace).SetCreated(time.Now().Unix()).Save(ctx)
	if err == nil {
		slog.Info("issuer grant changed", "trace_id", meta(ctx).Trace, "actor_id", actor, "target_id", target, "action", action)
	}
	return err
}

func (i *issuer) revokeGrant(w http.ResponseWriter, r *http.Request, actor identity, _ string) error {
	target, err := pathID(r, "user")
	if err != nil {
		return err
	}
	ctx := r.Context()
	tx, err := i.db.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	old, err := tx.Grant.Query().Where(grantent.UserIDEQ(target)).Only(ctx)
	if ent.IsNotFound(err) {
		old = nil
	} else if err != nil {
		return err
	}
	if old == nil {
		old, err = tx.Grant.Create().SetUserID(target).SetInvites(0).SetBudget(0).SetAmounts([]int64{}).SetEpoch(2).Save(ctx)
		if err != nil {
			return err
		}
	} else if _, err = tx.Grant.UpdateOne(old).SetInvites(0).SetBudget(0).SetAmounts([]int64{}).AddEpoch(1).Save(ctx); err != nil {
		return err
	}
	if err = auditGrant(ctx, tx, actor.ID, target, -old.Invites, -old.Budget, "revoke"); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	writeJSON(w, 200, struct{}{})
	return nil
}

func (i *issuer) reserve(ctx context.Context, actor identity, input issueInput) error {
	tx, err := i.db.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if exists, err := tx.Issue.Query().Where(issueent.OpEQ(input.Operation)).Exist(ctx); err != nil {
		return err
	} else if exists {
		return fail(409, "duplicate_operation")
	}
	epoch := int64(0)
	if actor.Role != "admin" {
		row, err := tx.Grant.Query().Where(grantent.UserIDEQ(actor.ID)).Only(ctx)
		if ent.IsNotFound(err) {
			return fail(403, "grant_exhausted")
		}
		if err != nil {
			return err
		}
		epoch = row.Epoch
		update := tx.Grant.UpdateOne(row)
		switch input.Kind {
		case "balance":
			if row.Budget < input.Amount || !slices.Contains(row.Amounts, input.Amount) {
				return fail(403, "grant_exhausted")
			}
			update.AddBudget(-input.Amount).Where(grantent.BudgetGTE(input.Amount))
		case "invitation":
			if row.Invites < 1 {
				return fail(403, "grant_exhausted")
			}
			update.AddInvites(-1).Where(grantent.InvitesGTE(1))
		}
		if _, err = update.Save(ctx); err != nil {
			if ent.IsNotFound(err) {
				return fail(403, "grant_exhausted")
			}
			return err
		}
	}
	if _, err = tx.Issue.Create().SetOp(input.Operation).SetActor(actor.ID).SetKind(input.Kind).SetAmount(input.Amount).SetGrantEpoch(epoch).SetState("pending").SetCreated(time.Now().Unix()).SetTrace(meta(ctx).Trace).Save(ctx); err != nil {
		return err
	}
	return tx.Commit()
}

func (i *issuer) finishIssue(ctx context.Context, operation, state string, codeID *int64, reviewer int64) error {
	tx, err := i.db.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	event, err := tx.Issue.Query().Where(issueent.OpEQ(operation)).Only(ctx)
	if ent.IsNotFound(err) {
		return fail(404, "not_found")
	}
	if err != nil {
		return err
	}
	if event.State != "pending" && event.State != "unknown" || reviewer != 0 && event.State == "pending" && time.Now().Unix()-event.Created < 60 {
		return fail(409, "operation_unavailable")
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

func (i *issuer) issue(w http.ResponseWriter, r *http.Request, actor identity, _ string) error {
	input, err := decodeBody[issueInput](r)
	if err != nil {
		return err
	}
	if !operationPattern.MatchString(input.Operation) || !(input.Kind == "invitation" && input.Amount == 0 || input.Kind == "balance" && input.Amount > 0 && input.Amount <= maxCents) {
		return fail(400, "invalid_input")
	}
	days := 30
	if input.Kind == "invitation" {
		days = 7
		settings, err := upstream[struct {
			Enabled *bool `json:"invitation_code_enabled"`
		}](r.Context(), i.upstream, "GET", "/settings/public", "", "", "", nil)
		if err != nil {
			return err
		}
		if settings.Enabled == nil {
			return fail(502, "invalid_upstream")
		}
		if !*settings.Enabled {
			return fail(403, "not_allowed")
		}
	}
	ctx := context.WithoutCancel(r.Context())
	if err := i.reserve(ctx, actor, input); err != nil {
		return err
	}
	slog.Info("issue quota reserved", "trace_id", meta(ctx).Trace, "actor_id", actor.ID, "operation_id", input.Operation)
	codes, err := upstream[[]issuedCode](ctx, i.upstream, "POST", "/admin/redeem-codes/generate", "", i.adminKey, input.Operation, struct {
		Count int     `json:"count"`
		Kind  string  `json:"type"`
		Value float64 `json:"value"`
		Days  int     `json:"expires_in_days"`
	}{1, input.Kind, float64(input.Amount) / 100, days})
	state := "unknown"
	var id *int64
	var refused upstreamStatus
	switch {
	case err == nil && len(codes) == 1 && codes[0].ID > 0 && codes[0].Code != "" && codes[0].Kind == input.Kind && codes[0].Value == float64(input.Amount)/100:
		state, id = "issued", &codes[0].ID
	case errors.As(err, &refused) && (refused.status == 400 || refused.status == 401 || refused.status == 403 || refused.status == 404 || refused.status == 423 || refused.status == 429):
		state = "failed"
	}
	if err := i.finishIssue(ctx, input.Operation, state, id, 0); err != nil {
		return fail(503, "issue_unknown")
	}
	slog.Info("issue completed", "trace_id", meta(ctx).Trace, "actor_id", actor.ID, "operation_id", input.Operation, "state", state, "code_id", codeIDValue(id))
	switch state {
	case "issued":
		writeJSON(w, 201, issueResult{codes[0].Code})
		return nil
	case "failed":
		return fail(502, "issue_rejected")
	default:
		return fail(503, "issue_unknown")
	}
}

func (i *issuer) history(w http.ResponseWriter, r *http.Request, actor identity, _ string) error {
	page, err := requestedPage(r)
	if err != nil {
		return err
	}
	query := i.db.Issue.Query()
	if actor.Role != "admin" {
		query.Where(issueent.ActorEQ(actor.ID))
	}
	rows, err := query.Order(issueent.ByCreated(entsql.OrderDesc())).Limit(pageSize + 1).Offset((page - 1) * pageSize).All(r.Context())
	if err != nil {
		return err
	}
	items := make([]issueEvent, 0, len(rows))
	for _, row := range rows {
		delta := int64(0)
		if row.GrantEpoch != 0 && !row.Refunded {
			if row.Kind == "invitation" {
				delta = -1
			} else {
				delta = -row.Amount
			}
		}
		item := issueEvent{Operation: row.Op, Actor: row.Actor, Kind: row.Kind, Amount: row.Amount, State: row.State, CodeID: row.CodeID, Created: row.Created, RequestID: row.Trace, QuotaDelta: delta}
		if actor.Role != "admin" {
			item.Actor, item.RequestID = 0, ""
		}
		items = append(items, item)
	}
	writeJSON(w, 200, windowPage(items))
	return nil
}

func (i *issuer) reconcile(w http.ResponseWriter, r *http.Request, actor identity, token string) error {
	operation := r.PathValue("operation")
	if !operationPattern.MatchString(operation) {
		return fail(400, "invalid_input")
	}
	input, err := decodeBody[reconciliation](r)
	if err != nil {
		return err
	}
	state := "failed"
	if input.CodeID != nil {
		if *input.CodeID <= 0 {
			return fail(400, "invalid_input")
		}
		row, err := i.db.Issue.Query().Where(issueent.OpEQ(operation)).Only(r.Context())
		if ent.IsNotFound(err) {
			return fail(404, "not_found")
		}
		if err != nil {
			return err
		}
		code, err := upstream[issuedCode](r.Context(), i.upstream, "GET", fmt.Sprintf("/admin/redeem-codes/%d", *input.CodeID), token, "", "", nil)
		if err != nil {
			return err
		}
		if code.ID != *input.CodeID || code.Kind != row.Kind || code.Value != float64(row.Amount)/100 {
			return fail(400, "code_mismatch")
		}
		state = "issued"
	}
	if err := i.finishIssue(r.Context(), operation, state, input.CodeID, actor.ID); err != nil {
		return err
	}
	writeJSON(w, 200, struct{}{})
	return nil
}
