// 变更类型: 新事实
// 权威来源: patrol_runs 表为只追加的分组巡检与低峰免检事实账本
// 查询方式: 按 (group_id, created_at DESC) 取最近 60 次历史柱状条，按 created_at 清理过期记录及统计当日预算 → Index(group_id, created_at), Index(created_at)
// 保留期限: 默认 30 天后由后台巡检协程按 created_at 自动清理
// 旧数据: 新表，无历史行
// 关联决策: docs/decisions/0002-流量门控智能巡检与数据模型.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type PatrolRun struct{ ent.Schema }

func (PatrolRun) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_id").Positive(),
		field.String("group_name").Default(""),
		field.String("trigger").Default("scheduled"), // scheduled | peak | recheck | manual | idle_skip | budget_fuse
		field.String("verdict").Default("unknown"),   // ok | degraded | failed | skipped_idle
		field.String("engine").Default(""),
		field.String("model").Default(""),
		field.String("effort").Default(""),
		field.Int("active_users").Default(0).NonNegative(),
		field.Int64("window_requests").Default(0).NonNegative(),
		field.Int64("duration_ms").Default(0).NonNegative(),
		field.Int64("ttft_ms").Default(0).NonNegative(),
		field.Int64("input_tokens").Default(0).NonNegative(),
		field.Int64("output_tokens").Default(0).NonNegative(),
		field.Int64("reasoning_tokens").Default(0).NonNegative(),
		field.Int64("cached_tokens").Default(0).NonNegative(),
		field.Int64("estimated_micros").Default(0).NonNegative(),
		field.Float("confidence").Optional().Nillable(),
		field.String("predicted_model").Default(""),
		field.String("summary").Default(""),
		field.String("reply_excerpt").Default(""),
		field.Time("created_at").Immutable(),
	}
}

func (PatrolRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "created_at"),
		index.Fields("created_at"),
	}
}
