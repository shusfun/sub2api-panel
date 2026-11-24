// 变更类型: 新事实
// 权威来源: probe_templates 表保存管理员自定义的提示词探针与命中规则配置
// 查询方式: 按主键 id 查询或按 updated_at 倒序展示全部模板 → 默认主键索引
// 保留期限: 永久（由管理员手动增删改）
// 旧数据: 新表，无历史行
// 关联决策: docs/decisions/0002-流量门控智能巡检与数据模型.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type ProbeTemplate struct{ ent.Schema }

func (ProbeTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.String("protocol").Default("chat_completions"), // responses | chat_completions | messages
		field.String("system_prompt").Default(""),
		field.String("user_prompt"),
		field.String("effort").Default(""),
		field.Bool("stream").Default(true),
		field.Int("max_tokens").Default(2048).Positive(),
		field.String("match_kind").Default("contains"), // contains | exact_number | regex | status_ok
		field.String("expected").Default(""),
		field.Int64("min_reasoning_tokens").Default(0).NonNegative(),
		field.Int64("max_ttft_ms").Default(0).NonNegative(),
		field.Int64("updated_at").Positive(),
	}
}
