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

func (s *PostgresStore) CreateBrokerSecret(ctx context.Context, secret *store.BrokerSecret) error {
	if secret.BrokerID == "" {
		return store.ErrInvalidInput
	}

	now := time.Now()
	if secret.CreatedAt.IsZero() {
		secret.CreatedAt = now
	}
	if secret.Algorithm == "" {
		secret.Algorithm = store.BrokerSecretAlgorithmHMACSHA256
	}
	if secret.Status == "" {
		secret.Status = store.BrokerSecretStatusActive
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO broker_secrets (
			broker_id, secret_key, algorithm,
			created_at, rotated_at, expires_at, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
	`,
		secret.BrokerID, secret.SecretKey, secret.Algorithm,
		secret.CreatedAt, nullableTime(secret.RotatedAt), nullableTime(secret.ExpiresAt), secret.Status,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return store.ErrAlreadyExists
		}
		if strings.Contains(err.Error(), "foreign key") {
			return fmt.Errorf("broker %s does not exist: %w", secret.BrokerID, store.ErrNotFound)
		}
		return err
	}
	return nil
}

func (s *PostgresStore) GetBrokerSecret(ctx context.Context, brokerID string) (*store.BrokerSecret, error) {
	secret := &store.BrokerSecret{}
	var rotatedAt, expiresAt sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT broker_id, secret_key, algorithm,
			created_at, rotated_at, expires_at, status
		FROM broker_secrets WHERE broker_id = $1
	`, brokerID).Scan(
		&secret.BrokerID, &secret.SecretKey, &secret.Algorithm,
		&secret.CreatedAt, &rotatedAt, &expiresAt, &secret.Status,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	if rotatedAt.Valid {
		secret.RotatedAt = rotatedAt.Time
	}
	if expiresAt.Valid {
		secret.ExpiresAt = expiresAt.Time
	}
	return secret, nil
}

func (s *PostgresStore) GetActiveSecrets(ctx context.Context, brokerID string) ([]*store.BrokerSecret, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT broker_id, secret_key, algorithm,
			created_at, rotated_at, expires_at, status
		FROM broker_secrets
		WHERE broker_id = $1 AND status IN ($2,$3)
		ORDER BY created_at DESC
	`, brokerID, store.BrokerSecretStatusActive, store.BrokerSecretStatusDeprecated)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var secrets []*store.BrokerSecret
	for rows.Next() {
		secret := &store.BrokerSecret{}
		var rotatedAt, expiresAt sql.NullTime

		if err := rows.Scan(
			&secret.BrokerID, &secret.SecretKey, &secret.Algorithm,
			&secret.CreatedAt, &rotatedAt, &expiresAt, &secret.Status,
		); err != nil {
			return nil, err
		}

		if rotatedAt.Valid {
			secret.RotatedAt = rotatedAt.Time
		}
		if expiresAt.Valid {
			secret.ExpiresAt = expiresAt.Time
		}
		secrets = append(secrets, secret)
	}
	return secrets, rows.Err()
}

func (s *PostgresStore) UpdateBrokerSecret(ctx context.Context, secret *store.BrokerSecret) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE broker_secrets SET
			secret_key = $1,
			algorithm = $2,
			rotated_at = $3,
			expires_at = $4,
			status = $5
		WHERE broker_id = $6
	`,
		secret.SecretKey, secret.Algorithm,
		nullableTime(secret.RotatedAt), nullableTime(secret.ExpiresAt), secret.Status,
		secret.BrokerID,
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

func (s *PostgresStore) DeleteBrokerSecret(ctx context.Context, brokerID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM broker_secrets WHERE broker_id = $1", brokerID)
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

func (s *PostgresStore) CreateJoinToken(ctx context.Context, token *store.BrokerJoinToken) error {
	if token.BrokerID == "" || token.TokenHash == "" {
		return store.ErrInvalidInput
	}

	now := time.Now()
	if token.CreatedAt.IsZero() {
		token.CreatedAt = now
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO broker_join_tokens (
			broker_id, token_hash, expires_at, created_at, created_by
		) VALUES ($1,$2,$3,$4,$5)
	`,
		token.BrokerID, token.TokenHash, token.ExpiresAt, token.CreatedAt, token.CreatedBy,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return store.ErrAlreadyExists
		}
		if strings.Contains(err.Error(), "foreign key") {
			return store.ErrNotFound
		}
		return err
	}
	return nil
}

func (s *PostgresStore) GetJoinToken(ctx context.Context, tokenHash string) (*store.BrokerJoinToken, error) {
	token := &store.BrokerJoinToken{}
	err := s.db.QueryRowContext(ctx, `
		SELECT broker_id, token_hash, expires_at, created_at, created_by
		FROM broker_join_tokens WHERE token_hash = $1
	`, tokenHash).Scan(
		&token.BrokerID, &token.TokenHash, &token.ExpiresAt, &token.CreatedAt, &token.CreatedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return token, nil
}

func (s *PostgresStore) GetJoinTokenByBrokerID(ctx context.Context, brokerID string) (*store.BrokerJoinToken, error) {
	token := &store.BrokerJoinToken{}
	err := s.db.QueryRowContext(ctx, `
		SELECT broker_id, token_hash, expires_at, created_at, created_by
		FROM broker_join_tokens WHERE broker_id = $1
	`, brokerID).Scan(
		&token.BrokerID, &token.TokenHash, &token.ExpiresAt, &token.CreatedAt, &token.CreatedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return token, nil
}

func (s *PostgresStore) DeleteJoinToken(ctx context.Context, brokerID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM broker_join_tokens WHERE broker_id = $1", brokerID)
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

func (s *PostgresStore) CleanExpiredJoinTokens(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM broker_join_tokens WHERE expires_at < $1", time.Now())
	return err
}
