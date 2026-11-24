// 变更类型: 新事实
// 权威来源: usage_snapshots 表保存每个活跃查询 scope 最新一版实时用量汇总快照与游标
// 查询方式: 按 scope 唯一读写最新快照，按 sampled_at 清理超期快照 → Unique(scope)
// 保留期限: 95 天后由后台采样链路按 sampled_at 自动清理
// 旧数据: 派生缓存表，可随时从官方数据源重建
// 关联决策: docs/decisions/0005-发码授权账本与用量快照领域数据模型.md
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type SnapshotTokens struct {
	Input      int64 `json:"input_tokens"`
	Output     int64 `json:"output_tokens"`
	CacheRead  int64 `json:"cache_read_tokens"`
	CacheWrite int64 `json:"cache_creation_tokens"`
}

type SnapshotTotals struct {
	SnapshotTokens
	Requests int64   `json:"requests"`
	Cost     float64 `json:"actual_cost"`
}

type SnapshotDay struct {
	Date string `json:"date"`
	SnapshotTokens
	Requests int64   `json:"requests"`
	Cost     float64 `json:"actual_cost"`
}

type SnapshotBucket struct {
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	Pending bool      `json:"pending,omitempty"`
	SnapshotTotals
}

type SnapshotModel struct {
	Model    string  `json:"model"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"total_tokens"`
	Cost     float64 `json:"actual_cost"`
}

type SnapshotModelPage struct {
	Items []SnapshotModel `json:"items"`
	More  bool            `json:"more"`
	Pages int             `json:"pages,omitempty"`
}

type SnapshotModelBucket struct {
	Start  time.Time       `json:"start"`
	Models []SnapshotModel `json:"models"`
}

type SnapshotModelSeries struct {
	ReadyDays   int                   `json:"ready_days"`
	TotalDays   int                   `json:"total_days"`
	Buckets     []SnapshotModelBucket `json:"buckets"`
	Unavailable bool                  `json:"unavailable,omitempty"`
}

type UsageSnapshotDoc struct {
	Timezone    string              `json:"timezone"`
	Start       string              `json:"start_date"`
	End         string              `json:"end_date"`
	Totals      SnapshotTotals      `json:"totals"`
	Days        []SnapshotDay       `json:"days"`
	Models      SnapshotModelPage   `json:"models"`
	Granularity string              `json:"granularity"`
	Buckets     []SnapshotBucket    `json:"buckets"`
	Ranking     []SnapshotModel     `json:"ranking"`
	ModelSeries SnapshotModelSeries `json:"model_series"`
}

type UsageSnapshot struct{ ent.Schema }

func (UsageSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope").Unique(),
		field.String("filter_key"),
		field.Int64("version").Positive(),
		field.Int64("sampled_at").Positive(),
		field.String("cursor"),
		field.String("fingerprint"),
		field.JSON("payload", UsageSnapshotDoc{}),
	}
}
