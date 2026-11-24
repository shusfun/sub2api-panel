package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/runtimesetting"
)

const balanceSettingPrefix = "account_balance/"
const balanceConfigLimit = 500

var nonPublicBalanceNetworks = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("2002::/16"),
}

type balanceConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	Key      string `json:"api_key"`
}

type balanceSample struct {
	value      *accountBalanceView
	next       time.Time
	failures   int
	running    bool
	generation string
}

type balanceMonitor struct {
	mu     sync.Mutex
	states map[int64]balanceSample
	slots  chan struct{}
	http   *http.Client
}

func newBalanceMonitor() *balanceMonitor {
	transport := &http.Transport{DialContext: dialBalanceHost, DisableKeepAlives: true}
	return &balanceMonitor{states: make(map[int64]balanceSample), slots: make(chan struct{}, 4), http: &http.Client{
		Transport: transport, Timeout: 8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func dialBalanceHost(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: 4 * time.Second}
	for _, ip := range ips {
		ip = ip.Unmap()
		if !publicBalanceIP(ip) {
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("balance upstream unavailable")
}

func publicBalanceIP(ip netip.Addr) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	for _, prefix := range nonPublicBalanceNetworks {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func balanceEndpoint(config balanceConfig) (string, error) {
	switch config.Provider {
	case "deepseek":
		return "https://api.deepseek.com/user/balance", nil
	case "kimi":
		return "https://api.moonshot.cn/v1/users/me/balance", nil
	case "sub2api":
		u, err := url.Parse(strings.TrimSpace(config.BaseURL))
		if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery || (u.Port() != "" && u.Port() != "443") || u.RawPath != "" {
			return "", fail(400, "invalid_input")
		}
		if ip, err := netip.ParseAddr(u.Hostname()); err == nil && !publicBalanceIP(ip.Unmap()) {
			return "", fail(400, "invalid_input")
		}
		u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), "/v1") + "/v1/usage"
		return u.String(), nil
	default:
		return "", fail(400, "invalid_input")
	}
}

func (p *panel) balanceCipher() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte("panel upstream balances\x00" + p.secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithRandomNonce(block)
}

func (p *panel) decryptBalanceConfig(name, ciphertext string) (balanceConfig, error) {
	var config balanceConfig
	aead, err := p.balanceCipher()
	if err != nil {
		return config, err
	}
	raw, err := base64.RawStdEncoding.DecodeString(ciphertext)
	if err != nil {
		return config, err
	}
	plaintext, err := aead.Open(nil, nil, raw, []byte(name))
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(plaintext, &config)
	return config, err
}

func (p *panel) findBalanceConfig(ctx context.Context, id int64) (*ent.RuntimeSetting, balanceConfig, error) {
	row, err := p.database.RuntimeSetting.Query().Where(runtimesetting.KeyEQ(balanceSettingPrefix + strconv.FormatInt(id, 10))).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, balanceConfig{}, nil
	}
	if err != nil {
		return nil, balanceConfig{}, err
	}
	config, err := p.decryptBalanceConfig(row.Key, row.Value)
	return row, config, err
}

func (p *panel) saveBalanceConfig(ctx context.Context, id int64, row *ent.RuntimeSetting, config balanceConfig) error {
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	aead, err := p.balanceCipher()
	if err != nil {
		return err
	}
	name := balanceSettingPrefix + strconv.FormatInt(id, 10)
	ciphertext := base64.RawStdEncoding.EncodeToString(aead.Seal(nil, nil, raw, []byte(name)))
	if row == nil {
		count, err := p.database.RuntimeSetting.Query().Where(runtimesetting.KeyHasPrefix(balanceSettingPrefix)).Count(ctx)
		if err != nil {
			return err
		}
		if count >= balanceConfigLimit {
			return fail(409, "invalid_input")
		}
		_, err = p.database.RuntimeSetting.Create().SetKey(name).SetValue(ciphertext).SetUpdated(time.Now().Unix()).Save(ctx)
		return err
	}
	_, err = p.database.RuntimeSetting.UpdateOne(row).SetValue(ciphertext).SetUpdated(time.Now().Unix()).Save(ctx)
	return err
}

func (p *panel) startBalanceMonitor(ctx context.Context) {
	p.balances = newBalanceMonitor()
	if p.database == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			p.sampleDueBalances(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (p *panel) sampleDueBalances(ctx context.Context) {
	ctx, end := backgroundContext(ctx, "轮询上游账号余额")
	defer end()
	rows, err := p.database.RuntimeSetting.Query().Where(runtimesetting.KeyHasPrefix(balanceSettingPrefix)).Limit(balanceConfigLimit).All(ctx)
	if err != nil {
		return
	}
	for _, row := range rows {
		id, err := strconv.ParseInt(strings.TrimPrefix(row.Key, balanceSettingPrefix), 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		config, err := p.decryptBalanceConfig(row.Key, row.Value)
		if err == nil && config.Enabled {
			p.balances.sampleIfDue(ctx, id, row.Value, config, false)
		}
	}
}

func (m *balanceMonitor) sampleIfDue(ctx context.Context, id int64, generation string, config balanceConfig, manual bool) <-chan error {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.states[id]
	if state.running || (!manual && state.generation == generation && time.Now().Before(state.next)) {
		return nil
	}
	select {
	case m.slots <- struct{}{}:
	default:
		return nil
	}
	if state.generation != generation {
		state = balanceSample{generation: generation}
	}
	state.running = true
	m.states[id] = state
	done := make(chan error, 1)
	go func() {
		defer func() { <-m.slots }()
		value, err := m.fetch(ctx, config)
		if err == nil && config.Provider == "sub2api" {
			value.Billing = m.fetchBilling(ctx, config)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		latest := m.states[id]
		if latest.generation != generation {
			done <- fail(409, "invalid_input")
			return
		}
		latest.running = false
		if err == nil {
			if value.Billing == nil && latest.value != nil && latest.value.Billing != nil {
				billing := *latest.value.Billing
				billing.Stale = true
				value.Billing = &billing
			}
			latest.value = &value
			latest.next = time.Now().Add(30 * time.Minute)
			latest.failures = 0
		} else {
			latest.failures = min(latest.failures+1, 4)
			latest.next = time.Now().Add(time.Minute * time.Duration(1<<latest.failures))
			if latest.value != nil {
				copy := *latest.value
				copy.Stale = true
				latest.value = &copy
			}
		}
		m.states[id] = latest
		done <- err
	}()
	return done
}

func (m *balanceMonitor) fetch(ctx context.Context, config balanceConfig) (accountBalanceView, error) {
	endpoint, err := balanceEndpoint(config)
	if err != nil {
		return accountBalanceView{}, err
	}
	body, err := m.open(ctx, endpoint, config.Key)
	if err != nil {
		return accountBalanceView{}, err
	}
	defer body.Close()
	value, err := readBalanceResponse(io.LimitReader(body, 1<<20), config.Provider)
	if err != nil {
		return accountBalanceView{}, err
	}
	now := time.Now().Unix()
	value.FetchedAt = &now
	return value, nil
}

func (m *balanceMonitor) open(ctx context.Context, endpoint, key string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := m.http.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fail(http.StatusBadGateway, "balance_unavailable")
	}
	return res.Body, nil
}

func (m *balanceMonitor) fetchBilling(ctx context.Context, config balanceConfig) *accountBillingView {
	endpoint, err := balanceEndpoint(config)
	if err != nil {
		return nil
	}
	body, err := m.open(ctx, strings.TrimSuffix(endpoint, "/v1/usage")+"/v1/sub2api/billing", config.Key)
	if err != nil {
		return nil
	}
	defer body.Close()
	var billing struct {
		Object    string    `json:"object"`
		Version   int       `json:"schema_version"`
		Scope     string    `json:"billing_scope"`
		Effective *float64  `json:"effective_rate_multiplier"`
		Observed  time.Time `json:"observed_at"`
	}
	if json.NewDecoder(io.LimitReader(body, 65536)).Decode(&billing) != nil ||
		billing.Object != "sub2api.key_billing" || billing.Version != 1 || billing.Scope != "token" ||
		billing.Effective == nil || *billing.Effective < 0 || billing.Observed.IsZero() {
		return nil
	}
	return &accountBillingView{Effective: *billing.Effective, Observed: billing.Observed}
}

func readBalanceResponse(body io.Reader, provider string) (accountBalanceView, error) {
	var wire struct {
		Mode      string   `json:"mode"`
		Unit      string   `json:"unit"`
		Plan      string   `json:"planName"`
		Balance   *float64 `json:"balance"`
		Remaining *float64 `json:"remaining"`
		Available *bool    `json:"is_available"`
		Data      struct {
			Available *float64 `json:"available_balance"`
		} `json:"data"`
		Balances []struct {
			Currency string `json:"currency"`
			Total    string `json:"total_balance"`
		} `json:"balance_infos"`
	}
	if json.NewDecoder(body).Decode(&wire) != nil {
		return accountBalanceView{}, fail(502, "invalid_upstream")
	}
	result := accountBalanceView{Currency: "USD", Kind: "account", Available: wire.Available}
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
				result.Balances = append(result.Balances, accountBalanceItem{Currency: item.Currency, Balance: value})
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
	}
	if amount == nil || !(*amount > -1e12 && *amount < 1e12) {
		return accountBalanceView{}, fail(502, "balance_unavailable")
	}
	result.Balance = *amount
	return result, nil
}

func (p *panel) applyBalanceSnapshots(items []accountAdminView) {
	if p.balances == nil {
		return
	}
	p.balances.mu.Lock()
	defer p.balances.mu.Unlock()
	for i := range items {
		if sample := p.balances.states[items[i].ID]; sample.value != nil {
			items[i].Balance = sample.value
		}
	}
}

func (p *panel) accountBalanceSettings(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if p.database == nil || p.balances == nil {
		return fail(503, "service_unavailable")
	}
	raw, err := upstream[accountDetailWire](r.Context(), p.upstream, "GET", fmt.Sprintf("/admin/accounts/%d", id), a.tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if raw.ID != id || raw.Type != "apikey" {
		return fail(400, "invalid_input")
	}
	row, config, err := p.findBalanceConfig(r.Context(), id)
	if err != nil {
		return err
	}
	switch r.Method {
	case "PUT":
		next, err := decodeBody[balanceConfig](r)
		if err != nil {
			return err
		}
		next.BaseURL = strings.TrimSpace(next.BaseURL)
		next.Key = strings.TrimSpace(next.Key)
		if next.Provider != "sub2api" && (next.Provider != raw.Platform || (next.Provider != "deepseek" && next.Provider != "kimi")) {
			return fail(400, "invalid_input")
		}
		if next.Enabled || next.BaseURL != "" || config.Key != "" {
			if _, err := balanceEndpoint(next); err != nil {
				return err
			}
		}
		if next.Key == "" && next.Provider == config.Provider && next.BaseURL == config.BaseURL {
			next.Key = config.Key
		}
		if (next.Enabled && next.Key == "") || len(next.Key) > 4096 || strings.ContainsAny(next.Key, "\r\n") {
			return fail(400, "invalid_input")
		}
		p.balances.mu.Lock()
		defer p.balances.mu.Unlock()
		current, _, err := p.findBalanceConfig(r.Context(), id)
		if err != nil {
			return err
		}
		if (current == nil) != (row == nil) || (current != nil && current.Value != row.Value) {
			return fail(409, "invalid_input")
		}
		if err := p.saveBalanceConfig(r.Context(), id, row, next); err != nil {
			return err
		}
		delete(p.balances.states, id)
		config = next
	case "POST":
		if row == nil || config.Key == "" {
			return fail(400, "invalid_input")
		}
		completed := p.balances.sampleIfDue(r.Context(), id, row.Value, config, true)
		if completed == nil {
			return fail(429, "rate_limited")
		}
		select {
		case err := <-completed:
			if err != nil {
				return fail(502, "balance_unavailable")
			}
		case <-r.Context().Done():
			return r.Context().Err()
		}
	}
	writeJSON(w, 200, struct {
		Enabled    bool   `json:"enabled"`
		Provider   string `json:"provider"`
		Base       string `json:"base_url"`
		Configured bool   `json:"configured"`
	}{config.Enabled, config.Provider, config.BaseURL, config.Key != ""})
	return nil
}
