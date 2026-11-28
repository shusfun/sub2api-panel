// 变更类型: 新事实
// 权威来源: ledger_events 表保存充值、推断充值、人工调整、退款、额度上调与基线重置事件，是充值与调整的唯一账本
// 查询方式: 按钱包与发生时间倒序列出事件，按钱包加 external_id 去重 → Unique(wallet_id, external_id)、Index(wallet_id, occurred_at)
// 保留期限: 永久（财务事件不可自动删除，作废只改 status）
// 旧数据: 既有基线表，无缺失值
// 关联决策: docs/decisions/0008-上游账本对账与站内告警.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type LedgerEvent struct{ ent.Schema }

func (LedgerEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("wallet_id").Positive(),
		field.Int64("account_id").Optional().Nillable(),
		field.String("kind"),
		field.Float("amount_usd"),
		field.Int64("occurred_at").Positive(),
		field.String("source").Default(""),
		field.String("external_id").Optional().Nillable(),
		field.String("status").Default("confirmed"),
		field.String("note").Default(""),
		field.Int64("created").Positive(),
		field.Int64("updated").Positive(),
	}
}

func (LedgerEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("wallet_id", "external_id").Unique(),
		index.Fields("wallet_id", "occurred_at"),
	}
}
