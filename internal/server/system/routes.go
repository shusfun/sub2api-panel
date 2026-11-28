package system

import (
	"net/http"

	"sub2api-panel/internal/server/core"
)

type handler struct {
	s *core.Server
}

func RegisterRoutes(mux *http.ServeMux, s *core.Server) {
	h := &handler{s: s}
	s.Route(mux, "GET /api/admin/custom-menu", true, h.adminCustomMenu)
	s.Route(mux, "GET /api/admin/compliance", true, h.adminCompliance)
	s.Route(mux, "POST /api/admin/compliance/accept", true, h.adminCompliance)
	s.Route(mux, "GET /api/admin/audit-logs", true, h.auditLogList)
	s.Route(mux, "GET /api/admin/audit-logs/{id}", true, h.auditLogDetail)
	s.Route(mux, "POST /api/admin/audit-logs/clear", true, h.auditLogClear)
	h.settingsRoutes(mux)
	h.pluginRoutes(mux)
	h.siteContentRoutes(mux)
	h.backupRoutes(mux)
	h.riskRoutes(mux)
	h.promptAuditRoutes(mux)
}
