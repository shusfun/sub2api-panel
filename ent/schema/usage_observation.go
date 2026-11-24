// 变更类型: 新事实
// 权威来源: usage_observations 表为未配官方只读库时逐笔用量记录的派生聚合缓存
// 查询方式: 按 (scope, upstream_id) 唯一去重、按 (scope, created_at, upstream_id) 分桶聚合与按 created_at 清理超期行 → Unique(scope, upstream_id), Index(scope, created_at, upstream_id)
// 保留期限: 95 天后由后台采样链路按 created_at 自动清理
// 旧数据: 派生缓存表，可随时从官方数据源重建
// 关联决策: docs/decisions/0005-发码授权账本与用量快照领域数据模型.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type UsageObservation struct{ ent.Schema }

func (UsageObservation) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope"),
		field.Int64("upstream_id").Positive(),
		field.String("model"),
		field.String("key_name").Default(""),
		field.Time("created_at"),
		field.Int64("input_tokens").NonNegative(),
		field.Int64("output_tokens").NonNegative(),
		field.Int64("cache_read_tokens").NonNegative(),
		field.Int64("cache_creation_tokens").NonNegative(),
		field.Float("actual_cost").Optional().Nillable(),
		field.Int64("first_seen").Positive(),
		field.Int64("last_seen").Positive(),
		field.String("fingerprint"),
	}
}

func (UsageObservation) Indexes() []ent.Index {
	return []ent.Index{index.Fields("scope", "upstream_id").Unique(), index.Fields("scope", "created_at", "upstream_id")}
}
