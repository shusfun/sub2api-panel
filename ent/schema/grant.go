// 变更类型: 新事实
// 权威来源: grants 表按 user_id 唯一保存子管理员当前发码配额与授权世代 epoch
// 查询方式: 按 user_id 唯一查询与事务更新 → Unique(user_id)
// 保留期限: 永久（撤权时清零额度并递增 epoch，保留行供对账校验）
// 旧数据: 既有基线表，无缺失值
// 关联决策: docs/decisions/0005-发码授权账本与用量快照领域数据模型.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Grant struct{ ent.Schema }

func (Grant) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id").Unique(),
		field.Int64("invites").NonNegative(),
		field.Int64("budget").NonNegative(),
		field.JSON("amounts", []int64{}),
		field.Int64("epoch").Default(1).Positive(),
	}
}
