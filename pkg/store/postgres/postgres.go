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
	"errors"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/store"
)

var errNotImplemented = errors.New("postgres: not implemented")

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

// ============================================================================
// Agent Operations
// ============================================================================

func (s *PostgresStore) CreateAgent(ctx context.Context, agent *store.Agent) error {
	return errNotImplemented
}

func (s *PostgresStore) GetAgent(ctx context.Context, id string) (*store.Agent, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetAgentBySlug(ctx context.Context, projectID, slug string) (*store.Agent, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateAgent(ctx context.Context, agent *store.Agent) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteAgent(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) ListAgents(ctx context.Context, filter store.AgentFilter, opts store.ListOptions) (*store.ListResult[store.Agent], error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateAgentStatus(ctx context.Context, id string, status store.AgentStatusUpdate) error {
	return errNotImplemented
}

func (s *PostgresStore) PurgeDeletedAgents(ctx context.Context, cutoff time.Time) (int, error) {
	return 0, errNotImplemented
}

func (s *PostgresStore) MarkStaleAgentsOffline(ctx context.Context, threshold time.Time) ([]store.Agent, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) MarkStalledAgents(ctx context.Context, activityThreshold, heartbeatRecency time.Time) ([]store.Agent, error) {
	return nil, errNotImplemented
}

// ============================================================================
// Project Operations
// ============================================================================

func (s *PostgresStore) CreateProject(ctx context.Context, project *store.Project) error {
	return errNotImplemented
}

func (s *PostgresStore) GetProject(ctx context.Context, id string) (*store.Project, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetProjectBySlug(ctx context.Context, slug string) (*store.Project, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetProjectBySlugCaseInsensitive(ctx context.Context, slug string) (*store.Project, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetProjectsByGitRemote(ctx context.Context, gitRemote string) ([]*store.Project, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) NextAvailableSlug(ctx context.Context, baseSlug string) (string, error) {
	return "", errNotImplemented
}

func (s *PostgresStore) UpdateProject(ctx context.Context, project *store.Project) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteProject(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) ListProjects(ctx context.Context, filter store.ProjectFilter, opts store.ListOptions) (*store.ListResult[store.Project], error) {
	return nil, errNotImplemented
}

// ============================================================================
// RuntimeBroker Operations
// ============================================================================

func (s *PostgresStore) CreateRuntimeBroker(ctx context.Context, broker *store.RuntimeBroker) error {
	return errNotImplemented
}

func (s *PostgresStore) GetRuntimeBroker(ctx context.Context, id string) (*store.RuntimeBroker, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetRuntimeBrokerByName(ctx context.Context, name string) (*store.RuntimeBroker, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateRuntimeBroker(ctx context.Context, broker *store.RuntimeBroker) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteRuntimeBroker(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) ListRuntimeBrokers(ctx context.Context, filter store.RuntimeBrokerFilter, opts store.ListOptions) (*store.ListResult[store.RuntimeBroker], error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateRuntimeBrokerHeartbeat(ctx context.Context, id string, status string) error {
	return errNotImplemented
}

// ============================================================================
// Template Operations
// ============================================================================

func (s *PostgresStore) CreateTemplate(ctx context.Context, template *store.Template) error {
	return errNotImplemented
}

func (s *PostgresStore) GetTemplate(ctx context.Context, id string) (*store.Template, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetTemplateBySlug(ctx context.Context, slug, scope, scopeID string) (*store.Template, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateTemplate(ctx context.Context, template *store.Template) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteTemplate(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteTemplatesByScope(ctx context.Context, scope, scopeID string) (int, error) {
	return 0, errNotImplemented
}

func (s *PostgresStore) ListTemplates(ctx context.Context, filter store.TemplateFilter, opts store.ListOptions) (*store.ListResult[store.Template], error) {
	return nil, errNotImplemented
}

// ============================================================================
// HarnessConfig Operations
// ============================================================================

func (s *PostgresStore) CreateHarnessConfig(ctx context.Context, hc *store.HarnessConfig) error {
	return errNotImplemented
}

func (s *PostgresStore) GetHarnessConfig(ctx context.Context, id string) (*store.HarnessConfig, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetHarnessConfigBySlug(ctx context.Context, slug, scope, scopeID string) (*store.HarnessConfig, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateHarnessConfig(ctx context.Context, hc *store.HarnessConfig) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteHarnessConfig(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteHarnessConfigsByScope(ctx context.Context, scope, scopeID string) (int, error) {
	return 0, errNotImplemented
}

func (s *PostgresStore) ListHarnessConfigs(ctx context.Context, filter store.HarnessConfigFilter, opts store.ListOptions) (*store.ListResult[store.HarnessConfig], error) {
	return nil, errNotImplemented
}

// ============================================================================
// User Operations
// ============================================================================

func (s *PostgresStore) CreateUser(ctx context.Context, user *store.User) error {
	return errNotImplemented
}

func (s *PostgresStore) GetUser(ctx context.Context, id string) (*store.User, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*store.User, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateUser(ctx context.Context, user *store.User) error {
	return errNotImplemented
}

func (s *PostgresStore) UpdateUserLastSeen(ctx context.Context, id string, t time.Time) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteUser(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) ListUsers(ctx context.Context, filter store.UserFilter, opts store.ListOptions) (*store.ListResult[store.User], error) {
	return nil, errNotImplemented
}

// ============================================================================
// AllowList Operations
// ============================================================================

func (s *PostgresStore) AddAllowListEntry(ctx context.Context, entry *store.AllowListEntry) error {
	return errNotImplemented
}

func (s *PostgresStore) RemoveAllowListEntry(ctx context.Context, email string) error {
	return errNotImplemented
}

func (s *PostgresStore) GetAllowListEntry(ctx context.Context, email string) (*store.AllowListEntry, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) ListAllowListEntries(ctx context.Context, opts store.ListOptions) (*store.ListResult[store.AllowListEntry], error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) IsEmailAllowListed(ctx context.Context, email string) (bool, error) {
	return false, errNotImplemented
}

func (s *PostgresStore) BulkAddAllowListEntries(ctx context.Context, entries []*store.AllowListEntry) (added int, skipped int, err error) {
	return 0, 0, errNotImplemented
}

func (s *PostgresStore) ListEmailDomains(ctx context.Context) ([]string, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateAllowListEntryInviteID(ctx context.Context, email string, inviteID string) error {
	return errNotImplemented
}

func (s *PostgresStore) ListAllowListEntriesWithInvites(ctx context.Context, opts store.ListOptions) (*store.ListResult[store.AllowListEntryWithInvite], error) {
	return nil, errNotImplemented
}

// ============================================================================
// InviteCode Operations
// ============================================================================

func (s *PostgresStore) CreateInviteCode(ctx context.Context, invite *store.InviteCode) error {
	return errNotImplemented
}

func (s *PostgresStore) GetInviteCodeByHash(ctx context.Context, codeHash string) (*store.InviteCode, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetInviteCode(ctx context.Context, id string) (*store.InviteCode, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) ListInviteCodes(ctx context.Context, opts store.ListOptions) (*store.ListResult[store.InviteCode], error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) IncrementInviteUseCount(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) RevokeInviteCode(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteInviteCode(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) GetInviteStats(ctx context.Context) (*store.InviteStats, error) {
	return nil, errNotImplemented
}

// ============================================================================
// ProjectProvider Operations
// ============================================================================

func (s *PostgresStore) AddProjectProvider(ctx context.Context, provider *store.ProjectProvider) error {
	return errNotImplemented
}

func (s *PostgresStore) RemoveProjectProvider(ctx context.Context, projectID, brokerID string) error {
	return errNotImplemented
}

func (s *PostgresStore) GetProjectProvider(ctx context.Context, projectID, brokerID string) (*store.ProjectProvider, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetProjectProviders(ctx context.Context, projectID string) ([]store.ProjectProvider, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetBrokerProjects(ctx context.Context, brokerID string) ([]store.ProjectProvider, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateProviderStatus(ctx context.Context, projectID, brokerID, status string) error {
	return errNotImplemented
}

// ============================================================================
// EnvVar Operations
// ============================================================================

func (s *PostgresStore) CreateEnvVar(ctx context.Context, envVar *store.EnvVar) error {
	return errNotImplemented
}

func (s *PostgresStore) GetEnvVar(ctx context.Context, key, scope, scopeID string) (*store.EnvVar, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateEnvVar(ctx context.Context, envVar *store.EnvVar) error {
	return errNotImplemented
}

func (s *PostgresStore) UpsertEnvVar(ctx context.Context, envVar *store.EnvVar) (bool, error) {
	return false, errNotImplemented
}

func (s *PostgresStore) DeleteEnvVar(ctx context.Context, key, scope, scopeID string) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteEnvVarsByScope(ctx context.Context, scope, scopeID string) (int, error) {
	return 0, errNotImplemented
}

func (s *PostgresStore) ListEnvVars(ctx context.Context, filter store.EnvVarFilter) ([]store.EnvVar, error) {
	return nil, errNotImplemented
}

// ============================================================================
// Secret Operations
// ============================================================================

func (s *PostgresStore) CreateSecret(ctx context.Context, secret *store.Secret) error {
	return errNotImplemented
}

func (s *PostgresStore) GetSecret(ctx context.Context, key, scope, scopeID string) (*store.Secret, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateSecret(ctx context.Context, secret *store.Secret) error {
	return errNotImplemented
}

func (s *PostgresStore) UpsertSecret(ctx context.Context, secret *store.Secret) (bool, error) {
	return false, errNotImplemented
}

func (s *PostgresStore) DeleteSecret(ctx context.Context, key, scope, scopeID string) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteSecretsByScope(ctx context.Context, scope, scopeID string) (int, error) {
	return 0, errNotImplemented
}

func (s *PostgresStore) ListSecrets(ctx context.Context, filter store.SecretFilter) ([]store.Secret, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetSecretValue(ctx context.Context, key, scope, scopeID string) (string, error) {
	return "", errNotImplemented
}

func (s *PostgresStore) ListProgenySecrets(ctx context.Context, ancestorIDs []string) ([]store.Secret, error) {
	return nil, errNotImplemented
}

// ============================================================================
// Group Operations
// ============================================================================

func (s *PostgresStore) CreateGroup(ctx context.Context, group *store.Group) error {
	return errNotImplemented
}

func (s *PostgresStore) GetGroup(ctx context.Context, id string) (*store.Group, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetGroupBySlug(ctx context.Context, slug string) (*store.Group, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateGroup(ctx context.Context, group *store.Group) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteGroup(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) ListGroups(ctx context.Context, filter store.GroupFilter, opts store.ListOptions) (*store.ListResult[store.Group], error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) AddGroupMember(ctx context.Context, member *store.GroupMember) error {
	return errNotImplemented
}

func (s *PostgresStore) UpdateGroupMemberRole(ctx context.Context, groupID, memberType, memberID, newRole string) error {
	return errNotImplemented
}

func (s *PostgresStore) RemoveGroupMember(ctx context.Context, groupID, memberType, memberID string) error {
	return errNotImplemented
}

func (s *PostgresStore) GetGroupMembers(ctx context.Context, groupID string) ([]store.GroupMember, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetUserGroups(ctx context.Context, userID string) ([]store.GroupMember, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetGroupMembership(ctx context.Context, groupID, memberType, memberID string) (*store.GroupMember, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) WouldCreateCycle(ctx context.Context, groupID, memberGroupID string) (bool, error) {
	return false, errNotImplemented
}

func (s *PostgresStore) GetGroupByProjectID(ctx context.Context, projectID string) (*store.Group, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetEffectiveGroups(ctx context.Context, userID string) ([]string, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetEffectiveGroupsForAgent(ctx context.Context, agentID string) ([]string, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) CheckDelegatedAccess(ctx context.Context, agentID string, conditions *store.PolicyConditions) (bool, error) {
	return false, errNotImplemented
}

func (s *PostgresStore) CountGroupMembersByRole(ctx context.Context, groupID, role string) (int, error) {
	return 0, errNotImplemented
}

func (s *PostgresStore) GetGroupsByIDs(ctx context.Context, ids []string) ([]store.Group, error) {
	return nil, errNotImplemented
}

// ============================================================================
// Policy Operations
// ============================================================================

func (s *PostgresStore) CreatePolicy(ctx context.Context, policy *store.Policy) error {
	return errNotImplemented
}

func (s *PostgresStore) GetPolicy(ctx context.Context, id string) (*store.Policy, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdatePolicy(ctx context.Context, policy *store.Policy) error {
	return errNotImplemented
}

func (s *PostgresStore) DeletePolicy(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) ListPolicies(ctx context.Context, filter store.PolicyFilter, opts store.ListOptions) (*store.ListResult[store.Policy], error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) AddPolicyBinding(ctx context.Context, binding *store.PolicyBinding) error {
	return errNotImplemented
}

func (s *PostgresStore) RemovePolicyBinding(ctx context.Context, policyID, principalType, principalID string) error {
	return errNotImplemented
}

func (s *PostgresStore) GetPolicyBindings(ctx context.Context, policyID string) ([]store.PolicyBinding, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetPoliciesForPrincipal(ctx context.Context, principalType, principalID string) ([]store.Policy, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetPoliciesForPrincipals(ctx context.Context, principals []store.PrincipalRef) ([]store.Policy, error) {
	return nil, errNotImplemented
}

// ============================================================================
// UserAccessToken Operations
// ============================================================================

func (s *PostgresStore) CreateUserAccessToken(ctx context.Context, token *store.UserAccessToken) error {
	return errNotImplemented
}

func (s *PostgresStore) GetUserAccessToken(ctx context.Context, id string) (*store.UserAccessToken, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetUserAccessTokenByHash(ctx context.Context, hash string) (*store.UserAccessToken, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateUserAccessTokenLastUsed(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) RevokeUserAccessToken(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteUserAccessToken(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) ListUserAccessTokens(ctx context.Context, userID string) ([]store.UserAccessToken, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) CountUserAccessTokens(ctx context.Context, userID string) (int, error) {
	return 0, errNotImplemented
}

// ============================================================================
// BrokerSecret Operations
// ============================================================================

func (s *PostgresStore) CreateBrokerSecret(ctx context.Context, secret *store.BrokerSecret) error {
	return errNotImplemented
}

func (s *PostgresStore) GetBrokerSecret(ctx context.Context, brokerID string) (*store.BrokerSecret, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetActiveSecrets(ctx context.Context, brokerID string) ([]*store.BrokerSecret, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateBrokerSecret(ctx context.Context, secret *store.BrokerSecret) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteBrokerSecret(ctx context.Context, brokerID string) error {
	return errNotImplemented
}

func (s *PostgresStore) CreateJoinToken(ctx context.Context, token *store.BrokerJoinToken) error {
	return errNotImplemented
}

func (s *PostgresStore) GetJoinToken(ctx context.Context, tokenHash string) (*store.BrokerJoinToken, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetJoinTokenByBrokerID(ctx context.Context, brokerID string) (*store.BrokerJoinToken, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) DeleteJoinToken(ctx context.Context, brokerID string) error {
	return errNotImplemented
}

func (s *PostgresStore) CleanExpiredJoinTokens(ctx context.Context) error {
	return errNotImplemented
}

// ============================================================================
// Notification Operations
// ============================================================================

func (s *PostgresStore) CreateNotificationSubscription(ctx context.Context, sub *store.NotificationSubscription) error {
	return errNotImplemented
}

func (s *PostgresStore) GetNotificationSubscription(ctx context.Context, id string) (*store.NotificationSubscription, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetNotificationSubscriptions(ctx context.Context, agentID string) ([]store.NotificationSubscription, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetNotificationSubscriptionsByProject(ctx context.Context, projectID string) ([]store.NotificationSubscription, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetNotificationSubscriptionsByProjectScope(ctx context.Context, projectID string) ([]store.NotificationSubscription, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetSubscriptionsForSubscriber(ctx context.Context, subscriberType, subscriberID string) ([]store.NotificationSubscription, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateNotificationSubscriptionTriggers(ctx context.Context, id string, triggerActivities []string) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteNotificationSubscription(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteNotificationSubscriptionsForAgent(ctx context.Context, agentID string) error {
	return errNotImplemented
}

func (s *PostgresStore) CreateNotification(ctx context.Context, notif *store.Notification) error {
	return errNotImplemented
}

func (s *PostgresStore) GetNotifications(ctx context.Context, subscriberType, subscriberID string, onlyUnacknowledged bool) ([]store.Notification, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetNotificationsByAgent(ctx context.Context, agentID, subscriberType, subscriberID string, onlyUnacknowledged bool) ([]store.Notification, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) AcknowledgeNotification(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) AcknowledgeAllNotifications(ctx context.Context, subscriberType, subscriberID string) error {
	return errNotImplemented
}

func (s *PostgresStore) MarkNotificationDispatched(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) GetLastNotificationStatus(ctx context.Context, subscriptionID string) (string, error) {
	return "", errNotImplemented
}

func (s *PostgresStore) CreateSubscriptionTemplate(ctx context.Context, tmpl *store.SubscriptionTemplate) error {
	return errNotImplemented
}

func (s *PostgresStore) GetSubscriptionTemplate(ctx context.Context, id string) (*store.SubscriptionTemplate, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) ListSubscriptionTemplates(ctx context.Context, projectID string) ([]store.SubscriptionTemplate, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) DeleteSubscriptionTemplate(ctx context.Context, id string) error {
	return errNotImplemented
}

// ============================================================================
// ScheduledEvent Operations
// ============================================================================

func (s *PostgresStore) CreateScheduledEvent(ctx context.Context, event *store.ScheduledEvent) error {
	return errNotImplemented
}

func (s *PostgresStore) GetScheduledEvent(ctx context.Context, id string) (*store.ScheduledEvent, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) ListPendingScheduledEvents(ctx context.Context) ([]store.ScheduledEvent, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateScheduledEventStatus(ctx context.Context, id string, status string, firedAt *time.Time, errMsg string) error {
	return errNotImplemented
}

func (s *PostgresStore) CancelScheduledEvent(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) ListScheduledEvents(ctx context.Context, filter store.ScheduledEventFilter, opts store.ListOptions) (*store.ListResult[store.ScheduledEvent], error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) PurgeOldScheduledEvents(ctx context.Context, cutoff time.Time) (int, error) {
	return 0, errNotImplemented
}

// ============================================================================
// Schedule Operations
// ============================================================================

func (s *PostgresStore) CreateSchedule(ctx context.Context, schedule *store.Schedule) error {
	return errNotImplemented
}

func (s *PostgresStore) GetSchedule(ctx context.Context, id string) (*store.Schedule, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) ListSchedules(ctx context.Context, filter store.ScheduleFilter, opts store.ListOptions) (*store.ListResult[store.Schedule], error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateSchedule(ctx context.Context, schedule *store.Schedule) error {
	return errNotImplemented
}

func (s *PostgresStore) UpdateScheduleStatus(ctx context.Context, id string, status string) error {
	return errNotImplemented
}

func (s *PostgresStore) UpdateScheduleAfterRun(ctx context.Context, id string, ranAt time.Time, nextRunAt time.Time, errMsg string) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteSchedule(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) ListDueSchedules(ctx context.Context, now time.Time) ([]store.Schedule, error) {
	return nil, errNotImplemented
}

// ============================================================================
// GCPServiceAccount Operations
// ============================================================================

func (s *PostgresStore) CreateGCPServiceAccount(ctx context.Context, sa *store.GCPServiceAccount) error {
	return errNotImplemented
}

func (s *PostgresStore) GetGCPServiceAccount(ctx context.Context, id string) (*store.GCPServiceAccount, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateGCPServiceAccount(ctx context.Context, sa *store.GCPServiceAccount) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteGCPServiceAccount(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) ListGCPServiceAccounts(ctx context.Context, filter store.GCPServiceAccountFilter) ([]store.GCPServiceAccount, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) CountGCPServiceAccounts(ctx context.Context, filter store.GCPServiceAccountFilter) (int, error) {
	return 0, errNotImplemented
}

// ============================================================================
// GitHubInstallation Operations
// ============================================================================

func (s *PostgresStore) CreateGitHubInstallation(ctx context.Context, installation *store.GitHubInstallation) error {
	return errNotImplemented
}

func (s *PostgresStore) GetGitHubInstallation(ctx context.Context, installationID int64) (*store.GitHubInstallation, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateGitHubInstallation(ctx context.Context, installation *store.GitHubInstallation) error {
	return errNotImplemented
}

func (s *PostgresStore) DeleteGitHubInstallation(ctx context.Context, installationID int64) error {
	return errNotImplemented
}

func (s *PostgresStore) ListGitHubInstallations(ctx context.Context, filter store.GitHubInstallationFilter) ([]store.GitHubInstallation, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetInstallationForRepository(ctx context.Context, repoFullName string) (*store.GitHubInstallation, error) {
	return nil, errNotImplemented
}

// ============================================================================
// Message Operations
// ============================================================================

func (s *PostgresStore) CreateMessage(ctx context.Context, msg *store.Message) error {
	return errNotImplemented
}

func (s *PostgresStore) GetMessage(ctx context.Context, id string) (*store.Message, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) ListMessages(ctx context.Context, filter store.MessageFilter, opts store.ListOptions) (*store.ListResult[store.Message], error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) MarkMessageRead(ctx context.Context, id string) error {
	return errNotImplemented
}

func (s *PostgresStore) MarkAllMessagesRead(ctx context.Context, recipientID string) error {
	return errNotImplemented
}

func (s *PostgresStore) PurgeOldMessages(ctx context.Context, readCutoff time.Time, unreadCutoff time.Time) (int, error) {
	return 0, errNotImplemented
}

// ============================================================================
// Maintenance Operations
// ============================================================================

func (s *PostgresStore) ListMaintenanceOperations(ctx context.Context) ([]store.MaintenanceOperation, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) GetMaintenanceOperation(ctx context.Context, key string) (*store.MaintenanceOperation, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) UpdateMaintenanceOperation(ctx context.Context, op *store.MaintenanceOperation) error {
	return errNotImplemented
}

func (s *PostgresStore) CreateMaintenanceRun(ctx context.Context, run *store.MaintenanceOperationRun) error {
	return errNotImplemented
}

func (s *PostgresStore) UpdateMaintenanceRun(ctx context.Context, run *store.MaintenanceOperationRun) error {
	return errNotImplemented
}

func (s *PostgresStore) GetMaintenanceRun(ctx context.Context, id string) (*store.MaintenanceOperationRun, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) ListMaintenanceRuns(ctx context.Context, operationKey string, limit int) ([]store.MaintenanceOperationRun, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) AbortRunningMaintenanceOps(ctx context.Context) (int64, int64, error) {
	return 0, 0, errNotImplemented
}

// ============================================================================
// ProjectSyncState Operations
// ============================================================================

func (s *PostgresStore) UpsertProjectSyncState(ctx context.Context, state *store.ProjectSyncState) error {
	return errNotImplemented
}

func (s *PostgresStore) GetProjectSyncState(ctx context.Context, projectID, brokerID string) (*store.ProjectSyncState, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) ListProjectSyncStates(ctx context.Context, projectID string) ([]store.ProjectSyncState, error) {
	return nil, errNotImplemented
}

func (s *PostgresStore) DeleteProjectSyncState(ctx context.Context, projectID, brokerID string) error {
	return errNotImplemented
}
