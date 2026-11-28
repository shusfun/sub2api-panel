package system

import (
	"net/http"

	"sub2api-panel/internal/panelupdate"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func (h *handler) panelUpdateStatus(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
	status, err := h.s.PanelUpdater.Status(r.Context())
	return platform.Send(w, http.StatusOK, status, err)
}

func (h *handler) panelUpdateCheck(w http.ResponseWriter, r *http.Request, _ core.SessionAuth) error {
	ctx, finish := platform.StartFlow(r.Context(), "检查面板更新")
	defer finish()
	check, err := h.s.PanelUpdater.Check(ctx)
	return platform.Send(w, http.StatusOK, check, err)
}

func (h *handler) panelUpdateSubmit(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Version string `json:"version"`
		TOTP    string `json:"totp"`
	}](r)
	if err != nil {
		return err
	}
	if len(input.Version) > 40 || !panelupdate.ValidVersion(input.Version) {
		return platform.Fail(400, "invalid_input")
	}
	ctx, finish := platform.StartFlow(r.Context(), "提交面板更新", "actor_id", a.User.ID, "version", input.Version)
	defer finish()
	r = r.WithContext(ctx)
	if err := h.s.StepUp(r, a, input.TOTP); err != nil {
		return err
	}
	job, err := h.s.PanelUpdater.Submit(ctx, panelupdate.Request{Version: input.Version, Actor: a.User.ID})
	return platform.Send(w, http.StatusAccepted, job, err)
}
