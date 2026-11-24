// 变更类型: 新事实
// 权威来源: usage_cursors 表为未配官方只读库时按日用量观测分页游标的派生缓存
// 查询方式: 按 scope 唯一查询或批量 In 查询，按 updated 清理超期缓存 → Unique(scope)
// 保留期限: 95 天后由后台采样链路按 updated 自动清理
// 旧数据: 派生缓存表，可随时从官方数据源重建
// 关联决策: docs/decisions/0005-发码授权账本与用量快照领域数据模型.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type UsageCursor struct{ ent.Schema }

func (UsageCursor) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope").Unique(),
		field.String("cursor"),
		field.Int64("sampled_at").Positive(),
		field.Int64("updated").Positive(),
	}
}
