// 变更类型: 新事实
// 权威来源: ledger_accounts 表保存账号与钱包的归属关系及名称、平台快照，账号删除只打标记
// 查询方式: 按 account_id 唯一查询，按 wallet_id 列出同钱包账号 → Unique(account_id)、Index(wallet_id)
// 保留期限: 永久（账号删除只写 deleted_at，账本保留）
// 旧数据: 既有基线表，无缺失值
// 关联决策: docs/decisions/0008-上游账本对账与站内告警.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type LedgerAccount struct{ ent.Schema }

func (LedgerAccount) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id").Positive().Unique(),
		field.Int64("wallet_id").Positive(),
		field.String("key_fingerprint").Default(""),
		field.String("name").Default(""),
		field.String("platform").Default(""),
		field.Int64("deleted_at").Optional().Nillable(),
		field.Int64("created").Positive(),
		field.Int64("updated").Positive(),
	}
}

func (LedgerAccount) Indexes() []ent.Index {
	return []ent.Index{index.Fields("wallet_id")}
}
