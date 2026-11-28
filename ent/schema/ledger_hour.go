// 变更类型: 新事实
// 权威来源: ledger_hours 表保存每个账号每小时（按 PANEL_USAGE_TIMEZONE 切分）的本站收入、预计花费与上游实际花费
// 查询方式: 按账号与小时桶 upsert 与区间查询，按 bucket 清理过期行 → Unique(account_id, bucket)、Index(bucket)
// 保留期限: 默认 365 天后由后台汇总链路按 bucket 自动清理（可由 ledger_settings.hour_retention_days 调整）
// 旧数据: 既有基线表，无缺失值
// 关联决策: docs/decisions/0008-上游账本对账与站内告警.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type LedgerHour struct{ ent.Schema }

func (LedgerHour) Fields() []ent.Field {
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

func (LedgerHour) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "bucket").Unique(),
		index.Fields("bucket"),
	}
}
