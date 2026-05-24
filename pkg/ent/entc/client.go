// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package entc provides factory functions for creating Ent clients with
// SQLite or PostgreSQL backends.
package entc

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/GoogleCloudPlatform/scion/pkg/ent"
	"github.com/GoogleCloudPlatform/scion/pkg/ent/migrate"
	_ "github.com/jackc/pgx/v5/stdlib" // Registers "pgx" driver for Postgres
)

// OpenSQLite creates an Ent client backed by SQLite.
// The dsn should be a SQLite connection string (e.g. "file:ent?mode=memory&cache=shared").
// Foreign keys and WAL journal mode are enabled automatically.
// This uses the modernc.org/sqlite pure-Go driver which registers as "sqlite".
func OpenSQLite(dsn string, opts ...ent.Option) (*ent.Client, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite connection: %w", err)
	}
	// Enable foreign keys and WAL mode, matching existing store pattern.
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(append(opts, ent.Driver(drv))...)
	return client, nil
}

// OpenPostgres creates an Ent client backed by PostgreSQL.
// The dsn should be a PostgreSQL connection string
// (e.g. "host=localhost port=5432 user=scion dbname=scion sslmode=disable").
func OpenPostgres(dsn string, opts ...ent.Option) (*ent.Client, error) {
	client, err := ent.Open(dialect.Postgres, dsn, opts...)
	if err != nil {
		return nil, fmt.Errorf("opening postgres connection: %w", err)
	}
	return client, nil
}

// AutoMigrate runs automatic schema migration on the given client.
func AutoMigrate(ctx context.Context, client *ent.Client) error {
	if err := client.Schema.Create(ctx, migrate.WithDropIndex(true), migrate.WithDropColumn(true)); err != nil {
		return fmt.Errorf("running auto-migration: %w", err)
	}
	return nil
}

// appendSchemaToPostgresDSN appends a schema name to the PostgreSQL DSN.
// Handles both URL format (postgres://...) and key=value format.
func appendSchemaToPostgresDSN(dsn, schema string) (string, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", fmt.Errorf("parsing postgres URL: %w", err)
		}
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		return u.String(), nil
	}

	// Key-value format (e.g., "host=localhost user=scion dbname=scion")
	if !strings.Contains(dsn, "search_path=") {
		return dsn + " search_path=" + schema, nil
	}
	return dsn, nil
}

// OpenPostgresInSchema opens a Postgres Ent client using a dedicated schema.
// If the schema doesn't exist, it creates it. All Ent tables will be created
// in this schema, avoiding conflicts with tables from other database systems.
func OpenPostgresInSchema(ctx context.Context, dsn, schema string, opts ...ent.Option) (*ent.Client, error) {
	// Step 1: Open a raw SQL connection to create the schema in the default schema
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening pgx connection for schema creation: %w", err)
	}
	defer db.Close()

	// Create the schema if it doesn't exist
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, schema)); err != nil {
		return nil, fmt.Errorf("creating postgres schema %q: %w", schema, err)
	}

	// Step 2: Modify the DSN to set search_path to the schema
	schemaDSN, err := appendSchemaToPostgresDSN(dsn, schema)
	if err != nil {
		return nil, fmt.Errorf("appending schema to DSN: %w", err)
	}

	// Step 3: Open the actual Ent client with the schema-specific DSN
	client, err := OpenPostgres(schemaDSN, opts...)
	if err != nil {
		return nil, fmt.Errorf("opening postgres connection with schema %q: %w", schema, err)
	}

	return client, nil
}
