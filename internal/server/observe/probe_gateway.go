package observe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

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
		Type    string `json:"type"`
		Delta   any    `json:"delta"`
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
