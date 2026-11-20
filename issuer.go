package main

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const maxCents = 100_000_000

type grant struct {
	Invites int64   `json:"invites"`
	Budget  int64   `json:"budget_cents"`
	Amounts []int64 `json:"amounts_cents"`
}

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
	db       *sql.DB
	upstream serviceClient
	secret   string
	adminKey string
}

func openIssuerDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA busy_timeout = 5000;
		CREATE TABLE IF NOT EXISTS grants(
			user_id INTEGER PRIMARY KEY, invites INTEGER NOT NULL CHECK(invites >= 0),
			budget INTEGER NOT NULL CHECK(budget >= 0), amounts TEXT NOT NULL,
			epoch INTEGER NOT NULL DEFAULT 1
		);
		CREATE TABLE IF NOT EXISTS issues(
			op TEXT PRIMARY KEY, actor INTEGER NOT NULL, kind TEXT NOT NULL,
			amount INTEGER NOT NULL, grant_epoch INTEGER NOT NULL, state TEXT NOT NULL,
			code_id INTEGER UNIQUE, created INTEGER NOT NULL, trace TEXT NOT NULL,
			refunded INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS issues_actor ON issues(actor, created);
		CREATE TABLE IF NOT EXISTS grant_audit(
			id INTEGER PRIMARY KEY, actor INTEGER NOT NULL, target INTEGER NOT NULL,
			invite_delta INTEGER NOT NULL, budget_delta INTEGER NOT NULL,
			action TEXT NOT NULL, trace TEXT NOT NULL, created INTEGER NOT NULL
		);
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

type rowReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readGrant(ctx context.Context, db rowReader, user int64) (grant, error) {
	g := grant{Amounts: []int64{}}
	var amounts string
	err := db.QueryRowContext(ctx, "SELECT invites,budget,amounts FROM grants WHERE user_id=?", user).Scan(&g.Invites, &g.Budget, &amounts)
	if errors.Is(err, sql.ErrNoRows) {
		return g, nil
	}
	if err != nil || amounts == "" {
		return g, err
	}
	for _, text := range strings.Split(amounts, ",") {
		cents, err := strconv.ParseInt(text, 10, 64)
		if err != nil || cents < 1 || cents > maxCents {
			return g, fail(503, "invalid_grant")
		}
		g.Amounts = append(g.Amounts, cents)
	}
	return g, nil
}

func (i *issuer) routes() http.Handler {
	mux := http.NewServeMux()
	i.route(mux, "GET /internal/grant", false, func(w http.ResponseWriter, r *http.Request, actor identity, _ string) error {
		g, err := readGrant(r.Context(), i.db, actor.ID)
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
		g, err := readGrant(r.Context(), i.db, id)
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
	formatted := make([]string, 0, len(input.Amounts))
	for _, cents := range input.Amounts {
		if cents < 1 || cents > maxCents {
			return fail(400, "invalid_grant")
		}
		formatted = append(formatted, strconv.FormatInt(cents, 10))
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
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	old, err := readGrant(ctx, tx, target)
	if err != nil {
		return err
	}
	g := grant{Invites: old.Invites + input.Invites, Budget: old.Budget + input.Budget, Amounts: input.Amounts}
	var reservedInvites, reservedBudget int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(kind='invitation'),0),COALESCE(SUM(amount),0)
		FROM issues WHERE actor=? AND grant_epoch=(SELECT epoch FROM grants WHERE user_id=?)
		AND state IN ('pending','unknown')`, target, target).Scan(&reservedInvites, &reservedBudget); err != nil {
		return err
	}
	if g.Invites+reservedInvites > 10000 || g.Budget+reservedBudget > maxCents || g.Budget+reservedBudget > 0 && len(g.Amounts) == 0 {
		return fail(400, "invalid_grant")
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO grants(user_id,invites,budget,amounts) VALUES (?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET invites=excluded.invites,budget=excluded.budget,amounts=excluded.amounts",
		target, g.Invites, g.Budget, strings.Join(formatted, ","))
	if err != nil {
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

func auditGrant(ctx context.Context, tx *sql.Tx, actor, target, invites, budget int64, action string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO grant_audit(actor,target,invite_delta,budget_delta,action,trace,created) VALUES(?,?,?,?,?,?,?)",
		actor, target, invites, budget, action, meta(ctx).Trace, time.Now().Unix())
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
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	old, err := readGrant(ctx, tx, target)
	if err != nil {
		return err
	}
	// Keep the row so a failed in-flight issue cannot resurrect a revoked grant.
	_, err = tx.ExecContext(ctx, "UPDATE grants SET invites=0,budget=0,amounts='',epoch=epoch+1 WHERE user_id=?", target)
	if err != nil {
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
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state string
	err = tx.QueryRowContext(ctx, "SELECT state FROM issues WHERE op=?", input.Operation).Scan(&state)
	if err == nil {
		return fail(409, "duplicate_operation")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var epoch int64
	if actor.Role != "admin" {
		g, err := readGrant(ctx, tx, actor.ID)
		if err != nil {
			return err
		}
		switch input.Kind {
		case "balance":
			if g.Budget < input.Amount || !slices.Contains(g.Amounts, input.Amount) {
				return fail(403, "grant_exhausted")
			}
			_, err = tx.ExecContext(ctx, "UPDATE grants SET budget=budget-? WHERE user_id=?", input.Amount, actor.ID)
		case "invitation":
			if g.Invites < 1 {
				return fail(403, "grant_exhausted")
			}
			_, err = tx.ExecContext(ctx, "UPDATE grants SET invites=invites-1 WHERE user_id=?", actor.ID)
		}
		if err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, "SELECT epoch FROM grants WHERE user_id=?", actor.ID).Scan(&epoch); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO issues(op,actor,kind,amount,grant_epoch,state,code_id,created,trace) VALUES(?,?,?,?,?,'pending',NULL,?,?)", input.Operation, actor.ID, input.Kind, input.Amount,
		epoch, time.Now().Unix(), meta(ctx).Trace)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (i *issuer) finishIssue(ctx context.Context, operation, state string, codeID *int64, reviewer int64) error {
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var event issueEvent
	var epoch int64
	err = tx.QueryRowContext(ctx, "SELECT actor,kind,amount,state,grant_epoch,created FROM issues WHERE op=?", operation).
		Scan(&event.Actor, &event.Kind, &event.Amount, &event.State, &epoch, &event.Created)
	if errors.Is(err, sql.ErrNoRows) {
		return fail(404, "not_found")
	}
	if err != nil {
		return err
	}
	if event.State != "pending" && event.State != "unknown" ||
		reviewer != 0 && event.State == "pending" && time.Now().Unix()-event.Created < 60 {
		return fail(409, "operation_unavailable")
	}
	refunded := false
	if state == "failed" && epoch != 0 {
		// A later revocation cancels the earlier grant, including any reserved portion.
		result, err := tx.ExecContext(ctx, `UPDATE grants SET invites=invites+?,budget=budget+?
			WHERE user_id=? AND epoch=?`,
			boolInt(event.Kind == "invitation"), event.Amount, event.Actor, epoch)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		refunded = rows == 1
	}
	if _, err = tx.ExecContext(ctx, "UPDATE issues SET state=?,code_id=?,refunded=? WHERE op=?", state, codeID, refunded, operation); err != nil {
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

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
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
	if !operationPattern.MatchString(input.Operation) ||
		!(input.Kind == "invitation" && input.Amount == 0 || input.Kind == "balance" && input.Amount > 0 && input.Amount <= maxCents) {
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
	case err == nil && len(codes) == 1 && codes[0].ID > 0 && codes[0].Code != "" &&
		codes[0].Kind == input.Kind && codes[0].Value == float64(input.Amount)/100:
		state, id = "issued", &codes[0].ID
	case errors.As(err, &refused) && (refused.status == 400 || refused.status == 401 ||
		refused.status == 403 || refused.status == 404 || refused.status == 423 || refused.status == 429):
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
	query := `SELECT op,actor,kind,amount,state,code_id,created,trace,
		CASE WHEN grant_epoch=0 OR refunded=1 THEN 0 WHEN kind='invitation' THEN -1 ELSE -amount END
		FROM issues WHERE actor=? OR ? ORDER BY rowid DESC LIMIT ? OFFSET ?`
	rows, err := i.db.QueryContext(r.Context(), query, actor.ID, actor.Role == "admin", pageSize+1, (page-1)*pageSize)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := make([]issueEvent, 0, pageSize)
	for rows.Next() {
		var item issueEvent
		if err := rows.Scan(&item.Operation, &item.Actor, &item.Kind, &item.Amount, &item.State, &item.CodeID, &item.Created, &item.RequestID, &item.QuotaDelta); err != nil {
			return err
		}
		if actor.Role != "admin" {
			item.Actor = 0
			item.RequestID = ""
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
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
	ctx := r.Context()
	state := "failed"
	if input.CodeID != nil {
		if *input.CodeID <= 0 {
			return fail(400, "invalid_input")
		}
		var kind string
		var amount int64
		if err := i.db.QueryRowContext(ctx, "SELECT kind,amount FROM issues WHERE op=?", operation).Scan(&kind, &amount); err != nil {
			return fail(404, "not_found")
		}
		code, err := upstream[issuedCode](ctx, i.upstream, "GET", fmt.Sprintf("/admin/redeem-codes/%d", *input.CodeID), token, "", "", nil)
		if err != nil {
			return err
		}
		if code.ID != *input.CodeID || code.Kind != kind || code.Value != float64(amount)/100 {
			return fail(400, "code_mismatch")
		}
		state = "issued"
	}
	if err := i.finishIssue(ctx, operation, state, input.CodeID, actor.ID); err != nil {
		return err
	}
	writeJSON(w, 200, struct{}{})
	return nil
}
