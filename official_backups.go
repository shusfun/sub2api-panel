package main

import (
	"net/http"
	"net/url"
	"regexp"
)

const backupPath = "/admin/backups"

var backupIDPattern = regexp.MustCompile("^[0-9a-f]{8}$")

type backupRow struct {
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

func (p *panel) backupRoutes(mux *http.ServeMux) {
	p.route(mux, "GET /api"+backupPath, true, p.backupList)
	p.route(mux, "POST /api"+backupPath, true, p.backupCreate)
	p.route(mux, "GET /api"+backupPath+"/s3-config", true, p.backupStorage)
	p.route(mux, "PUT /api"+backupPath+"/s3-config", true, p.backupStorage)
	p.route(mux, "POST /api"+backupPath+"/s3-config/test", true, p.backupStorage)
	p.route(mux, "GET /api"+backupPath+"/schedule", true, p.backupSchedule)
	p.route(mux, "PUT /api"+backupPath+"/schedule", true, p.backupSchedule)
	p.route(mux, "DELETE /api"+backupPath+"/{id}", true, p.backupDelete)
	p.route(mux, "POST /api"+backupPath+"/{id}/download-url", true, p.backupDownload)
	p.route(mux, "POST /api"+backupPath+"/{id}/restore", true, p.backupRestore)
}

type backupAuthorization struct {
	TOTP string `json:"totp"`
}

func (p *panel) backupStepUp(r *http.Request, a sessionAuth, auth backupAuthorization) error {
	return p.requireOptionalStepUp(r, a, auth.TOTP)
}

func (p *panel) backupList(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	data, err := upstream[struct {
		Items []backupRow `json:"items"`
	}](r.Context(), p.upstream, r.Method, backupPath, a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	items, err := projectList(data.Items, func(row backupRow) (backupRow, error) {
		return row, validateBackupID(row.ID, http.StatusBadGateway, "invalid_upstream")
	})
	return send(w, http.StatusOK, items, err)
}

func validateBackupID(id string, status int, code string) error {
	if !backupIDPattern.MatchString(id) {
		return fail(status, code)
	}
	return nil
}

func (p *panel) backupCreate(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		backupAuthorization
		ExpireDays *int `json:"expire_days"`
	}](r)
	if err != nil {
		return err
	}
	if input.ExpireDays != nil && (*input.ExpireDays < 0 || *input.ExpireDays > 36500) {
		return fail(http.StatusBadRequest, "invalid_input")
	}
	if err := p.backupStepUp(r, a, input.backupAuthorization); err != nil {
		return err
	}
	data, err := upstream[backupRow](r.Context(), p.upstream, r.Method, backupPath, a.tokens.Access, "", "", struct {
		ExpireDays *int `json:"expire_days,omitempty"`
	}{input.ExpireDays})
	if err == nil {
		err = validateBackupID(data.ID, http.StatusBadGateway, "invalid_upstream")
	}
	return send(w, http.StatusAccepted, data, err)
}

func (p *panel) backupStorage(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	path := backupPath + "/s3-config"
	var input any
	if r.Method != http.MethodGet {
		config, err := decodeBody[struct {
			backupStorageInput
			backupAuthorization
		}](r)
		if err != nil {
			return err
		}
		if r.Method == http.MethodPut {
			if err := p.backupStepUp(r, a, config.backupAuthorization); err != nil {
				return err
			}
		}
		input = config.backupStorageInput
	}
	if r.Method == http.MethodPost {
		data, err := upstream[struct {
			OK bool `json:"ok"`
		}](r.Context(), p.upstream, r.Method, path+"/test", a.tokens.Access, "", "", input)
		return send(w, http.StatusOK, data, err)
	}
	data, err := upstream[backupStorage](r.Context(), p.upstream, r.Method, path, a.tokens.Access, "", "", input)
	return send(w, http.StatusOK, data, err)
}

func (p *panel) backupSchedule(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	var input any
	if r.Method == http.MethodPut {
		config, err := decodeBody[backupSchedule](r)
		if err != nil {
			return err
		}
		input = config
	}
	data, err := upstream[backupSchedule](r.Context(), p.upstream, r.Method, backupPath+"/schedule", a.tokens.Access, "", "", input)
	return send(w, http.StatusOK, data, err)
}

func (p *panel) backupDelete(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id := r.PathValue("id")
	if err := validateBackupID(id, http.StatusBadRequest, "invalid_input"); err != nil {
		return err
	}
	path := backupPath + "/" + id
	if r.URL.Query().Get("delete_archived") == "true" {
		path += "?delete_archived=true"
	}
	data, err := upstream[struct{}](r.Context(), p.upstream, r.Method, path, a.tokens.Access, "", "", nil)
	return send(w, http.StatusOK, data, err)
}

func (p *panel) backupDownload(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id := r.PathValue("id")
	if err := validateBackupID(id, http.StatusBadRequest, "invalid_input"); err != nil {
		return err
	}
	input, err := decodeBody[backupAuthorization](r)
	if err != nil {
		return err
	}
	if err := p.backupStepUp(r, a, input); err != nil {
		return err
	}
	data, err := upstream[backupDownload](r.Context(), p.upstream, http.MethodGet, backupPath+"/"+id+"/download-url", a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	valid := func(link string) bool {
		u, err := url.Parse(link)
		return err == nil && u.Hostname() != "" && u.User == nil && (u.Scheme == "https" || u.Scheme == "http")
	}
	if data.URL != "" && !valid(data.URL) || data.URL == "" && len(data.Parts) == 0 {
		return fail(http.StatusBadGateway, "invalid_upstream")
	}
	for _, part := range data.Parts {
		if !valid(part.URL) {
			return fail(http.StatusBadGateway, "invalid_upstream")
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	return send(w, http.StatusOK, data, nil)
}

func (p *panel) backupRestore(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id := r.PathValue("id")
	if err := validateBackupID(id, http.StatusBadRequest, "invalid_input"); err != nil {
		return err
	}
	input, err := decodeBody[struct {
		backupAuthorization
		Password string `json:"password"`
	}](r)
	if err != nil {
		return err
	}
	if input.Password == "" || len(input.Password) > 1024 {
		return fail(http.StatusBadRequest, "invalid_input")
	}
	if err := p.backupStepUp(r, a, input.backupAuthorization); err != nil {
		return err
	}
	data, err := upstream[backupRow](r.Context(), p.upstream, r.Method, backupPath+"/"+id+"/restore", a.tokens.Access, "", "", struct {
		Password string `json:"password"`
	}{input.Password})
	if err == nil {
		err = validateBackupID(data.ID, http.StatusBadGateway, "invalid_upstream")
	}
	return send(w, http.StatusAccepted, data, err)
}
