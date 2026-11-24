package system

import (
	"net/http"
	"net/url"
	"regexp"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

const backupPath = "/admin/backups"

var backupIDPattern = regexp.MustCompile("^[0-9a-f]{8}$")

type BackupRow struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	File          string `json:"file_name"`
	Size          int64  `json:"size_bytes"`
	Started       string `json:"started_at"`
	Expires       string `json:"expires_at"`
	Progress      string `json:"progress"`
	RestoreStatus string `json:"restore_status"`
}

type backupDownload struct {
	URL   string `json:"url,omitempty"`
	Parts []struct {
		Index int    `json:"index"`
		Size  int64  `json:"size_bytes"`
		URL   string `json:"url"`
	} `json:"parts,omitempty"`
}

type backupStorage struct {
	Endpoint       string `json:"endpoint"`
	Region         string `json:"region"`
	Bucket         string `json:"bucket"`
	AccessKeyID    string `json:"access_key_id"`
	Prefix         string `json:"prefix"`
	ForcePathStyle bool   `json:"force_path_style"`
}

type backupStorageInput struct {
	backupStorage
	Secret string `json:"secret_access_key,omitempty"`
}

type backupSchedule struct {
	Enabled    bool   `json:"enabled"`
	Cron       string `json:"cron_expr"`
	RetainDays int    `json:"retain_days"`
	Retain     int    `json:"retain_count"`
	Archive    *struct {
		Enabled  bool  `json:"enabled"`
		Days     []int `json:"days"`
		MonthEnd bool  `json:"include_month_end"`
		Retain   int   `json:"retain_count"`
	} `json:"monthly_archive,omitempty"`
}

func (h *handler) backupRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api"+backupPath, true, h.backupList)
	h.s.Route(mux, "POST /api"+backupPath, true, h.backupCreate)
	h.s.Route(mux, "GET /api"+backupPath+"/s3-config", true, h.backupStorage)
	h.s.Route(mux, "PUT /api"+backupPath+"/s3-config", true, h.backupStorage)
	h.s.Route(mux, "POST /api"+backupPath+"/s3-config/test", true, h.backupStorage)
	h.s.Route(mux, "GET /api"+backupPath+"/schedule", true, h.backupSchedule)
	h.s.Route(mux, "PUT /api"+backupPath+"/schedule", true, h.backupSchedule)
	h.s.Route(mux, "DELETE /api"+backupPath+"/{id}", true, h.backupDelete)
	h.s.Route(mux, "POST /api"+backupPath+"/{id}/download-url", true, h.backupDownload)
	h.s.Route(mux, "POST /api"+backupPath+"/{id}/restore", true, h.backupRestore)
}

type backupAuthorization struct {
	TOTP string `json:"totp"`
}

func (h *handler) backupStepUp(r *http.Request, a core.SessionAuth, auth backupAuthorization) error {
	return h.s.RequireOptionalStepUp(r, a, auth.TOTP)
}

func (h *handler) backupList(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	data, err := platform.Upstream[struct {
		Items []BackupRow `json:"items"`
	}](r.Context(), h.s.Upstream, r.Method, backupPath, a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	items, err := platform.ProjectList(data.Items, func(row BackupRow) (BackupRow, error) {
		return row, validateBackupID(row.ID, http.StatusBadGateway, "invalid_upstream")
	})
	return platform.Send(w, http.StatusOK, items, err)
}

func validateBackupID(id string, status int, code string) error {
	if !backupIDPattern.MatchString(id) {
		return platform.Fail(status, code)
	}
	return nil
}

func (h *handler) backupCreate(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		backupAuthorization
		ExpireDays *int `json:"expire_days"`
	}](r)
	if err != nil {
		return err
	}
	if input.ExpireDays != nil && (*input.ExpireDays < 0 || *input.ExpireDays > 36500) {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	if err := h.backupStepUp(r, a, input.backupAuthorization); err != nil {
		return err
	}
	data, err := platform.Upstream[BackupRow](r.Context(), h.s.Upstream, r.Method, backupPath, a.Tokens.Access, "", "", struct {
		ExpireDays *int `json:"expire_days,omitempty"`
	}{input.ExpireDays})
	if err == nil {
		err = validateBackupID(data.ID, http.StatusBadGateway, "invalid_upstream")
	}
	return platform.Send(w, http.StatusAccepted, data, err)
}

