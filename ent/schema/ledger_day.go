// 变更类型: 新事实
// 权威来源: ledger_days 表保存每个账号每日（按 PANEL_USAGE_TIMEZONE 切分）的汇总，由小时表每日汇总而来，字段与 ledger_hours 一致
// 查询方式: 按账号与日桶 upsert 与区间查询 → Unique(account_id, bucket)、Index(bucket)
// 保留期限: 永久（小时表过期清理后，日表保留长期对账依据）
// 旧数据: 既有基线表，无缺失值
// 关联决策: docs/decisions/0008-上游账本对账与站内告警.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type LedgerDay struct{ ent.Schema }

func (LedgerDay) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id").Positive(),
		field.Int64("bucket").Positive(),
		field.Int64("requests").Default(0),
		field.Float("revenue_usd").Default(0),
		field.Float("expected_balance_cost_usd").Default(0),
		field.Float("expected_subscription_cost_usd").Default(0),
		field.Float("actual_cost_usd").Optional().Nillable(),
		field.String("actual_source").Default(""),
		field.Int64("updated").Positive(),
	}
}

func (LedgerDay) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "bucket").Unique(),
		index.Fields("bucket"),
	}
}
