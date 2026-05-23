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

func (s *PostgresStore) CreateScheduledEvent(ctx context.Context, event *store.ScheduledEvent) error {
	if event.ID == "" || event.ProjectID == "" || event.EventType == "" {
		return store.ErrInvalidInput
	}

	now := time.Now()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	if event.Status == "" {
		event.Status = store.ScheduledEventPending
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO scheduled_events (
			id, project_id, event_type, fire_at, payload, status,
			created_at, created_by, fired_at, error, schedule_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`,
		event.ID, event.ProjectID, event.EventType, event.FireAt, event.Payload, event.Status,
		event.CreatedAt, nullableString(event.CreatedBy), nullableTimePtr(event.FiredAt), nullableString(event.Error),
		nullableString(event.ScheduleID),
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return store.ErrAlreadyExists
		}
		if strings.Contains(err.Error(), "foreign key") {
			return fmt.Errorf("project %s does not exist: %w", event.ProjectID, store.ErrInvalidInput)
		}
		return err
	}
	return nil
}

func (s *PostgresStore) GetScheduledEvent(ctx context.Context, id string) (*store.ScheduledEvent, error) {
	event := &store.ScheduledEvent{}
	var createdBy, errMsg, scheduleID sql.NullString
	var firedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, event_type, fire_at, payload, status,
			created_at, created_by, fired_at, error, schedule_id
		FROM scheduled_events WHERE id = $1
	`, id).Scan(
		&event.ID, &event.ProjectID, &event.EventType, &event.FireAt, &event.Payload, &event.Status,
		&event.CreatedAt, &createdBy, &firedAt, &errMsg, &scheduleID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	if createdBy.Valid {
		event.CreatedBy = createdBy.String
	}
	if firedAt.Valid {
		event.FiredAt = &firedAt.Time
	}
	if errMsg.Valid {
		event.Error = errMsg.String
	}
	if scheduleID.Valid {
		event.ScheduleID = scheduleID.String
	}
	return event, nil
}

func (s *PostgresStore) ListPendingScheduledEvents(ctx context.Context) ([]store.ScheduledEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, event_type, fire_at, payload, status,
			created_at, created_by, fired_at, error, schedule_id
		FROM scheduled_events WHERE status = $1
		ORDER BY fire_at ASC
	`, store.ScheduledEventPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanScheduledEvents(rows)
}

func (s *PostgresStore) UpdateScheduledEventStatus(ctx context.Context, id string, status string, firedAt *time.Time, errMsg string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE scheduled_events SET status = $1, fired_at = $2, error = $3 WHERE id = $4
	`, status, nullableTimePtr(firedAt), nullableString(errMsg), id)
	return err
}

func (s *PostgresStore) CancelScheduledEvent(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE scheduled_events SET status = $1 WHERE id = $2 AND status = $3
	`, store.ScheduledEventCancelled, id, store.ScheduledEventPending)
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

func (s *PostgresStore) ListScheduledEvents(ctx context.Context, filter store.ScheduledEventFilter, opts store.ListOptions) (*store.ListResult[store.ScheduledEvent], error) {
	var conditions []string
	var args []interface{}
	idx := 1

	if filter.ProjectID != "" {
		conditions = append(conditions, fmt.Sprintf("project_id = $%d", idx))
		args = append(args, filter.ProjectID)
		idx++
	}
	if filter.EventType != "" {
		conditions = append(conditions, fmt.Sprintf("event_type = $%d", idx))
		args = append(args, filter.EventType)
		idx++
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", idx))
		args = append(args, filter.Status)
		idx++
	}
	if filter.ScheduleID != "" {
		conditions = append(conditions, fmt.Sprintf("schedule_id = $%d", idx))
		args = append(args, filter.ScheduleID)
		idx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var totalCount int
	if err := s.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM scheduled_events %s", where), args...,
	).Scan(&totalCount); err != nil {
		return nil, err
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var queryArgs []interface{}
	var query string

	if opts.Cursor != "" {
		cursorCond := fmt.Sprintf("id < $%d", idx)
		if where == "" {
			where = "WHERE " + cursorCond
		} else {
			where += " AND " + cursorCond
		}
		queryArgs = append(args, opts.Cursor, limit+1)
		idx++
	} else {
		queryArgs = append(args, limit+1)
	}

	query = fmt.Sprintf(`
		SELECT id, project_id, event_type, fire_at, payload, status,
			created_at, created_by, fired_at, error, schedule_id
		FROM scheduled_events %s
		ORDER BY created_at DESC LIMIT $%d
	`, where, idx)

	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events, err := scanScheduledEvents(rows)
	if err != nil {
		return nil, err
	}

	result := &store.ListResult[store.ScheduledEvent]{TotalCount: totalCount}
	if len(events) > limit {
		result.Items = events[:limit]
		result.NextCursor = events[limit-1].ID
	} else {
		result.Items = events
	}
	return result, nil
}

func (s *PostgresStore) PurgeOldScheduledEvents(ctx context.Context, cutoff time.Time) (int, error) {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM scheduled_events WHERE status != $1 AND created_at < $2",
		store.ScheduledEventPending, cutoff,
	)
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func scanScheduledEvents(rows *sql.Rows) ([]store.ScheduledEvent, error) {
	var events []store.ScheduledEvent
	for rows.Next() {
		var event store.ScheduledEvent
		var createdBy, errMsg, scheduleID sql.NullString
		var firedAt sql.NullTime

		if err := rows.Scan(
			&event.ID, &event.ProjectID, &event.EventType, &event.FireAt, &event.Payload, &event.Status,
			&event.CreatedAt, &createdBy, &firedAt, &errMsg, &scheduleID,
		); err != nil {
			return nil, err
		}

		if createdBy.Valid {
			event.CreatedBy = createdBy.String
		}
		if firedAt.Valid {
			event.FiredAt = &firedAt.Time
		}
		if errMsg.Valid {
			event.Error = errMsg.String
		}
		if scheduleID.Valid {
			event.ScheduleID = scheduleID.String
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
