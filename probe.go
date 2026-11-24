package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

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

var secretScrubber = regexp.MustCompile(`(?i)(sk-[a-z0-9_-]{8,}|Bearer\s+[a-z0-9._-]{8,})`)

func scrubText(raw string, maxLen int) string {
	clean := secretScrubber.ReplaceAllString(strings.TrimSpace(raw), "[REDACTED]")
	if maxLen > 0 && len(clean) > maxLen {
		return clean[:maxLen] + "…"
	}
	return clean
}

func (p *panel) probeRoutes(mux *http.ServeMux) {
	p.route(mux, "POST /api/admin/accounts/{id}/probe", true, p.adminProbeAccount)
	p.route(mux, "POST /api/admin/accounts/{id}/probe-pass", true, p.adminMintAccountProbePass)
	p.route(mux, "POST /api/admin/groups/{id}/probe", true, p.adminProbeGroup)
	p.route(mux, "POST /api/admin/groups/{id}/probe-pass", true, p.adminMintGroupProbePass)
	p.public(mux, "POST /api/patrol/guest-probe", p.publicGuestProbe)
}

func (p *panel) gatewayBaseURL(ctx context.Context) (loopbackOrInternal string, publicURL string) {
	p.runtimeMu.RLock()
	base := strings.TrimRight(p.upstream.base, "/")
	p.runtimeMu.RUnlock()
	pub := strings.TrimRight(p.official, "/")
	if s, err := p.settings(ctx); err == nil && strings.TrimSpace(s.APIBaseURL) != "" {
		pub = strings.TrimRight(strings.TrimSpace(s.APIBaseURL), "/")
	}
	return base, pub
}

// mintGroupProbeKey 为指定分组签发一把短效限额临时 Key，返回明文密钥与销毁函数。
func (p *panel) mintGroupProbeKey(ctx context.Context, adminToken string, groupID int64, ttl time.Duration) (string, int64, func(), error) {
	exp := time.Now().Add(ttl).UTC()
	created, err := upstream[keyWire](ctx, p.upstream, "POST", "/keys", adminToken, "", "", map[string]any{
		"name":       fmt.Sprintf("panel-ephemeral-probe-%s", randomHex(4)),
		"group_id":   groupID,
		"quota":      0.35,
		"expires_at": exp.Format(time.RFC3339),
	})
	if err != nil {
		return "", 0, func() {}, err
	}
	if created.ID < 1 || strings.TrimSpace(created.Key) == "" {
		return "", 0, func() {}, fail(502, "invalid_upstream")
	}
	cleanup := func() {
		cctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		_, _ = upstream[struct{}](cctx, p.upstream, "DELETE", fmt.Sprintf("/keys/%d", created.ID), adminToken, "", "", nil)
	}
	return created.Key, exp.Unix(), cleanup, nil
}

