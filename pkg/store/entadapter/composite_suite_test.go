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

package entadapter

import (
	"context"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/agent/state"
	"github.com/GoogleCloudPlatform/scion/pkg/store"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCompositeStoreSuite runs the full CompositeStore test suite using the
// provided store factory. Each subtest calls newStore(t) independently.
func runCompositeStoreSuite(t *testing.T, newStore func(t *testing.T) *CompositeStore) {
	t.Helper()

	t.Run("AddGroupMember_UserShadowRecord", func(t *testing.T) {
		cs := newStore(t)
		ctx := context.Background()

		userID := uuid.New().String()
		err := cs.Store.CreateUser(ctx, &store.User{
			ID:          userID,
			Email:       "test@example.com",
			DisplayName: "Test User",
			Role:        store.UserRoleMember,
			Status:      "active",
			Created:     time.Now(),
		})
		require.NoError(t, err)

		groupID := uuid.New().String()
		err = cs.CreateGroup(ctx, &store.Group{
			ID:        groupID,
			Name:      "Test Group",
			Slug:      "test-group",
			GroupType: store.GroupTypeExplicit,
		})
		require.NoError(t, err)

		err = cs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    groupID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   userID,
			Role:       store.GroupMemberRoleMember,
		})
		require.NoError(t, err, "AddGroupMember should succeed for user that exists only in base store")

		membership, err := cs.GetGroupMembership(ctx, groupID, store.GroupMemberTypeUser, userID)
		require.NoError(t, err)
		assert.Equal(t, userID, membership.MemberID)

		groups, err := cs.GetEffectiveGroups(ctx, userID)
		require.NoError(t, err)
		assert.Contains(t, groups, groupID)
	})

	t.Run("AddGroupMember_UserAlreadyInEnt", func(t *testing.T) {
		cs := newStore(t)
		ctx := context.Background()

		userID := uuid.New().String()
		userUID, _ := uuid.Parse(userID)

		err := cs.Store.CreateUser(ctx, &store.User{
			ID:          userID,
			Email:       "already@example.com",
			DisplayName: "Already Here",
			Role:        store.UserRoleMember,
			Status:      "active",
			Created:     time.Now(),
		})
		require.NoError(t, err)

		_, err = cs.client.User.Create().
			SetID(userUID).
			SetEmail("already@example.com").
			SetDisplayName("Already Here").
			Save(ctx)
		require.NoError(t, err)

		groupID := uuid.New().String()
		err = cs.CreateGroup(ctx, &store.Group{
			ID:        groupID,
			Name:      "Test Group 2",
			Slug:      "test-group-2",
			GroupType: store.GroupTypeExplicit,
		})
		require.NoError(t, err)

		err = cs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    groupID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   userID,
			Role:       store.GroupMemberRoleMember,
		})
		require.NoError(t, err)
	})

	t.Run("AddGroupMember_AgentShadowRecord", func(t *testing.T) {
		cs := newStore(t)
		ctx := context.Background()

		projectID := uuid.New().String()
		err := cs.Store.CreateProject(ctx, &store.Project{
			ID:      projectID,
			Name:    "Test Project",
			Slug:    "test-project",
			Created: time.Now(),
			Updated: time.Now(),
		})
		require.NoError(t, err)

		agentID := uuid.New().String()
		err = cs.Store.CreateAgent(ctx, &store.Agent{
			ID:           agentID,
			Name:         "Test Agent",
			Slug:         "test-agent",
			ProjectID:    projectID,
			Phase:        string(state.PhaseStopped),
			StateVersion: 1,
			Created:      time.Now(),
			Updated:      time.Now(),
		})
		require.NoError(t, err)

		groupID := uuid.New().String()
		err = cs.CreateGroup(ctx, &store.Group{
			ID:        groupID,
			Name:      "Test Agent Group",
			Slug:      "test-agent-group",
			GroupType: store.GroupTypeExplicit,
		})
		require.NoError(t, err)

		err = cs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    groupID,
			MemberType: store.GroupMemberTypeAgent,
			MemberID:   agentID,
			Role:       store.GroupMemberRoleMember,
		})
		require.NoError(t, err, "AddGroupMember should succeed for agent that exists only in base store")

		membership, err := cs.GetGroupMembership(ctx, groupID, store.GroupMemberTypeAgent, agentID)
		require.NoError(t, err)
		assert.Equal(t, agentID, membership.MemberID)
	})

	t.Run("AddGroupMember_Idempotent", func(t *testing.T) {
		cs := newStore(t)
		ctx := context.Background()

		userID := uuid.New().String()
		err := cs.Store.CreateUser(ctx, &store.User{
			ID:          userID,
			Email:       "idempotent@example.com",
			DisplayName: "Idempotent User",
			Role:        store.UserRoleMember,
			Status:      "active",
			Created:     time.Now(),
		})
		require.NoError(t, err)

		groupID := uuid.New().String()
		err = cs.CreateGroup(ctx, &store.Group{
			ID:        groupID,
			Name:      "Idempotent Group",
			Slug:      "idempotent-group",
			GroupType: store.GroupTypeExplicit,
		})
		require.NoError(t, err)

		member := &store.GroupMember{
			GroupID:    groupID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   userID,
			Role:       store.GroupMemberRoleMember,
		}
		err = cs.AddGroupMember(ctx, member)
		require.NoError(t, err)

		err = cs.AddGroupMember(ctx, member)
		assert.ErrorIs(t, err, store.ErrAlreadyExists)
	})

	t.Run("CreateGroup_WithProjectID", func(t *testing.T) {
		cs := newStore(t)
		ctx := context.Background()

		projectID := uuid.New().String()
		err := cs.Store.CreateProject(ctx, &store.Project{
			ID:      projectID,
			Name:    "Shadow Project",
			Slug:    "shadow-project",
			Created: time.Now(),
			Updated: time.Now(),
		})
		require.NoError(t, err)

		groupID := uuid.New().String()
		err = cs.CreateGroup(ctx, &store.Group{
			ID:        groupID,
			Name:      "Shadow Project Agents",
			Slug:      "project:shadow-project:agents",
			GroupType: store.GroupTypeProjectAgents,
			ProjectID: projectID,
		})
		require.NoError(t, err, "CreateGroup should succeed for project that exists only in base store")

		group, err := cs.GetGroup(ctx, groupID)
		require.NoError(t, err)
		assert.Equal(t, projectID, group.ProjectID)
		assert.Equal(t, "project:shadow-project:agents", group.Slug)
	})

	t.Run("CreateGroup_MultipleGroupsPerProject", func(t *testing.T) {
		cs := newStore(t)
		ctx := context.Background()

		projectID := uuid.New().String()
		err := cs.Store.CreateProject(ctx, &store.Project{
			ID:      projectID,
			Name:    "Multi-Group Project",
			Slug:    "multi-group-project",
			Created: time.Now(),
			Updated: time.Now(),
		})
		require.NoError(t, err)

		agentsGroupID := uuid.New().String()
		err = cs.CreateGroup(ctx, &store.Group{
			ID:        agentsGroupID,
			Name:      "Multi-Group Project Agents",
			Slug:      "project:multi-group-project:agents",
			GroupType: store.GroupTypeProjectAgents,
			ProjectID: projectID,
		})
		require.NoError(t, err, "agents group creation should succeed")

		membersGroupID := uuid.New().String()
		err = cs.CreateGroup(ctx, &store.Group{
			ID:        membersGroupID,
			Name:      "Multi-Group Project Members",
			Slug:      "project:multi-group-project:members",
			GroupType: store.GroupTypeExplicit,
			ProjectID: projectID,
		})
		require.NoError(t, err, "members group creation should succeed for same project")

		agents, err := cs.GetGroup(ctx, agentsGroupID)
		require.NoError(t, err)
		assert.Equal(t, projectID, agents.ProjectID)

		members, err := cs.GetGroup(ctx, membersGroupID)
		require.NoError(t, err)
		assert.Equal(t, projectID, members.ProjectID)
	})
}
