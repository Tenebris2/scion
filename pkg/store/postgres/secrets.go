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

func (s *PostgresStore) CreateSecret(ctx context.Context, secret *store.Secret) error {
	now := time.Now()
	secret.Created = now
	secret.Updated = now
	secret.Version = 1

	if secret.SecretType == "" {
		secret.SecretType = store.SecretTypeEnvironment
	}
	if secret.Target == "" {
		secret.Target = secret.Key
	}
	if secret.InjectionMode == "" {
		secret.InjectionMode = store.InjectionModeAsNeeded
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO secrets (id, key, encrypted_value, secret_ref, secret_type, target, scope, scope_id, description, injection_mode, allow_progeny, version, created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`,
		secret.ID, secret.Key, secret.EncryptedValue, nullableString(secret.SecretRef),
		secret.SecretType, nullableString(secret.Target),
		secret.Scope, secret.ScopeID,
		secret.Description, secret.InjectionMode, boolToInt(secret.AllowProgeny), secret.Version,
		secret.Created, secret.Updated, secret.CreatedBy, secret.UpdatedBy,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return store.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (s *PostgresStore) GetSecret(ctx context.Context, key, scope, scopeID string) (*store.Secret, error) {
	secret := &store.Secret{}
	var secretRef, target sql.NullString
	var allowProgeny int

	err := s.db.QueryRowContext(ctx, `
		SELECT id, key, encrypted_value, secret_ref, secret_type, COALESCE(target, key), scope, scope_id, description, injection_mode, allow_progeny, version, created_at, updated_at, created_by, updated_by
		FROM secrets WHERE key = $1 AND scope = $2 AND scope_id = $3
	`, key, scope, scopeID).Scan(
		&secret.ID, &secret.Key, &secret.EncryptedValue, &secretRef,
		&secret.SecretType, &target,
		&secret.Scope, &secret.ScopeID,
		&secret.Description, &secret.InjectionMode, &allowProgeny, &secret.Version,
		&secret.Created, &secret.Updated, &secret.CreatedBy, &secret.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	if secretRef.Valid {
		secret.SecretRef = secretRef.String
	}
	if target.Valid {
		secret.Target = target.String
	}
	secret.AllowProgeny = allowProgeny != 0
	return secret, nil
}

func (s *PostgresStore) UpdateSecret(ctx context.Context, secret *store.Secret) error {
	secret.Updated = time.Now()
	secret.Version++

	if secret.SecretType == "" {
		secret.SecretType = store.SecretTypeEnvironment
	}
	if secret.Target == "" {
		secret.Target = secret.Key
	}
	if secret.InjectionMode == "" {
		secret.InjectionMode = store.InjectionModeAsNeeded
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE secrets SET
			encrypted_value = $1, secret_ref = $2, secret_type = $3, target = $4,
			description = $5, injection_mode = $6, allow_progeny = $7, version = $8, updated_at = $9, updated_by = $10
		WHERE key = $11 AND scope = $12 AND scope_id = $13
	`,
		secret.EncryptedValue, nullableString(secret.SecretRef),
		secret.SecretType, nullableString(secret.Target),
		secret.Description, secret.InjectionMode, boolToInt(secret.AllowProgeny), secret.Version, secret.Updated, secret.UpdatedBy,
		secret.Key, secret.Scope, secret.ScopeID,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) UpsertSecret(ctx context.Context, secret *store.Secret) (bool, error) {
	now := time.Now()
	secret.Updated = now

	existing, err := s.GetSecret(ctx, secret.Key, secret.Scope, secret.ScopeID)
	if err != nil && err != store.ErrNotFound {
		return false, err
	}

	if existing != nil {
		secret.ID = existing.ID
		secret.Created = existing.Created
		secret.CreatedBy = existing.CreatedBy
		secret.Version = existing.Version
		if err := s.UpdateSecret(ctx, secret); err != nil {
			return false, err
		}
		return false, nil
	}

	secret.Created = now
	if err := s.CreateSecret(ctx, secret); err != nil {
		return false, err
	}
	return true, nil
}

func (s *PostgresStore) DeleteSecret(ctx context.Context, key, scope, scopeID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM secrets WHERE key = $1 AND scope = $2 AND scope_id = $3", key, scope, scopeID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) DeleteSecretsByScope(ctx context.Context, scope, scopeID string) (int, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM secrets WHERE scope = $1 AND scope_id = $2", scope, scopeID)
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *PostgresStore) ListSecrets(ctx context.Context, filter store.SecretFilter) ([]store.Secret, error) {
	var conditions []string
	var args []interface{}
	idx := 1

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
	if filter.Key != "" {
		conditions = append(conditions, fmt.Sprintf("key = $%d", idx))
		args = append(args, filter.Key)
		idx++
	}
	if filter.Type != "" {
		conditions = append(conditions, fmt.Sprintf("secret_type = $%d", idx))
		args = append(args, filter.Type)
		idx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, key, secret_ref, secret_type, COALESCE(target, key), scope, scope_id, description, injection_mode, allow_progeny, version, created_at, updated_at, created_by, updated_by
		FROM secrets %s ORDER BY key
	`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSecrets(rows)
}

func (s *PostgresStore) ListProgenySecrets(ctx context.Context, ancestorIDs []string) ([]store.Secret, error) {
	if len(ancestorIDs) == 0 {
		return nil, nil
	}

	args := make([]interface{}, len(ancestorIDs))
	for i, id := range ancestorIDs {
		args[i] = id
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, key, secret_ref, secret_type, COALESCE(target, key), scope, scope_id, description, injection_mode, allow_progeny, version, created_at, updated_at, created_by, updated_by
		FROM secrets
		WHERE scope = 'user' AND allow_progeny = 1 AND created_by IN (%s)
		ORDER BY key
	`, ph(1, len(ancestorIDs))), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSecrets(rows)
}

func (s *PostgresStore) GetSecretValue(ctx context.Context, key, scope, scopeID string) (string, error) {
	var encryptedValue string
	err := s.db.QueryRowContext(ctx, `
		SELECT encrypted_value FROM secrets WHERE key = $1 AND scope = $2 AND scope_id = $3
	`, key, scope, scopeID).Scan(&encryptedValue)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", store.ErrNotFound
		}
		return "", err
	}
	return encryptedValue, nil
}

func scanSecrets(rows *sql.Rows) ([]store.Secret, error) {
	var secrets []store.Secret
	for rows.Next() {
		var secret store.Secret
		var secretRef, target sql.NullString
		var allowProgeny int
		if err := rows.Scan(
			&secret.ID, &secret.Key, &secretRef, &secret.SecretType, &target,
			&secret.Scope, &secret.ScopeID,
			&secret.Description, &secret.InjectionMode, &allowProgeny, &secret.Version,
			&secret.Created, &secret.Updated, &secret.CreatedBy, &secret.UpdatedBy,
		); err != nil {
			return nil, err
		}
		if secretRef.Valid {
			secret.SecretRef = secretRef.String
		}
		if target.Valid {
			secret.Target = target.String
		}
		secret.AllowProgeny = allowProgeny != 0
		secrets = append(secrets, secret)
	}
	return secrets, rows.Err()
}
