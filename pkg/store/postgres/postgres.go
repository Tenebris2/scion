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

// Package postgres provides a PostgreSQL implementation of the Store interface.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/store"
)

// compile-time interface check
var _ store.Store = (*PostgresStore)(nil)

// PostgresStore implements the Store interface using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// New creates a new PostgresStore connected to the given DSN.
func New(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(8)
	return &PostgresStore{db: db}, nil
}

// Close closes the database connection.
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for direct access in tests.
func (s *PostgresStore) DB() *sql.DB {
	return s.db
}

// Ping checks database connectivity.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// migrationLockKey is a stable pg_advisory_lock key shared by all Hub replicas.
const migrationLockKey = 0x5C104DB5 // "SCION" in hex-ish

// Migrate applies outstanding database migrations under an advisory lock so that
// concurrent Hub replicas do not race on first startup.
func (s *PostgresStore) Migrate(ctx context.Context) error {
	// Acquire session-level advisory lock — released automatically when the
	// connection is returned to the pool (or closed).
	if _, err := s.db.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("postgres migrate: acquire advisory lock: %w", err)
	}
	defer s.db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey) //nolint:errcheck

	// Ensure schema_migrations table exists before we query it.
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`); err != nil {
		return fmt.Errorf("postgres migrate: create schema_migrations: %w", err)
	}

	var current int
	row := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations")
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("postgres migrate: query current version: %w", err)
	}

	type migration struct {
		version int
		sql     string
	}
	migrations := []migration{
		{1, migrationV1},
	}

	for _, m := range migrations {
		if current >= m.version {
			continue
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("postgres migrate v%d: begin tx: %w", m.version, err)
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("postgres migrate v%d: apply: %w", m.version, err)
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)",
			m.version, time.Now().UTC(),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("postgres migrate v%d: record version: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("postgres migrate v%d: commit: %w", m.version, err)
		}
	}
	return nil
}