func (h *handler) backupStorage(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	path := backupPath + "/s3-config"
	var input any
	if r.Method != http.MethodGet {
		config, err := platform.DecodeBody[struct {
			backupStorageInput
			backupAuthorization
		}](r)
		if err != nil {
			return err
		}
		if r.Method == http.MethodPut {
			if err := h.backupStepUp(r, a, config.backupAuthorization); err != nil {
				return err
			}
		}
		input = config.backupStorageInput
	}
	if r.Method == http.MethodPost {
		data, err := platform.Upstream[struct {
			OK bool `json:"ok"`
		}](r.Context(), h.s.Upstream, r.Method, path+"/test", a.Tokens.Access, "", "", input)
		return platform.Send(w, http.StatusOK, data, err)
	}
	data, err := platform.Upstream[backupStorage](r.Context(), h.s.Upstream, r.Method, path, a.Tokens.Access, "", "", input)
	return platform.Send(w, http.StatusOK, data, err)
}

func (h *handler) backupSchedule(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	var input any
	if r.Method == http.MethodPut {
		config, err := platform.DecodeBody[backupSchedule](r)
		if err != nil {
			return err
		}
		input = config
	}
	data, err := platform.Upstream[backupSchedule](r.Context(), h.s.Upstream, r.Method, backupPath+"/schedule", a.Tokens.Access, "", "", input)
	return platform.Send(w, http.StatusOK, data, err)
}

func (h *handler) backupDelete(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id := r.PathValue("id")
	if err := validateBackupID(id, http.StatusBadRequest, "invalid_input"); err != nil {
		return err
	}
	path := backupPath + "/" + id
	if r.URL.Query().Get("delete_archived") == "true" {
		path += "?delete_archived=true"
	}
	data, err := platform.Upstream[struct{}](r.Context(), h.s.Upstream, r.Method, path, a.Tokens.Access, "", "", nil)
	return platform.Send(w, http.StatusOK, data, err)
}

func (h *handler) backupDownload(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id := r.PathValue("id")
	if err := validateBackupID(id, http.StatusBadRequest, "invalid_input"); err != nil {
		return err
	}
	input, err := platform.DecodeBody[backupAuthorization](r)
	if err != nil {
		return err
	}
	if err := h.backupStepUp(r, a, input); err != nil {
		return err
	}
	data, err := platform.Upstream[backupDownload](r.Context(), h.s.Upstream, http.MethodGet, backupPath+"/"+id+"/download-url", a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	valid := func(link string) bool {
		u, err := url.Parse(link)
		return err == nil && u.Hostname() != "" && u.User == nil && (u.Scheme == "https" || u.Scheme == "http")
	}
	if data.URL != "" && !valid(data.URL) || data.URL == "" && len(data.Parts) == 0 {
		return platform.Fail(http.StatusBadGateway, "invalid_upstream")
	}
	for _, part := range data.Parts {
		if !valid(part.URL) {
			return platform.Fail(http.StatusBadGateway, "invalid_upstream")
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	return platform.Send(w, http.StatusOK, data, nil)
}

func (h *handler) backupRestore(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id := r.PathValue("id")
	if err := validateBackupID(id, http.StatusBadRequest, "invalid_input"); err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		backupAuthorization
		Password string `json:"password"`
	}](r)
	if err != nil {
		return err
	}
	if input.Password == "" || len(input.Password) > 1024 {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	if err := h.backupStepUp(r, a, input.backupAuthorization); err != nil {
		return err
	}
	data, err := platform.Upstream[BackupRow](r.Context(), h.s.Upstream, r.Method, backupPath+"/"+id+"/restore", a.Tokens.Access, "", "", struct {
		Password string `json:"password"`
	}{input.Password})
	if err == nil {
		err = validateBackupID(data.ID, http.StatusBadGateway, "invalid_upstream")
	}
	return platform.Send(w, http.StatusAccepted, data, err)
}
