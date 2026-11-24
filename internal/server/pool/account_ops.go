package pool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

type quotaWire struct {
	Usage   *float64   `json:"utilization"`
	Unknown bool       `json:"utilization_unknown"`
	Reset   *time.Time `json:"resets_at"`
}

type quotaView struct {
	Usage *float64   `json:"percent_used"`
	Reset *time.Time `json:"resets_at"`
}

type accountQuotaWire struct {
	Five             *quotaWire `json:"five_hour"`
	Week             *quotaWire `json:"seven_day"`
	SonnetWeek       *quotaWire `json:"seven_day_sonnet"`
	GeminiDaily      *quotaWire `json:"gemini_shared_daily"`
	GeminiProDaily   *quotaWire `json:"gemini_pro_daily"`
	GeminiFlashDaily *quotaWire `json:"gemini_flash_daily"`
	ThirtyDay        *quotaWire `json:"thirty_day"`
	SubscriptionTier string     `json:"subscription_tier"`
	NeedsReauth      bool       `json:"needs_reauth"`
	IsBanned         bool       `json:"is_banned"`
	NeedsVerify      bool       `json:"needs_verify"`
}

type accountQuotaView struct {
	Five             *quotaView `json:"five_hours"`
	Week             *quotaView `json:"week"`
	SonnetWeek       *quotaView `json:"sonnet_week,omitempty"`
	GeminiDaily      *quotaView `json:"gemini_daily,omitempty"`
	GeminiProDaily   *quotaView `json:"gemini_pro_daily,omitempty"`
	GeminiFlashDaily *quotaView `json:"gemini_flash_daily,omitempty"`
	ThirtyDay        *quotaView `json:"thirty_day,omitempty"`
	SubscriptionTier string     `json:"subscription_tier,omitempty"`
	NeedsReauth      bool       `json:"needs_reauth,omitempty"`
	IsBanned         bool       `json:"is_banned,omitempty"`
	NeedsVerify      bool       `json:"needs_verify,omitempty"`
}

type accountWire struct {
	platform.NamedItem
	Platform    string `json:"platform"`
	Status      string `json:"status"`
	Schedulable *bool  `json:"schedulable"`
}

func projectQuotaInfo(raw accountQuotaWire) accountQuotaView {
	project := func(window *quotaWire) *quotaView {
		if window == nil {
			return nil
		}
		usage := window.Usage
		if window.Unknown {
			usage = nil
		}
		return &quotaView{Usage: usage, Reset: window.Reset}
	}
	return accountQuotaView{
		Five: project(raw.Five), Week: project(raw.Week),
		SonnetWeek: project(raw.SonnetWeek), GeminiDaily: project(raw.GeminiDaily),
		GeminiProDaily: project(raw.GeminiProDaily), GeminiFlashDaily: project(raw.GeminiFlashDaily),
		ThirtyDay:        project(raw.ThirtyDay),
		SubscriptionTier: raw.SubscriptionTier, NeedsReauth: raw.NeedsReauth,
		IsBanned: raw.IsBanned, NeedsVerify: raw.NeedsVerify,
	}
}

func (h *handler) accountQuota(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	source := "passive"
	if r.URL.Query().Get("source") == "active" {
		source = "active"
	}
	raw, err := platform.Upstream[accountQuotaWire](r.Context(), h.s.Upstream, "GET", fmt.Sprintf("/admin/accounts/%d/usage?source=%s", id, source), a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, 200, projectQuotaInfo(raw))
	return nil
}

func (h *handler) scheduleAccount(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		On *bool `json:"schedulable"`
	}](r)
	if err != nil {
		return err
	}
	if input.On == nil {
		return platform.Fail(400, "invalid_input")
	}
	path := fmt.Sprintf("/admin/accounts/%d", id)
	_, err = platform.Upstream[struct{}](r.Context(), h.s.Upstream, "POST", path+"/schedulable", a.Tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	data, err := platform.Upstream[accountWire](r.Context(), h.s.Upstream, "GET", path, a.Tokens.Access, "", "", nil)
	if err != nil {
		return err
	}
	if data.ID != id || data.Schedulable == nil {
		return platform.Fail(502, "invalid_upstream")
	}
	if *data.Schedulable != *input.On {
		return platform.Fail(409, "schedule_changed")
	}
	platform.WriteJSON(w, 200, struct{}{})
	return nil
}

func (h *handler) testAccount(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[struct {
		Model string `json:"model_id,omitempty"`
	}](r)
	if err != nil || len(input.Model) > 128 {
		return platform.Fail(400, "invalid_input")
	}
	stream := h.s.Upstream.WithTimeout(70 * time.Second)
	res, err := stream.Open(r.Context(), "POST", fmt.Sprintf("/api/v1/admin/accounts/%d/test", id), a.Tokens.Access, "", "", input)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return platform.UpstreamStatus{Status: res.StatusCode}
	}
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		return platform.Fail(502, "invalid_upstream")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 2048), 256<<10)
	start := time.Now()
	emitAccountTest(w, "running")
	for scanner.Scan() {
		frame, ok := strings.CutPrefix(scanner.Text(), "data:")
		if !ok {
			continue
		}
		var evt struct {
			Type    string `json:"type"`
			Success bool   `json:"success"`
		}
		if json.Unmarshal([]byte(frame), &evt) == nil {
			if evt.Type == "test_complete" && evt.Success {
				emitAccountTest(w, "passed")
				slogAccountResult(r.Context(), id, true, start)
				return nil
			}
			if evt.Type == "error" || evt.Type == "test_complete" {
				break
			}
		}
	}
	emitAccountTest(w, "failed")
	slogAccountResult(r.Context(), id, false, start)
	return nil
}

func emitAccountTest(w http.ResponseWriter, state string) {
	fmt.Fprintf(w, "data: {\"state\":\"%s\"}\n\n", state)
	_ = http.NewResponseController(w).Flush()
}

func slogAccountResult(ctx context.Context, id int64, passed bool, start time.Time) {
	slog.Info("account test finished", "trace_id", platform.Meta(ctx).Trace, "account_id", id,
		"passed", passed, "elapsed_ms", time.Since(start).Milliseconds())
}
