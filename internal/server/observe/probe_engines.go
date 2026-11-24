package observe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/plugins/probelmfpd"
)

func streamOfficialAccountProbe(ctx context.Context, s *core.Server, w http.ResponseWriter, token string, accountID int64, req probeRequest) {
	stream := s.Upstream.WithTimeout(75 * time.Second)
	body := map[string]any{}
	if strings.TrimSpace(req.Model) != "" {
		body["model_id"] = strings.TrimSpace(req.Model)
	}
	if strings.TrimSpace(req.Prompt) != "" {
		body["prompt"] = strings.TrimSpace(req.Prompt)
	}
	start := time.Now()
	emitProbeSSE(w, probeStreamFrame{Type: "status", Message: "正在通过 Sub2API 原生测活通道探测…"})
	res, err := stream.Open(ctx, "POST", fmt.Sprintf("/api/v1/admin/accounts/%d/test", accountID), token, "", "", body)
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

func executeGatewayProbe(ctx context.Context, s *core.Server, apiKey string, req probeRequest, emit func(probeStreamFrame)) (probeReportView, error) {
	internalBase, publicBase := gatewayBaseURL(ctx, s)
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
		svc := s.LMFPDService()
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
		_, ext := s.EnsurePlugins()
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
				finalVerdictText = platform.FirstNonEmpty(rep.Evaluation.Summary, rep.Evaluation.Verdict, fmt.Sprintf("综合得分 %.0f（%s 级）", finalScore, finalGrade))
			}
		default:
			if emit != nil {
				var msg struct {
					Title   string `json:"title"`
					Message string `json:"message"`
					ProbeID string `json:"probeId"`
				}
				if json.Unmarshal([]byte(data), &msg) == nil {
					txt := platform.FirstNonEmpty(msg.Title, msg.Message, msg.ProbeID)
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
