package pool

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type ProxyWire struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	Protocol       string     `json:"protocol"`
	Host           string     `json:"host"`
	Port           int        `json:"port"`
	Username       *string    `json:"username"`
	Password       *string    `json:"password"`
	Status         string     `json:"status"`
	AccountCount   int64      `json:"account_count"`
	Latency        *float64   `json:"latency_ms"`
	LatencyStatus  string     `json:"latency_status"`
	IPAddress      string     `json:"ip_address"`
	Country        string     `json:"country"`
	CountryCode    string     `json:"country_code"`
	Region         string     `json:"region"`
	City           string     `json:"city"`
	QualityStatus  string     `json:"quality_status"`
	QualityScore   *float64   `json:"quality_score"`
	QualityGrade   string     `json:"quality_grade"`
	QualitySummary string     `json:"quality_summary"`
	QualityChecked int64      `json:"quality_checked"`
	Expires        *time.Time `json:"expires_at"`
	Fallback       string     `json:"fallback_mode"`
	BackupProxyID  *int64     `json:"backup_proxy_id"`
	ExpiryWarnDays int        `json:"expiry_warn_days"`
	Created        time.Time  `json:"created_at"`
	Updated        time.Time  `json:"updated_at"`
}

type ProxyView struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	Protocol       string     `json:"protocol"`
	Host           string     `json:"host"`
	Port           int        `json:"port"`
	HasAuth        bool       `json:"has_auth"`
	Status         string     `json:"status"`
	AccountCount   int64      `json:"account_count"`
	Latency        *float64   `json:"latency_ms"`
	LatencyStatus  string     `json:"latency_status"`
	IPAddress      string     `json:"ip_address"`
	Country        string     `json:"country"`
	CountryCode    string     `json:"country_code"`
	Region         string     `json:"region"`
	City           string     `json:"city"`
	QualityStatus  string     `json:"quality_status"`
	QualityScore   *float64   `json:"quality_score"`
	QualityGrade   string     `json:"quality_grade"`
	QualitySummary string     `json:"quality_summary"`
	QualityChecked int64      `json:"quality_checked"`
	Expires        *time.Time `json:"expires_at"`
	Fallback       string     `json:"fallback_mode"`
	BackupProxyID  *int64     `json:"backup_proxy_id"`
	ExpiryWarnDays int        `json:"expiry_warn_days"`
	Created        time.Time  `json:"created_at"`
	Updated        time.Time  `json:"updated_at"`
}

func ProjectProxy(proxy ProxyWire) (ProxyView, error) {
	if proxy.ID < 1 || proxy.Name == "" || proxy.Host == "" || proxy.Port < 1 || proxy.Port > 65535 ||
		proxy.AccountCount < 0 || proxy.ExpiryWarnDays < 0 ||
		proxy.Protocol != "http" && proxy.Protocol != "https" && proxy.Protocol != "socks5" && proxy.Protocol != "socks5h" ||
		proxy.Status != "active" && proxy.Status != "inactive" && proxy.Status != "expired" {
		return ProxyView{}, platform.Fail(502, "invalid_upstream")
	}
	return ProxyView{
		ID: proxy.ID, Name: proxy.Name, Protocol: proxy.Protocol, Host: proxy.Host, Port: proxy.Port,
		HasAuth: proxy.Username != nil && *proxy.Username != "" || proxy.Password != nil && *proxy.Password != "",
		Status:  proxy.Status, AccountCount: proxy.AccountCount, Latency: proxy.Latency, LatencyStatus: proxy.LatencyStatus,
		IPAddress: proxy.IPAddress, Country: proxy.Country, CountryCode: proxy.CountryCode, Region: proxy.Region, City: proxy.City,
		QualityStatus: proxy.QualityStatus, QualityScore: proxy.QualityScore, QualityGrade: proxy.QualityGrade,
		QualitySummary: proxy.QualitySummary, QualityChecked: proxy.QualityChecked, Expires: proxy.Expires,
		Fallback: proxy.Fallback, BackupProxyID: proxy.BackupProxyID, ExpiryWarnDays: proxy.ExpiryWarnDays,
		Created: proxy.Created, Updated: proxy.Updated,
	}, nil
}

