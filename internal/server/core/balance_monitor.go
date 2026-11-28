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

// defaultQuotaPerUnit 是 New API / One API 的默认额度换算（500000 额度 = 1 美元），
// 仅在 /api/status 取不到 quota_per_unit 时兜底。
const defaultQuotaPerUnit = 500000.0

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
	// Sample 只给账本模块在服务端内部使用，不进入任何接口响应。
	Sample *BalanceSample `json:"-"`
}

// BalanceSample 是一次采样里账本对账需要的附加读数，金额均已折算为美元。
type BalanceSample struct {
	KeyUsedUSD    *float64
	WalletUsedUSD *float64
	QuotaLimitUSD *float64
	QuotaPerUSD   float64
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
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`
	BaseURL        string `json:"base_url"`
	Key            string `json:"api_key"`
	AccessToken    string `json:"access_token,omitempty"`
	UpstreamUserID int64  `json:"upstream_user_id,omitempty"`
}

// BalanceHooks 是采样结果的回调，由账本模块在 router.WireHubs 里接入。
// 回调在采样协程里同步执行，且不持有 BalanceMonitor 的锁。
type BalanceHooks struct {
	OnSample  func(ctx context.Context, id int64, config BalanceConfig, view AccountBalanceView)
	OnFailure func(ctx context.Context, id int64, config BalanceConfig, err error)
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
	hooks  BalanceHooks
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

// SetHooks 替换采样回调；可在监控启动后调用。
func (m *BalanceMonitor) SetHooks(hooks BalanceHooks) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.hooks = hooks
	m.mu.Unlock()
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

// balanceBaseURL 校验并返回站点根地址：必须 https、默认端口、无凭据与查询串、主机不是非公网 IP。
func balanceBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery || (u.Port() != "" && u.Port() != "443") || u.RawPath != "" {
		return nil, platform.Fail(400, "invalid_input")
	}
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil && !publicBalanceIP(ip.Unmap()) {
		return nil, platform.Fail(400, "invalid_input")
	}
	return u, nil
}

func balanceJoin(base *url.URL, path string) string {
	u := *base
	u.Path = strings.TrimRight(u.Path, "/") + path
	return u.String()
}

func BalanceEndpoint(config BalanceConfig) (string, error) {
	switch config.Provider {
	case "deepseek":
		return "https://api.deepseek.com/user/balance", nil
	case "kimi":
		return "https://api.moonshot.cn/v1/users/me/balance", nil
	case "sub2api":
		u, err := balanceBaseURL(config.BaseURL)
		if err != nil {
			return "", err
		}
		u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), "/v1") + "/v1/usage"
		return u.String(), nil
	case "newapi", "oneapi":
		u, err := balanceBaseURL(config.BaseURL)
		if err != nil {
			return "", err
		}
		if config.AccessToken != "" {
			return balanceJoin(u, "/api/user/self"), nil
		}
		if config.Provider == "oneapi" {
			return "", platform.Fail(400, "invalid_input")
		}
		return balanceJoin(u, "/api/usage/token/"), nil
	default:
		return "", platform.Fail(400, "invalid_input")
	}
}

// sealSecret 用面板密钥派生的 AES-GCM 加密任意明文，name 作为附加认证数据绑定到存储键。
func (s *Server) sealSecret(name string, plaintext []byte) (string, error) {
	aead, err := s.secretCipher()
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(aead.Seal(nil, nil, plaintext, []byte(name))), nil
}

func (s *Server) openSecret(name, ciphertext string) ([]byte, error) {
	aead, err := s.secretCipher()
	if err != nil {
		return nil, err
	}
	raw, err := base64.RawStdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nil, raw, []byte(name))
}

func (s *Server) secretCipher() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte("panel upstream balances\x00" + s.Secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithRandomNonce(block)
}

func (s *Server) decryptBalanceConfig(name, ciphertext string) (BalanceConfig, error) {
	var config BalanceConfig
	plaintext, err := s.openSecret(name, ciphertext)
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
	name := BalanceSettingPrefix + strconv.FormatInt(id, 10)
	ciphertext, err := s.sealSecret(name, raw)
	if err != nil {
		return err
	}
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
		latest := m.states[id]
		if latest.generation != generation {
			m.mu.Unlock()
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
		hooks := m.hooks
		m.mu.Unlock()
		if err == nil && hooks.OnSample != nil {
			hooks.OnSample(ctx, id, config, value)
		} else if err != nil && hooks.OnFailure != nil {
			hooks.OnFailure(ctx, id, config, err)
		}
		done <- err
	}()
	return done
}

func (m *BalanceMonitor) fetch(ctx context.Context, config BalanceConfig) (AccountBalanceView, error) {
	var value AccountBalanceView
	var err error
	switch config.Provider {
	case "newapi", "oneapi":
		value, err = m.fetchQuota(ctx, config)
	default:
		value, err = m.fetchBalance(ctx, config)
	}
	if err != nil {
		return AccountBalanceView{}, err
	}
	now := time.Now().Unix()
	value.FetchedAt = &now
	return value, nil
}

func (m *BalanceMonitor) fetchBalance(ctx context.Context, config BalanceConfig) (AccountBalanceView, error) {
	endpoint, err := BalanceEndpoint(config)
	if err != nil {
		return AccountBalanceView{}, err
	}
	body, err := m.open(ctx, endpoint, config.Key)
	if err != nil {
		return AccountBalanceView{}, err
	}
	defer body.Close()
	return ReadBalanceResponse(io.LimitReader(body, 1<<20), config.Provider)
}

// fetchQuota 读取 New API / One API：有访问令牌时读钱包（/api/user/self），
// 只有 Key 时读 Key 自身的额度（/api/usage/token/，仅 New API）。额度按 /api/status 的 quota_per_unit 换算成美元。
func (m *BalanceMonitor) fetchQuota(ctx context.Context, config BalanceConfig) (AccountBalanceView, error) {
	base, err := balanceBaseURL(config.BaseURL)
	if err != nil {
		return AccountBalanceView{}, err
	}
	endpoint, err := BalanceEndpoint(config)
	if err != nil {
		return AccountBalanceView{}, err
	}
	perUnit := m.quotaPerUnit(ctx, base)
	wallet := config.AccessToken != ""
	token, headers := config.Key, map[string]string{}
	if wallet {
		token = config.AccessToken
		if config.Provider == "newapi" && config.UpstreamUserID > 0 {
			headers["New-Api-User"] = strconv.FormatInt(config.UpstreamUserID, 10)
		}
	}
	body, err := m.openWith(ctx, endpoint, token, headers)
	if err != nil {
		return AccountBalanceView{}, err
	}
	defer body.Close()
	value, err := ReadQuotaResponse(io.LimitReader(body, 1<<20), wallet, perUnit)
	if err != nil || !wallet || config.Provider != "newapi" || config.Key == "" {
		return value, err
	}
	// 钱包读数成功后，尽力补读 Key 级已用额度；失败不影响钱包读数。
	if keyBody, err := m.open(ctx, balanceJoin(base, "/api/usage/token/"), config.Key); err == nil {
		defer keyBody.Close()
		if keyView, err := ReadQuotaResponse(io.LimitReader(keyBody, 1<<20), false, perUnit); err == nil && keyView.Sample != nil {
			value.Sample.KeyUsedUSD = keyView.Sample.KeyUsedUSD
		}
	}
	return value, nil
}

func (m *BalanceMonitor) quotaPerUnit(ctx context.Context, base *url.URL) float64 {
	body, err := m.open(ctx, balanceJoin(base, "/api/status"), "")
	if err != nil {
		return defaultQuotaPerUnit
	}
	defer body.Close()
	var wire struct {
		Data struct {
			QuotaPerUnit *float64 `json:"quota_per_unit"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(body, 1<<20)).Decode(&wire) != nil || wire.Data.QuotaPerUnit == nil || !(*wire.Data.QuotaPerUnit > 0 && *wire.Data.QuotaPerUnit < 1e12) {
		return defaultQuotaPerUnit
	}
	return *wire.Data.QuotaPerUnit
}

func (m *BalanceMonitor) open(ctx context.Context, endpoint, key string) (io.ReadCloser, error) {
	return m.openWith(ctx, endpoint, key, nil)
}

func (m *BalanceMonitor) openWith(ctx context.Context, endpoint, key string, headers map[string]string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
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
