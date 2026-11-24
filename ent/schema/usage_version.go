// 变更类型: 新事实
// 权威来源: usage_versions 表为实时用量 SSE 断线游标恢复使用的拉链历史快照表
// 查询方式: 按 (scope, version) 唯一读取基准快照并裁剪超出 64 版的旧行，按 valid_from 清理超期行 → Unique(scope, version)
// 保留期限: 每个 scope 最多保留最近 64 版，且超过 95 天未活动的版本由后台自动清理
// 旧数据: 派生缓存表，游标缺失时自动回退下发完整最新快照
// 关联决策: docs/decisions/0005-发码授权账本与用量快照领域数据模型.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type UsageVersion struct{ ent.Schema }

func (UsageVersion) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope"),
		field.Int64("version").Positive(),
		field.Int64("valid_from").Positive(),
		field.Int64("valid_to").Optional().Nillable(),
		field.String("fingerprint"),
		field.JSON("payload", UsageSnapshotDoc{}),
	}
}

func (UsageVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("scope", "version").Unique()}
}