type proxyInput struct {
	Name           string  `json:"name,omitempty"`
	Protocol       string  `json:"protocol,omitempty"`
	Host           string  `json:"host,omitempty"`
	Port           int     `json:"port,omitempty"`
	Username       *string `json:"username,omitempty"`
	Password       *string `json:"password,omitempty"`
	Status         string  `json:"status,omitempty"`
	Expires        *int64  `json:"expires_at,omitempty"`
	Fallback       string  `json:"fallback_mode,omitempty"`
	BackupProxyID  *int64  `json:"backup_proxy_id,omitempty"`
	ExpiryWarnDays *int    `json:"expiry_warn_days,omitempty"`
	ClearExpires   bool    `json:"clear_expires,omitempty"`
	ClearBackup    bool    `json:"clear_backup_proxy_id,omitempty"`
	ClearAuth      bool    `json:"clear_auth,omitempty"`
}

func (input proxyInput) valid(create bool) bool {
	if create && (strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Host) == "" || input.Port < 1 || input.Port > 65535 ||
		input.Protocol != "http" && input.Protocol != "https" && input.Protocol != "socks5" && input.Protocol != "socks5h") {
		return false
	}
	if input.Name != "" && (strings.TrimSpace(input.Name) == "" || len(input.Name) > 100) ||
		input.Host != "" && len(input.Host) > 255 || input.Port < 0 || input.Port > 65535 ||
		input.Protocol != "" && input.Protocol != "http" && input.Protocol != "https" && input.Protocol != "socks5" && input.Protocol != "socks5h" ||
		input.Status != "" && input.Status != "active" && input.Status != "inactive" ||
		input.Fallback != "" && input.Fallback != "none" && input.Fallback != "proxy" && input.Fallback != "direct" ||
		input.BackupProxyID != nil && *input.BackupProxyID < 1 || input.Expires != nil && *input.Expires < 0 ||
		input.ExpiryWarnDays != nil && *input.ExpiryWarnDays < 0 || input.Username != nil && len(*input.Username) > 255 || input.Password != nil && len(*input.Password) > 1024 {
		return false
	}
	return true
}

func (h *handler) registerProxyRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/admin/proxies", true, h.adminProxies)
	h.s.Route(mux, "GET /api/admin/proxies/{id}", true, h.adminProxy)
	h.s.Route(mux, "POST /api/admin/proxies", true, h.createProxy)
	h.s.Route(mux, "PUT /api/admin/proxies/{id}", true, h.updateProxy)
	h.s.Route(mux, "DELETE /api/admin/proxies/{id}", true, h.deleteProxy)
	h.s.Route(mux, "POST /api/admin/proxies/{id}/test", true, h.testProxy)
	h.s.Route(mux, "POST /api/admin/proxies/{id}/quality-check", true, h.checkProxyQuality)
	h.s.Route(mux, "GET /api/admin/proxies/{id}/stats", true, h.proxyStats)
	h.s.Route(mux, "GET /api/admin/proxies/{id}/accounts", true, h.proxyAccounts)
	h.s.Route(mux, "POST /api/admin/proxies/batch-delete", true, h.batchDeleteProxies)
	h.s.Route(mux, "POST /api/admin/proxies/batch", true, h.batchCreateProxies)
	h.s.Route(mux, "POST /api/admin/proxies/import", true, h.importProxies)
	h.s.Route(mux, "GET /api/admin/proxies/export", true, h.exportProxies)
}

