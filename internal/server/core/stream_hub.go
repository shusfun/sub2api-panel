package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"sub2api-panel/internal/platform"
)

type UsageStreamEvent struct {
	Type       string                     `json:"type"`
	Cursor     string                     `json:"cursor,omitempty"`
	Snapshot   json.RawMessage            `json:"snapshot,omitempty"`
	Delta      map[string]json.RawMessage `json:"delta,omitempty"`
	Base       string                     `json:"base,omitempty"`
	RequestID  string                     `json:"request_id,omitempty"`
	SampleID   string                     `json:"sample_id,omitempty"`
	Reason     string                     `json:"reason,omitempty"`
	ServerTime string                     `json:"server_time"`
}

type usageStreamSubscriber struct {
	ctx          context.Context
	cancel       context.CancelFunc
	token, query string
	trace        platform.CallMeta
	channels     map[chan []byte]platform.CallMeta
	latest       []byte
	latestEvent  []byte
	version      int64
}

type UsageStreamHub struct {
	mu      sync.Mutex
	entries map[string]*usageStreamSubscriber
	Build   func(context.Context, string, string) ([]byte, error)
	Commit  func(context.Context, string, []byte) (int64, error)
}

func NewUsageStreamHub(build func(context.Context, string, string) ([]byte, error)) *UsageStreamHub {
	return &UsageStreamHub{entries: make(map[string]*usageStreamSubscriber), Build: build}
}

func (h *UsageStreamHub) Subscribe(ctx context.Context, scope, token, query string, _ int64, trace platform.CallMeta) (<-chan []byte, func()) {
	h.mu.Lock()
	entry := h.entries[scope]
	if entry == nil {
		worker, cancel := context.WithCancel(context.WithoutCancel(ctx))
		entry = &usageStreamSubscriber{ctx: worker, cancel: cancel, query: query, trace: trace, channels: make(map[chan []byte]platform.CallMeta)}
		h.entries[scope] = entry
		go h.sample(scope, entry)
	}
	entry.token = token
	ch := make(chan []byte, 1)
	entry.channels[ch] = trace
	if entry.latestEvent != nil {
		var event UsageStreamEvent
		if json.Unmarshal(entry.latestEvent, &event) == nil {
			event.RequestID, event.SampleID = trace.Trace, entry.trace.Trace
			if payload, err := json.Marshal(event); err == nil {
				ch <- payload
			}
		}
	}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.entries[scope] != entry {
			return
		}
		delete(entry.channels, ch)
		if len(entry.channels) == 0 {
			entry.cancel()
			delete(h.entries, scope)
		}
	}
}

