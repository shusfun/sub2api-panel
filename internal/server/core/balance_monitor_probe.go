package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

// 站点识别与保存前试采样。沿用 BalanceMonitor 的 HTTP 客户端，因此公网限制、https 要求与禁止重定向保持不变。

// SiteProbe 是对一个站点首页地址的识别结果。Type 为 sub2api / newapi / oneapi，识别不出时为空。
type SiteProbe struct {
	Type         string
	Origin       string
	QuotaPerUnit *float64
}

func (m *BalanceMonitor) probeGet(ctx context.Context, endpoint string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := m.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return res.StatusCode, body, err
}

// ProbeSite 只请求无需凭据的公开接口：
// Sub2API 看 /api/v1/settings/public 是否返回 code=0；
// New API 与 One API 都有 /api/status（success=true），再用 /api/usage/token/ 区分，只有 New API 有这条路由。
// 识别不出时 Type 为空，调用方返回站点首页。
func (m *BalanceMonitor) ProbeSite(ctx context.Context, raw string) (SiteProbe, error) {
	base, err := balanceBaseURL(raw)
	if err != nil {
		return SiteProbe{}, err
	}
	result := SiteProbe{Origin: (&url.URL{Scheme: base.Scheme, Host: base.Host}).String()}
	if status, body, err := m.probeGet(ctx, balanceJoin(base, "/api/v1/settings/public")); err == nil && status == http.StatusOK {
		var wire struct {
			Code *int           `json:"code"`
			Data map[string]any `json:"data"`
		}
		if json.Unmarshal(body, &wire) == nil && wire.Code != nil && *wire.Code == 0 && wire.Data != nil {
			result.Type = "sub2api"
			return result, nil
		}
	}
	status, body, err := m.probeGet(ctx, balanceJoin(base, "/api/status"))
	if err != nil || status != http.StatusOK {
		return result, nil
	}
	var wire struct {
		Success *bool `json:"success"`
		Data    struct {
			QuotaPerUnit *float64 `json:"quota_per_unit"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &wire) != nil || wire.Success == nil || !*wire.Success {
		return result, nil
	}
	if v := wire.Data.QuotaPerUnit; v != nil && *v > 0 && *v < 1e12 {
		result.QuotaPerUnit = v
	}
	result.Type = "oneapi"
	if tokenStatus, _, err := m.probeGet(ctx, balanceJoin(base, "/api/usage/token/")); err == nil && tokenStatus != http.StatusNotFound {
		result.Type = "newapi"
	}
	return result, nil
}

// TrialFetch 用待保存的配置试采样一次，不写任何状态；保存前用它确认配置可用。
func (m *BalanceMonitor) TrialFetch(ctx context.Context, config BalanceConfig) (AccountBalanceView, error) {
	return m.fetch(ctx, config)
}