func (h *handler) adminProxies(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	page, params, err := platform.ListQuery(r, "search", "protocol", "status", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/admin/proxies", page, params, ProjectProxy)
	return platform.Send(w, 200, rows, err)
}

func (h *handler) adminProxy(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	proxy, err := platform.Upstream[ProxyWire](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/proxies/%d", id), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if proxy.ID != id {
		return platform.Fail(502, "invalid_upstream")
	}
	view, err := ProjectProxy(proxy)
	return platform.Send(w, 200, view, err)
}

func (h *handler) createProxy(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[proxyInput](r)
	if err != nil || !input.valid(true) {
		return platform.Fail(400, "invalid_input")
	}
	proxy, err := platform.Upstream[ProxyWire](r.Context(), h.s.Upstream, "POST", "/admin/proxies", a.Tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := ProjectProxy(proxy)
	return platform.Send(w, 201, view, err)
}

func (h *handler) updateProxy(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[proxyInput](r)
	if err != nil || !input.valid(false) {
		return platform.Fail(400, "invalid_input")
	}
	body := map[string]any{}
	if input.Name != "" {
		body["name"] = input.Name
	}
	if input.Protocol != "" {
		body["protocol"] = input.Protocol
	}
	if input.Host != "" {
		body["host"] = input.Host
	}
	if input.Port != 0 {
		body["port"] = input.Port
	}
	if input.Username != nil {
		body["username"] = *input.Username
	}
	if input.Password != nil {
		body["password"] = *input.Password
	}
	if input.Status != "" {
		body["status"] = input.Status
	}
	if input.Expires != nil {
		body["expires_at"] = *input.Expires
	} else if input.ClearExpires {
		body["expires_at"] = nil
	}
	if input.Fallback != "" {
		body["fallback_mode"] = input.Fallback
	}
	if input.BackupProxyID != nil {
		body["backup_proxy_id"] = *input.BackupProxyID
	} else if input.ClearBackup {
		body["backup_proxy_id"] = nil
	}
	if input.ExpiryWarnDays != nil {
		body["expiry_warn_days"] = *input.ExpiryWarnDays
	}
	if input.ClearAuth {
		body["username"] = ""
		body["password"] = ""
	}
	proxy, err := platform.Upstream[ProxyWire](r.Context(), h.s.Upstream, "PUT", fmt.Sprintf("/admin/proxies/%d", id), a.Tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	if proxy.ID != id {
		return platform.Fail(502, "invalid_upstream")
	}
	view, err := ProjectProxy(proxy)
	return platform.Send(w, 200, view, err)
}

func (h *handler) deleteProxy(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "DELETE", fmt.Sprintf("/admin/proxies/%d", id), a.Tokens.Access, "", "", nil)
	return platform.Send(w, 200, struct{}{}, err)
}

type proxyTestView struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Latency   *int64 `json:"latency_ms,omitempty"`
	IPAddress string `json:"ip_address,omitempty"`
	City      string `json:"city,omitempty"`
	Region    string `json:"region,omitempty"`
	Country   string `json:"country,omitempty"`
	Code      string `json:"country_code,omitempty"`
}

func (h *handler) testProxy(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	result, err := platform.Upstream[proxyTestView](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/proxies/%d/test", id), a.Tokens.Access, "", "", nil)
	if result.Latency != nil && *result.Latency < 0 {
		return platform.Fail(502, "invalid_upstream")
	}
	return platform.Send(w, 200, result, err)
}

type proxyQualityView struct {
	ProxyID     int64   `json:"proxy_id"`
	Score       float64 `json:"score"`
	Grade       string  `json:"grade"`
	Summary     string  `json:"summary"`
	ExitIP      string  `json:"exit_ip,omitempty"`
	Country     string  `json:"country,omitempty"`
	CountryCode string  `json:"country_code,omitempty"`
	Passed      int     `json:"passed_count"`
	Warnings    int     `json:"warn_count"`
	Failed      int     `json:"failed_count"`
	Message     string  `json:"message,omitempty"`
}

func (h *handler) checkProxyQuality(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	result, err := platform.Upstream[proxyQualityView](r.Context(), h.s.Upstream, "POST", fmt.Sprintf("/admin/proxies/%d/quality-check", id), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if result.ProxyID != id || result.Score < 0 || result.Score > 100 || result.Passed < 0 || result.Warnings < 0 || result.Failed < 0 {
		return platform.Fail(502, "invalid_upstream")
	}
	return platform.Send(w, 200, result, nil)
}

func (h *handler) proxyStats(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	result, err := platform.Upstream[struct {
		Accounts       int64   `json:"total_accounts"`
		ActiveAccounts int64   `json:"active_accounts"`
		Requests       int64   `json:"total_requests"`
		SuccessRate    float64 `json:"success_rate"`
		Latency        float64 `json:"average_latency"`
	}](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/proxies/%d/stats", id), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if result.Accounts < 0 || result.ActiveAccounts < 0 || result.Requests < 0 || result.SuccessRate < 0 || result.SuccessRate > 100 || result.Latency < 0 {
		return platform.Fail(502, "invalid_upstream")
	}
	return platform.Send(w, 200, result, nil)
}

func (h *handler) proxyAccounts(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	rows, err := platform.Upstream[[]struct {
		ID       int64   `json:"id"`
		Name     string  `json:"name"`
		Platform string  `json:"platform"`
		Type     string  `json:"type"`
		Notes    *string `json:"notes"`
	}](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/proxies/%d/accounts", id), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.ID < 1 || row.Name == "" {
			return platform.Fail(502, "invalid_upstream")
		}
	}
	return platform.Send(w, 200, rows, nil)
}

func (h *handler) batchDeleteProxies(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		IDs []int64 `json:"ids"`
	}](r)
	if err != nil || len(input.IDs) == 0 || len(input.IDs) > 100 {
		return platform.Fail(400, "invalid_input")
	}
	for _, id := range input.IDs {
		if id < 1 {
			return platform.Fail(400, "invalid_input")
		}
	}
	result, err := platform.Upstream[struct {
		Deleted []int64 `json:"deleted_ids"`
		Skipped []struct {
			ID     int64  `json:"id"`
			Reason string `json:"reason"`
		} `json:"skipped"`
	}](r.Context(), h.s.Upstream, "POST", "/admin/proxies/batch-delete", a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, result, err)
}

