// 变更类型: 新事实
// 权威来源: ledger_alerts 表保存站内告警（偏差、倍率变化、可撑时长、采样失败、疑似共用钱包、低余额、缺账单），同一账号同一类型最多一条处于 open
// 查询方式: 按状态与开启时间倒序列出，按账号加类型查开启中的告警 → Unique(account_id, kind) WHERE status = 'open'、Index(status, opened_at)
// 保留期限: 已恢复或已忽略的告警保留 90 天后由后台汇总链路按 resolved_at 自动清理
// 旧数据: 既有基线表，无缺失值
// 关联决策: docs/decisions/0008-上游账本对账与站内告警.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type LedgerAlert struct{ ent.Schema }

func (LedgerAlert) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id").Positive(),
		field.Int64("wallet_id").Optional().Nillable(),
		field.String("kind"),
		field.String("severity").Default("warn"),
		field.String("status").Default("open"),
		field.Int64("opened_at").Positive(),
		field.Int64("resolved_at").Optional().Nillable(),
		field.Float("value").Default(0),
		field.Int64("read_at").Optional().Nillable(),
		field.Int64("dismissed_at").Optional().Nillable(),
	}
}

func (LedgerAlert) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "kind").
			Unique().
			Annotations(entsql.IndexWhere("status = 'open'")),
		index.Fields("status", "opened_at"),
	}
}