// mintAccountSandbox 为单个账号创建临时单号专属分组 + 短效限额 Key，测完自动恢复原分组并销毁临时资源。
func (p *panel) mintAccountSandbox(ctx context.Context, adminToken string, accountID int64, ttl time.Duration) (accountDetailWire, string, int64, func(), error) {
	acc, err := upstream[accountDetailWire](ctx, p.upstream, "GET", fmt.Sprintf("/admin/accounts/%d", accountID), adminToken, "", "", nil)
	if err != nil {
		return accountDetailWire{}, "", 0, func() {}, err
	}
	origGroups := append([]int64(nil), acc.GroupIDs...)
	if len(origGroups) == 0 && len(acc.Groups) > 0 {
		for _, g := range acc.Groups {
			origGroups = append(origGroups, g.ID)
		}
	}
	platform := acc.Platform
	if platform == "" {
		platform = "openai"
	}
	tempGroup, err := upstream[struct {
		ID int64 `json:"id"`
	}](ctx, p.upstream, "POST", "/admin/groups", adminToken, "", "", map[string]any{
		"name":              fmt.Sprintf("panel-probe-acc-%d-%s", accountID, randomHex(3)),
		"description":       "面板单号隔离探测临时分组（自动销毁）",
		"platform":          platform,
		"rate_multiplier":   1.0,
		"is_exclusive":      true,
		"status":            "active",
		"subscription_type": "standard",
	})
	if err != nil || tempGroup.ID < 1 {
		if err == nil {
			err = fail(502, "invalid_upstream")
		}
		return acc, "", 0, func() {}, err
	}
	nextGroups := append(append([]int64(nil), origGroups...), tempGroup.ID)
	if _, err := upstream[accountDetailWire](ctx, p.upstream, "PUT", fmt.Sprintf("/admin/accounts/%d", accountID), adminToken, "", "", map[string]any{
		"group_ids":                 nextGroups,
		"confirm_mixed_channel_risk": true,
	}); err != nil {
		_, _ = upstream[struct{}](ctx, p.upstream, "DELETE", fmt.Sprintf("/admin/groups/%d", tempGroup.ID), adminToken, "", "", nil)
		return acc, "", 0, func() {}, err
	}
	key, expUnix, revokeKey, err := p.mintGroupProbeKey(ctx, adminToken, tempGroup.ID, ttl)
	if err != nil {
		_, _ = upstream[accountDetailWire](ctx, p.upstream, "PUT", fmt.Sprintf("/admin/accounts/%d", accountID), adminToken, "", "", map[string]any{
			"group_ids":                 origGroups,
			"confirm_mixed_channel_risk": true,
		})
		_, _ = upstream[struct{}](ctx, p.upstream, "DELETE", fmt.Sprintf("/admin/groups/%d", tempGroup.ID), adminToken, "", "", nil)
		return acc, "", 0, func() {}, err
	}
	cleanup := func() {
		revokeKey()
		cctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_, _ = upstream[accountDetailWire](cctx, p.upstream, "PUT", fmt.Sprintf("/admin/accounts/%d", accountID), adminToken, "", "", map[string]any{
			"group_ids":                 origGroups,
			"confirm_mixed_channel_risk": true,
		})
		_, _ = upstream[struct{}](cctx, p.upstream, "DELETE", fmt.Sprintf("/admin/groups/%d", tempGroup.ID), adminToken, "", "", nil)
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

func (p *panel) adminMintGroupProbePass(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	key, expUnix, cleanup, err := p.mintGroupProbeKey(r.Context(), a.tokens.Access, id, 10*time.Minute)
	if err != nil {
		return err
	}
	time.AfterFunc(10*time.Minute+5*time.Second, cleanup)
	writeJSON(w, 200, p.buildPassView(r.Context(), key, expUnix))
	return nil
}

func (p *panel) adminMintAccountProbePass(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	_, key, expUnix, cleanup, err := p.mintAccountSandbox(r.Context(), a.tokens.Access, id, 10*time.Minute)
	if err != nil {
		return err
	}
	time.AfterFunc(10*time.Minute+5*time.Second, cleanup)
	writeJSON(w, 200, p.buildPassView(r.Context(), key, expUnix))
	return nil
}

func (p *panel) buildPassView(ctx context.Context, key string, expUnix int64) ephemeralPassView {
	_, pub := p.gatewayBaseURL(ctx)
	ikaleURL := "https://lm.ikale.io/#/?mode=api"
	inspectorURL := "https://gpttest.xyz"
	_, ext := p.ensurePlugins()
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

func (p *panel) adminProbeAccount(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	req, err := decodeBody[probeRequest](r)
	if err != nil {
		return err
	}
	if req.Engine == "" {
		req.Engine = "official"
	}
	prepareProbeSSE(w)
	if req.Engine == "official" {
		p.streamOfficialAccountProbe(r.Context(), w, a.tokens.Access, id, req)
		return nil
	}
	emitProbeSSE(w, probeStreamFrame{Type: "status", Message: "正在建立单账号临时隔离沙箱与短效限额通行证…"})
	acc, key, _, cleanup, sErr := p.mintAccountSandbox(r.Context(), a.tokens.Access, id, 5*time.Minute)
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
	report, runErr := p.executeGatewayProbe(r.Context(), key, req, func(frame probeStreamFrame) {
		emitProbeSSE(w, frame)
	})
	if runErr != nil {
		emitProbeSSE(w, probeStreamFrame{Type: "error", Message: scrubText(runErr.Error(), 260)})
		return nil
	}
	emitProbeSSE(w, probeStreamFrame{Type: "complete", Report: &report})
	return nil
}

func (p *panel) adminProbeGroup(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	req, err := decodeBody[probeRequest](r)
	if err != nil {
		return err
	}
	if req.Engine == "" || req.Engine == "official" {
		req.Engine = "custom"
	}
	prepareProbeSSE(w)
	emitProbeSSE(w, probeStreamFrame{Type: "status", Message: "正在签发分组一次性限额探测 Key…"})
	key, _, cleanup, kErr := p.mintGroupProbeKey(r.Context(), a.tokens.Access, id, 5*time.Minute)
	if kErr != nil {
		emitProbeSSE(w, probeStreamFrame{Type: "error", Message: "签发探测密钥失败，请检查当前账号余额或分组状态。"})
		return nil
	}
	defer cleanup()
	report, runErr := p.executeGatewayProbe(r.Context(), key, req, func(frame probeStreamFrame) {
		emitProbeSSE(w, frame)
	})
	if runErr != nil {
		emitProbeSSE(w, probeStreamFrame{Type: "error", Message: scrubText(runErr.Error(), 260)})
		return nil
	}
	emitProbeSSE(w, probeStreamFrame{Type: "complete", Report: &report})
	return nil
}

func defaultModelForPlatform(platform string) string {
	switch platform {
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

func defaultProtocolForPlatform(platform string) string {
	switch platform {
	case "anthropic":
		return "messages"
	case "openai", "grok":
		return "responses"
	default:
		return "chat_completions"
	}
}

func (p *panel) streamOfficialAccountProbe(ctx context.Context, w http.ResponseWriter, token string, accountID int64, req probeRequest) {
	stream := p.upstream
	client := *stream.client
	client.Timeout = 75 * time.Second
	stream.client = &client
	body := map[string]any{}
	if strings.TrimSpace(req.Model) != "" {
		body["model_id"] = strings.TrimSpace(req.Model)
	}
	if strings.TrimSpace(req.Prompt) != "" {
		body["prompt"] = strings.TrimSpace(req.Prompt)
	}
	start := time.Now()
	emitProbeSSE(w, probeStreamFrame{Type: "status", Message: "正在通过 Sub2API 原生测活通道探测…"})
	res, err := stream.open(ctx, "POST", fmt.Sprintf("/api/v1/admin/accounts/%d/test", accountID), token, "", "", body)
	if err != nil {
		emitProbeSSE(w, probeStreamFrame{Type: "error", Message: "连接上游测活接口失败。"})
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		emitProbeSSE(w, probeStreamFrame{Type: "error", Message: fmt.Sprintf("上游测活返回 HTTP %d", res.StatusCode)})
		return
	}
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 2048), 256<<10)
	var ttftMs int64
	var reply strings.Builder
	resolvedModel := strings.TrimSpace(req.Model)
	passed := false
	errDetail := ""
	for scanner.Scan() {
		line, ok := strings.CutPrefix(scanner.Text(), "data:")
		if !ok {
			continue
		}
		var ev struct {
			Type    string `json:"type"`
			Model   string `json:"model"`
			Text    string `json:"text"`
			Error   string `json:"error"`
			Success bool   `json:"success"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "test_start":
			if ev.Model != "" {
				resolvedModel = ev.Model
			}
		case "status":
			if ev.Text != "" {
				emitProbeSSE(w, probeStreamFrame{Type: "status", Message: scrubText(ev.Text, 160)})
			}
		case "content":
			if ttftMs == 0 {
				ttftMs = time.Since(start).Milliseconds()
			}
			chunk := scrubText(ev.Text, 600)
			if reply.Len() < 2400 {
				reply.WriteString(chunk)
			}
			emitProbeSSE(w, probeStreamFrame{Type: "delta", Delta: chunk})
		case "error":
			errDetail = scrubText(ev.Error, 240)
		case "test_complete":
			passed = ev.Success
		}
	}
	verdict := "failed"
	summary := "原生连通性探测未通过"
	if passed {
		verdict = "ok"
		summary = "原生连通性探测通过"
	} else if errDetail != "" {
		summary = errDetail
	}
	emitProbeSSE(w, probeStreamFrame{
		Type: "complete",
		Report: &probeReportView{
			Verdict:    verdict,
			Engine:     "official",
			Model:      resolvedModel,
			DurationMs: time.Since(start).Milliseconds(),
			TTFTMs:     ttftMs,
			Summary:    summary,
			Reply:      reply.String(),
		},
	})
}

func (p *panel) executeGatewayProbe(ctx context.Context, apiKey string, req probeRequest, emit func(probeStreamFrame)) (probeReportView, error) {
	internalBase, publicBase := p.gatewayBaseURL(ctx)
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "gpt-5.4"
	}
	protocol := strings.TrimSpace(req.Protocol)
	if protocol == "" {
		protocol = "chat_completions"
	}
	switch req.Engine {
	case "lmfpd":
		svc := p.lmfpdService()
		if svc == nil {
			return probeReportView{}, errors.New("LM 指纹检测内置插件当前已关闭，请先在「插件」页启用 probe-lmfpd")
		}
		out, err := svc.Run(ctx, probelmfpd.RunInput{
			BaseURL:  internalBase + "/v1",
			APIKey:   apiKey,
			Model:    model,
			Protocol: protocol,
			Effort:   req.Effort,
		}, func(step string) {
			if emit != nil {
				emit(probeStreamFrame{Type: "status", Message: step})
			}
		})
		if err != nil {
			return probeReportView{}, err
		}
		verdict := "ok"
		summary := fmt.Sprintf("识别模型：%s（样本有效 %d 组）", out.PredictedName, out.AcceptedCount)
		if out.Decision == "unscorable" || out.Decision == "not_model" {
			verdict = "degraded"
			summary = "采样特征不足以匹配参考模型（" + out.Decision + "）"
		} else if !modelsAlign(model, out.PredictedModel) {
			verdict = "degraded"
			summary = fmt.Sprintf("请求 %s，数字指纹最接近 %s", model, out.PredictedName)
		}
		return probeReportView{
			Verdict:        verdict,
			Engine:         "lmfpd",
			Model:          model,
			Effort:         req.Effort,
			DurationMs:     out.DurationMs,
			TTFTMs:         out.TTFTMs,
			Confidence:     out.Probability,
			PredictedModel: out.PredictedName,
			Summary:        summary,
			Reply:          out.Excerpt,
			Candidates:     out.Candidates,
		}, nil

	case "gpt-relay-inspector":
		_, ext := p.ensurePlugins()
		plug, ok := ext.FindReady("gpt-relay-inspector")
		if !ok {
			return probeReportView{}, errors.New("少点套路插件未启用，请先在「插件」页启用 gpt-relay-inspector")
		}
		inspectorEP := strings.TrimRight(plug.Options["endpoint"], "/")
		if inspectorEP == "" {
			inspectorEP = "https://gpttest.xyz"
		}
		targetBase := publicBase
		if customGW := strings.TrimSpace(plug.Options["public_gateway_url"]); customGW != "" {
			targetBase = strings.TrimRight(customGW, "/")
		}
		if strings.Contains(inspectorEP, "gpttest.xyz") && isLoopbackHost(targetBase) {
			return probeReportView{}, errors.New("公网 gpttest.xyz 无法反向回调本机回环网关地址；请配置公网网关域名，或在插件设置中填入本机自建少点套路节点地址")
		}
		if strings.HasPrefix(inspectorEP, "http://127.0.0.1") || strings.HasPrefix(inspectorEP, "http://localhost") {
			targetBase = internalBase
		}
		return runInspectorSSE(ctx, inspectorEP, targetBase, apiKey, model, req.Preset, req.Effort, emit)

	default:
		return runCustomGatewayRequest(ctx, internalBase, apiKey, req, emit)
	}
}

func isLoopbackHost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	h := u.Hostname()
	if h == "localhost" || strings.HasSuffix(h, ".test") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}

func modelsAlign(requested, predicted string) bool {
	r := strings.ToLower(strings.TrimSpace(requested))
	p := strings.ToLower(strings.TrimSpace(predicted))
	if r == "" || p == "" || strings.Contains(r, p) || strings.Contains(p, r) {
		return true
	}
	// 同代主家族比对（如 claude-sonnet-4-6 vs claude-sonnet-4.6）
	norm := strings.NewReplacer("-", "", ".", "", "_", "")
	rn, pn := norm.Replace(r), norm.Replace(p)
	return strings.Contains(rn, pn) || strings.Contains(pn, rn)
}

func runInspectorSSE(ctx context.Context, inspectorEP, targetBase, apiKey, model, preset, effort string, emit func(probeStreamFrame)) (probeReportView, error) {
	if preset != "full" {
		preset = "fast"
	}
	payload, _ := json.Marshal(map[string]any{
		"baseUrl":         targetBase,
		"apiKey":          apiKey,
		"model":           model,
		"preset":          preset,
		"reasoningEffort": effort,
	})
	cctx, cancel := context.WithTimeout(ctx, 110*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(cctx, http.MethodPost, inspectorEP+"/api/inspect", bytes.NewReader(payload))
	if err != nil {
		return probeReportView{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	start := time.Now()
	resp, err := (&http.Client{Timeout: 115 * time.Second}).Do(httpReq)
	if err != nil {
		return probeReportView{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return probeReportView{}, fmt.Errorf("少点套路服务返回 HTTP %d：%s", resp.StatusCode, scrubText(string(raw), 180))
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 512<<10)
	var curEvent string
	var finalScore float64
	var finalGrade, finalVerdictText string
	completed := false
	for scanner.Scan() {
		line := scanner.Text()
		if after, ok := strings.CutPrefix(line, "event:"); ok {
			curEvent = strings.TrimSpace(after)
			continue
		}
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		switch curEvent {
		case "fatal_error":
			var fe struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal([]byte(data), &fe)
			return probeReportView{}, errors.New(scrubText(fe.Message, 220))
		case "complete":
			var rep struct {
				Evaluation struct {
					Score   float64 `json:"score"`
					Grade   string  `json:"grade"`
					Summary string  `json:"summary"`
					Verdict string  `json:"verdict"`
				} `json:"evaluation"`
			}
			if json.Unmarshal([]byte(data), &rep) == nil {
				completed = true
				finalScore = rep.Evaluation.Score
				finalGrade = rep.Evaluation.Grade
				finalVerdictText = firstNonEmpty(rep.Evaluation.Summary, rep.Evaluation.Verdict, fmt.Sprintf("综合得分 %.0f（%s 级）", finalScore, finalGrade))
			}
		default:
			if emit != nil {
				var msg struct {
					Title   string `json:"title"`
					Message string `json:"message"`
					ProbeID string `json:"probeId"`
				}
				if json.Unmarshal([]byte(data), &msg) == nil {
					txt := firstNonEmpty(msg.Title, msg.Message, msg.ProbeID)
					if txt != "" {
						emit(probeStreamFrame{Type: "status", Message: scrubText(txt, 140)})
					}
				}
			}
		}
	}
	if !completed {
		return probeReportView{}, errors.New("少点套路流连接未返回完整取证结论")
	}
	verdict := "ok"
	if finalScore < 75 || finalGrade == "C" || finalGrade == "D" || finalGrade == "F" {
		verdict = "degraded"
	}
	return probeReportView{
		Verdict:        verdict,
		Engine:         "gpt-relay-inspector",
		Model:          model,
		Effort:         effort,
		DurationMs:     time.Since(start).Milliseconds(),
		Summary:        scrubText(finalVerdictText, 240),
		InspectorScore: &finalScore,
		InspectorGrade: finalGrade,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

const defaultCandyPrompt = "桌上有 9 颗草莓味糖果、14 颗柠檬味糖果和 7 颗薄荷糖。吃掉 3 颗柠檬糖并把 6 颗草莓糖换成薄荷糖后，现在柠檬糖和草莓糖加起来共有多少颗？只输出最终阿拉伯数字。"

func runCustomGatewayRequest(ctx context.Context, gatewayBase, apiKey string, req probeRequest, emit func(probeStreamFrame)) (probeReportView, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "gpt-5.4"
	}
	userPrompt := strings.TrimSpace(req.Prompt)
	matchKind := strings.TrimSpace(req.MatchKind)
	expected := strings.TrimSpace(req.Expected)
	if userPrompt == "" {
		userPrompt = defaultCandyPrompt
		if matchKind == "" {
			matchKind = "exact_number"
			expected = "14"
		}
	}
	if matchKind == "" {
		matchKind = "status_ok"
	}
	sysPrompt := strings.TrimSpace(req.SystemPrompt)
	protocol := strings.ToLower(strings.TrimSpace(req.Protocol))
	if protocol == "" {
		protocol = "chat_completions"
	}

	var endpoint string
	var bodyMap map[string]any
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "text/event-stream",
	}

	switch protocol {
	case "messages":
		endpoint = strings.TrimRight(gatewayBase, "/") + "/v1/messages"
		headers["x-api-key"] = apiKey
		headers["anthropic-version"] = "2023-06-01"
		bodyMap = map[string]any{
			"model":      model,
			"max_tokens": 2048,
			"stream":     true,
			"messages":   []map[string]string{{"role": "user", "content": userPrompt}},
		}
		if sysPrompt != "" {
			bodyMap["system"] = sysPrompt
		}
	case "responses":
		endpoint = strings.TrimRight(gatewayBase, "/") + "/v1/responses"
		headers["Authorization"] = "Bearer " + apiKey
		inputItems := make([]map[string]string, 0, 2)
		if sysPrompt != "" {
			inputItems = append(inputItems, map[string]string{"role": "system", "content": sysPrompt})
		}
		inputItems = append(inputItems, map[string]string{"role": "user", "content": userPrompt})
		bodyMap = map[string]any{
			"model":  model,
			"stream": true,
			"input":  inputItems,
		}
		if req.Effort != "" {
			bodyMap["reasoning"] = map[string]string{"effort": req.Effort}
		}
	default:
		endpoint = strings.TrimRight(gatewayBase, "/") + "/v1/chat/completions"
		headers["Authorization"] = "Bearer " + apiKey
		msgs := make([]map[string]string, 0, 2)
		if sysPrompt != "" {
			msgs = append(msgs, map[string]string{"role": "system", "content": sysPrompt})
		}
		msgs = append(msgs, map[string]string{"role": "user", "content": userPrompt})
		bodyMap = map[string]any{
			"model":          model,
			"stream":         true,
			"stream_options": map[string]bool{"include_usage": true},
			"messages":       msgs,
		}
		if req.Effort != "" {
			bodyMap["reasoning_effort"] = req.Effort
		}
	}

	rawBody, _ := json.Marshal(bodyMap)
	cctx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(cctx, http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return probeReportView{}, err
	}
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}
	if emit != nil {
		emit(probeStreamFrame{Type: "status", Message: fmt.Sprintf("正在请求 %s (%s)…", model, protocol)})
	}
	start := time.Now()
	resp, err := (&http.Client{Timeout: 80 * time.Second}).Do(httpReq)
	if err != nil {
		return probeReportView{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errRaw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return probeReportView{
			Verdict:    "failed",
			Engine:     "custom",
			Model:      model,
			Effort:     req.Effort,
			DurationMs: time.Since(start).Milliseconds(),
			Summary:    fmt.Sprintf("网关返回 HTTP %d", resp.StatusCode),
			Reply:      scrubText(string(errRaw), 300),
		}, nil
	}

	var reply strings.Builder
	var ttftMs, inTok, outTok, reasonTok, cacheTok int64
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 4096), 512<<10)
		for scanner.Scan() {
			line, ok := strings.CutPrefix(scanner.Text(), "data:")
			if !ok {
				continue
			}
			line = strings.TrimSpace(line)
			if line == "" || line == "[DONE]" {
				continue
			}
			chunk, uIn, uOut, uReason, uCache := parseGatewaySSEChunk(line)
			if uIn > 0 {
				inTok = uIn
			}
			if uOut > 0 {
				outTok = uOut
			}
			if uReason > 0 {
				reasonTok = uReason
			}
			if uCache > 0 {
				cacheTok = uCache
			}
			if chunk != "" {
				if ttftMs == 0 {
					ttftMs = time.Since(start).Milliseconds()
				}
				safe := scrubText(chunk, 400)
				if reply.Len() < 4000 {
					reply.WriteString(chunk)
				}
				if emit != nil {
					emit(probeStreamFrame{Type: "delta", Delta: safe})
				}
			}
		}
	} else {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
		ttftMs = time.Since(start).Milliseconds()
		txt, uIn, uOut, uReason, uCache := parseGatewayJSONReply(raw)
		inTok, outTok, reasonTok, cacheTok = uIn, uOut, uReason, uCache
		reply.WriteString(txt)
	}

	fullReply := scrubText(reply.String(), 3000)
	dur := time.Since(start).Milliseconds()
	matched, summary := evaluateReplyMatch(fullReply, matchKind, expected)
	verdict := "ok"
	if !matched {
		verdict = "degraded"
	}
	micros := (inTok*3 + outTok*15) // 轻量基准微美元估算
	return probeReportView{
		Verdict:         verdict,
		Engine:          "custom",
		Model:           model,
		Effort:          req.Effort,
		DurationMs:      dur,
		TTFTMs:          ttftMs,
		InputTokens:     inTok,
		OutputTokens:    outTok,
		ReasoningTokens: reasonTok,
		CachedTokens:    cacheTok,
		EstimatedMicros: micros,
		Summary:         summary,
		Reply:           fullReply,
	}, nil
}

func parseGatewaySSEChunk(raw string) (delta string, inTok, outTok, reasonTok, cacheTok int64) {
	var frame struct {
		Type  string `json:"type"`
		Delta any    `json:"delta"`
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			InputTokens      int64 `json:"input_tokens"`
			OutputTokens     int64 `json:"output_tokens"`
			CacheRead        int64 `json:"cache_read_input_tokens"`
			CompletionDetail *struct {
				Reasoning int64 `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
			PromptDetail *struct {
				Cached int64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
		Response *struct {
			Usage *struct {
				InputTokens  int64 `json:"input_tokens"`
				OutputTokens int64 `json:"output_tokens"`
				OutputDetail *struct {
					Reasoning int64 `json:"reasoning_tokens"`
				} `json:"output_tokens_details"`
				InputDetail *struct {
					Cached int64 `json:"cached_tokens"`
				} `json:"input_tokens_details"`
			} `json:"usage"`
		} `json:"response"`
	}
	if json.Unmarshal([]byte(raw), &frame) != nil {
		return "", 0, 0, 0, 0
	}
	if len(frame.Choices) > 0 {
		delta = frame.Choices[0].Delta.Content
	} else if s, ok := frame.Delta.(string); ok {
		delta = s
	} else if m, ok := frame.Delta.(map[string]any); ok {
		if t, ok := m["text"].(string); ok {
			delta = t
		}
	}
	if frame.Usage != nil {
		inTok = max(frame.Usage.PromptTokens, frame.Usage.InputTokens)
		outTok = max(frame.Usage.CompletionTokens, frame.Usage.OutputTokens)
		cacheTok = frame.Usage.CacheRead
		if frame.Usage.CompletionDetail != nil {
			reasonTok = frame.Usage.CompletionDetail.Reasoning
		}
		if frame.Usage.PromptDetail != nil && frame.Usage.PromptDetail.Cached > cacheTok {
			cacheTok = frame.Usage.PromptDetail.Cached
		}
	}
	if frame.Response != nil && frame.Response.Usage != nil {
		u := frame.Response.Usage
		inTok, outTok = u.InputTokens, u.OutputTokens
		if u.OutputDetail != nil {
			reasonTok = u.OutputDetail.Reasoning
		}
		if u.InputDetail != nil {
			cacheTok = u.InputDetail.Cached
		}
	}
	return delta, inTok, outTok, reasonTok, cacheTok
}

func parseGatewayJSONReply(raw []byte) (text string, inTok, outTok, reasonTok, cacheTok int64) {
	var env struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		OutputText string `json:"output_text"`
		Usage      struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			InputTokens      int64 `json:"input_tokens"`
			OutputTokens     int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(raw, &env)
	if len(env.Choices) > 0 {
		text = env.Choices[0].Message.Content
	} else if len(env.Content) > 0 {
		text = env.Content[0].Text
	} else {
		text = env.OutputText
	}
	return text, max(env.Usage.PromptTokens, env.Usage.InputTokens), max(env.Usage.CompletionTokens, env.Usage.OutputTokens), 0, 0
}

func evaluateReplyMatch(reply, kind, expected string) (bool, string) {
	if strings.TrimSpace(reply) == "" {
		return false, "模型返回空内容"
	}
	switch kind {
	case "contains":
		if expected == "" || strings.Contains(strings.ToLower(reply), strings.ToLower(expected)) {
			return true, fmt.Sprintf("回答包含预期特征「%s」", expected)
		}
		return false, fmt.Sprintf("回答未包含预期特征「%s」", expected)
	case "exact_number":
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(expected) + `\b`)
		if expected == "" || re.MatchString(reply) {
			return true, fmt.Sprintf("回答命中独立数字 %s，符合本题判定", expected)
		}
		return false, fmt.Sprintf("回答未出现独立数字 %s，判定为降智或答错", expected)
	case "regex":
		re, err := regexp.Compile(expected)
		if err != nil {
			return false, "判定正则表达式无效"
		}
		if re.MatchString(reply) {
			return true, "回答命中正则规则"
		}
		return false, "回答未命中正则规则"
	default:
		return true, "流式响应正常完成"
	}
}

var (
	guestRateMu sync.Mutex
	guestHits   = map[string][]int64{}
)

func allowGuestIP(ip string, limitPerMin int) bool {
	guestRateMu.Lock()
	defer guestRateMu.Unlock()
	now := time.Now().Unix()
	cutoff := now - 60
	list := guestHits[ip]
	kept := list[:0]
	for _, ts := range list {
		if ts > cutoff {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= limitPerMin {
		guestHits[ip] = kept
		return false
	}
	guestHits[ip] = append(kept, now)
	return true
}

func (p *panel) publicGuestProbe(w http.ResponseWriter, r *http.Request) error {
	_, _, _, guestEnabled, _ := p.patrolVisibility()
	if !guestEnabled {
		return fail(403, "not_allowed")
	}
	input, err := decodeBody[struct {
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
		Protocol string `json:"protocol,omitempty"`
		Effort   string `json:"effort,omitempty"`
	}](r)
	if err != nil {
		return err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	apiKey := strings.TrimSpace(input.APIKey)
	model := strings.TrimSpace(input.Model)
	if baseURL == "" || apiKey == "" || model == "" || len(apiKey) > 512 || len(model) > 128 {
		return fail(400, "invalid_input")
	}
	u, pErr := url.Parse(baseURL)
	if pErr != nil || u.Scheme != "https" || isLoopbackHost(baseURL) {
		return fail(400, "invalid_input")
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if host == "" {
		host = r.RemoteAddr
	}
	rt, _ := p.ensurePlugins()
	_, gCfg := rt.ConfigFor("guest-self-check")
	rpm, _ := gCfg["rpm_per_ip"].(int)
	if rpm < 1 {
		rpm = 4
	}
	if !allowGuestIP(host, rpm) {
		return fail(429, "rate_limited")
	}
	slog.Info("guest self check started", "trace_id", meta(r.Context()).Trace, "model", model)
	report, rErr := runCustomGatewayRequest(r.Context(), baseURL, apiKey, probeRequest{
		Engine:   "custom",
		Model:    model,
		Protocol: input.Protocol,
		Effort:   input.Effort,
	}, nil)
	apiKey = ""
	if rErr != nil {
		return fail(502, "operation_rejected")
	}
	writeJSON(w, 200, report)
	return nil
}

func formatProbeMicrosUSD(micros int64) string {
	return strconv.FormatFloat(float64(micros)/1_000_000, 'f', 4, 64)
}
