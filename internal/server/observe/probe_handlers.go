package observe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/plugins/probelmfpd"
)

type probeRequest struct {
	Engine       string `json:"engine"`                  // official | custom | lmfpd | gpt-relay-inspector
	Model        string `json:"model,omitempty"`         // target model id
	Protocol     string `json:"protocol,omitempty"`      // responses | chat_completions | messages
	SystemPrompt string `json:"system_prompt,omitempty"` // custom system prompt
	Prompt       string `json:"prompt,omitempty"`        // custom user prompt
	Effort       string `json:"effort,omitempty"`        // low | medium | high | xhigh
	Preset       string `json:"preset,omitempty"`        // fast | full (for gpt-relay-inspector)
	MatchKind    string `json:"match_kind,omitempty"`    // contains | exact_number | regex | status_ok
	Expected     string `json:"expected,omitempty"`
}

type probeReportView struct {
	Verdict         string                 `json:"verdict"` // ok | degraded | failed
	Engine          string                 `json:"engine"`
	Model           string                 `json:"model"`
	Effort          string                 `json:"effort,omitempty"`
	DurationMs      int64                  `json:"duration_ms"`
	TTFTMs          int64                  `json:"ttft_ms"`
	InputTokens     int64                  `json:"input_tokens"`
	OutputTokens    int64                  `json:"output_tokens"`
	ReasoningTokens int64                  `json:"reasoning_tokens"`
	CachedTokens    int64                  `json:"cached_tokens"`
	EstimatedMicros int64                  `json:"estimated_micros"`
	Confidence      *float64               `json:"confidence,omitempty"`
	PredictedModel  string                 `json:"predicted_model,omitempty"`
	Summary         string                 `json:"summary"`
	Reply           string                 `json:"reply,omitempty"`
	Candidates      []probelmfpd.Candidate `json:"candidates,omitempty"`
	InspectorScore  *float64               `json:"inspector_score,omitempty"`
	InspectorGrade  string                 `json:"inspector_grade,omitempty"`
}

type probeStreamFrame struct {
	Type    string           `json:"type"` // status | delta | complete | error
	Message string           `json:"message,omitempty"`
	Delta   string           `json:"delta,omitempty"`
	Report  *probeReportView `json:"report,omitempty"`
}

type ephemeralPassView struct {
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key"`
	ExpiresAt    int64  `json:"expires_at"`
	QuotaUSD     string `json:"quota_usd"`
	IkaleURL     string `json:"ikale_url"`
	InspectorURL string `json:"inspector_url"`
}

type mintedKeyWire struct {
	ID  int64  `json:"id"`
	Key string `json:"key"`
}

type probeAccountWire struct {
	Platform string               `json:"platform"`
	GroupIDs []int64              `json:"group_ids"`
	Groups   []platform.NamedItem `json:"groups"`
}

var secretScrubber = regexp.MustCompile(`(?i)(sk-[a-z0-9_-]{8,}|Bearer\s+[a-z0-9._-]{8,})`)

func scrubText(raw string, maxLen int) string {
	clean := secretScrubber.ReplaceAllString(strings.TrimSpace(raw), "[REDACTED]")
	if maxLen > 0 && len(clean) > maxLen {
		return clean[:maxLen] + "…"
	}
	return clean
}

func gatewayBaseURL(ctx context.Context, s *core.Server) (loopbackOrInternal string, publicURL string) {
	s.RuntimeMu.RLock()
	base := strings.TrimRight(s.Upstream.Base, "/")
	s.RuntimeMu.RUnlock()
	pub := strings.TrimRight(s.Official, "/")
	if settings, err := s.Settings(ctx); err == nil && strings.TrimSpace(settings.APIBaseURL) != "" {
		pub = strings.TrimRight(strings.TrimSpace(settings.APIBaseURL), "/")
	}
	return base, pub
}

// mintGroupProbeKey 为指定分组签发一把短效限额临时 Key，返回明文密钥与销毁函数。
func mintGroupProbeKey(ctx context.Context, s *core.Server, adminToken string, groupID int64, ttl time.Duration) (string, int64, func(), error) {
	exp := time.Now().Add(ttl).UTC()
	created, err := platform.Upstream[mintedKeyWire](ctx, s.Upstream, "POST", "/keys", adminToken, "", "", map[string]any{
		"name":       fmt.Sprintf("panel-ephemeral-probe-%s", platform.RandomHex(4)),
		"group_id":   groupID,
		"quota":      0.35,
		"expires_at": exp.Format(time.RFC3339),
	})
	if err != nil {
		return "", 0, func() {}, err
	}
	if created.ID < 1 || strings.TrimSpace(created.Key) == "" {
		return "", 0, func() {}, platform.Fail(502, "invalid_upstream")
	}
	cleanup := func() {
		cctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		_, _ = platform.Upstream[struct{}](cctx, s.Upstream, "DELETE", fmt.Sprintf("/keys/%d", created.ID), adminToken, "", "", nil)
	}
	return created.Key, exp.Unix(), cleanup, nil
}