func (h *handler) batchCreateProxies(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	input, err := platform.DecodeBody[struct {
		Proxies []proxyInput `json:"proxies"`
	}](r)
	if err != nil || len(input.Proxies) == 0 || len(input.Proxies) > 100 {
		return platform.Fail(400, "invalid_input")
	}
	for _, proxy := range input.Proxies {
		if !proxy.valid(true) {
			return platform.Fail(400, "invalid_input")
		}
	}
	result, err := platform.Upstream[struct {
		Created int `json:"created"`
		Skipped int `json:"skipped"`
	}](r.Context(), h.s.Upstream, "POST", "/admin/proxies/batch", a.Tokens.Access, "", "", input)
	return platform.Send(w, 200, result, err)
}

func (h *handler) importProxies(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	var input struct {
		Proxies []proxyInput `json:"proxies"`
	}
	if err := platform.DecodeBodyInto(r, &input); err != nil || len(input.Proxies) == 0 || len(input.Proxies) > 100 {
		return platform.Fail(400, "invalid_input")
	}
	for _, proxy := range input.Proxies {
		if !proxy.valid(true) {
			return platform.Fail(400, "invalid_input")
		}
	}
	created, skipped := 0, 0
	for _, proxy := range input.Proxies {
		_, err := platform.Upstream[ProxyWire](r.Context(), h.s.Upstream, "POST", "/admin/proxies", a.Tokens.Access, "", "", proxy)
		if err != nil {
			skipped++
			continue
		}
		created++
	}
	platform.WriteJSON(w, 200, struct {
		Created int `json:"created"`
		Skipped int `json:"skipped"`
	}{created, skipped})
	return nil
}

func (h *handler) exportProxies(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	_, params, err := platform.ListQuery(r, "search", "protocol", "status", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	all := make([]ProxyView, 0)
	for page := 1; page <= 100; page++ {
		rows, err := core.QueryPage(r.Context(), h.s.Upstream, a.Tokens.Access, "/admin/proxies", page, params, ProjectProxy)
		if err != nil {
			return err
		}
		all = append(all, rows.Items...)
		if !rows.More {
			platform.WriteJSON(w, 200, struct {
				ExportedAt time.Time   `json:"exported_at"`
				Proxies    []ProxyView `json:"proxies"`
			}{time.Now().UTC(), all})
			return nil
		}
	}
	return platform.Fail(413, "invalid_input")
}
