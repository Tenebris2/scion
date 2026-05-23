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

func (s *PostgresStore) CreateNotificationSubscription(ctx context.Context, sub *store.NotificationSubscription) error {
	if sub.ID == "" || sub.SubscriberID == "" || sub.ProjectID == "" {
		return store.ErrInvalidInput
	}

	if sub.Scope == "" {
		sub.Scope = store.SubscriptionScopeAgent
	}

	switch sub.Scope {
	case store.SubscriptionScopeAgent:
		if sub.AgentID == "" {
			return store.ErrInvalidInput
		}
	case store.SubscriptionScopeProject:
		sub.AgentID = ""
	default:
		return fmt.Errorf("invalid scope %q: %w", sub.Scope, store.ErrInvalidInput)
	}

	now := time.Now()
	if sub.CreatedAt.IsZero() {
		sub.CreatedAt = now
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO notification_subscriptions (
			id, scope, agent_id, subscriber_type, subscriber_id, project_id,
			trigger_activities, created_at, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`,
		sub.ID, sub.Scope, nullableString(sub.AgentID), sub.SubscriberType, sub.SubscriberID, sub.ProjectID,
		marshalJSON(sub.TriggerActivities), sub.CreatedAt, sub.CreatedBy,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return store.ErrAlreadyExists
		}
		if strings.Contains(err.Error(), "foreign key") {
			return fmt.Errorf("agent %s does not exist: %w", sub.AgentID, store.ErrInvalidInput)
		}
		return err
	}
	return nil
}

func (s *PostgresStore) GetNotificationSubscription(ctx context.Context, id string) (*store.NotificationSubscription, error) {
	var sub store.NotificationSubscription
	var agentID sql.NullString
	var triggerActivitiesJSON string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, scope, agent_id, subscriber_type, subscriber_id, project_id,
			trigger_activities, created_at, created_by
		FROM notification_subscriptions WHERE id = $1
	`, id).Scan(
		&sub.ID, &sub.Scope, &agentID, &sub.SubscriberType, &sub.SubscriberID, &sub.ProjectID,
		&triggerActivitiesJSON, &sub.CreatedAt, &sub.CreatedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	if agentID.Valid {
		sub.AgentID = agentID.String
	}
	unmarshalJSON(triggerActivitiesJSON, &sub.TriggerActivities)
	return &sub, nil
}

func (s *PostgresStore) GetNotificationSubscriptions(ctx context.Context, agentID string) ([]store.NotificationSubscription, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, scope, agent_id, subscriber_type, subscriber_id, project_id,
			trigger_activities, created_at, created_by
		FROM notification_subscriptions WHERE agent_id = $1
		ORDER BY created_at ASC
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubscriptions(rows)
}

func (s *PostgresStore) GetNotificationSubscriptionsByProject(ctx context.Context, projectID string) ([]store.NotificationSubscription, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, scope, agent_id, subscriber_type, subscriber_id, project_id,
			trigger_activities, created_at, created_by
		FROM notification_subscriptions WHERE project_id = $1
		ORDER BY created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubscriptions(rows)
}

func (s *PostgresStore) GetNotificationSubscriptionsByProjectScope(ctx context.Context, projectID string) ([]store.NotificationSubscription, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, scope, agent_id, subscriber_type, subscriber_id, project_id,
			trigger_activities, created_at, created_by
		FROM notification_subscriptions WHERE project_id = $1 AND scope = 'project'
		ORDER BY created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubscriptions(rows)
}

func (s *PostgresStore) GetSubscriptionsForSubscriber(ctx context.Context, subscriberType, subscriberID string) ([]store.NotificationSubscription, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, scope, agent_id, subscriber_type, subscriber_id, project_id,
			trigger_activities, created_at, created_by
		FROM notification_subscriptions WHERE subscriber_type = $1 AND subscriber_id = $2
		ORDER BY created_at ASC
	`, subscriberType, subscriberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubscriptions(rows)
}

func (s *PostgresStore) UpdateNotificationSubscriptionTriggers(ctx context.Context, id string, triggerActivities []string) error {
	if id == "" || len(triggerActivities) == 0 {
		return store.ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		"UPDATE notification_subscriptions SET trigger_activities = $1 WHERE id = $2",
		marshalJSON(triggerActivities), id,
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

func (s *PostgresStore) DeleteNotificationSubscription(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM notification_subscriptions WHERE id = $1", id)
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

func (s *PostgresStore) DeleteNotificationSubscriptionsForAgent(ctx context.Context, agentID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM notification_subscriptions WHERE agent_id = $1", agentID)
	return err
}

func (s *PostgresStore) CreateNotification(ctx context.Context, notif *store.Notification) error {
	if notif.ID == "" || notif.SubscriptionID == "" || notif.AgentID == "" {
		return store.ErrInvalidInput
	}

	now := time.Now()
	if notif.CreatedAt.IsZero() {
		notif.CreatedAt = now
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO notifications (
			id, subscription_id, agent_id, project_id,
			subscriber_type, subscriber_id,
			status, message, dispatched, acknowledged, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`,
		notif.ID, notif.SubscriptionID, notif.AgentID, notif.ProjectID,
		notif.SubscriberType, notif.SubscriberID,
		notif.Status, notif.Message,
		boolToInt(notif.Dispatched), boolToInt(notif.Acknowledged),
		notif.CreatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") {
			return fmt.Errorf("subscription %s does not exist: %w", notif.SubscriptionID, store.ErrInvalidInput)
		}
		return err
	}
	return nil
}