func (h *UsageStreamHub) sample(scope string, entry *usageStreamSubscriber) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	failed := false
	for {
		h.mu.Lock()
		token, trace := entry.token, entry.trace
		h.mu.Unlock()
		if trace.Trace == "" {
			trace = platform.CallMeta{Trace: platform.RandomHex(16), Span: platform.RandomHex(8)}
		}
		ctx := platform.WithMeta(entry.ctx, trace)
		payload, err := h.Build(ctx, token, entry.query)
		if ctx.Err() != nil {
			return
		}
		changed := !bytes.Equal(payload, entry.latest)
		if err == nil && (changed || failed) {
			version := entry.version
			if changed {
				version++
				if h.Commit != nil {
					version, err = h.Commit(ctx, scope, payload)
				}
			}
			if err == nil && json.Valid(payload) {
				entry.latest, entry.version = payload, version
				h.broadcast(entry, UsageStreamEvent{Type: "snapshot", Cursor: scope + "." + strconv.FormatInt(version, 10), Snapshot: json.RawMessage(payload)}, trace)
				failed = false
			}
		}
		if err != nil && ctx.Err() == nil {
			reason := "upstream_error"
			var prob *platform.Problem
			if errors.As(err, &prob) && (prob.Status == 401 || prob.Status == 403) {
				reason = "not_allowed"
			}
			if !failed || reason == "not_allowed" {
				slog.Info("usage sample unavailable", "trace_id", trace.Trace, "reason", reason, "error", err)
				h.broadcast(entry, UsageStreamEvent{Type: "unavailable", Reason: reason}, trace)
			}
			failed = true
		}
		select {
		case <-entry.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *UsageStreamHub) broadcast(entry *usageStreamSubscriber, event UsageStreamEvent, trace platform.CallMeta) {
	event.ServerTime = time.Now().UTC().Format(time.RFC3339Nano)
	event.RequestID = trace.Trace
	event.SampleID = trace.Trace
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	entry.latestEvent = payload
	for ch, subscriber := range entry.channels {
		subscriberEvent := event
		subscriberEvent.RequestID, subscriberEvent.SampleID = subscriber.Trace, trace.Trace
		subscriberPayload, err := json.Marshal(subscriberEvent)
		if err != nil {
			continue
		}
		select {
		case <-ch:
		default:
		}
		ch <- subscriberPayload
	}
}

type OpsStreamEvent struct {
	Type       string          `json:"type"`
	Snapshot   json.RawMessage `json:"snapshot,omitempty"`
	RequestID  string          `json:"request_id,omitempty"`
	SampleID   string          `json:"sample_id,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	ServerTime string          `json:"server_time"`
}

type opsStreamSubscriber struct {
	ctx         context.Context
	cancel      context.CancelFunc
	token       string
	query       string
	trace       platform.CallMeta
	channels    map[chan []byte]platform.CallMeta
	latest      []byte
	latestEvent []byte
}

type OpsStreamHub struct {
	mu      sync.Mutex
	entries map[string]*opsStreamSubscriber
	Build   func(context.Context, string, string) ([]byte, error)
}

func NewOpsStreamHub(build func(context.Context, string, string) ([]byte, error)) *OpsStreamHub {
	return &OpsStreamHub{entries: make(map[string]*opsStreamSubscriber), Build: build}
}

func (h *OpsStreamHub) Subscribe(ctx context.Context, scope, token, query string, trace platform.CallMeta) (<-chan []byte, func()) {
	h.mu.Lock()
	entry := h.entries[scope]
	if entry == nil {
		worker, cancel := context.WithCancel(context.WithoutCancel(ctx))
		entry = &opsStreamSubscriber{ctx: worker, cancel: cancel, query: query, trace: trace, channels: make(map[chan []byte]platform.CallMeta)}
		h.entries[scope] = entry
		go h.sample(scope, entry)
	}
	entry.token = token
	ch := make(chan []byte, 1)
	entry.channels[ch] = trace
	if entry.latestEvent != nil {
		var event OpsStreamEvent
		if json.Unmarshal(entry.latestEvent, &event) == nil {
			event.RequestID, event.SampleID = trace.Trace, entry.trace.Trace
			if payload, err := json.Marshal(event); err == nil {
				ch <- payload
			}
		}
	}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.entries[scope] != entry {
			return
		}
		delete(entry.channels, ch)
		if len(entry.channels) == 0 {
			entry.cancel()
			delete(h.entries, scope)
		}
	}
}

func (h *OpsStreamHub) sample(_ string, entry *opsStreamSubscriber) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	failed := false
	for {
		h.mu.Lock()
		token, trace := entry.token, entry.trace
		h.mu.Unlock()
		ctx := platform.WithMeta(entry.ctx, trace)
		payload, err := h.Build(ctx, token, entry.query)
		if ctx.Err() != nil {
			return
		}
		changed := !bytes.Equal(payload, entry.latest)
		if err == nil && (changed || failed) {
			entry.latest = payload
			h.broadcast(entry, OpsStreamEvent{Type: "snapshot", Snapshot: json.RawMessage(payload)}, trace)
			failed = false
		}
		if err != nil && ctx.Err() == nil {
			reason := "upstream_error"
			var prob *platform.Problem
			if errors.As(err, &prob) && (prob.Status == 401 || prob.Status == 403) {
				reason = "not_allowed"
			}
			if !failed || reason == "not_allowed" {
				slog.Info("ops sample unavailable", "trace_id", trace.Trace, "reason", reason, "error", err)
				h.broadcast(entry, OpsStreamEvent{Type: "unavailable", Reason: reason}, trace)
			}
			failed = true
		}
		select {
		case <-entry.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *OpsStreamHub) broadcast(entry *opsStreamSubscriber, event OpsStreamEvent, trace platform.CallMeta) {
	event.ServerTime = time.Now().UTC().Format(time.RFC3339Nano)
	event.RequestID = trace.Trace
	event.SampleID = trace.Trace
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	entry.latestEvent = payload
	for ch, subscriber := range entry.channels {
		sub := event
		sub.RequestID, sub.SampleID = subscriber.Trace, trace.Trace
		subPayload, err := json.Marshal(sub)
		if err != nil {
			continue
		}
		select {
		case <-ch:
		default:
		}
		ch <- subPayload
	}
}

func ServeSnapshotStream(w http.ResponseWriter, r *http.Request, hub *OpsStreamHub, scope, token, query string) error {
	ch, unsubscribe := hub.Subscribe(r.Context(), scope, token, query, platform.Meta(r.Context()))
	defer unsubscribe()
	flusher, ok := w.(http.Flusher)
	if !ok {
		return platform.Fail(500, "service_unavailable")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	response := http.NewResponseController(w)
	_ = response.SetWriteDeadline(time.Time{})
	flusher.Flush()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		var payload []byte
		eventType := "snapshot"
		select {
		case <-r.Context().Done():
			return nil
		case <-heartbeat.C:
			eventType = "heartbeat"
			payload, _ = json.Marshal(struct {
				Type       string `json:"type"`
				ServerTime string `json:"server_time"`
			}{"heartbeat", time.Now().UTC().Format(time.RFC3339Nano)})
		case raw := <-ch:
			var envelope struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
			}
			if json.Unmarshal(raw, &envelope) == nil && envelope.Type != "" {
				eventType = envelope.Type
			}
			payload = raw
			if envelope.Reason == "not_allowed" {
				_ = response.SetWriteDeadline(time.Now().Add(10 * time.Second))
				_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload)
				flusher.Flush()
				return nil
			}
		}
		_ = response.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload); err != nil {
			return nil
		}
		flusher.Flush()
		_ = response.SetWriteDeadline(time.Time{})
	}
}
