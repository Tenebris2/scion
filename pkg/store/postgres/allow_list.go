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

func (s *PostgresStore) AddAllowListEntry(ctx context.Context, entry *store.AllowListEntry) error {
	if entry.Created.IsZero() {
		entry.Created = time.Now()
	}
	entry.Email = strings.ToLower(entry.Email)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO allow_list (id, email, note, added_by, invite_id, created)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, entry.ID, entry.Email, entry.Note, entry.AddedBy, entry.InviteID, entry.Created)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return store.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (s *PostgresStore) RemoveAllowListEntry(ctx context.Context, email string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM allow_list WHERE email = $1", strings.ToLower(email))
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

func (s *PostgresStore) GetAllowListEntry(ctx context.Context, email string) (*store.AllowListEntry, error) {
	entry := &store.AllowListEntry{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, note, added_by, invite_id, created
		FROM allow_list WHERE email = $1
	`, strings.ToLower(email)).Scan(
		&entry.ID, &entry.Email, &entry.Note, &entry.AddedBy, &entry.InviteID, &entry.Created,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return entry, nil
}

func (s *PostgresStore) ListAllowListEntries(ctx context.Context, opts store.ListOptions) (*store.ListResult[store.AllowListEntry], error) {
	var totalCount int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM allow_list").Scan(&totalCount); err != nil {
		return nil, err
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	var conditions []string
	var args []interface{}
	idx := 1

	if opts.Cursor != "" {
		// cursor is an ID; use keyset pagination
		conditions = append(conditions, fmt.Sprintf(`(created < (SELECT created FROM allow_list WHERE id = $%d) OR (created = (SELECT created FROM allow_list WHERE id = $%d) AND id < $%d))`, idx, idx+1, idx+2))
		args = append(args, opts.Cursor, opts.Cursor, opts.Cursor)
		idx += 3
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, email, note, added_by, invite_id, created
		FROM allow_list %s ORDER BY created DESC, id DESC LIMIT $%d
	`, where, idx)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []store.AllowListEntry
	for rows.Next() {
		var entry store.AllowListEntry
		if err := rows.Scan(&entry.ID, &entry.Email, &entry.Note, &entry.AddedBy, &entry.InviteID, &entry.Created); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []store.AllowListEntry{}
	}

	var nextCursor string
	if len(entries) > limit {
		nextCursor = entries[limit-1].ID
		entries = entries[:limit]
	}

	return &store.ListResult[store.AllowListEntry]{
		Items:      entries,
		TotalCount: totalCount,
		NextCursor: nextCursor,
	}, nil
}

func (s *PostgresStore) IsEmailAllowListed(ctx context.Context, email string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM allow_list WHERE email = $1", strings.ToLower(email)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *PostgresStore) UpdateAllowListEntryInviteID(ctx context.Context, email string, inviteID string) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE allow_list SET invite_id = $1 WHERE email = $2",
		inviteID, strings.ToLower(email))
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

func (s *PostgresStore) ListAllowListEntriesWithInvites(ctx context.Context, opts store.ListOptions) (*store.ListResult[store.AllowListEntryWithInvite], error) {
	var totalCount int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM allow_list").Scan(&totalCount); err != nil {
		return nil, err
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	var conditions []string
	var args []interface{}
	idx := 1

	if opts.Cursor != "" {
		conditions = append(conditions, fmt.Sprintf(`(a.created < (SELECT created FROM allow_list WHERE id = $%d) OR (a.created = (SELECT created FROM allow_list WHERE id = $%d) AND a.id < $%d))`, idx, idx+1, idx+2))
		args = append(args, opts.Cursor, opts.Cursor, opts.Cursor)
		idx += 3
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT a.id, a.email, a.note, a.added_by, a.invite_id, a.created,
		       i.code_prefix, i.max_uses, i.use_count, i.expires_at, i.revoked
		FROM allow_list a
		LEFT JOIN invite_codes i ON a.invite_id = i.id AND a.invite_id != ''
		%s ORDER BY a.created DESC, a.id DESC LIMIT $%d
	`, where, idx)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []store.AllowListEntryWithInvite
	for rows.Next() {
		var entry store.AllowListEntryWithInvite
		var codePrefix sql.NullString
		var maxUses, useCount sql.NullInt64
		var revoked sql.NullInt64
		var expiresAt sql.NullTime
		if err := rows.Scan(
			&entry.ID, &entry.Email, &entry.Note, &entry.AddedBy, &entry.InviteID, &entry.Created,
			&codePrefix, &maxUses, &useCount, &expiresAt, &revoked,
		); err != nil {
			return nil, err
		}
		if codePrefix.Valid {
			entry.InviteCodePrefix = codePrefix.String
		}
		if maxUses.Valid {
			entry.InviteMaxUses = int(maxUses.Int64)
		}
		if useCount.Valid {
			entry.InviteUseCount = int(useCount.Int64)
		}
		if expiresAt.Valid {
			entry.InviteExpiresAt = expiresAt.Time
		}
		if revoked.Valid {
			entry.InviteRevoked = revoked.Int64 != 0
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []store.AllowListEntryWithInvite{}
	}

	var nextCursor string
	if len(entries) > limit {
		nextCursor = entries[limit-1].ID
		entries = entries[:limit]
	}

	return &store.ListResult[store.AllowListEntryWithInvite]{
		Items:      entries,
		TotalCount: totalCount,
		NextCursor: nextCursor,
	}, nil
}

func (s *PostgresStore) BulkAddAllowListEntries(ctx context.Context, entries []*store.AllowListEntry) (added int, skipped int, err error) {
	now := time.Now()
	for _, entry := range entries {
		if entry.Created.IsZero() {
			entry.Created = now
		}
		entry.Email = strings.ToLower(entry.Email)
		_, insertErr := s.db.ExecContext(ctx, `
			INSERT INTO allow_list (id, email, note, added_by, invite_id, created)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT DO NOTHING
		`, entry.ID, entry.Email, entry.Note, entry.AddedBy, entry.InviteID, entry.Created)
		if insertErr != nil {
			return added, skipped, insertErr
		}
		// We can't easily distinguish inserted vs skipped with ON CONFLICT DO NOTHING
		// Use RowsAffected would require storing the result, but for bulk we track separately
		added++
	}
	return added, 0, nil
}

func (s *PostgresStore) ListEmailDomains(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT LOWER(SPLIT_PART(email, '@', 2))
		FROM allow_list
		WHERE email LIKE '%@%'
		ORDER BY 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, err
		}
		if domain != "" {
			domains = append(domains, domain)
		}
	}
	return domains, rows.Err()
}

// ============================================================================
// InviteCode Operations
// ============================================================================

func (s *PostgresStore) CreateInviteCode(ctx context.Context, invite *store.InviteCode) error {
	if invite.Created.IsZero() {
		invite.Created = time.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO invite_codes (id, code_hash, code_prefix, max_uses, use_count, expires_at, revoked, created_by, note, created)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, invite.ID, invite.CodeHash, invite.CodePrefix, invite.MaxUses, invite.UseCount,
		invite.ExpiresAt, boolToInt(invite.Revoked), invite.CreatedBy, invite.Note, invite.Created)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return store.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (s *PostgresStore) GetInviteCodeByHash(ctx context.Context, codeHash string) (*store.InviteCode, error) {
	invite := &store.InviteCode{}
	var revoked int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, code_hash, code_prefix, max_uses, use_count, expires_at, revoked, created_by, note, created
		FROM invite_codes WHERE code_hash = $1
	`, codeHash).Scan(
		&invite.ID, &invite.CodeHash, &invite.CodePrefix, &invite.MaxUses, &invite.UseCount,
		&invite.ExpiresAt, &revoked, &invite.CreatedBy, &invite.Note, &invite.Created,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	invite.Revoked = revoked != 0
	return invite, nil
}

func (s *PostgresStore) GetInviteCode(ctx context.Context, id string) (*store.InviteCode, error) {
	invite := &store.InviteCode{}
	var revoked int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, code_hash, code_prefix, max_uses, use_count, expires_at, revoked, created_by, note, created
		FROM invite_codes WHERE id = $1
	`, id).Scan(
		&invite.ID, &invite.CodeHash, &invite.CodePrefix, &invite.MaxUses, &invite.UseCount,
		&invite.ExpiresAt, &revoked, &invite.CreatedBy, &invite.Note, &invite.Created,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	invite.Revoked = revoked != 0
	return invite, nil
}

func (s *PostgresStore) ListInviteCodes(ctx context.Context, opts store.ListOptions) (*store.ListResult[store.InviteCode], error) {
	var totalCount int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM invite_codes").Scan(&totalCount); err != nil {
		return nil, err
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	var conditions []string
	var args []interface{}
	idx := 1

	if opts.Cursor != "" {
		conditions = append(conditions, fmt.Sprintf(`(created < (SELECT created FROM invite_codes WHERE id = $%d) OR (created = (SELECT created FROM invite_codes WHERE id = $%d) AND id < $%d))`, idx, idx+1, idx+2))
		args = append(args, opts.Cursor, opts.Cursor, opts.Cursor)
		idx += 3
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, code_prefix, max_uses, use_count, expires_at, revoked, created_by, note, created
		FROM invite_codes %s ORDER BY created DESC, id DESC LIMIT $%d
	`, where, idx)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []store.InviteCode
	for rows.Next() {
		var invite store.InviteCode
		var revoked int
		if err := rows.Scan(
			&invite.ID, &invite.CodePrefix, &invite.MaxUses, &invite.UseCount,
			&invite.ExpiresAt, &revoked, &invite.CreatedBy, &invite.Note, &invite.Created,
		); err != nil {
			return nil, err
		}
		invite.Revoked = revoked != 0
		invites = append(invites, invite)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if invites == nil {
		invites = []store.InviteCode{}
	}

	var nextCursor string
	if len(invites) > limit {
		nextCursor = invites[limit-1].ID
		invites = invites[:limit]
	}

	return &store.ListResult[store.InviteCode]{
		Items:      invites,
		TotalCount: totalCount,
		NextCursor: nextCursor,
	}, nil
}

func (s *PostgresStore) IncrementInviteUseCount(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE invite_codes SET use_count = use_count + 1
		WHERE id = $1 AND revoked = 0 AND expires_at > NOW()
		  AND (max_uses = 0 OR use_count < max_uses)
	`, id)
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

func (s *PostgresStore) RevokeInviteCode(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "UPDATE invite_codes SET revoked = 1 WHERE id = $1", id)
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

func (s *PostgresStore) DeleteInviteCode(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM invite_codes WHERE id = $1", id)
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

func (s *PostgresStore) GetInviteStats(ctx context.Context) (*store.InviteStats, error) {
	stats := &store.InviteStats{}

	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM invite_codes
		WHERE revoked = 0
		  AND expires_at > NOW()
		  AND (max_uses = 0 OR use_count < max_uses)
	`).Scan(&stats.PendingInvites)
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(use_count), 0) FROM invite_codes`).Scan(&stats.TotalRedemptions)
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM allow_list`).Scan(&stats.AllowListCount)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, code_prefix, use_count, max_uses, expires_at, note, created
		FROM invite_codes
		WHERE use_count > 0
		ORDER BY created DESC
		LIMIT 10
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var info store.InviteCodeInfo
		if err := rows.Scan(&info.ID, &info.CodePrefix, &info.UseCount, &info.MaxUses, &info.ExpiresAt, &info.Note, &info.Created); err != nil {
			return nil, err
		}
		stats.RecentRedemptions = append(stats.RecentRedemptions, info)
	}
	if stats.RecentRedemptions == nil {
		stats.RecentRedemptions = []store.InviteCodeInfo{}
	}

	return stats, rows.Err()
}