func (s *PostgresStore) GetNotifications(ctx context.Context, subscriberType, subscriberID string, onlyUnacknowledged bool) ([]store.Notification, error) {
	query := `
		SELECT id, subscription_id, agent_id, project_id,
			subscriber_type, subscriber_id,
			status, message, dispatched, acknowledged, created_at
		FROM notifications
		WHERE subscriber_type = $1 AND subscriber_id = $2
	`
	args := []interface{}{subscriberType, subscriberID}

	if onlyUnacknowledged {
		query += ` AND acknowledged = 0`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotifications(rows)
}

func (s *PostgresStore) GetNotificationsByAgent(ctx context.Context, agentID, subscriberType, subscriberID string, onlyUnacknowledged bool) ([]store.Notification, error) {
	query := `
		SELECT id, subscription_id, agent_id, project_id,
			subscriber_type, subscriber_id,
			status, message, dispatched, acknowledged, created_at
		FROM notifications
		WHERE agent_id = $1 AND subscriber_type = $2 AND subscriber_id = $3
	`
	args := []interface{}{agentID, subscriberType, subscriberID}

	if onlyUnacknowledged {
		query += ` AND acknowledged = 0`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotifications(rows)
}

func (s *PostgresStore) AcknowledgeNotification(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "UPDATE notifications SET acknowledged = 1 WHERE id = $1", id)
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

func (s *PostgresStore) AcknowledgeAllNotifications(ctx context.Context, subscriberType, subscriberID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE notifications SET acknowledged = 1
		WHERE subscriber_type = $1 AND subscriber_id = $2
	`, subscriberType, subscriberID)
	return err
}

func (s *PostgresStore) MarkNotificationDispatched(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "UPDATE notifications SET dispatched = 1 WHERE id = $1", id)
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

func (s *PostgresStore) GetLastNotificationStatus(ctx context.Context, subscriptionID string) (string, error) {
	var status string
	err := s.db.QueryRowContext(ctx, `
		SELECT status FROM notifications WHERE subscription_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, subscriptionID).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return status, nil
}

func (s *PostgresStore) CreateSubscriptionTemplate(ctx context.Context, tmpl *store.SubscriptionTemplate) error {
	if tmpl.ID == "" || tmpl.Name == "" || len(tmpl.TriggerActivities) == 0 {
		return store.ErrInvalidInput
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO subscription_templates (id, name, scope, trigger_activities, project_id, created_by)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, tmpl.ID, tmpl.Name, tmpl.Scope, marshalJSON(tmpl.TriggerActivities), tmpl.ProjectID, tmpl.CreatedBy)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return store.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (s *PostgresStore) GetSubscriptionTemplate(ctx context.Context, id string) (*store.SubscriptionTemplate, error) {
	var tmpl store.SubscriptionTemplate
	var triggersJSON string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, scope, trigger_activities, project_id, created_by
		FROM subscription_templates WHERE id = $1
	`, id).Scan(&tmpl.ID, &tmpl.Name, &tmpl.Scope, &triggersJSON, &tmpl.ProjectID, &tmpl.CreatedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	unmarshalJSON(triggersJSON, &tmpl.TriggerActivities)
	return &tmpl, nil
}

func (s *PostgresStore) ListSubscriptionTemplates(ctx context.Context, projectID string) ([]store.SubscriptionTemplate, error) {
	var rows *sql.Rows
	var err error

	if projectID != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, name, scope, trigger_activities, project_id, created_by
			FROM subscription_templates
			WHERE project_id = '' OR project_id = $1
			ORDER BY project_id ASC, name ASC
		`, projectID)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, name, scope, trigger_activities, project_id, created_by
			FROM subscription_templates WHERE project_id = ''
			ORDER BY name ASC
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []store.SubscriptionTemplate
	for rows.Next() {
		var tmpl store.SubscriptionTemplate
		var triggersJSON string
		if err := rows.Scan(&tmpl.ID, &tmpl.Name, &tmpl.Scope, &triggersJSON, &tmpl.ProjectID, &tmpl.CreatedBy); err != nil {
			return nil, err
		}
		unmarshalJSON(triggersJSON, &tmpl.TriggerActivities)
		templates = append(templates, tmpl)
	}
	return templates, rows.Err()
}

func (s *PostgresStore) DeleteSubscriptionTemplate(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM subscription_templates WHERE id = $1", id)
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

func scanSubscriptions(rows *sql.Rows) ([]store.NotificationSubscription, error) {
	var subs []store.NotificationSubscription
	for rows.Next() {
		var sub store.NotificationSubscription
		var agentID sql.NullString
		var triggerActivitiesJSON string

		if err := rows.Scan(
			&sub.ID, &sub.Scope, &agentID, &sub.SubscriberType, &sub.SubscriberID, &sub.ProjectID,
			&triggerActivitiesJSON, &sub.CreatedAt, &sub.CreatedBy,
		); err != nil {
			return nil, err
		}
		if agentID.Valid {
			sub.AgentID = agentID.String
		}
		unmarshalJSON(triggerActivitiesJSON, &sub.TriggerActivities)
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

func scanNotifications(rows *sql.Rows) ([]store.Notification, error) {
	var notifs []store.Notification
	for rows.Next() {
		var notif store.Notification
		var dispatched, acknowledged int

		if err := rows.Scan(
			&notif.ID, &notif.SubscriptionID, &notif.AgentID, &notif.ProjectID,
			&notif.SubscriberType, &notif.SubscriberID,
			&notif.Status, &notif.Message, &dispatched, &acknowledged, &notif.CreatedAt,
		); err != nil {
			return nil, err
		}
		notif.Dispatched = dispatched != 0
		notif.Acknowledged = acknowledged != 0
		notifs = append(notifs, notif)
	}
	return notifs, rows.Err()
}