// mintAccountSandbox 为单个账号创建临时单号专属分组 + 短效限额 Key，测完自动恢复原分组并销毁临时资源。
func mintAccountSandbox(ctx context.Context, s *core.Server, adminToken string, accountID int64, ttl time.Duration) (probeAccountWire, string, int64, func(), error) {
	acc, err := platform.Upstream[probeAccountWire](ctx, s.Upstream, "GET", fmt.Sprintf("/admin/accounts/%d", accountID), adminToken, "", "", nil)
	if err != nil {
		return probeAccountWire{}, "", 0, func() {}, err
	}
	origGroups := append([]int64(nil), acc.GroupIDs...)
	if len(origGroups) == 0 && len(acc.Groups) > 0 {
		for _, g := range acc.Groups {
			origGroups = append(origGroups, g.ID)
		}
	}
	plat := acc.Platform
	if plat == "" {
		plat = "openai"
	}
	tempGroup, err := platform.Upstream[struct {
		ID int64 `json:"id"`
	}](ctx, s.Upstream, "POST", "/admin/groups", adminToken, "", "", map[string]any{
		"name":              fmt.Sprintf("panel-probe-acc-%d-%s", accountID, platform.RandomHex(3)),
		"description":       "面板单号隔离探测临时分组（自动销毁）",
		"platform":          plat,
		"rate_multiplier":   1.0,
		"is_exclusive":      true,
		"status":            "active",
		"subscription_type": "standard",
	})
	if err != nil || tempGroup.ID < 1 {
		if err == nil {
			err = platform.Fail(502, "invalid_upstream")
		}
		return acc, "", 0, func() {}, err
	}
	nextGroups := append(append([]int64(nil), origGroups...), tempGroup.ID)
	if _, err := platform.Upstream[probeAccountWire](ctx, s.Upstream, "PUT", fmt.Sprintf("/admin/accounts/%d", accountID), adminToken, "", "", map[string]any{
		"group_ids":                  nextGroups,
		"confirm_mixed_channel_risk": true,
	}); err != nil {
		_, _ = platform.Upstream[struct{}](ctx, s.Upstream, "DELETE", fmt.Sprintf("/admin/groups/%d", tempGroup.ID), adminToken, "", "", nil)
		return acc, "", 0, func() {}, err
	}
	key, expUnix, revokeKey, err := mintGroupProbeKey(ctx, s, adminToken, tempGroup.ID, ttl)
	if err != nil {
		_, _ = platform.Upstream[probeAccountWire](ctx, s.Upstream, "PUT", fmt.Sprintf("/admin/accounts/%d", accountID), adminToken, "", "", map[string]any{
			"group_ids":                  origGroups,
			"confirm_mixed_channel_risk": true,
		})
		_, _ = platform.Upstream[struct{}](ctx, s.Upstream, "DELETE", fmt.Sprintf("/admin/groups/%d", tempGroup.ID), adminToken, "", "", nil)
		return acc, "", 0, func() {}, err
	}
	cleanup := func() {
		revokeKey()
		cctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_, _ = platform.Upstream[probeAccountWire](cctx, s.Upstream, "PUT", fmt.Sprintf("/admin/accounts/%d", accountID), adminToken, "", "", map[string]any{
			"group_ids":                  origGroups,
			"confirm_mixed_channel_risk": true,
		})
		_, _ = platform.Upstream[struct{}](cctx, s.Upstream, "DELETE", fmt.Sprintf("/admin/groups/%d", tempGroup.ID), adminToken, "", "", nil)
	}
	return acc, key, expUnix, cleanup, nil
}

func emitProbeSSE(w http.ResponseWriter, frame probeStreamFrame) {
	raw, err := json.Marshal(frame)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", raw)
	_ = http.NewResponseController(w).Flush()
}

func prepareProbeSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
}

func adminMintGroupProbePass(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		key, expUnix, cleanup, err := mintGroupProbeKey(r.Context(), s, a.Tokens.Access, id, 10*time.Minute)
		if err != nil {
			return err
		}
		time.AfterFunc(10*time.Minute+5*time.Second, cleanup)
		platform.WriteJSON(w, 200, buildPassView(r.Context(), s, key, expUnix))
		return nil
	}
}

