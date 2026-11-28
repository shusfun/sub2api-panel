package billing

import (
	"net/http"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

// POST /api/admin/ledger/probe：入参只有 url，返回站点类型、额度换算与令牌页地址。
// 令牌页路径：New API 为 /keys（由其前端路由目录推断），One API 为 /token（读到路由声明），
// Sub2API 为 /keys（前端路由 frontend/src/router/index.ts:207 已核实）；识别不出时返回站点首页。
func (h *handler) ledgerProbe(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	if h.s.Balances == nil {
		return platform.Fail(503, "service_unavailable")
	}
	input, err := platform.DecodeBody[LedgerProbeInput](r)
	if err != nil {
		return err
	}
	if input.URL == "" || len(input.URL) > 2048 {
		return platform.Fail(400, "invalid_input")
	}
	found, err := h.s.Balances.ProbeSite(r.Context(), input.URL)
	if err != nil {
		return err
	}
	view := LedgerProbeView{Type: found.Type, Origin: found.Origin, QuotaPerUnit: found.QuotaPerUnit, TokenPageURL: found.Origin, Scopes: []string{}}
	switch found.Type {
	case "newapi":
		view.TokenPageURL, view.Scopes = found.Origin+"/keys", []string{"key", "wallet"}
	case "oneapi":
		view.TokenPageURL, view.Scopes = found.Origin+"/token", []string{"wallet"}
	case "sub2api":
		view.TokenPageURL, view.Scopes = found.Origin+"/keys", []string{"key"}
	}
	return platform.Send(w, 200, view, nil)
}
