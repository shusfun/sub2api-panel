package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type proxyWire struct {
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

type proxyView struct {
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

func projectProxy(proxy proxyWire) (proxyView, error) {
	if proxy.ID < 1 || proxy.Name == "" || proxy.Host == "" || proxy.Port < 1 || proxy.Port > 65535 ||
		proxy.AccountCount < 0 || proxy.ExpiryWarnDays < 0 ||
		proxy.Protocol != "http" && proxy.Protocol != "https" && proxy.Protocol != "socks5" && proxy.Protocol != "socks5h" ||
		proxy.Status != "active" && proxy.Status != "inactive" && proxy.Status != "expired" {
		return proxyView{}, fail(502, "invalid_upstream")
	}
	return proxyView{
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

func (p *panel) adminProxies(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	page, params, err := listQuery(r, "search", "protocol", "status", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/proxies", page, params, projectProxy)
	return send(w, 200, rows, err)
}

func (p *panel) adminProxy(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	proxy, err := upstream[proxyWire](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/proxies/%d", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if proxy.ID != id {
		return fail(502, "invalid_upstream")
	}
	view, err := projectProxy(proxy)
	return send(w, 200, view, err)
}

func (p *panel) createProxy(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[proxyInput](r)
	if err != nil || !input.valid(true) {
		return fail(400, "invalid_input")
	}
	proxy, err := upstream[proxyWire](r.Context(), p.upstream, "POST", "/admin/proxies", a.tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	view, err := projectProxy(proxy)
	return send(w, 201, view, err)
}

func (p *panel) updateProxy(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	input, err := decodeBody[proxyInput](r)
	if err != nil || !input.valid(false) {
		return fail(400, "invalid_input")
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
	proxy, err := upstream[proxyWire](r.Context(), p.upstream, "PUT", fmt.Sprintf("/admin/proxies/%d", id), a.tokens.Access, "", "", body)
	if err != nil {
		return err
	}
	if proxy.ID != id {
		return fail(502, "invalid_upstream")
	}
	view, err := projectProxy(proxy)
	return send(w, 200, view, err)
}

func (p *panel) deleteProxy(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	_, err = upstream[struct{}](r.Context(), p.upstream, "DELETE", fmt.Sprintf("/admin/proxies/%d", id), a.tokens.Access, "", "", nil)
	return send(w, 200, struct{}{}, err)
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

func (p *panel) testProxy(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	result, err := upstream[proxyTestView](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/proxies/%d/test", id), a.tokens.Access, "", "", nil)
	if result.Latency != nil && *result.Latency < 0 {
		return fail(502, "invalid_upstream")
	}
	return send(w, 200, result, err)
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

func (p *panel) checkProxyQuality(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	result, err := upstream[proxyQualityView](r.Context(), p.upstream, "POST", fmt.Sprintf("/admin/proxies/%d/quality-check", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if result.ProxyID != id || result.Score < 0 || result.Score > 100 || result.Passed < 0 || result.Warnings < 0 || result.Failed < 0 {
		return fail(502, "invalid_upstream")
	}
	return send(w, 200, result, nil)
}

func (p *panel) proxyStats(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	result, err := upstream[struct {
		Accounts       int64   `json:"total_accounts"`
		ActiveAccounts int64   `json:"active_accounts"`
		Requests       int64   `json:"total_requests"`
		SuccessRate    float64 `json:"success_rate"`
		Latency        float64 `json:"average_latency"`
	}](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/proxies/%d/stats", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if result.Accounts < 0 || result.ActiveAccounts < 0 || result.Requests < 0 || result.SuccessRate < 0 || result.SuccessRate > 100 || result.Latency < 0 {
		return fail(502, "invalid_upstream")
	}
	return send(w, 200, result, nil)
}

func (p *panel) proxyAccounts(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	rows, err := upstream[[]struct {
		ID       int64   `json:"id"`
		Name     string  `json:"name"`
		Platform string  `json:"platform"`
		Type     string  `json:"type"`
		Notes    *string `json:"notes"`
	}](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/proxies/%d/accounts", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.ID < 1 || row.Name == "" {
			return fail(502, "invalid_upstream")
		}
	}
	return send(w, 200, rows, nil)
}

func (p *panel) batchDeleteProxies(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		IDs []int64 `json:"ids"`
	}](r)
	if err != nil || len(input.IDs) == 0 || len(input.IDs) > 100 {
		return fail(400, "invalid_input")
	}
	for _, id := range input.IDs {
		if id < 1 {
			return fail(400, "invalid_input")
		}
	}
	result, err := upstream[struct {
		Deleted []int64 `json:"deleted_ids"`
		Skipped []struct {
			ID     int64  `json:"id"`
			Reason string `json:"reason"`
		} `json:"skipped"`
	}](r.Context(), p.upstream, "POST", "/admin/proxies/batch-delete", a.tokens.Access, "", "", input)
	return send(w, 200, result, err)
}

func (p *panel) batchCreateProxies(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	input, err := decodeBody[struct {
		Proxies []proxyInput `json:"proxies"`
	}](r)
	if err != nil || len(input.Proxies) == 0 || len(input.Proxies) > 100 {
		return fail(400, "invalid_input")
	}
	for _, proxy := range input.Proxies {
		if !proxy.valid(true) {
			return fail(400, "invalid_input")
		}
	}
	result, err := upstream[struct {
		Created int `json:"created"`
		Skipped int `json:"skipped"`
	}](r.Context(), p.upstream, "POST", "/admin/proxies/batch", a.tokens.Access, "", "", input)
	return send(w, 200, result, err)
}

func (p *panel) importProxies(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	var input struct {
		Proxies []proxyInput `json:"proxies"`
	}
	if err := decodeBodyInto(r, &input); err != nil || len(input.Proxies) == 0 || len(input.Proxies) > 100 {
		return fail(400, "invalid_input")
	}
	for _, proxy := range input.Proxies {
		if !proxy.valid(true) {
			return fail(400, "invalid_input")
		}
	}
	created, skipped := 0, 0
	for _, proxy := range input.Proxies {
		_, err := upstream[proxyWire](r.Context(), p.upstream, "POST", "/admin/proxies", a.tokens.Access, "", "", proxy)
		if err != nil {
			skipped++
			continue
		}
		created++
	}
	writeJSON(w, 200, struct {
		Created int `json:"created"`
		Skipped int `json:"skipped"`
	}{created, skipped})
	return nil
}

func decodeBodyInto[T any](r *http.Request, output *T) error {
	value, err := decodeBody[T](r)
	if err == nil {
		*output = value
	}
	return err
}

func (p *panel) exportProxies(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	_, params, err := listQuery(r, "search", "protocol", "status", "sort_by", "sort_order")
	if err != nil {
		return err
	}
	all := make([]proxyView, 0)
	for page := 1; page <= 100; page++ {
		rows, err := queryPage(r.Context(), p.upstream, a.tokens.Access, "/admin/proxies", page, params, projectProxy)
		if err != nil {
			return err
		}
		all = append(all, rows.Items...)
		if !rows.More {
			writeJSON(w, 200, struct {
				ExportedAt time.Time   `json:"exported_at"`
				Proxies    []proxyView `json:"proxies"`
			}{time.Now().UTC(), all})
			return nil
		}
	}
	return fail(413, "invalid_input")
}
