// 变更类型: 新事实
// 权威来源: runtime_settings 表按 key 保存已确认的运行期上游地址、加密余额探测凭据（含 access_token）与 ledger_settings 账本设置
// 查询方式: 按 key 唯一查询或按 account_balance/ 前缀扫描 → Unique(key)
// 保留期限: 永久（随管理员更新或移除对应账号余额配置、账本设置维护）
// 旧数据: 既有基线表，无缺失值
// 关联决策: docs/decisions/0005-发码授权账本与用量快照领域数据模型.md
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type RuntimeSetting struct{ ent.Schema }

func (RuntimeSetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").Unique(),
		field.String("value"),
		field.Int64("updated").Positive(),
	}
}
