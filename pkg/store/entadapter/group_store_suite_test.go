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

	"github.com/GoogleCloudPlatform/scion/pkg/store"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testUserUID    = uuid.MustParse("10000000-0000-0000-0000-000000000001")
	testAgentUID   = uuid.MustParse("20000000-0000-0000-0000-000000000001")
	testProjectUID = uuid.MustParse("30000000-0000-0000-0000-000000000001")
)

// runGroupStoreSuite runs the full GroupStore test suite using the provided
// store factory. Each subtest calls newStore(t) independently, so the factory
// must return a store with a clean, freshly-seeded database.
func runGroupStoreSuite(t *testing.T, newStore func(t *testing.T) *GroupStore) {
	t.Helper()

	t.Run("CreateGroup", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:          uuid.New().String(),
			Name:        "Engineering",
			Slug:        "engineering",
			Description: "Engineering team",
		}

		err := gs.CreateGroup(ctx, g)
		require.NoError(t, err)
		assert.False(t, g.Created.IsZero())
		assert.False(t, g.Updated.IsZero())
		assert.Equal(t, store.GroupTypeExplicit, g.GroupType)
	})

	t.Run("CreateGroupDuplicate", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Engineering",
			Slug: "engineering",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		g2 := &store.Group{
			ID:   uuid.New().String(),
			Name: "Engineering 2",
			Slug: "engineering", // same slug
		}
		err := gs.CreateGroup(ctx, g2)
		assert.ErrorIs(t, err, store.ErrAlreadyExists)
	})

	t.Run("GetGroup", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		id := uuid.New().String()
		g := &store.Group{
			ID:          id,
			Name:        "Platform",
			Slug:        "platform",
			Description: "Platform team",
			Labels:      map[string]string{"dept": "eng"},
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		got, err := gs.GetGroup(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, id, got.ID)
		assert.Equal(t, "Platform", got.Name)
		assert.Equal(t, "platform", got.Slug)
		assert.Equal(t, "Platform team", got.Description)
		assert.Equal(t, "eng", got.Labels["dept"])
		assert.Equal(t, store.GroupTypeExplicit, got.GroupType)
	})

	t.Run("GetGroupNotFound", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		_, err := gs.GetGroup(ctx, uuid.New().String())
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("GetGroupBySlug", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Ops Team",
			Slug: "ops-team",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		got, err := gs.GetGroupBySlug(ctx, "ops-team")
		require.NoError(t, err)
		assert.Equal(t, g.ID, got.ID)
		assert.Equal(t, "Ops Team", got.Name)
	})

	t.Run("GetGroupBySlugNotFound", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		_, err := gs.GetGroupBySlug(ctx, "nonexistent")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("UpdateGroup", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Old Name",
			Slug: "old-name",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		g.Name = "New Name"
		g.Description = "Updated"
		err := gs.UpdateGroup(ctx, g)
		require.NoError(t, err)

		got, err := gs.GetGroup(ctx, g.ID)
		require.NoError(t, err)
		assert.Equal(t, "New Name", got.Name)
		assert.Equal(t, "Updated", got.Description)
	})

	t.Run("UpdateGroupNotFound", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Ghost",
			Slug: "ghost",
		}
		err := gs.UpdateGroup(ctx, g)
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("DeleteGroup", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Delete Me",
			Slug: "delete-me",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		err := gs.DeleteGroup(ctx, g.ID)
		require.NoError(t, err)

		_, err = gs.GetGroup(ctx, g.ID)
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("DeleteGroupNotFound", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		err := gs.DeleteGroup(ctx, uuid.New().String())
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("ListGroups", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		for i := 0; i < 3; i++ {
			g := &store.Group{
				ID:   uuid.New().String(),
				Name: "Group " + string(rune('A'+i)),
				Slug: "group-" + string(rune('a'+i)),
			}
			require.NoError(t, gs.CreateGroup(ctx, g))
		}

		result, err := gs.ListGroups(ctx, store.GroupFilter{}, store.ListOptions{})
		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount)
		assert.Len(t, result.Items, 3)
	})

	t.Run("ListGroupsWithGroupTypeFilter", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g1 := &store.Group{
			ID:        uuid.New().String(),
			Name:      "Explicit Group",
			Slug:      "explicit-group",
			GroupType: store.GroupTypeExplicit,
		}
		require.NoError(t, gs.CreateGroup(ctx, g1))

		g2 := &store.Group{
			ID:        uuid.New().String(),
			Name:      "Project Group",
			Slug:      "project-group",
			GroupType: store.GroupTypeProjectAgents,
		}
		require.NoError(t, gs.CreateGroup(ctx, g2))

		result, err := gs.ListGroups(ctx, store.GroupFilter{GroupType: store.GroupTypeExplicit}, store.ListOptions{})
		require.NoError(t, err)
		assert.Equal(t, 1, result.TotalCount)
		assert.Equal(t, store.GroupTypeExplicit, result.Items[0].GroupType)

		result, err = gs.ListGroups(ctx, store.GroupFilter{GroupType: store.GroupTypeProjectAgents}, store.ListOptions{})
		require.NoError(t, err)
		assert.Equal(t, 1, result.TotalCount)
		assert.Equal(t, store.GroupTypeProjectAgents, result.Items[0].GroupType)
	})

	t.Run("ListGroupsWithLimit", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		for i := 0; i < 5; i++ {
			g := &store.Group{
				ID:   uuid.New().String(),
				Name: "Group " + string(rune('A'+i)),
				Slug: "group-" + string(rune('a'+i)),
			}
			require.NoError(t, gs.CreateGroup(ctx, g))
		}

		result, err := gs.ListGroups(ctx, store.GroupFilter{}, store.ListOptions{Limit: 2})
		require.NoError(t, err)
		assert.Equal(t, 5, result.TotalCount)
		assert.Len(t, result.Items, 2)
	})

	t.Run("AddGroupMemberUser", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Test Group",
			Slug: "test-group",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		member := &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   testUserUID.String(),
			Role:       store.GroupMemberRoleMember,
		}

		err := gs.AddGroupMember(ctx, member)
		require.NoError(t, err)
		assert.False(t, member.AddedAt.IsZero())
	})

	t.Run("AddGroupMemberAgent", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Test Group",
			Slug: "test-group-agent",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		member := &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeAgent,
			MemberID:   testAgentUID.String(),
			Role:       store.GroupMemberRoleMember,
		}

		err := gs.AddGroupMember(ctx, member)
		require.NoError(t, err)
		assert.False(t, member.AddedAt.IsZero())

		members, err := gs.GetGroupMembers(ctx, g.ID)
		require.NoError(t, err)
		require.Len(t, members, 1)
		assert.Equal(t, store.GroupMemberTypeAgent, members[0].MemberType)
		assert.Equal(t, testAgentUID.String(), members[0].MemberID)
	})

	t.Run("AddGroupMemberDuplicate", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Test Group",
			Slug: "test-group-dup",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		member := &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   testUserUID.String(),
			Role:       store.GroupMemberRoleMember,
		}

		require.NoError(t, gs.AddGroupMember(ctx, member))

		err := gs.AddGroupMember(ctx, member)
		assert.ErrorIs(t, err, store.ErrAlreadyExists)
	})

	t.Run("AddGroupMemberGroupNesting", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		parent := &store.Group{
			ID:   uuid.New().String(),
			Name: "Parent",
			Slug: "parent",
		}
		child := &store.Group{
			ID:   uuid.New().String(),
			Name: "Child",
			Slug: "child",
		}
		require.NoError(t, gs.CreateGroup(ctx, parent))
		require.NoError(t, gs.CreateGroup(ctx, child))

		member := &store.GroupMember{
			GroupID:    parent.ID,
			MemberType: store.GroupMemberTypeGroup,
			MemberID:   child.ID,
			Role:       store.GroupMemberRoleMember,
		}

		err := gs.AddGroupMember(ctx, member)
		require.NoError(t, err)

		members, err := gs.GetGroupMembers(ctx, parent.ID)
		require.NoError(t, err)
		require.Len(t, members, 1)
		assert.Equal(t, store.GroupMemberTypeGroup, members[0].MemberType)
		assert.Equal(t, child.ID, members[0].MemberID)
	})

	t.Run("RemoveGroupMemberUser", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Test Group",
			Slug: "test-group-rm",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		member := &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   testUserUID.String(),
			Role:       store.GroupMemberRoleMember,
		}
		require.NoError(t, gs.AddGroupMember(ctx, member))

		err := gs.RemoveGroupMember(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
		require.NoError(t, err)

		_, err = gs.GetGroupMembership(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("RemoveGroupMemberAgent", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Test Group",
			Slug: "test-group-rm-agent",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		member := &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeAgent,
			MemberID:   testAgentUID.String(),
			Role:       store.GroupMemberRoleMember,
		}
		require.NoError(t, gs.AddGroupMember(ctx, member))

		err := gs.RemoveGroupMember(ctx, g.ID, store.GroupMemberTypeAgent, testAgentUID.String())
		require.NoError(t, err)

		_, err = gs.GetGroupMembership(ctx, g.ID, store.GroupMemberTypeAgent, testAgentUID.String())
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("RemoveGroupMemberNotFound", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Test Group",
			Slug: "test-group-rm-nf",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		err := gs.RemoveGroupMember(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("GetGroupMembers", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Mixed Group",
			Slug: "mixed-group",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		child := &store.Group{
			ID:   uuid.New().String(),
			Name: "Child Group",
			Slug: "child-group",
		}
		require.NoError(t, gs.CreateGroup(ctx, child))

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   testUserUID.String(),
			Role:       store.GroupMemberRoleMember,
		}))

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeAgent,
			MemberID:   testAgentUID.String(),
			Role:       store.GroupMemberRoleMember,
		}))

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeGroup,
			MemberID:   child.ID,
			Role:       store.GroupMemberRoleMember,
		}))

		members, err := gs.GetGroupMembers(ctx, g.ID)
		require.NoError(t, err)
		assert.Len(t, members, 3)

		typeCounts := map[string]int{}
		for _, m := range members {
			typeCounts[m.MemberType]++
		}
		assert.Equal(t, 1, typeCounts[store.GroupMemberTypeUser])
		assert.Equal(t, 1, typeCounts[store.GroupMemberTypeAgent])
		assert.Equal(t, 1, typeCounts[store.GroupMemberTypeGroup])
	})

	t.Run("GetUserGroups", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g1 := &store.Group{
			ID:   uuid.New().String(),
			Name: "Group 1",
			Slug: "group-1",
		}
		g2 := &store.Group{
			ID:   uuid.New().String(),
			Name: "Group 2",
			Slug: "group-2",
		}
		require.NoError(t, gs.CreateGroup(ctx, g1))
		require.NoError(t, gs.CreateGroup(ctx, g2))

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    g1.ID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   testUserUID.String(),
			Role:       store.GroupMemberRoleMember,
		}))
		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    g2.ID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   testUserUID.String(),
			Role:       store.GroupMemberRoleAdmin,
		}))

		groups, err := gs.GetUserGroups(ctx, testUserUID.String())
		require.NoError(t, err)
		assert.Len(t, groups, 2)
	})

	t.Run("GetGroupMembershipUser", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Test",
			Slug: "test-membership",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   testUserUID.String(),
			Role:       store.GroupMemberRoleAdmin,
		}))

		m, err := gs.GetGroupMembership(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
		require.NoError(t, err)
		assert.Equal(t, store.GroupMemberRoleAdmin, m.Role)
		assert.Equal(t, store.GroupMemberTypeUser, m.MemberType)
		assert.Equal(t, testUserUID.String(), m.MemberID)
	})

	t.Run("GetGroupMembershipGroup", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		parent := &store.Group{
			ID:   uuid.New().String(),
			Name: "Parent",
			Slug: "parent-gm",
		}
		child := &store.Group{
			ID:   uuid.New().String(),
			Name: "Child",
			Slug: "child-gm",
		}
		require.NoError(t, gs.CreateGroup(ctx, parent))
		require.NoError(t, gs.CreateGroup(ctx, child))

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    parent.ID,
			MemberType: store.GroupMemberTypeGroup,
			MemberID:   child.ID,
			Role:       store.GroupMemberRoleMember,
		}))

		m, err := gs.GetGroupMembership(ctx, parent.ID, store.GroupMemberTypeGroup, child.ID)
		require.NoError(t, err)
		assert.Equal(t, store.GroupMemberTypeGroup, m.MemberType)
		assert.Equal(t, child.ID, m.MemberID)
	})

	t.Run("GetGroupMembershipNotFound", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Test",
			Slug: "test-gm-nf",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		_, err := gs.GetGroupMembership(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("WouldCreateCycleSelf", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Self",
			Slug: "self-cycle",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		wouldCycle, err := gs.WouldCreateCycle(ctx, g.ID, g.ID)
		require.NoError(t, err)
		assert.True(t, wouldCycle)
	})

	t.Run("WouldCreateCycleDirect", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		a := &store.Group{
			ID:   uuid.New().String(),
			Name: "A",
			Slug: "cycle-a",
		}
		b := &store.Group{
			ID:   uuid.New().String(),
			Name: "B",
			Slug: "cycle-b",
		}
		require.NoError(t, gs.CreateGroup(ctx, a))
		require.NoError(t, gs.CreateGroup(ctx, b))

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    a.ID,
			MemberType: store.GroupMemberTypeGroup,
			MemberID:   b.ID,
			Role:       store.GroupMemberRoleMember,
		}))

		wouldCycle, err := gs.WouldCreateCycle(ctx, b.ID, a.ID)
		require.NoError(t, err)
		assert.True(t, wouldCycle)
	})

	t.Run("WouldCreateCycleTransitive", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		a := &store.Group{ID: uuid.New().String(), Name: "A", Slug: "trans-a"}
		b := &store.Group{ID: uuid.New().String(), Name: "B", Slug: "trans-b"}
		c := &store.Group{ID: uuid.New().String(), Name: "C", Slug: "trans-c"}
		require.NoError(t, gs.CreateGroup(ctx, a))
		require.NoError(t, gs.CreateGroup(ctx, b))
		require.NoError(t, gs.CreateGroup(ctx, c))

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID: a.ID, MemberType: store.GroupMemberTypeGroup, MemberID: b.ID, Role: store.GroupMemberRoleMember,
		}))
		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID: b.ID, MemberType: store.GroupMemberTypeGroup, MemberID: c.ID, Role: store.GroupMemberRoleMember,
		}))

		wouldCycle, err := gs.WouldCreateCycle(ctx, c.ID, a.ID)
		require.NoError(t, err)
		assert.True(t, wouldCycle)

		wouldCycle, err = gs.WouldCreateCycle(ctx, a.ID, c.ID)
		require.NoError(t, err)
		assert.False(t, wouldCycle)
	})

	t.Run("WouldCreateCycleNoCycle", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		a := &store.Group{ID: uuid.New().String(), Name: "A", Slug: "nc-a"}
		b := &store.Group{ID: uuid.New().String(), Name: "B", Slug: "nc-b"}
		require.NoError(t, gs.CreateGroup(ctx, a))
		require.NoError(t, gs.CreateGroup(ctx, b))

		wouldCycle, err := gs.WouldCreateCycle(ctx, a.ID, b.ID)
		require.NoError(t, err)
		assert.False(t, wouldCycle)
	})

	t.Run("ProjectGroupGuardAddMember", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:        uuid.New().String(),
			Name:      "Project Group",
			Slug:      "project-guard",
			GroupType: store.GroupTypeProjectAgents,
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		member := &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   testUserUID.String(),
			Role:       store.GroupMemberRoleMember,
		}
		err := gs.AddGroupMember(ctx, member)
		assert.ErrorIs(t, err, store.ErrInvalidInput)
	})

	t.Run("ProjectGroupGuardRemoveMember", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:        uuid.New().String(),
			Name:      "Project Group",
			Slug:      "project-guard-rm",
			GroupType: store.GroupTypeProjectAgents,
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		err := gs.RemoveGroupMember(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
		assert.ErrorIs(t, err, store.ErrInvalidInput)
	})

	t.Run("GetEffectiveGroups", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		a := &store.Group{ID: uuid.New().String(), Name: "A", Slug: "eff-a"}
		b := &store.Group{ID: uuid.New().String(), Name: "B", Slug: "eff-b"}
		c := &store.Group{ID: uuid.New().String(), Name: "C", Slug: "eff-c"}
		require.NoError(t, gs.CreateGroup(ctx, a))
		require.NoError(t, gs.CreateGroup(ctx, b))
		require.NoError(t, gs.CreateGroup(ctx, c))

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID: a.ID, MemberType: store.GroupMemberTypeGroup, MemberID: b.ID, Role: store.GroupMemberRoleMember,
		}))
		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID: b.ID, MemberType: store.GroupMemberTypeGroup, MemberID: c.ID, Role: store.GroupMemberRoleMember,
		}))
		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID: c.ID, MemberType: store.GroupMemberTypeUser, MemberID: testUserUID.String(), Role: store.GroupMemberRoleMember,
		}))

		effective, err := gs.GetEffectiveGroups(ctx, testUserUID.String())
		require.NoError(t, err)

		assert.Len(t, effective, 3)

		found := make(map[string]bool)
		for _, gid := range effective {
			found[gid] = true
		}
		assert.True(t, found[a.ID], "expected group A")
		assert.True(t, found[b.ID], "expected group B")
		assert.True(t, found[c.ID], "expected group C")
	})

	t.Run("GetEffectiveGroupsNoMemberships", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		effective, err := gs.GetEffectiveGroups(ctx, testUserUID.String())
		require.NoError(t, err)
		assert.Empty(t, effective)
	})

	t.Run("DeleteGroupCascadesMemberships", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Group With Members",
			Slug: "group-cascade",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   testUserUID.String(),
			Role:       store.GroupMemberRoleMember,
		}))

		err := gs.DeleteGroup(ctx, g.ID)
		require.NoError(t, err)

		_, err = gs.GetGroup(ctx, g.ID)
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("CreateProjectGroup", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:        uuid.New().String(),
			Name:      "Test Project Agents",
			Slug:      "project:test-project:agents",
			GroupType: store.GroupTypeProjectAgents,
			ProjectID: testProjectUID.String(),
		}

		err := gs.CreateGroup(ctx, g)
		require.NoError(t, err)
		assert.False(t, g.Created.IsZero())
		assert.Equal(t, store.GroupTypeProjectAgents, g.GroupType)

		got, err := gs.GetGroup(ctx, g.ID)
		require.NoError(t, err)
		assert.Equal(t, store.GroupTypeProjectAgents, got.GroupType)
		assert.Equal(t, testProjectUID.String(), got.ProjectID)
	})

	t.Run("GetGroupByProjectID", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:        uuid.New().String(),
			Name:      "Test Project Agents",
			Slug:      "project:test-project:agents",
			GroupType: store.GroupTypeProjectAgents,
			ProjectID: testProjectUID.String(),
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		got, err := gs.GetGroupByProjectID(ctx, testProjectUID.String())
		require.NoError(t, err)
		assert.Equal(t, g.ID, got.ID)
		assert.Equal(t, store.GroupTypeProjectAgents, got.GroupType)
		assert.Equal(t, testProjectUID.String(), got.ProjectID)
	})

	t.Run("GetGroupByProjectIDNotFound", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		_, err := gs.GetGroupByProjectID(ctx, uuid.New().String())
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("GetGroupMembersProjectGroup", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		projectGroup := &store.Group{
			ID:        uuid.New().String(),
			Name:      "Test Project Agents",
			Slug:      "project:test-project:agents-members",
			GroupType: store.GroupTypeProjectAgents,
			ProjectID: testProjectUID.String(),
		}
		require.NoError(t, gs.CreateGroup(ctx, projectGroup))

		agent2UID := uuid.MustParse("20000000-0000-0000-0000-000000000002")
		project, err := gs.client.Project.Get(ctx, testProjectUID)
		require.NoError(t, err)
		_, err = gs.client.Agent.Create().
			SetID(agent2UID).
			SetName("test-agent-2").
			SetSlug("test-agent-2").
			SetProject(project).
			Save(ctx)
		require.NoError(t, err)

		members, err := gs.GetGroupMembers(ctx, projectGroup.ID)
		require.NoError(t, err)
		assert.Len(t, members, 2)

		memberIDs := make(map[string]bool)
		for _, m := range members {
			assert.Equal(t, store.GroupMemberTypeAgent, m.MemberType)
			assert.Equal(t, store.GroupMemberRoleMember, m.Role)
			assert.Equal(t, "system", m.AddedBy)
			memberIDs[m.MemberID] = true
		}
		assert.True(t, memberIDs[testAgentUID.String()])
		assert.True(t, memberIDs[agent2UID.String()])
	})

	t.Run("GetEffectiveGroupsForAgent", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		projectGroup := &store.Group{
			ID:        uuid.New().String(),
			Name:      "Test Project Agents",
			Slug:      "project:test-project:agents-eff",
			GroupType: store.GroupTypeProjectAgents,
			ProjectID: testProjectUID.String(),
		}
		require.NoError(t, gs.CreateGroup(ctx, projectGroup))

		parentGroup := &store.Group{
			ID:   uuid.New().String(),
			Name: "All Agents Parent",
			Slug: "all-agents-parent",
		}
		require.NoError(t, gs.CreateGroup(ctx, parentGroup))

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    parentGroup.ID,
			MemberType: store.GroupMemberTypeGroup,
			MemberID:   projectGroup.ID,
			Role:       store.GroupMemberRoleMember,
		}))

		explicitGroup := &store.Group{
			ID:   uuid.New().String(),
			Name: "Explicit Group",
			Slug: "explicit-group-eff",
		}
		require.NoError(t, gs.CreateGroup(ctx, explicitGroup))
		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    explicitGroup.ID,
			MemberType: store.GroupMemberTypeAgent,
			MemberID:   testAgentUID.String(),
			Role:       store.GroupMemberRoleMember,
		}))

		effective, err := gs.GetEffectiveGroupsForAgent(ctx, testAgentUID.String())
		require.NoError(t, err)

		found := make(map[string]bool)
		for _, gid := range effective {
			found[gid] = true
		}
		assert.True(t, found[projectGroup.ID], "expected project group")
		assert.True(t, found[parentGroup.ID], "expected parent group (transitive)")
		assert.True(t, found[explicitGroup.ID], "expected explicit group")
		assert.Len(t, effective, 3)
	})

	t.Run("GetEffectiveGroupsForAgentNoGroups", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		effective, err := gs.GetEffectiveGroupsForAgent(ctx, testAgentUID.String())
		require.NoError(t, err)
		assert.Empty(t, effective)
	})

	t.Run("ProjectGroupLifecycle", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		projectGroup := &store.Group{
			ID:        uuid.New().String(),
			Name:      "Test Project Agents",
			Slug:      "project:test-project:agents-lc",
			GroupType: store.GroupTypeProjectAgents,
			ProjectID: testProjectUID.String(),
		}
		require.NoError(t, gs.CreateGroup(ctx, projectGroup))

		got, err := gs.GetGroupByProjectID(ctx, testProjectUID.String())
		require.NoError(t, err)
		assert.Equal(t, projectGroup.ID, got.ID)

		members, err := gs.GetGroupMembers(ctx, projectGroup.ID)
		require.NoError(t, err)
		assert.Len(t, members, 1)
		assert.Equal(t, testAgentUID.String(), members[0].MemberID)

		require.NoError(t, gs.DeleteGroup(ctx, projectGroup.ID))

		_, err = gs.GetGroupByProjectID(ctx, testProjectUID.String())
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("CheckDelegatedAccess_Enabled", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		_, err := gs.client.Agent.UpdateOneID(testAgentUID).
			SetDelegationEnabled(true).
			SetCreatorID(testUserUID).
			Save(ctx)
		require.NoError(t, err)

		conditions := &store.PolicyConditions{
			DelegatedFrom: &store.DelegatedFromCondition{
				PrincipalType: "user",
				PrincipalID:   testUserUID.String(),
			},
		}
		result, err := gs.CheckDelegatedAccess(ctx, testAgentUID.String(), conditions)
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("CheckDelegatedAccess_Disabled", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		_, err := gs.client.Agent.UpdateOneID(testAgentUID).
			SetCreatorID(testUserUID).
			Save(ctx)
		require.NoError(t, err)

		conditions := &store.PolicyConditions{
			DelegatedFrom: &store.DelegatedFromCondition{
				PrincipalType: "user",
				PrincipalID:   testUserUID.String(),
			},
		}
		result, err := gs.CheckDelegatedAccess(ctx, testAgentUID.String(), conditions)
		require.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("CheckDelegatedAccess_SuspendedCreator", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		_, err := gs.client.User.UpdateOneID(testUserUID).
			SetStatus("suspended").
			Save(ctx)
		require.NoError(t, err)

		_, err = gs.client.Agent.UpdateOneID(testAgentUID).
			SetDelegationEnabled(true).
			SetCreatorID(testUserUID).
			Save(ctx)
		require.NoError(t, err)

		conditions := &store.PolicyConditions{
			DelegatedFrom: &store.DelegatedFromCondition{
				PrincipalType: "user",
				PrincipalID:   testUserUID.String(),
			},
		}
		result, err := gs.CheckDelegatedAccess(ctx, testAgentUID.String(), conditions)
		require.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("CheckDelegatedAccess_NoCreator", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		_, err := gs.client.Agent.UpdateOneID(testAgentUID).
			SetDelegationEnabled(true).
			Save(ctx)
		require.NoError(t, err)

		conditions := &store.PolicyConditions{
			DelegatedFrom: &store.DelegatedFromCondition{
				PrincipalType: "user",
				PrincipalID:   testUserUID.String(),
			},
		}
		result, err := gs.CheckDelegatedAccess(ctx, testAgentUID.String(), conditions)
		require.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("CheckDelegatedAccess_GroupCondition", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Platform Team",
			Slug: "platform-team-deleg",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))
		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   testUserUID.String(),
			Role:       store.GroupMemberRoleMember,
		}))

		_, err := gs.client.Agent.UpdateOneID(testAgentUID).
			SetDelegationEnabled(true).
			SetCreatorID(testUserUID).
			Save(ctx)
		require.NoError(t, err)

		conditions := &store.PolicyConditions{
			DelegatedFromGroup: g.ID,
		}
		result, err := gs.CheckDelegatedAccess(ctx, testAgentUID.String(), conditions)
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("CheckDelegatedAccess_GroupCondition_NotMember", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Other Team",
			Slug: "other-team-deleg",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		_, err := gs.client.Agent.UpdateOneID(testAgentUID).
			SetDelegationEnabled(true).
			SetCreatorID(testUserUID).
			Save(ctx)
		require.NoError(t, err)

		conditions := &store.PolicyConditions{
			DelegatedFromGroup: g.ID,
		}
		result, err := gs.CheckDelegatedAccess(ctx, testAgentUID.String(), conditions)
		require.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("CheckDelegatedAccess_NilConditions", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		result, err := gs.CheckDelegatedAccess(ctx, testAgentUID.String(), nil)
		require.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("CheckDelegatedAccess_NoDelegationConditions", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		conditions := &store.PolicyConditions{
			Labels: map[string]string{"env": "prod"},
		}
		result, err := gs.CheckDelegatedAccess(ctx, testAgentUID.String(), conditions)
		require.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("GetGroupsByIDs", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g1 := &store.Group{ID: uuid.New().String(), Name: "Group 1", Slug: "gbi-1"}
		g2 := &store.Group{ID: uuid.New().String(), Name: "Group 2", Slug: "gbi-2"}
		require.NoError(t, gs.CreateGroup(ctx, g1))
		require.NoError(t, gs.CreateGroup(ctx, g2))

		groups, err := gs.GetGroupsByIDs(ctx, []string{g1.ID, g2.ID})
		require.NoError(t, err)
		assert.Len(t, groups, 2)

		names := map[string]bool{}
		for _, g := range groups {
			names[g.Name] = true
		}
		assert.True(t, names["Group 1"])
		assert.True(t, names["Group 2"])
	})

	t.Run("GetGroupsByIDs_Empty", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		groups, err := gs.GetGroupsByIDs(ctx, []string{})
		require.NoError(t, err)
		assert.Nil(t, groups)
	})

	t.Run("GetGroupsByIDs_MissingIDs", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g1 := &store.Group{ID: uuid.New().String(), Name: "Exists", Slug: "gbi-exists"}
		require.NoError(t, gs.CreateGroup(ctx, g1))

		groups, err := gs.GetGroupsByIDs(ctx, []string{g1.ID, uuid.New().String()})
		require.NoError(t, err)
		assert.Len(t, groups, 1)
		assert.Equal(t, "Exists", groups[0].Name)
	})

	t.Run("UpdateGroupMemberRole", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Role Update",
			Slug: "role-update",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   testUserUID.String(),
			Role:       store.GroupMemberRoleMember,
		}))

		m, err := gs.GetGroupMembership(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
		require.NoError(t, err)
		assert.Equal(t, store.GroupMemberRoleMember, m.Role)

		err = gs.UpdateGroupMemberRole(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String(), store.GroupMemberRoleOwner)
		require.NoError(t, err)

		m, err = gs.GetGroupMembership(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String())
		require.NoError(t, err)
		assert.Equal(t, store.GroupMemberRoleOwner, m.Role)
	})

	t.Run("UpdateGroupMemberRoleNotFound", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Role NF",
			Slug: "role-nf",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		err := gs.UpdateGroupMemberRole(ctx, g.ID, store.GroupMemberTypeUser, testUserUID.String(), store.GroupMemberRoleOwner)
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("UpdateGroupMemberRoleAgent", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Agent Role",
			Slug: "agent-role-update",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeAgent,
			MemberID:   testAgentUID.String(),
			Role:       store.GroupMemberRoleMember,
		}))

		err := gs.UpdateGroupMemberRole(ctx, g.ID, store.GroupMemberTypeAgent, testAgentUID.String(), store.GroupMemberRoleAdmin)
		require.NoError(t, err)

		m, err := gs.GetGroupMembership(ctx, g.ID, store.GroupMemberTypeAgent, testAgentUID.String())
		require.NoError(t, err)
		assert.Equal(t, store.GroupMemberRoleAdmin, m.Role)
	})

	t.Run("CountGroupMembersByRole", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		testUser2UID := uuid.MustParse("10000000-0000-0000-0000-000000000002")
		_, err := gs.client.User.Create().
			SetID(testUser2UID).
			SetEmail("test2@example.com").
			SetDisplayName("Test User 2").
			Save(ctx)
		require.NoError(t, err)

		g := &store.Group{
			ID:   uuid.New().String(),
			Name: "Count Roles",
			Slug: "count-roles",
		}
		require.NoError(t, gs.CreateGroup(ctx, g))

		count, err := gs.CountGroupMembersByRole(ctx, g.ID, store.GroupMemberRoleOwner)
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   testUserUID.String(),
			Role:       store.GroupMemberRoleOwner,
		}))

		count, err = gs.CountGroupMembersByRole(ctx, g.ID, store.GroupMemberRoleOwner)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		require.NoError(t, gs.AddGroupMember(ctx, &store.GroupMember{
			GroupID:    g.ID,
			MemberType: store.GroupMemberTypeUser,
			MemberID:   testUser2UID.String(),
			Role:       store.GroupMemberRoleMember,
		}))

		count, err = gs.CountGroupMembersByRole(ctx, g.ID, store.GroupMemberRoleOwner)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		count, err = gs.CountGroupMembersByRole(ctx, g.ID, store.GroupMemberRoleMember)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("ListGroupsWithProjectIDFilter", func(t *testing.T) {
		gs := newStore(t)
		ctx := context.Background()

		projectID1 := testProjectUID.String()
		projectID2 := uuid.New().String()

		_, err := gs.client.Project.Create().
			SetID(uuid.MustParse(projectID2)).
			SetName("project-2").
			SetSlug("project-2").
			Save(ctx)
		require.NoError(t, err)

		g1 := &store.Group{
			ID:        uuid.New().String(),
			Name:      "Project 1 Agents",
			Slug:      "project:project-1:agents",
			GroupType: store.GroupTypeProjectAgents,
			ProjectID: projectID1,
		}
		g2 := &store.Group{
			ID:        uuid.New().String(),
			Name:      "Project 2 Agents",
			Slug:      "project:project-2:agents",
			GroupType: store.GroupTypeProjectAgents,
			ProjectID: projectID2,
		}
		require.NoError(t, gs.CreateGroup(ctx, g1))
		require.NoError(t, gs.CreateGroup(ctx, g2))

		result, err := gs.ListGroups(ctx, store.GroupFilter{ProjectID: projectID1}, store.ListOptions{})
		require.NoError(t, err)
		assert.Equal(t, 1, result.TotalCount)
		assert.Equal(t, g1.ID, result.Items[0].ID)
	})
}
