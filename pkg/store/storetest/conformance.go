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

// Package storetest provides a shared conformance test suite for store.Store implementations.
package storetest

import (
	"context"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/api"
	"github.com/GoogleCloudPlatform/scion/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RunAll runs all conformance tests against the store returned by newStore.
// newStore is called once per top-level subtest, so each domain gets a fresh store.
func RunAll(t *testing.T, newStore func(t *testing.T) store.Store) {
	t.Helper()
	t.Run("Agents", func(t *testing.T) { testAgents(t, newStore(t)) })
	t.Run("Projects", func(t *testing.T) { testProjects(t, newStore(t)) })
	t.Run("RuntimeBrokers", func(t *testing.T) { testRuntimeBrokers(t, newStore(t)) })
	t.Run("Users", func(t *testing.T) { testUsers(t, newStore(t)) })
	t.Run("EnvVars", func(t *testing.T) { testEnvVars(t, newStore(t)) })
	t.Run("Secrets", func(t *testing.T) { testSecrets(t, newStore(t)) })
	t.Run("Groups", func(t *testing.T) { testGroups(t, newStore(t)) })
	t.Run("Policies", func(t *testing.T) { testPolicies(t, newStore(t)) })
	t.Run("UserAccessTokens", func(t *testing.T) { testUserAccessTokens(t, newStore(t)) })
	t.Run("BrokerSecrets", func(t *testing.T) { testBrokerSecrets(t, newStore(t)) })
	t.Run("Messages", func(t *testing.T) { testMessages(t, newStore(t)) })
	t.Run("Notifications", func(t *testing.T) { testNotifications(t, newStore(t)) })
	t.Run("ScheduledEvents", func(t *testing.T) { testScheduledEvents(t, newStore(t)) })
	t.Run("Schedules", func(t *testing.T) { testSchedules(t, newStore(t)) })
	t.Run("GCPServiceAccounts", func(t *testing.T) { testGCPServiceAccounts(t, newStore(t)) })
	t.Run("GitHubInstallations", func(t *testing.T) { testGitHubInstallations(t, newStore(t)) })
	t.Run("MaintenanceOps", func(t *testing.T) { testMaintenanceOps(t, newStore(t)) })
	t.Run("ProjectSyncState", func(t *testing.T) { testProjectSyncState(t, newStore(t)) })
	t.Run("AllowList", func(t *testing.T) { testAllowList(t, newStore(t)) })
	t.Run("InviteCodes", func(t *testing.T) { testInviteCodes(t, newStore(t)) })
}

// ----------------------------------------------------------------------------
// Agents
// ----------------------------------------------------------------------------

func testAgents(t *testing.T, s store.Store) {
	ctx := context.Background()

	proj := makeProject(t, ctx, s)

	agent := &store.Agent{
		ID:         api.NewUUID(),
		Slug:       "test-agent",
		Name:       "Test Agent",
		Template:   "claude",
		ProjectID:  proj.ID,
		Phase:      "created",
		Visibility: store.VisibilityPrivate,
	}
	require.NoError(t, s.CreateAgent(ctx, agent))
	assert.NotZero(t, agent.Created)
	assert.Equal(t, int64(1), agent.StateVersion)

	got, err := s.GetAgent(ctx, agent.ID)
	require.NoError(t, err)
	assert.Equal(t, agent.ID, got.ID)
	assert.Equal(t, agent.Name, got.Name)

	bySlug, err := s.GetAgentBySlug(ctx, proj.ID, "test-agent")
	require.NoError(t, err)
	assert.Equal(t, agent.ID, bySlug.ID)

	got.Name = "Updated"
	require.NoError(t, s.UpdateAgent(ctx, got))
	assert.Equal(t, int64(2), got.StateVersion)

	// Version conflict
	got.StateVersion = 1
	assert.ErrorIs(t, s.UpdateAgent(ctx, got), store.ErrVersionConflict)

	require.NoError(t, s.DeleteAgent(ctx, agent.ID))
	_, err = s.GetAgent(ctx, agent.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// Projects
// ----------------------------------------------------------------------------

func testProjects(t *testing.T, s store.Store) {
	ctx := context.Background()

	proj := &store.Project{
		ID:         api.NewUUID(),
		Name:       "My Project",
		Slug:       "my-project",
		GitRemote:  "github.com/org/repo",
		Visibility: store.VisibilityPrivate,
	}
	require.NoError(t, s.CreateProject(ctx, proj))
	assert.NotZero(t, proj.Created)

	got, err := s.GetProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, proj.Name, got.Name)

	bySlug, err := s.GetProjectBySlug(ctx, "my-project")
	require.NoError(t, err)
	assert.Equal(t, proj.ID, bySlug.ID)

	byRemote, err := s.GetProjectsByGitRemote(ctx, "github.com/org/repo")
	require.NoError(t, err)
	require.Len(t, byRemote, 1)

	got.Name = "Updated Project"
	require.NoError(t, s.UpdateProject(ctx, got))

	got2, err := s.GetProject(ctx, proj.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Project", got2.Name)

	require.NoError(t, s.DeleteProject(ctx, proj.ID))
	_, err = s.GetProject(ctx, proj.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// RuntimeBrokers
// ----------------------------------------------------------------------------

func testRuntimeBrokers(t *testing.T, s store.Store) {
	ctx := context.Background()

	broker := &store.RuntimeBroker{
		ID:     api.NewUUID(),
		Name:   "Dev Laptop",
		Slug:   "dev-laptop",
		Status: store.BrokerStatusOnline,
	}
	require.NoError(t, s.CreateRuntimeBroker(ctx, broker))
	assert.NotZero(t, broker.Created)

	got, err := s.GetRuntimeBroker(ctx, broker.ID)
	require.NoError(t, err)
	assert.Equal(t, broker.Name, got.Name)

	byName, err := s.GetRuntimeBrokerByName(ctx, "Dev Laptop")
	require.NoError(t, err)
	assert.Equal(t, broker.ID, byName.ID)

	got.Status = store.BrokerStatusOffline
	require.NoError(t, s.UpdateRuntimeBroker(ctx, got))

	got2, err := s.GetRuntimeBroker(ctx, broker.ID)
	require.NoError(t, err)
	assert.Equal(t, store.BrokerStatusOffline, got2.Status)

	require.NoError(t, s.UpdateRuntimeBrokerHeartbeat(ctx, broker.ID, store.BrokerStatusOnline))

	require.NoError(t, s.DeleteRuntimeBroker(ctx, broker.ID))
	_, err = s.GetRuntimeBroker(ctx, broker.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// Users
// ----------------------------------------------------------------------------

func testUsers(t *testing.T, s store.Store) {
	ctx := context.Background()

	user := &store.User{
		ID:          api.NewUUID(),
		Email:       "alice@example.com",
		DisplayName: "Alice",
		Role:        store.UserRoleMember,
		Status:      "active",
	}
	require.NoError(t, s.CreateUser(ctx, user))
	assert.NotZero(t, user.Created)

	got, err := s.GetUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.Email, got.Email)

	byEmail, err := s.GetUserByEmail(ctx, "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, user.ID, byEmail.ID)

	got.DisplayName = "Alice Updated"
	require.NoError(t, s.UpdateUser(ctx, got))

	got2, err := s.GetUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "Alice Updated", got2.DisplayName)

	require.NoError(t, s.DeleteUser(ctx, user.ID))
	_, err = s.GetUser(ctx, user.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// EnvVars
// ----------------------------------------------------------------------------

func testEnvVars(t *testing.T, s store.Store) {
	ctx := context.Background()

	ev := &store.EnvVar{
		ID:      api.NewUUID(),
		Key:     "LOG_LEVEL",
		Value:   "debug",
		Scope:   store.ScopeUser,
		ScopeID: "user-123",
	}
	require.NoError(t, s.CreateEnvVar(ctx, ev))

	got, err := s.GetEnvVar(ctx, "LOG_LEVEL", store.ScopeUser, "user-123")
	require.NoError(t, err)
	assert.Equal(t, "debug", got.Value)

	got.Value = "info"
	require.NoError(t, s.UpdateEnvVar(ctx, got))

	got2, err := s.GetEnvVar(ctx, "LOG_LEVEL", store.ScopeUser, "user-123")
	require.NoError(t, err)
	assert.Equal(t, "info", got2.Value)

	list, err := s.ListEnvVars(ctx, store.EnvVarFilter{Scope: store.ScopeUser, ScopeID: "user-123"})
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, s.DeleteEnvVar(ctx, "LOG_LEVEL", store.ScopeUser, "user-123"))
	_, err = s.GetEnvVar(ctx, "LOG_LEVEL", store.ScopeUser, "user-123")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// Secrets
// ----------------------------------------------------------------------------

func testSecrets(t *testing.T, s store.Store) {
	ctx := context.Background()

	sec := &store.Secret{
		ID:             api.NewUUID(),
		Key:            "API_KEY",
		EncryptedValue: "enc:abc123",
		Scope:          store.ScopeProject,
		ScopeID:        "proj-123",
		SecretType:     store.SecretTypeEnvironment,
	}
	require.NoError(t, s.CreateSecret(ctx, sec))

	got, err := s.GetSecret(ctx, "API_KEY", store.ScopeProject, "proj-123")
	require.NoError(t, err)
	assert.Equal(t, sec.ID, got.ID)

	val, err := s.GetSecretValue(ctx, "API_KEY", store.ScopeProject, "proj-123")
	require.NoError(t, err)
	assert.Equal(t, "enc:abc123", val)

	got.EncryptedValue = "enc:xyz456"
	require.NoError(t, s.UpdateSecret(ctx, got))

	val2, err := s.GetSecretValue(ctx, "API_KEY", store.ScopeProject, "proj-123")
	require.NoError(t, err)
	assert.Equal(t, "enc:xyz456", val2)

	require.NoError(t, s.DeleteSecret(ctx, "API_KEY", store.ScopeProject, "proj-123"))
	_, err = s.GetSecret(ctx, "API_KEY", store.ScopeProject, "proj-123")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// Groups
// ----------------------------------------------------------------------------

func testGroups(t *testing.T, s store.Store) {
	ctx := context.Background()
	userID := api.NewUUID()

	grp := &store.Group{
		ID:        api.NewUUID(),
		Name:      "Eng Team",
		Slug:      "eng-team",
		GroupType: store.GroupTypeExplicit,
	}
	require.NoError(t, s.CreateGroup(ctx, grp))

	got, err := s.GetGroup(ctx, grp.ID)
	require.NoError(t, err)
	assert.Equal(t, grp.Name, got.Name)

	bySlug, err := s.GetGroupBySlug(ctx, "eng-team")
	require.NoError(t, err)
	assert.Equal(t, grp.ID, bySlug.ID)

	got.Name = "Engineering Team"
	require.NoError(t, s.UpdateGroup(ctx, got))

	member := &store.GroupMember{
		GroupID:    grp.ID,
		MemberType: store.GroupMemberTypeUser,
		MemberID:   userID,
		Role:       store.GroupMemberRoleMember,
	}
	require.NoError(t, s.AddGroupMember(ctx, member))

	members, err := s.GetGroupMembers(ctx, grp.ID)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, userID, members[0].MemberID)

	require.NoError(t, s.RemoveGroupMember(ctx, grp.ID, store.GroupMemberTypeUser, userID))

	require.NoError(t, s.DeleteGroup(ctx, grp.ID))
	_, err = s.GetGroup(ctx, grp.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// Policies
// ----------------------------------------------------------------------------

func testPolicies(t *testing.T, s store.Store) {
	ctx := context.Background()

	pol := &store.Policy{
		ID:           api.NewUUID(),
		Name:         "Read All",
		ScopeType:    store.PolicyScopeHub,
		ResourceType: "*",
		Actions:      []string{"read"},
		Effect:       store.PolicyEffectAllow,
	}
	require.NoError(t, s.CreatePolicy(ctx, pol))

	got, err := s.GetPolicy(ctx, pol.ID)
	require.NoError(t, err)
	assert.Equal(t, pol.Name, got.Name)

	got.Name = "Read All Updated"
	require.NoError(t, s.UpdatePolicy(ctx, got))

	got2, err := s.GetPolicy(ctx, pol.ID)
	require.NoError(t, err)
	assert.Equal(t, "Read All Updated", got2.Name)

	require.NoError(t, s.DeletePolicy(ctx, pol.ID))
	_, err = s.GetPolicy(ctx, pol.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// UserAccessTokens
// ----------------------------------------------------------------------------

func testUserAccessTokens(t *testing.T, s store.Store) {
	ctx := context.Background()
	proj := makeProject(t, ctx, s)
	user := makeUser(t, ctx, s)
	userID := user.ID
	projID := proj.ID
	exp := time.Now().Add(24 * time.Hour)

	tok := &store.UserAccessToken{
		ID:        api.NewUUID(),
		UserID:    userID,
		Name:      "My Token",
		Prefix:    "scion_pat_abc",
		KeyHash:   "sha256:abc123",
		ProjectID: projID,
		Scopes:    []string{store.UATScopeAgentRead},
		ExpiresAt: &exp,
	}
	require.NoError(t, s.CreateUserAccessToken(ctx, tok))

	got, err := s.GetUserAccessToken(ctx, tok.ID)
	require.NoError(t, err)
	assert.Equal(t, tok.Name, got.Name)

	byHash, err := s.GetUserAccessTokenByHash(ctx, "sha256:abc123")
	require.NoError(t, err)
	assert.Equal(t, tok.ID, byHash.ID)

	require.NoError(t, s.RevokeUserAccessToken(ctx, tok.ID))
	got2, err := s.GetUserAccessToken(ctx, tok.ID)
	require.NoError(t, err)
	assert.True(t, got2.Revoked)

	require.NoError(t, s.DeleteUserAccessToken(ctx, tok.ID))
	_, err = s.GetUserAccessToken(ctx, tok.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// BrokerSecrets
// ----------------------------------------------------------------------------

func testBrokerSecrets(t *testing.T, s store.Store) {
	ctx := context.Background()

	// Create broker first
	broker := &store.RuntimeBroker{
		ID:     api.NewUUID(),
		Name:   "Secret Broker",
		Slug:   "secret-broker",
		Status: store.BrokerStatusOnline,
	}
	require.NoError(t, s.CreateRuntimeBroker(ctx, broker))

	secret := &store.BrokerSecret{
		BrokerID:  broker.ID,
		SecretKey: []byte("supersecret"),
		Algorithm: "hmac-sha256",
		CreatedAt: time.Now(),
		Status:    "active",
	}
	require.NoError(t, s.CreateBrokerSecret(ctx, secret))

	got, err := s.GetBrokerSecret(ctx, broker.ID)
	require.NoError(t, err)
	assert.Equal(t, broker.ID, got.BrokerID)

	got.Status = "deprecated"
	require.NoError(t, s.UpdateBrokerSecret(ctx, got))

	got2, err := s.GetBrokerSecret(ctx, broker.ID)
	require.NoError(t, err)
	assert.Equal(t, "deprecated", got2.Status)

	require.NoError(t, s.DeleteBrokerSecret(ctx, broker.ID))
	_, err = s.GetBrokerSecret(ctx, broker.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// Messages
// ----------------------------------------------------------------------------

func testMessages(t *testing.T, s store.Store) {
	ctx := context.Background()
	proj := makeProject(t, ctx, s)

	msg := &store.Message{
		ID:          api.NewUUID(),
		ProjectID:   proj.ID,
		Sender:      "user:alice",
		SenderID:    "user-alice",
		Recipient:   "agent:bot",
		RecipientID: "agent-bot",
		AgentID:     "agent-bot",
		Msg:         "Hello",
		Type:        "instruction",
	}
	require.NoError(t, s.CreateMessage(ctx, msg))
	assert.NotZero(t, msg.CreatedAt)

	got, err := s.GetMessage(ctx, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, msg.Msg, got.Msg)
	assert.False(t, got.Read)

	require.NoError(t, s.MarkMessageRead(ctx, msg.ID))
	got2, err := s.GetMessage(ctx, msg.ID)
	require.NoError(t, err)
	assert.True(t, got2.Read)

	result, err := s.ListMessages(ctx, store.MessageFilter{ProjectID: proj.ID}, store.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
}

// ----------------------------------------------------------------------------
// Notifications
// ----------------------------------------------------------------------------

func testNotifications(t *testing.T, s store.Store) {
	ctx := context.Background()
	proj := makeProject(t, ctx, s)
	agent := makeAgent(t, ctx, s, proj.ID)
	agentID := agent.ID

	sub := &store.NotificationSubscription{
		ID:                api.NewUUID(),
		Scope:             store.SubscriptionScopeAgent,
		AgentID:           agentID,
		SubscriberType:    store.SubscriberTypeUser,
		SubscriberID:      "user-bob",
		ProjectID:         proj.ID,
		TriggerActivities: []string{"COMPLETED"},
		CreatedBy:         "user-bob",
	}
	require.NoError(t, s.CreateNotificationSubscription(ctx, sub))

	got, err := s.GetNotificationSubscription(ctx, sub.ID)
	require.NoError(t, err)
	assert.Equal(t, sub.ID, got.ID)
	assert.Equal(t, []string{"COMPLETED"}, got.TriggerActivities)

	require.NoError(t, s.UpdateNotificationSubscriptionTriggers(ctx, sub.ID, []string{"COMPLETED", "FAILED"}))
	got2, err := s.GetNotificationSubscription(ctx, sub.ID)
	require.NoError(t, err)
	assert.Len(t, got2.TriggerActivities, 2)

	notif := &store.Notification{
		ID:             api.NewUUID(),
		SubscriptionID: sub.ID,
		AgentID:        agentID,
		ProjectID:      proj.ID,
		SubscriberType: store.SubscriberTypeUser,
		SubscriberID:   "user-bob",
		Status:         "COMPLETED",
		Message:        "Agent finished",
	}
	require.NoError(t, s.CreateNotification(ctx, notif))

	notifs, err := s.GetNotifications(ctx, store.SubscriberTypeUser, "user-bob", false)
	require.NoError(t, err)
	assert.Len(t, notifs, 1)

	require.NoError(t, s.AcknowledgeNotification(ctx, notif.ID))

	require.NoError(t, s.DeleteNotificationSubscription(ctx, sub.ID))
	_, err = s.GetNotificationSubscription(ctx, sub.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// ScheduledEvents
// ----------------------------------------------------------------------------

func testScheduledEvents(t *testing.T, s store.Store) {
	ctx := context.Background()
	proj := makeProject(t, ctx, s)

	ev := &store.ScheduledEvent{
		ID:        api.NewUUID(),
		ProjectID: proj.ID,
		EventType: "message",
		FireAt:    time.Now().Add(time.Hour),
		Payload:   `{"msg":"hello"}`,
		Status:    store.ScheduledEventPending,
		CreatedAt: time.Now(),
		CreatedBy: "user-alice",
	}
	require.NoError(t, s.CreateScheduledEvent(ctx, ev))

	got, err := s.GetScheduledEvent(ctx, ev.ID)
	require.NoError(t, err)
	assert.Equal(t, ev.EventType, got.EventType)
	assert.Equal(t, store.ScheduledEventPending, got.Status)

	now := time.Now()
	require.NoError(t, s.UpdateScheduledEventStatus(ctx, ev.ID, store.ScheduledEventFired, &now, ""))

	got2, err := s.GetScheduledEvent(ctx, ev.ID)
	require.NoError(t, err)
	assert.Equal(t, store.ScheduledEventFired, got2.Status)
}

// ----------------------------------------------------------------------------
// Schedules
// ----------------------------------------------------------------------------

func testSchedules(t *testing.T, s store.Store) {
	ctx := context.Background()
	proj := makeProject(t, ctx, s)

	sched := &store.Schedule{
		ID:        api.NewUUID(),
		ProjectID: proj.ID,
		Name:      "Daily Ping",
		CronExpr:  "0 9 * * *",
		EventType: "message",
		Payload:   `{}`,
		Status:    store.ScheduleStatusActive,
	}
	require.NoError(t, s.CreateSchedule(ctx, sched))

	got, err := s.GetSchedule(ctx, sched.ID)
	require.NoError(t, err)
	assert.Equal(t, sched.Name, got.Name)
	assert.Equal(t, store.ScheduleStatusActive, got.Status)

	got.CronExpr = "0 10 * * *"
	require.NoError(t, s.UpdateSchedule(ctx, got))

	got2, err := s.GetSchedule(ctx, sched.ID)
	require.NoError(t, err)
	assert.Equal(t, "0 10 * * *", got2.CronExpr)

	list, err := s.ListSchedules(ctx, store.ScheduleFilter{ProjectID: proj.ID}, store.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, list.TotalCount)

	require.NoError(t, s.DeleteSchedule(ctx, sched.ID))
	_, err = s.GetSchedule(ctx, sched.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// GCP Service Accounts
// ----------------------------------------------------------------------------

func testGCPServiceAccounts(t *testing.T, s store.Store) {
	ctx := context.Background()

	sa := &store.GCPServiceAccount{
		ID:                 api.NewUUID(),
		Scope:              store.ScopeHub,
		ScopeID:            "hub-1",
		Email:              "agent-worker@myproject.iam.gserviceaccount.com",
		ProjectID:          "myproject",
		DisplayName:        "Agent Worker SA",
		VerificationStatus: "unverified",
		CreatedBy:          "admin",
		CreatedAt:          time.Now(),
	}
	require.NoError(t, s.CreateGCPServiceAccount(ctx, sa))

	got, err := s.GetGCPServiceAccount(ctx, sa.ID)
	require.NoError(t, err)
	assert.Equal(t, sa.Email, got.Email)

	got.DisplayName = "Updated SA"
	require.NoError(t, s.UpdateGCPServiceAccount(ctx, got))

	got2, err := s.GetGCPServiceAccount(ctx, sa.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated SA", got2.DisplayName)

	require.NoError(t, s.DeleteGCPServiceAccount(ctx, sa.ID))
	_, err = s.GetGCPServiceAccount(ctx, sa.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// GitHub Installations
// ----------------------------------------------------------------------------

func testGitHubInstallations(t *testing.T, s store.Store) {
	ctx := context.Background()

	inst := &store.GitHubInstallation{
		InstallationID: 12345,
		AccountLogin:   "myorg",
		AccountType:    "Organization",
		AppID:          67890,
		Repositories:   []string{"myorg/repo-a", "myorg/repo-b"},
		Status:         store.GitHubInstallationStatusActive,
	}
	require.NoError(t, s.CreateGitHubInstallation(ctx, inst))

	got, err := s.GetGitHubInstallation(ctx, 12345)
	require.NoError(t, err)
	assert.Equal(t, "myorg", got.AccountLogin)
	assert.Len(t, got.Repositories, 2)

	got.Repositories = append(got.Repositories, "myorg/repo-c")
	require.NoError(t, s.UpdateGitHubInstallation(ctx, got))

	got2, err := s.GetGitHubInstallation(ctx, 12345)
	require.NoError(t, err)
	assert.Len(t, got2.Repositories, 3)

	require.NoError(t, s.DeleteGitHubInstallation(ctx, 12345))
	_, err = s.GetGitHubInstallation(ctx, 12345)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// Maintenance Operations
// ----------------------------------------------------------------------------

func testMaintenanceOps(t *testing.T, s store.Store) {
	ctx := context.Background()

	// Schema migration seeds maintenance_operations rows; at least 1 must exist.
	ops, err := s.ListMaintenanceOperations(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, ops, "expected seeded maintenance operations after migration")

	key := ops[0].Key
	op, err := s.GetMaintenanceOperation(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, key, op.Key)

	// Create and update a run
	now := time.Now()
	run := &store.MaintenanceOperationRun{
		ID:           api.NewUUID(),
		OperationKey: key,
		Status:       "running",
		StartedAt:    now,
		StartedBy:    "user-admin",
		Log:          "starting...",
	}
	require.NoError(t, s.CreateMaintenanceRun(ctx, run))

	gotRun, err := s.GetMaintenanceRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, "running", gotRun.Status)

	run.Status = "success"
	fin := time.Now()
	run.CompletedAt = &fin
	run.Result = `{"ok":true}`
	require.NoError(t, s.UpdateMaintenanceRun(ctx, run))

	gotRun2, err := s.GetMaintenanceRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, "success", gotRun2.Status)
}

// ----------------------------------------------------------------------------
// Project Sync State
// ----------------------------------------------------------------------------

func testProjectSyncState(t *testing.T, s store.Store) {
	ctx := context.Background()
	proj := makeProject(t, ctx, s)

	state := &store.ProjectSyncState{
		ProjectID:  proj.ID,
		BrokerID:   "broker-abc",
		FileCount:  42,
		TotalBytes: 1024,
	}
	require.NoError(t, s.UpsertProjectSyncState(ctx, state))

	got, err := s.GetProjectSyncState(ctx, proj.ID, "broker-abc")
	require.NoError(t, err)
	assert.Equal(t, 42, got.FileCount)

	state.FileCount = 100
	require.NoError(t, s.UpsertProjectSyncState(ctx, state))

	got2, err := s.GetProjectSyncState(ctx, proj.ID, "broker-abc")
	require.NoError(t, err)
	assert.Equal(t, 100, got2.FileCount)

	require.NoError(t, s.DeleteProjectSyncState(ctx, proj.ID, "broker-abc"))
	_, err = s.GetProjectSyncState(ctx, proj.ID, "broker-abc")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// Allow List
// ----------------------------------------------------------------------------

func testAllowList(t *testing.T, s store.Store) {
	ctx := context.Background()

	entry := &store.AllowListEntry{
		ID:      api.NewUUID(),
		Email:   "bob@example.com",
		AddedBy: "admin",
	}
	require.NoError(t, s.AddAllowListEntry(ctx, entry))

	got, err := s.GetAllowListEntry(ctx, "bob@example.com")
	require.NoError(t, err)
	assert.Equal(t, "bob@example.com", got.Email)

	allowed, err := s.IsEmailAllowListed(ctx, "bob@example.com")
	require.NoError(t, err)
	assert.True(t, allowed)

	result, err := s.ListAllowListEntries(ctx, store.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)

	require.NoError(t, s.RemoveAllowListEntry(ctx, "bob@example.com"))
	_, err = s.GetAllowListEntry(ctx, "bob@example.com")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// Invite Codes
// ----------------------------------------------------------------------------

func testInviteCodes(t *testing.T, s store.Store) {
	ctx := context.Background()

	invite := &store.InviteCode{
		ID:         api.NewUUID(),
		CodeHash:   "hash:abc123",
		CodePrefix: "abcd1234",
		MaxUses:    5,
		ExpiresAt:  time.Now().Add(48 * time.Hour),
		CreatedBy:  "admin",
	}
	require.NoError(t, s.CreateInviteCode(ctx, invite))

	got, err := s.GetInviteCode(ctx, invite.ID)
	require.NoError(t, err)
	assert.Equal(t, invite.CodePrefix, got.CodePrefix)

	byHash, err := s.GetInviteCodeByHash(ctx, "hash:abc123")
	require.NoError(t, err)
	assert.Equal(t, invite.ID, byHash.ID)

	require.NoError(t, s.IncrementInviteUseCount(ctx, invite.ID))
	got2, err := s.GetInviteCode(ctx, invite.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, got2.UseCount)

	require.NoError(t, s.RevokeInviteCode(ctx, invite.ID))
	got3, err := s.GetInviteCode(ctx, invite.ID)
	require.NoError(t, err)
	assert.True(t, got3.Revoked)

	require.NoError(t, s.DeleteInviteCode(ctx, invite.ID))
	_, err = s.GetInviteCode(ctx, invite.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func makeProject(t *testing.T, ctx context.Context, s store.Store) *store.Project {
	t.Helper()
	proj := &store.Project{
		ID:         api.NewUUID(),
		Name:       "Test Project",
		Slug:       "test-project-" + api.NewUUID()[:8],
		Visibility: store.VisibilityPrivate,
	}
	require.NoError(t, s.CreateProject(ctx, proj))
	return proj
}

func makeUser(t *testing.T, ctx context.Context, s store.Store) *store.User {
	t.Helper()
	user := &store.User{
		ID:          api.NewUUID(),
		Email:       api.NewUUID()[:8] + "@example.com",
		DisplayName: "Test User",
		Role:        store.UserRoleMember,
		Status:      "active",
	}
	require.NoError(t, s.CreateUser(ctx, user))
	return user
}

func makeAgent(t *testing.T, ctx context.Context, s store.Store, projectID string) *store.Agent {
	t.Helper()
	agent := &store.Agent{
		ID:         api.NewUUID(),
		Slug:       "agent-" + api.NewUUID()[:8],
		Name:       "Test Agent",
		Template:   "claude",
		ProjectID:  projectID,
		Phase:      "created",
		Visibility: store.VisibilityPrivate,
	}
	require.NoError(t, s.CreateAgent(ctx, agent))
	return agent
}
