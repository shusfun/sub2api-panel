package observe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/schema"
	"sub2api-panel/ent/usagecursor"
	"sub2api-panel/ent/usageobservation"
	"sub2api-panel/ent/usagesnapshot"
	"sub2api-panel/ent/usageversion"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

const retainedUsageVersions = 64
const usageCacheRetentionDays = 95

var lastUsageCachePruneUnix atomic.Int64

func maybePruneUsageCache(s *core.Server, ctx context.Context) {
	if s.Database == nil {
		return
	}
	now := time.Now().Unix()
	prev := lastUsageCachePruneUnix.Load()
	if now-prev < 3600 || !lastUsageCachePruneUnix.CompareAndSwap(prev, now) {
		return
	}
	bg, end := platform.BackgroundFlow(ctx, "清理用量过期缓存")
	defer end()
	cutoffTime := time.Now().AddDate(0, 0, -usageCacheRetentionDays)
	cutoffMillis := cutoffTime.UnixMilli()
	_, _ = s.Database.UsageObservation.Delete().Where(usageobservation.CreatedAtLT(cutoffTime)).Exec(bg)
	_, _ = s.Database.UsageCursor.Delete().Where(usagecursor.UpdatedLT(cutoffMillis)).Exec(bg)
	_, _ = s.Database.UsageSnapshot.Delete().Where(usagesnapshot.SampledAtLT(cutoffMillis)).Exec(bg)
	_, _ = s.Database.UsageVersion.Delete().Where(usageversion.ValidFromLT(cutoffMillis)).Exec(bg)
}

func CommitUsageSnapshot(s *core.Server) func(context.Context, string, []byte) (int64, error) {
	return func(ctx context.Context, scope string, data []byte) (int64, error) {
		maybePruneUsageCache(s, ctx)
		hash := sha256.Sum256(data)
		fingerprint := hex.EncodeToString(hash[:])
		var payload schema.UsageSnapshotDoc
		if err := json.Unmarshal(data, &payload); err != nil {
			return 0, err
		}
		tx, err := s.Database.Tx(ctx)
		if err != nil {
			return 0, err
		}
		defer tx.Rollback()
		current, err := tx.UsageSnapshot.Query().Where(usagesnapshot.ScopeEQ(scope)).Only(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return 0, err
		}
		if current != nil && current.Fingerprint == fingerprint {
			return current.Version, nil
		}
		version := int64(1)
		now := time.Now().UnixMilli()
		if current != nil {
			version = current.Version + 1
			if _, err := tx.UsageVersion.Update().Where(usageversion.ScopeEQ(scope), usageversion.VersionEQ(current.Version)).SetValidTo(now).Save(ctx); err != nil {
				return 0, err
			}
		}
		cursor := scope + "." + strconv.FormatInt(version, 10)
		if current == nil {
			err = tx.UsageSnapshot.Create().SetScope(scope).SetFilterKey(scope).SetVersion(version).SetCursor(cursor).
				SetSampledAt(now).SetFingerprint(fingerprint).SetPayload(payload).Exec(ctx)
		} else {
			err = tx.UsageSnapshot.UpdateOneID(current.ID).Where(usagesnapshot.VersionEQ(current.Version)).SetVersion(version).SetCursor(cursor).
				SetSampledAt(now).SetFingerprint(fingerprint).SetPayload(payload).Exec(ctx)
		}
		if err != nil {
			return 0, err
		}
		if err := tx.UsageVersion.Create().SetScope(scope).SetVersion(version).SetValidFrom(now).SetFingerprint(fingerprint).SetPayload(payload).Exec(ctx); err != nil {
			return 0, err
		}
		if version > retainedUsageVersions {
			if _, err := tx.UsageVersion.Delete().Where(usageversion.ScopeEQ(scope), usageversion.VersionLTE(version-retainedUsageVersions)).Exec(ctx); err != nil {
				return 0, err
			}
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		slog.Info("usage snapshot committed", "trace_id", platform.Meta(ctx).Trace, "version", version, "sampled_at", now)
		return version, nil
	}
}

func ResumeUsage(ctx context.Context, s *core.Server, scope, cursor string) (*UsageSummary, error) {
	if s.Database == nil || !strings.HasPrefix(cursor, scope+".") {
		return nil, nil
	}
	version, err := strconv.ParseInt(strings.TrimPrefix(cursor, scope+"."), 10, 64)
	if err != nil || version < 1 {
		return nil, nil
	}
	row, err := s.Database.UsageVersion.Query().Where(usageversion.ScopeEQ(scope), usageversion.VersionEQ(version)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(row.Payload)
	if err != nil {
		return nil, err
	}
	var summary UsageSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}
