package observe

import (
	"net/http"

	"sub2api-panel/internal/server/core"
)

func RegisterRoutes(mux *http.ServeMux, s *core.Server) {
	// User & Admin Usage
	s.Route(mux, "GET /api/usage", false, usageRecords(s))
	s.Route(mux, "GET /api/usage/summary", false, usageSummaryHandler(s))
	s.Route(mux, "GET /api/usage/keys", false, usageKeys(s))
	s.Route(mux, "GET /api/usage/stream", false, usageStream(s))
	s.Route(mux, "POST /api/keys/usage", false, keyUsageBatch(s))
	s.Route(mux, "GET /api/admin/usage/users", true, adminUsageUsers(s))
	s.Route(mux, "GET /api/admin/usage", true, adminDashboardUsage(s))
	s.Route(mux, "GET /api/admin/usage/keys", true, adminDashboardKeys(s))
	s.Route(mux, "GET /api/admin/usage/summary", true, adminUsageSummaryHandler(s))
	s.Route(mux, "GET /api/admin/usage/stream", true, adminUsageStream(s))
	s.Route(mux, "GET /api/admin/users/{user}/usage", true, adminUsage(s))

	// Admin Overview
	s.Route(mux, "GET /api/admin/overview", true, adminOverview(s))

	// Ops
	registerOpsRoutes(mux, s)

	// Patrol
	registerPatrolRoutes(mux, s)

	// Probe
	registerProbeRoutes(mux, s)
}

func registerOpsRoutes(mux *http.ServeMux, s *core.Server) {
	s.Route(mux, "GET /api/admin/ops/overview", true, opsOverview(s))
	s.Route(mux, "GET /api/admin/ops/concurrency", true, opsConcurrency(s))
	s.Route(mux, "GET /api/admin/ops/stream", true, opsStream(s))
	s.Route(mux, "GET /api/admin/ops/requests", true, opsRequests(s))
	s.Route(mux, "GET /api/admin/ops/request-errors", true, opsErrorList(s, "/admin/ops/request-errors"))
	s.Route(mux, "GET /api/admin/ops/request-errors/{id}", true, opsErrorDetail(s, "/admin/ops/request-errors"))
	s.Route(mux, "GET /api/admin/ops/request-errors/{id}/upstream-errors", true, opsRequestUpstreamErrors(s))
	s.Route(mux, "PUT /api/admin/ops/request-errors/{id}/resolve", true, opsErrorResolve(s, "/admin/ops/request-errors"))
	s.Route(mux, "GET /api/admin/ops/upstream-errors", true, opsErrorList(s, "/admin/ops/upstream-errors"))
	s.Route(mux, "GET /api/admin/ops/upstream-errors/{id}", true, opsErrorDetail(s, "/admin/ops/upstream-errors"))
	s.Route(mux, "PUT /api/admin/ops/upstream-errors/{id}/resolve", true, opsErrorResolve(s, "/admin/ops/upstream-errors"))
	s.Route(mux, "GET /api/admin/ops/alert-rules", true, listOpsAlertRules(s))
	s.Route(mux, "POST /api/admin/ops/alert-rules", true, createOpsAlertRule(s))
	s.Route(mux, "PUT /api/admin/ops/alert-rules/{id}", true, updateOpsAlertRule(s))
	s.Route(mux, "DELETE /api/admin/ops/alert-rules/{id}", true, deleteOpsAlertRule(s))
	s.Route(mux, "GET /api/admin/ops/alert-events", true, listOpsAlertEvents(s))
	s.Route(mux, "PUT /api/admin/ops/alert-events/{id}/status", true, updateOpsAlertEventStatus(s))
	s.Route(mux, "POST /api/admin/ops/alert-silences", true, createOpsAlertSilence(s))
	s.Route(mux, "GET /api/admin/ops/settings", true, getOpsSettings(s))
	s.Route(mux, "PUT /api/admin/ops/settings", true, updateOpsSettings(s))
	s.Route(mux, "POST /api/admin/ops/runtime/logging/reset", true, resetOpsLogging(s))
}

func registerPatrolRoutes(mux *http.ServeMux, s *core.Server) {
	s.Public(mux, "GET /api/patrol/public", publicPatrolBoard(s))
	s.Public(mux, "GET /api/patrol/public/runs/{id}", publicPatrolRunDetail(s))
	s.Route(mux, "GET /api/patrol", false, userPatrolBoard(s))
	s.Route(mux, "GET /api/patrol/runs/{id}", false, userPatrolRunDetail(s))

	s.Route(mux, "GET /api/admin/patrol/policies", true, adminListPatrolPolicies(s))
	s.Route(mux, "PUT /api/admin/patrol/policies/{group_id}", true, adminUpsertPatrolPolicy(s))
	s.Route(mux, "DELETE /api/admin/patrol/policies/{group_id}", true, adminDeletePatrolPolicy(s))
	s.Route(mux, "POST /api/admin/patrol/policies/{group_id}/mint-key", true, adminMintPatrolPolicyKey(s))
	s.Route(mux, "POST /api/admin/patrol/policies/{group_id}/run", true, adminTriggerPatrolNow(s))

	s.Route(mux, "GET /api/admin/patrol/templates", true, adminListProbeTemplates(s))
	s.Route(mux, "POST /api/admin/patrol/templates", true, adminCreateProbeTemplate(s))
	s.Route(mux, "PUT /api/admin/patrol/templates/{id}", true, adminUpdateProbeTemplate(s))
	s.Route(mux, "DELETE /api/admin/patrol/templates/{id}", true, adminDeleteProbeTemplate(s))
}

func registerProbeRoutes(mux *http.ServeMux, s *core.Server) {
	s.Route(mux, "POST /api/admin/accounts/{id}/probe", true, adminProbeAccount(s))
	s.Route(mux, "POST /api/admin/accounts/{id}/probe-pass", true, adminMintAccountProbePass(s))
	s.Route(mux, "POST /api/admin/groups/{id}/probe", true, adminProbeGroup(s))
	s.Route(mux, "POST /api/admin/groups/{id}/probe-pass", true, adminMintGroupProbePass(s))
	s.Public(mux, "POST /api/patrol/guest-probe", publicGuestProbe(s))
}
