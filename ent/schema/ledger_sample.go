// 变更类型: 新事实
// 权威来源: ledger_samples 表保存每次上游余额与累计已用采样，余额、累计已用、倍率都没变的采样只更新 last_seen
// 查询方式: 按账号与时间倒序取最近采样，按 last_seen 清理过期行 → Index(account_id, last_seen)、Index(last_seen)
// 保留期限: 默认 30 天后由后台汇总链路按 last_seen 自动清理（可由 ledger_settings.sample_retention_days 调整）
// 旧数据: 既有基线表，无缺失值
// 关联决策: docs/decisions/0008-上游账本对账与站内告警.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type LedgerSample struct{ ent.Schema }

func (LedgerSample) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("wallet_id").Positive(),
		field.Int64("account_id").Positive(),
		field.Int64("first_seen").Positive(),
		field.Int64("last_seen").Positive(),
		field.Float("balance_usd").Optional().Nillable(),
		field.Float("key_used_usd").Optional().Nillable(),
		field.Float("wallet_used_usd").Optional().Nillable(),
		field.Float("multiplier").Optional().Nillable(),
		field.String("kind"),
		field.String("currency").Default("USD"),
	}
}

func (LedgerSample) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "last_seen"),
		index.Fields("last_seen"),
	}
}
