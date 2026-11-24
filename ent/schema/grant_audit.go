// 变更类型: 新事实
// 权威来源: grant_audits 表为只追加的发码配额调整与撤权审计账本
// 查询方式: 按主键顺序写入与审计核查 → 默认主键索引
// 保留期限: 永久（额度授权审计记录不可删除）
// 旧数据: 既有基线表，无缺失值
// 关联决策: docs/decisions/0005-发码授权账本与用量快照领域数据模型.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type GrantAudit struct{ ent.Schema }

func (GrantAudit) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("actor").Positive(),
		field.Int64("target").Positive(),
		field.Int64("invite_delta"),
		field.Int64("budget_delta"),
		field.String("action"),
		field.String("trace"),
		field.Int64("created").Positive(),
	}
}
