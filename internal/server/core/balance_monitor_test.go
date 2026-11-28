package core

import (
	"math"
	"strings"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func ptrEq(p *float64, want float64) bool { return p != nil && near(*p, want) }

func TestReadQuotaResponse(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wallet   bool
		perUnit  float64
		wantErr  bool
		balance  float64
		keyUsed  *float64
		walletUs *float64
	}{
		{"钱包额度换算", `{"success":true,"data":{"quota":1000000,"used_quota":250000}}`, true, 500000, false, 2, nil, ptr(0.5)},
		{"Key 级优先取 user_quota", `{"data":{"user_quota":{"total_available":1500000},"total_available":1,"total_used":500000}}`, false, 500000, false, 3, ptr(1), nil},
		{"Key 级回退 total_available", `{"data":{"total_available":500000,"total_used":0}}`, false, 500000, false, 1, ptr(0), nil},
		{"自定义换算单位", `{"data":{"quota":100,"used_quota":10}}`, true, 100, false, 1, nil, ptr(0.1)},
		{"success 为 false", `{"success":false,"data":{"quota":1}}`, true, 500000, true, 0, nil, nil},
		{"缺额度字段", `{"success":true,"data":{}}`, true, 500000, true, 0, nil, nil},
		{"换算单位非法", `{"data":{"quota":1}}`, true, 0, true, 0, nil, nil},
		{"非 JSON", `<html>`, true, 500000, true, 0, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			view, err := ReadQuotaResponse(strings.NewReader(c.body), c.wallet, c.perUnit)
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if view.Currency != "USD" || !near(view.Balance, c.balance) || view.Sample == nil {
				t.Fatalf("view=%+v", view)
			}
			if (c.keyUsed == nil) != (view.Sample.KeyUsedUSD == nil) || (c.keyUsed != nil && !ptrEq(view.Sample.KeyUsedUSD, *c.keyUsed)) {
				t.Fatalf("key used %v want %v", view.Sample.KeyUsedUSD, c.keyUsed)
			}
			if (c.walletUs == nil) != (view.Sample.WalletUsedUSD == nil) || (c.walletUs != nil && !ptrEq(view.Sample.WalletUsedUSD, *c.walletUs)) {
				t.Fatalf("wallet used %v want %v", view.Sample.WalletUsedUSD, c.walletUs)
			}
			if !near(view.Sample.QuotaPerUSD, c.perUnit) {
				t.Fatalf("per unit %v", view.Sample.QuotaPerUSD)
			}
		})
	}
}

func ptr(v float64) *float64 { return &v }

func TestReadBalanceResponseSub2API(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr bool
		kind    string
		balance float64
		used    *float64
		limit   *float64
	}{
		{"钱包余额与累计已用", `{"mode":"unrestricted","balance":12.5,"usage":{"total":{"actual_cost":3.25}}}`, false, "account", 12.5, ptr(3.25), nil},
		{"Key 额度上限", `{"mode":"quota_limited","remaining":4,"quota":{"limit":10},"usage":{"total":{"actual_cost":6}}}`, false, "key_quota", 4, ptr(6), ptr(10)},
		{"订阅", `{"mode":"unrestricted","planName":"pro","remaining":7}`, false, "subscription", 7, nil, nil},
		{"负累计值丢弃", `{"mode":"unrestricted","balance":1,"usage":{"total":{"actual_cost":-1}}}`, false, "account", 1, nil, nil},
		{"缺余额", `{"mode":"unrestricted"}`, true, "", 0, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			view, err := ReadBalanceResponse(strings.NewReader(c.body), "sub2api")
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if view.Kind != c.kind || !near(view.Balance, c.balance) || view.Sample == nil {
				t.Fatalf("view=%+v", view)
			}
			if (c.used == nil) != (view.Sample.KeyUsedUSD == nil) || (c.used != nil && !ptrEq(view.Sample.KeyUsedUSD, *c.used)) {
				t.Fatalf("used %v want %v", view.Sample.KeyUsedUSD, c.used)
			}
			if (c.limit == nil) != (view.Sample.QuotaLimitUSD == nil) || (c.limit != nil && !ptrEq(view.Sample.QuotaLimitUSD, *c.limit)) {
				t.Fatalf("limit %v want %v", view.Sample.QuotaLimitUSD, c.limit)
			}
		})
	}
}

