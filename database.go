package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"sub2api-panel/ent"
)

func databaseURL() string {
	return strings.TrimSpace(required("PANEL_DATABASE_URL"))
}

func validateDatabaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" || parsed.Host == "" {
		return fmt.Errorf("invalid PostgreSQL database URL")
	}
	return nil
}

func openDatabase(ctx context.Context, raw string) (*ent.Client, error) {
	if err := validateDatabaseURL(raw); err != nil {
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

func migrateDatabase(ctx context.Context, raw string) error {
	client, err := openDatabase(ctx, raw)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Schema.Create(ctx)
}
