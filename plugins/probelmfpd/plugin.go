package probelmfpd

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lmfpd "github.com/Ikaleio/lm-detector/sdk/go"
	"sub2api-panel/internal/pluginrt"
)

const ServiceName = "probe.lmfpd"

type RunInput struct {
	BaseURL  string
	APIKey   string
	Model    string
	Protocol string // responses | chat_completions | messages
	Effort   string
	Count    int
}

type Candidate struct {
	Model       string   `json:"model"`
	DisplayName string   `json:"display_name"`
	Family      string   `json:"family"`
	Score       float64  `json:"score"`
	Probability *float64 `json:"probability,omitempty"`
}

type RunOutput struct {
	Decision       string      `json:"decision"`
	PredictedModel string      `json:"predicted_model"`
	PredictedName  string      `json:"predicted_name"`
	FamilyName     string      `json:"family_name"`
	Probability    *float64    `json:"probability,omitempty"`
	TTFTMs         int64       `json:"ttft_ms"`
	DurationMs     int64       `json:"duration_ms"`
	AcceptedCount  int         `json:"accepted_count"`
	BuiltAt        string      `json:"built_at,omitempty"`
	Excerpt        string      `json:"excerpt,omitempty"`
	Candidates     []Candidate `json:"candidates"`
	Error          string      `json:"error,omitempty"`
}

type Service struct {
	sources  []string
	cache    string
	ttl      time.Duration
	count    int
	parallel int

	mu       sync.Mutex
	detector *lmfpd.Detector
}

func (s *Service) ensureDetector(ctx context.Context) (*lmfpd.Detector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.detector != nil {
		return s.detector, nil
	}
	d, err := lmfpd.Load(ctx, lmfpd.LoadOptions{
		Sources: s.sources,
		Cache:   s.cache,
		TTL:     s.ttl,
	})
	if err != nil {
		return nil, err
	}
	s.detector = d
	return d, nil
}

func (s *Service) Run(ctx context.Context, in RunInput, onProgress func(step string)) (RunOutput, error) {
	if onProgress != nil {
		onProgress("正在准备 LM 指纹参考库…")
	}
	d, err := s.ensureDetector(ctx)
	if err != nil {
		return RunOutput{}, err
	}
	apiKind := lmfpd.APIResponses
	switch strings.ToLower(strings.TrimSpace(in.Protocol)) {
	case "chat_completions":
		apiKind = lmfpd.APIChatCompletions
	case "messages":
		apiKind = lmfpd.APIMessages
	}
	count := in.Count
	if count < 1 || count > 3 {
		count = s.count
	}
	parallel := min(s.parallel, count)
	start := time.Now()
	res, err := d.Detect(ctx, lmfpd.DetectConfig{
		BaseURL:  in.BaseURL,
		APIKey:   in.APIKey,
		Model:    in.Model,
		API:      apiKind,
		Effort:   strings.TrimSpace(in.Effort),
		Count:    count,
		Parallel: parallel,
		Timeout:  75 * time.Second,
	}, lmfpd.OnSample(func(index int, sample lmfpd.Sample) {
		if onProgress != nil && (sample.State == "waiting" || sample.State == "complete" || sample.State == "failed") {
			onProgress("样本 #" + string(rune('1'+index)) + " · " + sample.State)
		}
	}))
	if err != nil {
		return RunOutput{}, err
	}
	out := RunOutput{
		DurationMs: time.Since(start).Milliseconds(),
		BuiltAt:    d.BuiltAt(),
		Candidates: []Candidate{},
	}
	for _, smp := range res.Samples {
		if smp.FirstByteMs != nil && *smp.FirstByteMs > 0 && (out.TTFTMs == 0 || *smp.FirstByteMs < out.TTFTMs) {
			out.TTFTMs = *smp.FirstByteMs
		}
		if out.Excerpt == "" && smp.Text != "" {
			if len(smp.Text) > 220 {
				out.Excerpt = smp.Text[:220] + "…"
			} else {
				out.Excerpt = smp.Text
			}
		}
	}
	if res.Error != nil && *res.Error != "" {
		out.Error = *res.Error
		if res.Analysis == nil {
			return out, errors.New(*res.Error)
		}
	}
	if res.Analysis != nil {
		out.Decision = res.Analysis.Decision
		out.PredictedModel = res.Analysis.Prediction
		out.PredictedName = res.Analysis.PredictionName
		out.FamilyName = res.Analysis.FamilyPredictionName
		out.Probability = res.Analysis.Probability
		out.AcceptedCount = res.Analysis.UsedOutputs
		limit := min(5, len(res.Analysis.Results))
		for i := range limit {
			r := res.Analysis.Results[i]
			out.Candidates = append(out.Candidates, Candidate{
				Model:       r.Model,
				DisplayName: r.DisplayName,
				Family:      r.FamilyName,
				Score:       r.Score,
				Probability: r.Probability,
			})
		}
	}
	return out, nil
}

func init() {
	pluginrt.Register(pluginrt.Manifest{
		ID:          "probe-lmfpd",
		Name:        "LM 模型指纹检测 (lm.ikale.io)",
		Description: "基于数字分布特征识别真实底层模型，首次调用自动拉取参考库并按周期后台静默更新，无需手动维护题库。",
		Category:    "verifier",
		DefaultOn:   true,
		Settings: []pluginrt.Setting{
			{Key: "source_url", Title: "自定义参考库镜像源", Description: "留空默认依次使用 lm.ikale.io/data/ 与国内镜像。", Kind: "string", Default: ""},
			{Key: "ttl_hours", Title: "参考库更新检查周期（小时）", Kind: "int", Default: 24, Min: 1, Max: 168},
			{Key: "sample_count", Title: "每轮挑战样本数（1-3）", Kind: "int", Default: 3, Min: 1, Max: 3},
			{Key: "parallel", Title: "并发采样路数（1-3）", Kind: "int", Default: 3, Min: 1, Max: 3},
		},
		Apply: func(c *pluginrt.Ctx) error {
			dataDir, _ := c.Service("data-dir").(string)
			if dataDir == "" {
				dataDir = "./data"
			}
			var sources []string
			if custom, _ := c.Config["source_url"].(string); strings.TrimSpace(custom) != "" {
				u := strings.TrimSpace(custom)
				if !strings.HasSuffix(u, "/") {
					u += "/"
				}
				sources = []string{u}
			}
			ttlHours, _ := c.Config["ttl_hours"].(int)
			if ttlHours < 1 {
				ttlHours = 24
			}
			cnt, _ := c.Config["sample_count"].(int)
			if cnt < 1 || cnt > 3 {
				cnt = 3
			}
			par, _ := c.Config["parallel"].(int)
			if par < 1 || par > 3 {
				par = 3
			}
			svc := &Service{
				sources:  sources,
				cache:    filepath.Join(dataDir, "cache", "lmfpd-data.bin"),
				ttl:      time.Duration(ttlHours) * time.Hour,
				count:    cnt,
				parallel: par,
			}
			c.Provide(ServiceName, svc)
			c.Note("按需加载参考库 · 缓存自动热更")
			return nil
		},
	})
}
