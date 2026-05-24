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

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/store"
)

func (s *PostgresStore) CreateGCPServiceAccount(ctx context.Context, sa *store.GCPServiceAccount) error {
	if sa.CreatedAt.IsZero() {
		sa.CreatedAt = time.Now()
	}

	scopesStr := strings.Join(sa.DefaultScopes, ",")

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO gcp_service_accounts (id, scope, scope_id, email, project_id, display_name, default_scopes, verified, verified_at, created_by, created_at, managed, managed_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
		sa.ID, sa.Scope, sa.ScopeID, sa.Email, sa.ProjectID, sa.DisplayName,
		scopesStr, boolToInt(sa.Verified), nullableTime(sa.VerifiedAt), sa.CreatedBy, sa.CreatedAt,
		boolToInt(sa.Managed), sa.ManagedBy,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return store.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (s *PostgresStore) GetGCPServiceAccount(ctx context.Context, id string) (*store.GCPServiceAccount, error) {
	var sa store.GCPServiceAccount
	var scopesStr string
	var verifiedAt sql.NullTime
	var verified, managed int

	err := s.db.QueryRowContext(ctx, `
		SELECT id, scope, scope_id, email, project_id, display_name, default_scopes, verified, verified_at, created_by, created_at, managed, managed_by
		FROM gcp_service_accounts WHERE id = $1
	`, id).Scan(
		&sa.ID, &sa.Scope, &sa.ScopeID, &sa.Email, &sa.ProjectID, &sa.DisplayName,
		&scopesStr, &verified, &verifiedAt, &sa.CreatedBy, &sa.CreatedAt,
		&managed, &sa.ManagedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	sa.Verified = verified != 0
	sa.Managed = managed != 0
	if scopesStr != "" {
		sa.DefaultScopes = strings.Split(scopesStr, ",")
	}
	if verifiedAt.Valid {
		sa.VerifiedAt = verifiedAt.Time
	}
	return &sa, nil
}

func (s *PostgresStore) UpdateGCPServiceAccount(ctx context.Context, sa *store.GCPServiceAccount) error {
	scopesStr := strings.Join(sa.DefaultScopes, ",")

	result, err := s.db.ExecContext(ctx, `
		UPDATE gcp_service_accounts
		SET email = $1, project_id = $2, display_name = $3, default_scopes = $4, verified = $5, verified_at = $6, managed = $7, managed_by = $8
		WHERE id = $9
	`,
		sa.Email, sa.ProjectID, sa.DisplayName, scopesStr, boolToInt(sa.Verified), nullableTime(sa.VerifiedAt),
		boolToInt(sa.Managed), sa.ManagedBy, sa.ID,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) DeleteGCPServiceAccount(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM gcp_service_accounts WHERE id = $1", id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) ListGCPServiceAccounts(ctx context.Context, filter store.GCPServiceAccountFilter) ([]store.GCPServiceAccount, error) {
	var conditions []string
	var args []interface{}
	idx := 1

	conditions = append(conditions, "1=1")

	if filter.Scope != "" {
		conditions = append(conditions, fmt.Sprintf("scope = $%d", idx))
		args = append(args, filter.Scope)
		idx++
	}
	if filter.ScopeID != "" {
		conditions = append(conditions, fmt.Sprintf("scope_id = $%d", idx))
		args = append(args, filter.ScopeID)
		idx++
	}
	if filter.Email != "" {
		conditions = append(conditions, fmt.Sprintf("email = $%d", idx))
		args = append(args, filter.Email)
		idx++
	}
	if filter.Managed != nil {
		conditions = append(conditions, fmt.Sprintf("managed = $%d", idx))
		args = append(args, boolToInt(*filter.Managed))
		idx++
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, scope, scope_id, email, project_id, display_name, default_scopes, verified, verified_at, created_by, created_at, managed, managed_by
		FROM gcp_service_accounts WHERE %s ORDER BY created_at DESC
	`, strings.Join(conditions, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []store.GCPServiceAccount
	for rows.Next() {
		var sa store.GCPServiceAccount
		var scopesStr string
		var verifiedAt sql.NullTime
		var verified, managed int

		if err := rows.Scan(
			&sa.ID, &sa.Scope, &sa.ScopeID, &sa.Email, &sa.ProjectID, &sa.DisplayName,
			&scopesStr, &verified, &verifiedAt, &sa.CreatedBy, &sa.CreatedAt,
			&managed, &sa.ManagedBy,
		); err != nil {
			return nil, err
		}

		sa.Verified = verified != 0
		sa.Managed = managed != 0
		if scopesStr != "" {
			sa.DefaultScopes = strings.Split(scopesStr, ",")
		}
		if verifiedAt.Valid {
			sa.VerifiedAt = verifiedAt.Time
		}
		results = append(results, sa)
	}
	return results, rows.Err()
}

func (s *PostgresStore) CountGCPServiceAccounts(ctx context.Context, filter store.GCPServiceAccountFilter) (int, error) {
	var conditions []string
	var args []interface{}
	idx := 1

	conditions = append(conditions, "1=1")

	if filter.Scope != "" {
		conditions = append(conditions, fmt.Sprintf("scope = $%d", idx))
		args = append(args, filter.Scope)
		idx++
	}
	if filter.ScopeID != "" {
		conditions = append(conditions, fmt.Sprintf("scope_id = $%d", idx))
		args = append(args, filter.ScopeID)
		idx++
	}
	if filter.Email != "" {
		conditions = append(conditions, fmt.Sprintf("email = $%d", idx))
		args = append(args, filter.Email)
		idx++
	}
	if filter.Managed != nil {
		conditions = append(conditions, fmt.Sprintf("managed = $%d", idx))
		args = append(args, boolToInt(*filter.Managed))
		idx++
	}

	var count int
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(
		"SELECT COUNT(*) FROM gcp_service_accounts WHERE %s",
		strings.Join(conditions, " AND "),
	), args...).Scan(&count)
	return count, err
}