func adminMintAccountProbePass(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		_, key, expUnix, cleanup, err := mintAccountSandbox(r.Context(), s, a.Tokens.Access, id, 10*time.Minute)
		if err != nil {
			return err
		}
		time.AfterFunc(10*time.Minute+5*time.Second, cleanup)
		platform.WriteJSON(w, 200, buildPassView(r.Context(), s, key, expUnix))
		return nil
	}
}

func buildPassView(ctx context.Context, s *core.Server, key string, expUnix int64) ephemeralPassView {
	_, pub := gatewayBaseURL(ctx, s)
	ikaleURL := "https://lm.ikale.io/#/?mode=api"
	inspectorURL := "https://gpttest.xyz"
	_, ext := s.EnsurePlugins()
	if comp, ok := ext.FindReady("lm-detector-companion"); ok && comp.Options["web_url"] != "" {
		ikaleURL = comp.Options["web_url"]
	}
	if insp, ok := ext.FindReady("gpt-relay-inspector"); ok {
		if insp.Options["endpoint"] != "" {
			inspectorURL = insp.Options["endpoint"]
		}
		if insp.Options["public_gateway_url"] != "" {
			pub = strings.TrimRight(insp.Options["public_gateway_url"], "/")
		}
	}
	return ephemeralPassView{
		BaseURL:      pub + "/v1",
		APIKey:       key,
		ExpiresAt:    expUnix,
		QuotaUSD:     "$0.35",
		IkaleURL:     ikaleURL,
		InspectorURL: inspectorURL,
	}
}

func adminProbeAccount(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		req, err := platform.DecodeBody[probeRequest](r)
		if err != nil {
			return err
		}
		if req.Engine == "" {
			req.Engine = "official"
		}
		prepareProbeSSE(w)
		if req.Engine == "official" {
			streamOfficialAccountProbe(r.Context(), s, w, a.Tokens.Access, id, req)
			return nil
		}
		emitProbeSSE(w, probeStreamFrame{Type: "status", Message: "正在建立单账号临时隔离沙箱与短效限额通行证…"})
		acc, key, _, cleanup, sErr := mintAccountSandbox(r.Context(), s, a.Tokens.Access, id, 5*time.Minute)
		if sErr != nil {
			emitProbeSSE(w, probeStreamFrame{Type: "error", Message: "无法创建单账号探测沙箱，请确认上游支持动态分组或改用原生快检。"})
			return nil
		}
		defer cleanup()
		if req.Model == "" {
			req.Model = defaultModelForPlatform(acc.Platform)
		}
		if req.Protocol == "" {
			req.Protocol = defaultProtocolForPlatform(acc.Platform)
		}
		report, runErr := executeGatewayProbe(r.Context(), s, key, req, func(frame probeStreamFrame) {
			emitProbeSSE(w, frame)
		})
		if runErr != nil {
			emitProbeSSE(w, probeStreamFrame{Type: "error", Message: scrubText(runErr.Error(), 260)})
			return nil
		}
		emitProbeSSE(w, probeStreamFrame{Type: "complete", Report: &report})
		return nil
	}
}

func adminProbeGroup(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		id, err := platform.PathID(r, "id")
		if err != nil {
			return err
		}
		req, err := platform.DecodeBody[probeRequest](r)
		if err != nil {
			return err
		}
		if req.Engine == "" || req.Engine == "official" {
			req.Engine = "custom"
		}
		prepareProbeSSE(w)
		emitProbeSSE(w, probeStreamFrame{Type: "status", Message: "正在签发分组一次性限额探测 Key…"})
		key, _, cleanup, kErr := mintGroupProbeKey(r.Context(), s, a.Tokens.Access, id, 5*time.Minute)
		if kErr != nil {
			emitProbeSSE(w, probeStreamFrame{Type: "error", Message: "签发探测密钥失败，请检查当前账号余额或分组状态。"})
			return nil
		}
		defer cleanup()
		report, runErr := executeGatewayProbe(r.Context(), s, key, req, func(frame probeStreamFrame) {
			emitProbeSSE(w, frame)
		})
		if runErr != nil {
			emitProbeSSE(w, probeStreamFrame{Type: "error", Message: scrubText(runErr.Error(), 260)})
			return nil
		}
		emitProbeSSE(w, probeStreamFrame{Type: "complete", Report: &report})
		return nil
	}
}

func defaultModelForPlatform(plat string) string {
	switch plat {
	case "anthropic":
		return "claude-sonnet-4-6"
	case "gemini":
		return "gemini-2.5-pro"
	case "deepseek":
		return "deepseek-chat"
	case "grok":
		return "grok-3"
	default:
		return "gpt-5.4"
	}
}

func defaultProtocolForPlatform(plat string) string {
	switch plat {
	case "anthropic":
		return "messages"
	case "openai", "grok":
		return "responses"
	default:
		return "chat_completions"
	}
}