func TestReadBalanceResponseOtherProviders(t *testing.T) {
	view, err := ReadBalanceResponse(strings.NewReader(`{"data":{"available_balance":8}}`), "kimi")
	if err != nil || view.Currency != "CNY" || !near(view.Balance, 8) || view.Sample != nil {
		t.Fatalf("kimi %+v %v", view, err)
	}
	view, err = ReadBalanceResponse(strings.NewReader(`{"balance_infos":[{"currency":"EUR","total_balance":"9"},{"currency":"CNY","total_balance":"5.5"}]}`), "deepseek")
	if err != nil || view.Currency != "CNY" || !near(view.Balance, 5.5) || len(view.Balances) != 1 {
		t.Fatalf("deepseek %+v %v", view, err)
	}
}

func TestBalanceEndpoint(t *testing.T) {
	cases := []struct {
		name    string
		config  BalanceConfig
		want    string
		wantErr bool
	}{
		{"sub2api", BalanceConfig{Provider: "sub2api", BaseURL: "https://a.example.com/v1"}, "https://a.example.com/v1/usage", false},
		{"newapi 只有 Key", BalanceConfig{Provider: "newapi", BaseURL: "https://n.example.com"}, "https://n.example.com/api/usage/token/", false},
		{"newapi 访问令牌", BalanceConfig{Provider: "newapi", BaseURL: "https://n.example.com/", AccessToken: "t"}, "https://n.example.com/api/user/self", false},
		{"oneapi 访问令牌", BalanceConfig{Provider: "oneapi", BaseURL: "https://o.example.com", AccessToken: "t"}, "https://o.example.com/api/user/self", false},
		{"oneapi 没有令牌用量接口", BalanceConfig{Provider: "oneapi", BaseURL: "https://o.example.com"}, "", true},
		{"要求 https", BalanceConfig{Provider: "newapi", BaseURL: "http://n.example.com"}, "", true},
		{"拒绝内网地址", BalanceConfig{Provider: "newapi", BaseURL: "https://10.0.0.1"}, "", true},
		{"拒绝回环地址", BalanceConfig{Provider: "oneapi", BaseURL: "https://127.0.0.1", AccessToken: "t"}, "", true},
		{"拒绝非默认端口", BalanceConfig{Provider: "newapi", BaseURL: "https://n.example.com:8443"}, "", true},
		{"拒绝凭据与查询串", BalanceConfig{Provider: "newapi", BaseURL: "https://u:p@n.example.com?x=1"}, "", true},
		{"未知 provider", BalanceConfig{Provider: "other", BaseURL: "https://n.example.com"}, "", true},
		{"deepseek 固定地址", BalanceConfig{Provider: "deepseek"}, "https://api.deepseek.com/user/balance", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := BalanceEndpoint(c.config)
			if (err != nil) != c.wantErr || got != c.want {
				t.Fatalf("got (%q,%v) want (%q,wantErr=%v)", got, err, c.want, c.wantErr)
			}
		})
	}
}

func TestSealOpenSecret(t *testing.T) {
	s := &Server{Secret: "unit-test-secret"}
	sealed, err := s.sealSecret("account_balance/1", []byte(`{"api_key":"k"}`))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := s.openSecret("account_balance/1", sealed)
	if err != nil || string(plain) != `{"api_key":"k"}` {
		t.Fatalf("round trip %q %v", plain, err)
	}
	if _, err := s.openSecret("account_balance/2", sealed); err == nil {
		t.Fatal("附加认证数据不同必须解密失败")
	}
	other := &Server{Secret: "another-secret"}
	if _, err := other.openSecret("account_balance/1", sealed); err == nil {
		t.Fatal("密钥不同必须解密失败")
	}
}
