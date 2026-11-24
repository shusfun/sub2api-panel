package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type usageStreamEvent struct {
	Type       string                     `json:"type"`
	Cursor     string                     `json:"cursor,omitempty"`
	Snapshot   *usageSummary              `json:"snapshot,omitempty"`
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
	trace        callMeta
	channels     map[chan []byte]callMeta
	latest       []byte
	latestEvent  []byte
	version      int64
}

type usageStreamHub struct {
	mu      sync.Mutex
	entries map[string]*usageStreamSubscriber
	build   func(context.Context, string, string) ([]byte, error)
	commit  func(context.Context, string, []byte) (int64, error)
}

func newUsageStreamHub(build func(context.Context, string, string) ([]byte, error)) *usageStreamHub {
	return &usageStreamHub{entries: make(map[string]*usageStreamSubscriber), build: build}
}

func (h *usageStreamHub) subscribe(ctx context.Context, scope, token, query string, _ int64, trace callMeta) (<-chan []byte, func()) {
	h.mu.Lock()
	entry := h.entries[scope]
	if entry == nil {
		worker, cancel := context.WithCancel(context.WithoutCancel(ctx))
		entry = &usageStreamSubscriber{ctx: worker, cancel: cancel, query: query, trace: trace, channels: make(map[chan []byte]callMeta)}
		h.entries[scope] = entry
		go h.sample(scope, entry)
	}
	entry.token = token
	ch := make(chan []byte, 1)
	entry.channels[ch] = trace
	if entry.latestEvent != nil {
		var event usageStreamEvent
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

func (h *usageStreamHub) sample(scope string, entry *usageStreamSubscriber) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	failed := false
	for {
		h.mu.Lock()
		token, trace := entry.token, entry.trace
		h.mu.Unlock()
		if trace.Trace == "" {
			trace = callMeta{Trace: randomHex(16), Span: randomHex(8)}
		}
		ctx := context.WithValue(entry.ctx, metaKey{}, trace)
		payload, err := h.build(ctx, token, entry.query)
		if ctx.Err() != nil {
			return
		}
		changed := !bytes.Equal(payload, entry.latest)
		if err == nil && (changed || failed) {
			version := entry.version
			if changed {
				version++
				if h.commit != nil {
					version, err = h.commit(ctx, scope, payload)
				}
			}
			if err == nil {
				var snapshot usageSummary
				err = json.Unmarshal(payload, &snapshot)
				if err == nil {
					entry.latest, entry.version = payload, version
					h.broadcast(entry, usageStreamEvent{Type: "snapshot", Cursor: scope + "." + strconv.FormatInt(version, 10), Snapshot: &snapshot}, trace)
					failed = false
				}
			}
		}
		if err != nil && ctx.Err() == nil {
			reason := "upstream_error"
			var problem *problem
			if errors.As(err, &problem) && (problem.Status == 401 || problem.Status == 403) {
				reason = "not_allowed"
			}
			if !failed || reason == "not_allowed" {
				slog.Info("usage sample unavailable", "trace_id", trace.Trace, "reason", reason, "error", err)
				h.broadcast(entry, usageStreamEvent{Type: "unavailable", Reason: reason}, trace)
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

func (h *usageStreamHub) broadcast(entry *usageStreamSubscriber, event usageStreamEvent, trace callMeta) {
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

func usageScope(userID int64, query string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s", userID, query)))
	return hex.EncodeToString(hash[:])
}

func (p *panel) buildUsageStreamSnapshot(ctx context.Context, token, rawQuery string) ([]byte, error) {
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, fail(400, "invalid_input")
	}
	filter, err := parseUsageFilter(query, time.Now().In(p.usageZone()), true)
	if err != nil {
		return nil, err
	}
	actor, err := strconv.ParseInt(query.Get("actor"), 10, 64)
	if err != nil || actor < 1 {
		return nil, fail(400, "invalid_input")
	}
	target, err := usageTarget(query, actor)
	if err != nil {
		return nil, err
	}
	admin := query.Get("admin") == "true"
	if admin {
		current, err := currentUser(ctx, p.upstream, token)
		if err != nil {
			return nil, err
		}
		if current.Role != "admin" || current.ID != actor {
			return nil, fail(403, "not_allowed")
		}
		if target == 0 {
			query.Set("user_id", "all")
		} else {
			query.Set("user_id", strconv.FormatInt(target, 10))
		}
		if err := p.verifyDashboardKey(ctx, token, target, target != actor, filter.key); err != nil {
			return nil, err
		}
	}
	var summary usageSummary
	if admin {
		summary, err = p.buildAdminUsageSummary(ctx, token, query, 1)
	} else {
		summary, err = p.fetchSelfUsageSummary(ctx, token, filter, 1)
	}
	if err != nil {
		return nil, err
	}
	if query.Get("analysis") == "models" {
		if err := p.populateObservedUsage(ctx, p.dashboardScope(actor, query, admin), filter, &summary, token, target, admin); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			slog.Info("usage observations unavailable", "trace_id", meta(ctx).Trace, "error", err)
			summary.ModelSeries.Unavailable = true
		}
	}
	return json.Marshal(summary)
}

func (p *panel) usageStream(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	query := r.URL.Query()
	if query.Has("user_id") || query.Has("group_id") || query.Has("account_id") {
		return fail(400, "invalid_input")
	}
	filter, err := parseUsageFilter(query, time.Now().In(p.usageZone()), true)
	if err != nil {
		return err
	}
	if err := p.verifyDashboardKey(r.Context(), a.tokens.Access, a.user.ID, false, filter.key); err != nil {
		return err
	}
	return p.openUsageStream(w, r, a, query, false)
}

func (p *panel) adminUsageStream(w http.ResponseWriter, r *http.Request, a sessionAuth) error {
	query, err := p.adminUsageQuery(r.Context(), a, r.URL.Query())
	if err != nil {
		return err
	}
	if _, err := parseUsageFilter(query, time.Now().In(p.usageZone()), true); err != nil {
		return err
	}
	return p.openUsageStream(w, r, a, query, true)
}

func (p *panel) openUsageStream(w http.ResponseWriter, r *http.Request, a sessionAuth, query url.Values, admin bool) error {
	if p.usageHub == nil {
		return fail(503, "service_unavailable")
	}
	cursor := query.Get("cursor")
	query.Del("cursor")
	query.Set("actor", strconv.FormatInt(a.user.ID, 10))
	query.Set("admin", strconv.FormatBool(admin))
	scope := p.dashboardScope(a.user.ID, query, admin)
	previous, err := p.resumeUsage(r.Context(), scope, cursor)
	if err != nil {
		return err
	}
	ch, unsubscribe := p.usageHub.subscribe(r.Context(), scope, a.tokens.Access, query.Encode(), a.user.ID, meta(r.Context()))
	defer unsubscribe()
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fail(500, "service_unavailable")
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
		var event usageStreamEvent
		select {
		case <-r.Context().Done():
			return nil
		case <-heartbeat.C:
			event = usageStreamEvent{Type: "heartbeat", ServerTime: time.Now().UTC().Format(time.RFC3339Nano)}
		case payload := <-ch:
			if json.Unmarshal(payload, &event) != nil {
				continue
			}
			if event.Snapshot != nil {
				next := event.Snapshot
				if previous != nil && next.Start == previous.Start && next.End == previous.End {
					if cursor == event.Cursor {
						event.Type = "heartbeat"
					} else {
						event.Type, event.Base, event.Delta = "delta", cursor, usageChanges(previous, next)
					}
					event.Snapshot = nil
				}
				previous, cursor = next, event.Cursor
			}
		}
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		_ = response.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payload); err != nil {
			return nil
		}
		flusher.Flush()
		_ = response.SetWriteDeadline(time.Time{})
		if event.Reason == "not_allowed" {
			return nil
		}
	}
}

func usageChanges(previous, next *usageSummary) map[string]json.RawMessage {
	before, after := usageFields(previous), usageFields(next)
	changes := make(map[string]json.RawMessage)
	for key, value := range after {
		if !bytes.Equal(before[key], value) {
			changes[key] = value
		}
	}
	return changes
}

func usageFields(summary *usageSummary) map[string]json.RawMessage {
	payload, _ := json.Marshal(summary)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(payload, &fields)
	return fields
}
