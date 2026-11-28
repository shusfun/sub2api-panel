package system

import (
	"net/http"
	"strings"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

const imageStoragePath = backupPath + "/image-storage"

// 生图对象存储：响应白名单只含非敏感配置，密钥只用 secret_configured 表示是否已配置。
type imageStorageView struct {
	Enabled          bool   `json:"enabled"`
	ReuseBackupS3    bool   `json:"reuse_backup_s3"`
	Bucket           string `json:"bucket"`
	Prefix           string `json:"prefix"`
	PublicBaseURL    string `json:"public_base_url"`
	PresignExpiry    int    `json:"presign_expiry_hours"`
	MaxDownloadBytes int64  `json:"max_download_bytes"`
	Endpoint         string `json:"endpoint"`
	Region           string `json:"region"`
	AccessKeyID      string `json:"access_key_id"`
	ForcePathStyle   bool   `json:"force_path_style"`
}

type imageStorageState struct {
	Config           imageStorageView `json:"config"`
	SecretConfigured bool             `json:"secret_configured"`
}

type imageStorageInput struct {
	imageStorageView
	Secret string `json:"secret_access_key,omitempty"`
}

func (h *handler) imageStorageRoutes(mux *http.ServeMux) {
	base := "/api" + imageStoragePath
	h.s.Route(mux, "GET "+base, true, h.imageStorage)
	h.s.Route(mux, "PUT "+base, true, h.imageStorage)
	h.s.Route(mux, "POST "+base+"/test", true, h.imageStorageTest)
}

func validImageStorage(in imageStorageInput) bool {
	return len(in.Bucket) <= 200 && len(in.Prefix) <= 200 && len(in.PublicBaseURL) <= 500 && len(in.Endpoint) <= 500 &&
		len(in.Region) <= 100 && len(in.AccessKeyID) <= 200 && len(in.Secret) <= 2000 &&
		in.PresignExpiry >= 0 && in.MaxDownloadBytes >= 0 &&
		(in.ReuseBackupS3 || !in.Enabled || strings.TrimSpace(in.Bucket) != "")
}

func (h *handler) imageStorage(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	if r.Method == http.MethodGet {
		row, err := platform.Upstream[imageStorageState](r.Context(), h.s.Upstream, r.Method, imageStoragePath, a.Tokens.Access, "", "", nil)
		return platform.Send(w, http.StatusOK, row, err)
	}
	input, err := platform.DecodeBody[struct {
		imageStorageInput
		backupAuthorization
	}](r)
	if err != nil {
		return err
	}
	if !validImageStorage(input.imageStorageInput) {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	if err := h.backupStepUp(r, a, input.backupAuthorization); err != nil {
		return err
	}
	row, err := platform.Upstream[imageStorageView](r.Context(), h.s.Upstream, http.MethodPut, imageStoragePath, a.Tokens.Access, "", "", input.imageStorageInput)
	return platform.Send(w, http.StatusOK, row, err)
}

func (h *handler) imageStorageTest(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[imageStorageInput](r)
	if err != nil || !validImageStorage(input) {
		return platform.Fail(http.StatusBadRequest, "invalid_input")
	}
	row, err := platform.Upstream[struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
	}](r.Context(), h.s.Upstream, http.MethodPost, imageStoragePath+"/test", a.Tokens.Access, "", "", input)
	if err == nil && len(row.Message) > 200 {
		row.Message = row.Message[:200]
	}
	return platform.Send(w, http.StatusOK, row, err)
}
