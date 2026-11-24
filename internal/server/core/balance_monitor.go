package core

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
	"sub2api-panel/internal/platform"
)

const BalanceSettingPrefix = "account_balance/"
const BalanceConfigLimit = 500

type AccountBalanceItem struct {
	Currency string  `json:"currency"`
	Balance  float64 `json:"balance"`
}

type AccountBalanceView struct {
	Kind      string               `json:"kind,omitempty"`
	Stale     bool                 `json:"stale,omitempty"`
	Currency  string               `json:"currency"`
	Balance   float64              `json:"balance"`
	Available *bool                `json:"available,omitempty"`
	Balances  []AccountBalanceItem `json:"balances,omitempty"`
	FetchedAt *int64               `json:"fetched_at,omitempty"`
	Billing   *AccountBillingView  `json:"billing,omitempty"`
}

type AccountBillingView struct {
	Effective float64   `json:"effective_multiplier"`
	Observed  time.Time `json:"observed_at"`
	Stale     bool      `json:"stale,omitempty"`
}

var nonPublicBalanceNetworks = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("2002::/16"),
}

type BalanceConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	Key      string `json:"api_key"`
}

type balanceSample struct {
	value      *AccountBalanceView
	next       time.Time
	failures   int
	running    bool
	generation string
}

type BalanceMonitor struct {
	mu     sync.Mutex
	states map[int64]balanceSample
	slots  chan struct{}
	http   *http.Client
}

func NewBalanceMonitor() *BalanceMonitor {
	transport := &http.Transport{DialContext: dialBalanceHost, DisableKeepAlives: true}
	return &BalanceMonitor{
		states: make(map[int64]balanceSample),
		slots:  make(chan struct{}, 4),
		http: &http.Client{
			Transport: transport,
			Timeout:   8 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
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

func BalanceEndpoint(config BalanceConfig) (string, error) {
	switch config.Provider {
	case "deepseek":
		return "https://api.deepseek.com/user/balance", nil
	case "kimi":
		return "https://api.moonshot.cn/v1/users/me/balance", nil
	case "sub2api":
		u, err := url.Parse(strings.TrimSpace(config.BaseURL))
		if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery || (u.Port() != "" && u.Port() != "443") || u.RawPath != "" {
			return "", platform.Fail(400, "invalid_input")
		}
		if ip, err := netip.ParseAddr(u.Hostname()); err == nil && !publicBalanceIP(ip.Unmap()) {
			return "", platform.Fail(400, "invalid_input")
		}
		u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), "/v1") + "/v1/usage"
		return u.String(), nil
	default:
		return "", platform.Fail(400, "invalid_input")
	}
}

func (s *Server) balanceCipher() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte("panel upstream balances\x00" + s.Secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithRandomNonce(block)
}

func (s *Server) decryptBalanceConfig(name, ciphertext string) (BalanceConfig, error) {
	var config BalanceConfig
	aead, err := s.balanceCipher()
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

func (s *Server) FindBalanceConfig(ctx context.Context, id int64) (*ent.RuntimeSetting, BalanceConfig, error) {
	row, err := s.Database.RuntimeSetting.Query().Where(runtimesetting.KeyEQ(BalanceSettingPrefix + strconv.FormatInt(id, 10))).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, BalanceConfig{}, nil
	}
	if err != nil {
		return nil, BalanceConfig{}, err
	}
	config, err := s.decryptBalanceConfig(row.Key, row.Value)
	return row, config, err
}

func (s *Server) SaveBalanceConfig(ctx context.Context, id int64, row *ent.RuntimeSetting, config BalanceConfig) error {
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	aead, err := s.balanceCipher()
	if err != nil {
		return err
	}
	name := BalanceSettingPrefix + strconv.FormatInt(id, 10)
	ciphertext := base64.RawStdEncoding.EncodeToString(aead.Seal(nil, nil, raw, []byte(name)))
	if row == nil {
		count, err := s.Database.RuntimeSetting.Query().Where(runtimesetting.KeyHasPrefix(BalanceSettingPrefix)).Count(ctx)
		if err != nil {
			return err
		}
		if count >= BalanceConfigLimit {
			return platform.Fail(409, "invalid_input")
		}
		_, err = s.Database.RuntimeSetting.Create().SetKey(name).SetValue(ciphertext).SetUpdated(time.Now().Unix()).Save(ctx)
		return err
	}
	_, err = s.Database.RuntimeSetting.UpdateOne(row).SetValue(ciphertext).SetUpdated(time.Now().Unix()).Save(ctx)
	return err
}

func (s *Server) StartBalanceMonitor(ctx context.Context) {
	s.Balances = NewBalanceMonitor()
	if s.Database == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			s.sampleDueBalances(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *Server) sampleDueBalances(ctx context.Context) {
	ctx, end := platform.BackgroundFlow(ctx, "轮询上游账号余额")
	defer end()
	rows, err := s.Database.RuntimeSetting.Query().Where(runtimesetting.KeyHasPrefix(BalanceSettingPrefix)).Limit(BalanceConfigLimit).All(ctx)
	if err != nil {
		return
	}
	for _, row := range rows {
		id, err := strconv.ParseInt(strings.TrimPrefix(row.Key, BalanceSettingPrefix), 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		config, err := s.decryptBalanceConfig(row.Key, row.Value)
		if err == nil && config.Enabled {
			s.Balances.SampleIfDue(ctx, id, row.Value, config, false)
		}
	}
}

func (m *BalanceMonitor) Lock()   { m.mu.Lock() }
func (m *BalanceMonitor) Unlock() { m.mu.Unlock() }

func (m *BalanceMonitor) DeleteStateLocked(id int64) {
	delete(m.states, id)
}

func (m *BalanceMonitor) Snapshot(id int64) *AccountBalanceView {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.states[id].value
}

func (m *BalanceMonitor) SampleIfDue(ctx context.Context, id int64, generation string, config BalanceConfig, manual bool) <-chan error {
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
			done <- platform.Fail(409, "invalid_input")
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
				copyVal := *latest.value
				copyVal.Stale = true
				latest.value = &copyVal
			}
		}
		m.states[id] = latest
		done <- err
	}()
	return done
}

func (m *BalanceMonitor) fetch(ctx context.Context, config BalanceConfig) (AccountBalanceView, error) {
	endpoint, err := BalanceEndpoint(config)
	if err != nil {
		return AccountBalanceView{}, err
	}
	body, err := m.open(ctx, endpoint, config.Key)
	if err != nil {
		return AccountBalanceView{}, err
	}
	defer body.Close()
	value, err := ReadBalanceResponse(io.LimitReader(body, 1<<20), config.Provider)
	if err != nil {
		return AccountBalanceView{}, err
	}
	now := time.Now().Unix()
	value.FetchedAt = &now
	return value, nil
}

func (m *BalanceMonitor) open(ctx context.Context, endpoint, key string) (io.ReadCloser, error) {
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
		return nil, platform.Fail(http.StatusBadGateway, "balance_unavailable")
	}
	return res.Body, nil
}

func (m *BalanceMonitor) fetchBilling(ctx context.Context, config BalanceConfig) *AccountBillingView {
	endpoint, err := BalanceEndpoint(config)
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
	return &AccountBillingView{Effective: *billing.Effective, Observed: billing.Observed}
}

func ReadBalanceResponse(body io.Reader, provider string) (AccountBalanceView, error) {
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
	}
	if amount == nil || !(*amount > -1e12 && *amount < 1e12) {
		return AccountBalanceView{}, platform.Fail(502, "balance_unavailable")
	}
	result.Balance = *amount
	return result, nil
}
