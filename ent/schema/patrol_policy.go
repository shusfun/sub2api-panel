// 变更类型: 新事实
// 权威来源: patrol_policies 表按 group_id 唯一保存每个调度分组的智能巡检与活跃门控配置
// 查询方式: 按 group_id 唯一查询、按 enabled 筛选调度、按 public_visible 筛选公开看板 → Unique(group_id), Index(enabled)
// 保留期限: 永久（随管理员删除巡检分组或关闭策略管理）
// 旧数据: 新表，无历史行
// 关联决策: docs/decisions/0002-流量门控智能巡检与数据模型.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type PatrolPolicy struct{ ent.Schema }

func (PatrolPolicy) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_id").Positive().Unique(),
		field.String("group_name").Default(""),
		field.String("platform").Default(""),
		field.Float("rate_multiplier").Default(1),
		field.Bool("enabled").Default(true),
		field.Bool("public_visible").Default(true),
		field.String("engine").Default("lmfpd"), // lmfpd | gpttest | template | official
		field.Int64("template_id").Optional().Nillable(),
		field.String("model").Default(""),
		field.String("protocol").Default("responses"), // responses | chat_completions | messages
		field.String("effort").Default(""),
		field.String("inspector_preset").Default("fast"),
		field.String("patrol_api_key").Default("").Sensitive(),
		field.Int("window_minutes").Default(15).Positive(),
		field.Int("min_active_users").Default(3).NonNegative(),
		field.Int("min_window_requests").Default(15).NonNegative(),
		field.Int("peak_active_users").Default(15).NonNegative(),
		field.Int("interval_minutes").Default(30).Positive(),
		field.Int("peak_interval_minutes").Default(15).Positive(),
		field.Int("recheck_minutes").Default(5).Positive(),
		field.Int64("daily_token_limit").Default(300000).NonNegative(),
		field.Int64("daily_budget_cents").Default(200).NonNegative(),
		field.Int64("next_check_at").Default(0),
		field.Int64("updated_at").Positive(),
	}
}

func (PatrolPolicy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled", "next_check_at"),
		index.Fields("public_visible"),
	}
}
