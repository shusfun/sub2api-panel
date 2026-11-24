// 变更类型: 新事实
// 权威来源: issues 表按幂等操作号 op 保存每次发码预扣、上游码 ID 与退回结算状态
// 查询方式: 按 op 唯一幂等查询、按 actor 与主键分页查询发码历史 → Unique(op)
// 保留期限: 永久（发码对账与预扣结算账本）
// 旧数据: 既有基线表，无缺失值
// 关联决策: docs/decisions/0005-发码授权账本与用量快照领域数据模型.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Issue struct{ ent.Schema }

func (Issue) Fields() []ent.Field {
	return []ent.Field{
		field.String("op").Unique(),
		field.Int64("actor").Positive(),
		field.String("kind"),
		field.Int64("amount").NonNegative(),
		field.Int64("grant_epoch").NonNegative(),
		field.String("state"),
		field.Int64("code_id").Optional().Nillable().Positive(),
		field.Int64("created").Positive(),
		field.String("trace"),
		field.Bool("refunded").Default(false),
	}
}
