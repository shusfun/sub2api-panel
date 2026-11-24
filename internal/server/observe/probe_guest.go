package observe

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

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

func publicGuestProbe(s *core.Server) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		_, _, _, guestEnabled, _ := s.PatrolVisibility()
		if !guestEnabled {
			return platform.Fail(403, "not_allowed")
		}
		input, err := platform.DecodeBody[struct {
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
			return platform.Fail(400, "invalid_input")
		}
		u, pErr := url.Parse(baseURL)
		if pErr != nil || u.Scheme != "https" || isLoopbackHost(baseURL) {
			return platform.Fail(400, "invalid_input")
		}
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if host == "" {
			host = r.RemoteAddr
		}
		rt, _ := s.EnsurePlugins()
		_, gCfg := rt.ConfigFor("guest-self-check")
		rpm, _ := gCfg["rpm_per_ip"].(int)
		if rpm < 1 {
			rpm = 4
		}
		if !allowGuestIP(host, rpm) {
			return platform.Fail(429, "rate_limited")
		}
		slog.Info("guest self check started", "trace_id", platform.Meta(r.Context()).Trace, "model", model)
		report, rErr := runCustomGatewayRequest(r.Context(), baseURL, apiKey, probeRequest{
			Engine:   "custom",
			Model:    model,
			Protocol: input.Protocol,
			Effort:   input.Effort,
		}, nil)
		apiKey = ""
		if rErr != nil {
			return platform.Fail(502, "operation_rejected")
		}
		platform.WriteJSON(w, 200, report)
		return nil
	}
}

func formatProbeMicrosUSD(micros int64) string {
	return strconv.FormatFloat(float64(micros)/1_000_000, 'f', 4, 64)
}
