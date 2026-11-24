package system

import (
	"net/http"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func (h *handler) adminMenu(r *http.Request, a core.SessionAuth) ([]core.CustomMenuItem, error) {
	result, err := platform.Upstream[struct {
		Items []core.CustomMenuItem `json:"custom_menu_items"`
	}](r.Context(), h.s.Upstream, "GET", "/admin/settings", a.Tokens.Access, "", "", nil)
	if err != nil {
		return nil, err
	}
	return core.ProjectCustomMenu(result.Items, "admin"), nil
}

func (h *handler) adminCustomMenu(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	items, err := h.adminMenu(r, a)
	return platform.Send(w, 200, items, err)
}
