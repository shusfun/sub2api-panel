package issuer

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"sub2api-panel/ent"
	grantent "sub2api-panel/ent/grant"
	issueent "sub2api-panel/ent/issue"
	"sub2api-panel/internal/platform"
)

const maxCents = 100_000_000

type Service struct {
	DB       *ent.Client
	Upstream platform.ServiceClient
	Secret   string
	AdminKey string
}

func grantFromEntity(row *ent.Grant) platform.Grant {
	amounts := append([]int64(nil), row.Amounts...)
	if amounts == nil {
		amounts = []int64{}
	}
	return platform.Grant{Invites: row.Invites, Budget: row.Budget, Amounts: amounts}
}

func readGrant(ctx context.Context, query func() *ent.GrantQuery, user int64) (platform.Grant, error) {
	row, err := query().Where(grantent.UserIDEQ(user)).Only(ctx)
	if ent.IsNotFound(err) {
		return platform.Grant{Amounts: []int64{}}, nil
	}
	if err != nil {
		return platform.Grant{}, err
	}
	return grantFromEntity(row), nil
}

func (i *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	i.route(mux, "GET /internal/grant", false, func(w http.ResponseWriter, r *http.Request, actor platform.Identity, _ string) error {
		g, err := readGrant(r.Context(), i.DB.Grant.Query, actor.ID)
		if err == nil {
			platform.WriteJSON(w, 200, g)
		}
		return err
	})
	i.route(mux, "GET /internal/grants/{user}", true, func(w http.ResponseWriter, r *http.Request, _ platform.Identity, _ string) error {
		id, err := platform.PathID(r, "user")
		if err != nil {
			return err
		}
		g, err := readGrant(r.Context(), i.DB.Grant.Query, id)
		if err == nil {
			platform.WriteJSON(w, 200, g)
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

func (i *Service) route(mux *http.ServeMux, pattern string, admin bool, run func(http.ResponseWriter, *http.Request, platform.Identity, string) error) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-API-Key")), []byte(i.Secret)) != 1 {
			http.Error(w, "Forbidden", 403)
			return
		}
		ctx := r.Context()
		info := platform.CallMeta{Trace: r.Header.Get("X-Request-ID"), Span: platform.RandomHex(8), UA: r.UserAgent()}
		parent := platform.SpanHex.FindStringSubmatch(r.Header.Get("traceparent"))
		if !platform.TraceHex.MatchString(info.Trace) {
			info.Trace = platform.RandomHex(16)
		}
		if len(parent) == 3 && parent[1] == info.Trace {
			slog.Info("issuer received request", "trace_id", info.Trace, "span_id", info.Span, "parent_span_id", parent[2])
		}
		w.Header().Set("X-Request-ID", info.Trace)
		w.Header().Set("Cache-Control", "no-store")
		r = r.WithContext(platform.WithMeta(ctx, info))
		r.Body = http.MaxBytesReader(w, r.Body, platform.MaxBody)
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			platform.WriteError(w, r, platform.Fail(401, "sign_in_required"))
			return
		}
		actor, err := platform.CurrentUser(r.Context(), i.Upstream, token)
		if err == nil {
			if admin && actor.Role != "admin" {
				err = platform.Fail(403, "not_allowed")
			} else {
				err = run(w, r, actor, token)
			}
		}
		if err != nil {
			platform.WriteError(w, r, platform.TranslateUpstream(err))
		}
	})
}

func (i *Service) changeGrant(w http.ResponseWriter, r *http.Request, actor platform.Identity, token string) error {
	target, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[platform.GrantChange](r)
	if err != nil {
		return err
	}
	if !platform.OTPPattern.MatchString(input.TOTP) || input.Invites < 0 || input.Invites > 10000 ||
		input.Budget < 0 || input.Budget > maxCents || len(input.Amounts) > 20 ||
		input.Budget > 0 && len(input.Amounts) == 0 {
		return platform.Fail(400, "invalid_grant")
	}
	slices.Sort(input.Amounts)
	input.Amounts = slices.Compact(input.Amounts)
	for _, cents := range input.Amounts {
		if cents < 1 || cents > maxCents {
			return platform.Fail(400, "invalid_grant")
		}
	}
	ctx := r.Context()
	targetUser, err := platform.Upstream[platform.Identity](ctx, i.Upstream, "GET", fmt.Sprintf("/admin/users/%d", target), token, "", "", nil)
	if err != nil {
		return err
	}
	if targetUser.ID != target || targetUser.Role != "user" {
		return platform.Fail(400, "invalid_grant")
	}
	verification, err := platform.Upstream[struct {
		Verified bool `json:"verified"`
	}](ctx, i.Upstream, "POST", "/user/totp/step-up", token, "", "", struct {
		Code string `json:"code"`
	}{input.TOTP})
	if err != nil {
		return err
	}
	if !verification.Verified {
		return platform.Fail(403, "totp_required")
	}
	tx, err := i.DB.Tx(ctx)
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
		return platform.Fail(400, "invalid_grant")
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
	platform.WriteJSON(w, 200, struct{}{})
	return nil
}

func auditGrant(ctx context.Context, tx *ent.Tx, actor, target, invites, budget int64, action string) error {
	_, err := tx.GrantAudit.Create().SetActor(actor).SetTarget(target).SetInviteDelta(invites).SetBudgetDelta(budget).SetAction(action).SetTrace(platform.Meta(ctx).Trace).SetCreated(time.Now().Unix()).Save(ctx)
	if err == nil {
		slog.Info("issuer grant changed", "trace_id", platform.Meta(ctx).Trace, "actor_id", actor, "target_id", target, "action", action)
	}
	return err
}

func (i *Service) revokeGrant(w http.ResponseWriter, r *http.Request, actor platform.Identity, _ string) error {
	target, err := platform.PathID(r, "user")
	if err != nil {
		return err
	}
	ctx := r.Context()
	tx, err := i.DB.Tx(ctx)
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
	platform.WriteJSON(w, 200, struct{}{})
	return nil
}
