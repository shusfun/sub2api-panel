package billing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type redemptionRecord struct {
	ID     int64      `json:"id"`
	Type   string     `json:"type"`
	Value  float64    `json:"value"`
	UsedAt *time.Time `json:"used_at"`
}

type affiliateInvitee struct {
	UserID      int64      `json:"user_id"`
	Email       string     `json:"email"`
	Username    string     `json:"username"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	TotalRebate float64    `json:"total_rebate"`
}

type affiliateDetail struct {
	AffCode    string             `json:"aff_code"`
	AffCount   int                `json:"aff_count"`
	Available  float64            `json:"aff_quota"`
	Frozen     float64            `json:"aff_frozen_quota"`
	Total      float64            `json:"aff_history_quota"`
	RebateRate float64            `json:"effective_rebate_rate_percent"`
	Invitees   []affiliateInvitee `json:"invitees"`
}

func (h *handler) registerRedeemAndGrantRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "POST /api/redeem", false, h.redeem)
	h.s.Route(mux, "GET /api/redeem/history", false, h.redeemHistory)
	h.s.Route(mux, "GET /api/referral", false, h.referral)
	h.s.Route(mux, "GET /api/affiliate", false, h.affiliateDetail)
	h.s.Route(mux, "POST /api/affiliate/transfer", false, h.affiliateTransfer)

	h.s.Route(mux, "GET /api/grant", false, issuerCall[platform.Grant](h.s, "GET", fixed("/internal/grant"), nil))
	h.s.Route(mux, "GET /api/issues", false, issuerCall[platform.PageResult[platform.IssueEvent]](h.s, "GET", issuePage, nil))
	h.s.Route(mux, "POST /api/issues", false, issuerCall[platform.IssueResult](h.s, "POST", fixed("/internal/issues"), readInput[platform.IssueInput]))
	h.s.Route(mux, "GET /api/admin/grants/{user}", true, issuerCall[platform.Grant](h.s, "GET", grantPath, nil))
	h.s.Route(mux, "PUT /api/admin/grants/{user}", true, issuerCall[struct{}](h.s, "PUT", grantPath, readInput[platform.GrantChange]))
	h.s.Route(mux, "DELETE /api/admin/grants/{user}", true, issuerCall[struct{}](h.s, "DELETE", grantPath, nil))
	h.s.Route(mux, "POST /api/admin/issues/{operation}/resolve", true, issuerCall[struct{}](h.s, "POST", resolvePath, readInput[platform.Reconciliation]))
}

func (h *handler) redeemHistory(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, _, err := platform.ListQuery(r)
	if err != nil {
		return err
	}
	result, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/redeem/history", page, nil,
		func(row redemptionRecord) (redemptionRecord, error) { return row, platform.RequiredID(row.ID) })
	return platform.Send(w, http.StatusOK, result, err)
}

func (h *handler) redeem(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Code string `json:"code"`
	}](r)
	if err != nil {
		return err
	}
	input.Code = strings.TrimSpace(input.Code)
	if len(input.Code) < 3 || len(input.Code) > 128 {
		return platform.Fail(400, "invalid_input")
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "POST", "/redeem", a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, struct{}{}, err)
}

func (h *handler) referral(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	settings, err := h.s.Settings(r.Context())
	if err != nil {
		return err
	}
	if !settings.Affiliate {
		return platform.Fail(403, "not_allowed")
	}
	data, err := platform.Upstream[struct {
		Code string `json:"aff_code"`
	}](r.Context(), h.s.Upstream, "GET", "/user/aff", a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if data.Code == "" {
		return platform.Fail(502, "invalid_upstream")
	}
	platform.WriteJSON(w, 200, struct {
		URL string `json:"url"`
	}{h.s.Origin + "/register?aff=" + url.QueryEscape(data.Code)})
	return nil
}

func (h *handler) requireAffiliate(r *http.Request) error {
	settings, err := h.s.Settings(r.Context())
	if err != nil {
		return err
	}
	if !settings.Affiliate {
		return platform.Fail(http.StatusForbidden, "not_allowed")
	}
	return nil
}

func (h *handler) affiliateDetail(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	if err := h.requireAffiliate(r); err != nil {
		return err
	}
	row, err := platform.Upstream[affiliateDetail](r.Context(), h.s.Upstream, http.MethodGet, "/user/aff", a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if row.AffCode == "" || row.AffCount < 0 || row.RebateRate < 0 || row.RebateRate > 100 {
		return platform.Fail(http.StatusBadGateway, "invalid_upstream")
	}
	row.Invitees, err = platform.ProjectList(row.Invitees, func(row affiliateInvitee) (affiliateInvitee, error) { return row, platform.RequiredID(row.UserID) })
	w.Header().Set("Cache-Control", "no-store")
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) affiliateTransfer(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	if err := h.requireAffiliate(r); err != nil {
		return err
	}
	row, err := platform.Upstream[struct {
		Transferred *float64 `json:"transferred_quota"`
		Balance     *float64 `json:"balance"`
	}](r.Context(), h.s.Upstream, http.MethodPost, "/user/aff/transfer", a.Tokens.Access, "", "", nil)
	if err == nil && (row.Transferred == nil || *row.Transferred <= 0 || row.Balance == nil) {
		return platform.Fail(http.StatusBadGateway, "invalid_upstream")
	}
	w.Header().Set("Cache-Control", "no-store")
	return platform.Send(w, http.StatusOK, row, err)
}

func fixed(path string) func(*http.Request) (string, error) {
	return func(*http.Request) (string, error) { return path, nil }
}

func issuePage(r *http.Request) (string, error) {
	page, err := platform.RequestedPage(r)
	return fmt.Sprintf("/internal/issues?page=%d", page), err
}

func grantPath(r *http.Request) (string, error) {
	user, err := platform.PathID(r, "user")
	return fmt.Sprintf("/internal/grants/%d", user), err
}

func resolvePath(r *http.Request) (string, error) {
	id := r.PathValue("operation")
	if !platform.OperationPattern.MatchString(id) {
		return "", platform.Fail(400, "invalid_input")
	}
	return "/internal/issues/" + id + "/resolve", nil
}

func readInput[T any](r *http.Request) (any, error) { return platform.DecodeBody[T](r) }

func issuerCall[T any](s *core.Server, method string, path func(*http.Request) (string, error),
	input func(*http.Request) (any, error)) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		route, err := path(r)
		if err != nil {
			return err
		}
		var body any
		if input != nil {
			body, err = input(r)
			if err != nil {
				return err
			}
		}
		res, err := s.Issuer.Open(r.Context(), method, route, a.Tokens.Access, s.Secret, "", body)
		if err != nil {
			return platform.Fail(503, "issue_service_unavailable")
		}
		defer res.Body.Close()
		dec := json.NewDecoder(http.MaxBytesReader(w, res.Body, 128<<10))
		if res.StatusCode >= 400 {
			var out struct {
				Code string `json:"code"`
			}
			if dec.Decode(&out) != nil || !strings.Contains("|invalid_input|invalid_grant|invalid_upstream|sign_in_required|not_allowed|official_confirmation_required|not_found|grant_exhausted|duplicate_operation|issue_unknown|issue_rejected|operation_unavailable|code_mismatch|totp_required|operation_rejected|rate_limited|service_unavailable|", "|"+out.Code+"|") {
				return platform.Fail(502, "issuer_error")
			}
			return platform.Fail(res.StatusCode, out.Code)
		}
		var out T
		if dec.Decode(&out) != nil {
			return platform.Fail(502, "issuer_error")
		}
		platform.WriteJSON(w, res.StatusCode, out)
		return nil
	}
}
