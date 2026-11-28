// Package schema 描述面板只读访问的官方 Sub2API 表。这些表归官方所有：
// 这里只声明读取用到的列，不参与 panel migrate，也不通过它们写入。
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

type UsageLog struct{ ent.Schema }

func (UsageLog) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "usage_logs"}}
}

func (UsageLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Int64("user_id").Optional().Nillable().Immutable(),
		field.Int64("api_key_id").Optional().Nillable().Immutable(),
		field.Int64("group_id").Optional().Nillable().Immutable(),
		field.Int64("account_id").Immutable(),
		field.String("model").Default("").Immutable(),
		field.Int64("input_tokens").Default(0).Immutable(),
		field.Int64("output_tokens").Default(0).Immutable(),
		field.Int64("cache_read_tokens").Default(0).Immutable(),
		field.Int64("cache_creation_tokens").Default(0).Immutable(),
		field.Float("total_cost").Default(0).Immutable(),
		field.Float("actual_cost").Immutable(),
		field.Float("account_rate_multiplier").Optional().Nillable().Immutable(),
		field.Float("account_stats_cost").Optional().Nillable().Immutable(),
		field.Int8("billing_type").Default(0).Immutable(),
		field.Int("duration_ms").Optional().Nillable().Immutable(),
		field.Time("created_at").Immutable(),
	}
}

type OpsErrorLog struct{ ent.Schema }

func (OpsErrorLog) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ops_error_logs"}}
}

func (OpsErrorLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Int64("account_id").Optional().Nillable().Immutable(),
		field.Time("created_at").Immutable(),
	}
}

type OfficialUser struct{ ent.Schema }

func (OfficialUser) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "users"}}
}

func (OfficialUser) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.String("email").Default("").Immutable(),
		field.String("username").Default("").Immutable(),
	}
}
