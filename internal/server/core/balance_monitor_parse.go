package core

import (
	"encoding/json"
	"io"
	"strconv"

	"sub2api-panel/internal/platform"
)

// ReadQuotaResponse 解析 New API / One API 的额度响应并把额度折算成美元。
// wallet=true 读 /api/user/self（data.quota、data.used_quota）；
// wallet=false 读 /api/usage/token/（data.user_quota.total_available 优先，其次 data.total_available；data.total_used）。
func ReadQuotaResponse(body io.Reader, wallet bool, perUnit float64) (AccountBalanceView, error) {
	var wire struct {
		Success *bool `json:"success"`
		Code    *bool `json:"code"`
		Data    struct {
			Quota          *float64 `json:"quota"`
			UsedQuota      *float64 `json:"used_quota"`
			TotalUsed      *float64 `json:"total_used"`
			TotalAvailable *float64 `json:"total_available"`
			UserQuota      struct {
				TotalAvailable *float64 `json:"total_available"`
			} `json:"user_quota"`
		} `json:"data"`
	}
	if json.NewDecoder(body).Decode(&wire) != nil {
		return AccountBalanceView{}, platform.Fail(502, "invalid_upstream")
	}
	if (wire.Success != nil && !*wire.Success) || (wire.Code != nil && !*wire.Code) || !(perUnit > 0) {
		return AccountBalanceView{}, platform.Fail(502, "balance_unavailable")
	}
	usd := func(v *float64) *float64 {
		if v == nil || !(*v > -1e15 && *v < 1e15) {
			return nil
		}
		out := *v / perUnit
		return &out
	}
	result := AccountBalanceView{Currency: "USD", Kind: "account", Sample: &BalanceSample{QuotaPerUSD: perUnit}}
	var amount *float64
	if wallet {
		amount = usd(wire.Data.Quota)
		result.Sample.WalletUsedUSD = usd(wire.Data.UsedQuota)
	} else {
		available := wire.Data.UserQuota.TotalAvailable
		if available == nil {
			available = wire.Data.TotalAvailable
		}
		amount = usd(available)
		result.Sample.KeyUsedUSD = usd(wire.Data.TotalUsed)
	}
	if amount == nil || !(*amount > -1e12 && *amount < 1e12) {
		return AccountBalanceView{}, platform.Fail(502, "balance_unavailable")
	}
	result.Balance = *amount
	return result, nil
}

func ReadBalanceResponse(body io.Reader, provider string) (AccountBalanceView, error) {
	var wire struct {
		Mode      string   `json:"mode"`
		Unit      string   `json:"unit"`
		Plan      string   `json:"planName"`
		Balance   *float64 `json:"balance"`
		Remaining *float64 `json:"remaining"`
		Available *bool    `json:"is_available"`
		Usage     struct {
			Total struct {
				ActualCost *float64 `json:"actual_cost"`
			} `json:"total"`
		} `json:"usage"`
		Quota struct {
			Limit *float64 `json:"limit"`
		} `json:"quota"`
		Data struct {
			Available *float64 `json:"available_balance"`
		} `json:"data"`
		Balances []struct {
			Currency string `json:"currency"`
			Total    string `json:"total_balance"`
		} `json:"balance_infos"`
	}
	if json.NewDecoder(body).Decode(&wire) != nil {
		return AccountBalanceView{}, platform.Fail(502, "invalid_upstream")
	}
	result := AccountBalanceView{Currency: "USD", Kind: "account", Available: wire.Available}
	var amount *float64
	switch provider {
	case "kimi":
		result.Currency, amount = "CNY", wire.Data.Available
	case "deepseek":
		for _, item := range wire.Balances {
			if item.Currency != "CNY" && item.Currency != "USD" {
				continue
			}
			value, err := strconv.ParseFloat(item.Total, 64)
			if err == nil && value >= 0 && value < 1e12 {
				result.Balances = append(result.Balances, AccountBalanceItem{Currency: item.Currency, Balance: value})
			}
		}
		if len(result.Balances) != 0 {
			result.Currency, amount = result.Balances[0].Currency, &result.Balances[0].Balance
		}
	case "sub2api":
		switch {
		case wire.Mode == "quota_limited":
			result.Kind, amount = "key_quota", wire.Remaining
		case wire.Mode == "unrestricted" && wire.Balance != nil:
			amount = wire.Balance
		case wire.Mode == "unrestricted" && wire.Plan != "":
			result.Kind, amount = "subscription", wire.Remaining
		}
		sample := &BalanceSample{}
		if v := wire.Usage.Total.ActualCost; v != nil && *v >= 0 && *v < 1e12 {
			sample.KeyUsedUSD = v
		}
		if v := wire.Quota.Limit; v != nil && *v > 0 && *v < 1e12 {
			sample.QuotaLimitUSD = v
		}
		result.Sample = sample
	}
	if amount == nil || !(*amount > -1e12 && *amount < 1e12) {
		return AccountBalanceView{}, platform.Fail(502, "balance_unavailable")
	}
	result.Balance = *amount
	return result, nil
}
