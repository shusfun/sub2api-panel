package observe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

func BuildUsageStreamSnapshot(s *core.Server) func(context.Context, string, string) ([]byte, error) {
	return func(ctx context.Context, token, rawQuery string) ([]byte, error) {
		query, err := url.ParseQuery(rawQuery)
		if err != nil {
			return nil, platform.Fail(400, "invalid_input")
		}
		filter, err := ParseUsageFilter(query, time.Now().In(s.UsageZone()), true)
		if err != nil {
			return nil, err
		}
		actor, err := strconv.ParseInt(query.Get("actor"), 10, 64)
		if err != nil || actor < 1 {
			return nil, platform.Fail(400, "invalid_input")
		}
		target, err := usageTarget(query, actor)
		if err != nil {
			return nil, err
		}
		admin := query.Get("admin") == "true"
		if admin {
			current, err := platform.CurrentUser(ctx, s.Upstream, token)
			if err != nil {
				return nil, err
			}
			if current.Role != "admin" || current.ID != actor {
				return nil, platform.Fail(403, "not_allowed")
			}
			if target == 0 {
				query.Set("user_id", "all")
			} else {
				query.Set("user_id", strconv.FormatInt(target, 10))
			}
			if err := verifyDashboardKey(s, ctx, token, target, target != actor, filter.Key); err != nil {
				return nil, err
			}
		}
		var summary UsageSummary
		if admin {
			summary, err = BuildAdminUsageSummary(s, ctx, token, query, 1)
		} else {
			summary, err = fetchSelfUsageSummary(s, ctx, token, filter, 1)
		}
		if err != nil {
			return nil, err
		}
		if query.Get("analysis") == "models" {
			if err := populateObservedUsage(s, ctx, DashboardScope(s, actor, query, admin), filter, &summary, token, target, admin); err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				slog.Info("usage observations unavailable", "trace_id", platform.Meta(ctx).Trace, "error", err)
				summary.ModelSeries.Unavailable = true
			}
		}
		return json.Marshal(summary)
	}
}

func usageStream(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		query := r.URL.Query()
		if query.Has("user_id") || query.Has("group_id") || query.Has("account_id") {
			return platform.Fail(400, "invalid_input")
		}
		filter, err := ParseUsageFilter(query, time.Now().In(s.UsageZone()), true)
		if err != nil {
			return err
		}
		if err := verifyDashboardKey(s, r.Context(), a.Tokens.Access, a.User.ID, false, filter.Key); err != nil {
			return err
		}
		return openUsageStream(s, w, r, a, query, false)
	}
}

func adminUsageStream(s *core.Server) core.Task {
	return func(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
		query, err := AdminUsageQuery(s, r.Context(), a, r.URL.Query())
		if err != nil {
			return err
		}
		if _, err := ParseUsageFilter(query, time.Now().In(s.UsageZone()), true); err != nil {
			return err
		}
		return openUsageStream(s, w, r, a, query, true)
	}
}

func openUsageStream(s *core.Server, w http.ResponseWriter, r *http.Request, a core.SessionAuth, query url.Values, admin bool) error {
	if s.UsageHub == nil {
		return platform.Fail(503, "service_unavailable")
	}
	cursor := query.Get("cursor")
	query.Del("cursor")
	query.Set("actor", strconv.FormatInt(a.User.ID, 10))
	query.Set("admin", strconv.FormatBool(admin))
	scope := DashboardScope(s, a.User.ID, query, admin)
	previous, err := ResumeUsage(r.Context(), s, scope, cursor)
	if err != nil {
		return err
	}
	ch, unsubscribe := s.UsageHub.Subscribe(r.Context(), scope, a.Tokens.Access, query.Encode(), a.User.ID, platform.Meta(r.Context()))
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
		var event core.UsageStreamEvent
		select {
		case <-r.Context().Done():
			return nil
		case <-heartbeat.C:
			event = core.UsageStreamEvent{Type: "heartbeat", ServerTime: time.Now().UTC().Format(time.RFC3339Nano)}
		case payload := <-ch:
			if json.Unmarshal(payload, &event) != nil {
				continue
			}
			if len(event.Snapshot) > 0 && string(event.Snapshot) != "null" {
				var next UsageSummary
				if json.Unmarshal(event.Snapshot, &next) == nil {
					if previous != nil && next.Start == previous.Start && next.End == previous.End {
						if cursor == event.Cursor {
							event.Type = "heartbeat"
						} else {
							event.Type, event.Base, event.Delta = "delta", cursor, usageChanges(previous, &next)
						}
						event.Snapshot = nil
					}
					nextCopy := next
					previous, cursor = &nextCopy, event.Cursor
				}
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

func usageChanges(previous, next *UsageSummary) map[string]json.RawMessage {
	before, after := usageFields(previous), usageFields(next)
	changes := make(map[string]json.RawMessage)
	for key, value := range after {
		if !bytes.Equal(before[key], value) {
			changes[key] = value
		}
	}
	return changes
}

func usageFields(summary *UsageSummary) map[string]json.RawMessage {
	payload, _ := json.Marshal(summary)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(payload, &fields)
	return fields
}
