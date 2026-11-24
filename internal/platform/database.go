package platform

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"sub2api-panel/ent"
)

func ValidateDatabaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" || parsed.Host == "" {
		return fmt.Errorf("invalid PostgreSQL database URL")
	}
	return nil
}

func OpenDatabase(ctx context.Context, raw string) (*ent.Client, error) {
	if err := ValidateDatabaseURL(raw); err != nil {
		return nil, err
	}
	db, err := sql.Open("pgx", raw)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db))), nil
}

func MigrateDatabase(ctx context.Context, raw string) error {
	client, err := OpenDatabase(ctx, raw)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Schema.Create(ctx)
}
