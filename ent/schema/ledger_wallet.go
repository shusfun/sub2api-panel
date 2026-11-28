// 变更类型: 新事实
// 权威来源: ledger_wallets 表保存上游钱包身份（Key 的 HMAC 指纹，或站点加 上游用户 ID 的指纹）与折算单位
// 查询方式: 按 identity 唯一查询，按 id 批量 In 查询 → Unique(identity)
// 保留期限: 永久（账号删除后钱包与账本一并保留）
// 旧数据: 既有基线表，无缺失值
// 关联决策: docs/decisions/0008-上游账本对账与站内告警.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type LedgerWallet struct{ ent.Schema }

func (LedgerWallet) Fields() []ent.Field {
	return []ent.Field{
		field.String("identity").Unique(),
		field.String("provider"),
		field.String("origin").Default(""),
		field.Float("quota_per_usd").Default(0),
		field.Int64("upstream_user_id").Default(0),
		field.Bool("auto_recharge").Default(true),
		field.Int64("created").Positive(),
		field.Int64("updated").Positive(),
	}
}
