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
	policyTestUserUID    = uuid.MustParse("10000000-0000-0000-0000-000000000010")
	policyTestUser2UID   = uuid.MustParse("10000000-0000-0000-0000-000000000020")
	policyTestAgentUID   = uuid.MustParse("20000000-0000-0000-0000-000000000010")
	policyTestProjectUID = uuid.MustParse("30000000-0000-0000-0000-000000000010")
	policyTestGroupUID   = uuid.MustParse("40000000-0000-0000-0000-000000000010")
)

// runPolicyStoreSuite runs the full PolicyStore test suite using the provided
// store factory. Each subtest calls newStore(t) independently, so the factory
// must return a store with a clean, freshly-seeded database.
func runPolicyStoreSuite(t *testing.T, newStore func(t *testing.T) *PolicyStore) {
	t.Helper()

	t.Run("CreatePolicy", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		p := &store.Policy{
			ID:           uuid.New().String(),
			Name:         "Allow Read",
			Description:  "Allow read access to agents",
			ScopeType:    "hub",
			ResourceType: "agent",
			Actions:      []string{"read"},
			Effect:       "allow",
			Priority:     10,
		}

		err := ps.CreatePolicy(ctx, p)
		require.NoError(t, err)
		assert.False(t, p.Created.IsZero())
		assert.False(t, p.Updated.IsZero())
	})

	t.Run("GetPolicy", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		id := uuid.New().String()
		p := &store.Policy{
			ID:           id,
			Name:         "Test Policy",
			ScopeType:    "project",
			ScopeID:      policyTestProjectUID.String(),
			ResourceType: "agent",
			Actions:      []string{"read", "update"},
			Effect:       "allow",
		}
		require.NoError(t, ps.CreatePolicy(ctx, p))

		got, err := ps.GetPolicy(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, id, got.ID)
		assert.Equal(t, "Test Policy", got.Name)
		assert.Equal(t, "project", got.ScopeType)
		assert.Equal(t, policyTestProjectUID.String(), got.ScopeID)
		assert.Equal(t, []string{"read", "update"}, got.Actions)
	})

	t.Run("GetPolicy_NotFound", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		_, err := ps.GetPolicy(ctx, uuid.New().String())
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("UpdatePolicy", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		p := &store.Policy{
			ID:           uuid.New().String(),
			Name:         "Original",
			ScopeType:    "hub",
			ResourceType: "agent",
			Actions:      []string{"read"},
			Effect:       "allow",
		}
		require.NoError(t, ps.CreatePolicy(ctx, p))

		p.Name = "Updated"
		p.Actions = []string{"read", "update"}
		err := ps.UpdatePolicy(ctx, p)
		require.NoError(t, err)

		got, err := ps.GetPolicy(ctx, p.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated", got.Name)
		assert.Equal(t, []string{"read", "update"}, got.Actions)
	})

	t.Run("DeletePolicy", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		p := &store.Policy{
			ID:           uuid.New().String(),
			Name:         "To Delete",
			ScopeType:    "hub",
			ResourceType: "*",
			Actions:      []string{"*"},
			Effect:       "deny",
		}
		require.NoError(t, ps.CreatePolicy(ctx, p))

		err := ps.DeletePolicy(ctx, p.ID)
		require.NoError(t, err)

		_, err = ps.GetPolicy(ctx, p.ID)
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("ListPolicies", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		for i := 0; i < 3; i++ {
			p := &store.Policy{
				ID:           uuid.New().String(),
				Name:         "Policy " + string(rune('A'+i)),
				ScopeType:    "hub",
				ResourceType: "agent",
				Actions:      []string{"read"},
				Effect:       "allow",
			}
			require.NoError(t, ps.CreatePolicy(ctx, p))
		}

		result, err := ps.ListPolicies(ctx, store.PolicyFilter{}, store.ListOptions{})
		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount)
		assert.Len(t, result.Items, 3)
	})

	t.Run("ListPolicies_Filter", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		require.NoError(t, ps.CreatePolicy(ctx, &store.Policy{
			ID: uuid.New().String(), Name: "Hub Allow", ScopeType: "hub",
			ResourceType: "*", Actions: []string{"*"}, Effect: "allow",
		}))
		require.NoError(t, ps.CreatePolicy(ctx, &store.Policy{
			ID: uuid.New().String(), Name: "Hub Deny", ScopeType: "hub",
			ResourceType: "*", Actions: []string{"*"}, Effect: "deny",
		}))
		require.NoError(t, ps.CreatePolicy(ctx, &store.Policy{
			ID: uuid.New().String(), Name: "Project Allow", ScopeType: "project",
			ScopeID: policyTestProjectUID.String(), ResourceType: "agent",
			Actions: []string{"read"}, Effect: "allow",
		}))

		result, err := ps.ListPolicies(ctx, store.PolicyFilter{Effect: "deny"}, store.ListOptions{})
		require.NoError(t, err)
		assert.Equal(t, 1, result.TotalCount)

		result, err = ps.ListPolicies(ctx, store.PolicyFilter{ScopeType: "project"}, store.ListOptions{})
		require.NoError(t, err)
		assert.Equal(t, 1, result.TotalCount)
	})

	t.Run("AddPolicyBinding_User", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		p := &store.Policy{
			ID: uuid.New().String(), Name: "Test", ScopeType: "hub",
			ResourceType: "*", Actions: []string{"*"}, Effect: "allow",
		}
		require.NoError(t, ps.CreatePolicy(ctx, p))

		err := ps.AddPolicyBinding(ctx, &store.PolicyBinding{
			PolicyID:      p.ID,
			PrincipalType: "user",
			PrincipalID:   policyTestUserUID.String(),
		})
		require.NoError(t, err)

		bindings, err := ps.GetPolicyBindings(ctx, p.ID)
		require.NoError(t, err)
		assert.Len(t, bindings, 1)
		assert.Equal(t, "user", bindings[0].PrincipalType)
		assert.Equal(t, policyTestUserUID.String(), bindings[0].PrincipalID)
	})

	t.Run("AddPolicyBinding_Group", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		p := &store.Policy{
			ID: uuid.New().String(), Name: "Test", ScopeType: "hub",
			ResourceType: "*", Actions: []string{"*"}, Effect: "allow",
		}
		require.NoError(t, ps.CreatePolicy(ctx, p))

		err := ps.AddPolicyBinding(ctx, &store.PolicyBinding{
			PolicyID:      p.ID,
			PrincipalType: "group",
			PrincipalID:   policyTestGroupUID.String(),
		})
		require.NoError(t, err)

		bindings, err := ps.GetPolicyBindings(ctx, p.ID)
		require.NoError(t, err)
		assert.Len(t, bindings, 1)
		assert.Equal(t, "group", bindings[0].PrincipalType)
	})

	t.Run("AddPolicyBinding_Agent", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		p := &store.Policy{
			ID: uuid.New().String(), Name: "Test", ScopeType: "hub",
			ResourceType: "*", Actions: []string{"*"}, Effect: "allow",
		}
		require.NoError(t, ps.CreatePolicy(ctx, p))

		err := ps.AddPolicyBinding(ctx, &store.PolicyBinding{
			PolicyID:      p.ID,
			PrincipalType: "agent",
			PrincipalID:   policyTestAgentUID.String(),
		})
		require.NoError(t, err)

		bindings, err := ps.GetPolicyBindings(ctx, p.ID)
		require.NoError(t, err)
		assert.Len(t, bindings, 1)
		assert.Equal(t, "agent", bindings[0].PrincipalType)
		assert.Equal(t, policyTestAgentUID.String(), bindings[0].PrincipalID)
	})

	t.Run("RemovePolicyBinding", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		p := &store.Policy{
			ID: uuid.New().String(), Name: "Test", ScopeType: "hub",
			ResourceType: "*", Actions: []string{"*"}, Effect: "allow",
		}
		require.NoError(t, ps.CreatePolicy(ctx, p))

		require.NoError(t, ps.AddPolicyBinding(ctx, &store.PolicyBinding{
			PolicyID:      p.ID,
			PrincipalType: "user",
			PrincipalID:   policyTestUserUID.String(),
		}))

		err := ps.RemovePolicyBinding(ctx, p.ID, "user", policyTestUserUID.String())
		require.NoError(t, err)

		bindings, err := ps.GetPolicyBindings(ctx, p.ID)
		require.NoError(t, err)
		assert.Len(t, bindings, 0)
	})

	t.Run("RemovePolicyBinding_NotFound", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		p := &store.Policy{
			ID: uuid.New().String(), Name: "Test", ScopeType: "hub",
			ResourceType: "*", Actions: []string{"*"}, Effect: "allow",
		}
		require.NoError(t, ps.CreatePolicy(ctx, p))

		err := ps.RemovePolicyBinding(ctx, p.ID, "user", policyTestUserUID.String())
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("GetPoliciesForPrincipal", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		p1 := &store.Policy{
			ID: uuid.New().String(), Name: "User Policy", ScopeType: "hub",
			ResourceType: "agent", Actions: []string{"read"}, Effect: "allow",
		}
		p2 := &store.Policy{
			ID: uuid.New().String(), Name: "Other Policy", ScopeType: "hub",
			ResourceType: "project", Actions: []string{"list"}, Effect: "allow",
		}
		require.NoError(t, ps.CreatePolicy(ctx, p1))
		require.NoError(t, ps.CreatePolicy(ctx, p2))

		require.NoError(t, ps.AddPolicyBinding(ctx, &store.PolicyBinding{
			PolicyID: p1.ID, PrincipalType: "user", PrincipalID: policyTestUserUID.String(),
		}))
		require.NoError(t, ps.AddPolicyBinding(ctx, &store.PolicyBinding{
			PolicyID: p2.ID, PrincipalType: "user", PrincipalID: policyTestUser2UID.String(),
		}))

		policies, err := ps.GetPoliciesForPrincipal(ctx, "user", policyTestUserUID.String())
		require.NoError(t, err)
		assert.Len(t, policies, 1)
		assert.Equal(t, "User Policy", policies[0].Name)
	})

	t.Run("GetPoliciesForPrincipals_BulkQuery", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		p1 := &store.Policy{
			ID: uuid.New().String(), Name: "User Direct", ScopeType: "hub",
			ResourceType: "*", Actions: []string{"read"}, Effect: "allow", Priority: 0,
		}
		p2 := &store.Policy{
			ID: uuid.New().String(), Name: "Group Policy", ScopeType: "project",
			ScopeID: policyTestProjectUID.String(), ResourceType: "agent",
			Actions: []string{"update"}, Effect: "allow", Priority: 10,
		}
		p3 := &store.Policy{
			ID: uuid.New().String(), Name: "Agent Direct", ScopeType: "hub",
			ResourceType: "project", Actions: []string{"list"}, Effect: "deny", Priority: 5,
		}
		require.NoError(t, ps.CreatePolicy(ctx, p1))
		require.NoError(t, ps.CreatePolicy(ctx, p2))
		require.NoError(t, ps.CreatePolicy(ctx, p3))

		require.NoError(t, ps.AddPolicyBinding(ctx, &store.PolicyBinding{
			PolicyID: p1.ID, PrincipalType: "user", PrincipalID: policyTestUserUID.String(),
		}))
		require.NoError(t, ps.AddPolicyBinding(ctx, &store.PolicyBinding{
			PolicyID: p2.ID, PrincipalType: "group", PrincipalID: policyTestGroupUID.String(),
		}))
		require.NoError(t, ps.AddPolicyBinding(ctx, &store.PolicyBinding{
			PolicyID: p3.ID, PrincipalType: "agent", PrincipalID: policyTestAgentUID.String(),
		}))

		principals := []store.PrincipalRef{
			{Type: "user", ID: policyTestUserUID.String()},
			{Type: "group", ID: policyTestGroupUID.String()},
		}
		policies, err := ps.GetPoliciesForPrincipals(ctx, principals)
		require.NoError(t, err)
		assert.Len(t, policies, 2)

		scopeTypes := map[string]bool{}
		for _, p := range policies {
			scopeTypes[p.ScopeType] = true
		}
		assert.True(t, scopeTypes["hub"])
		assert.True(t, scopeTypes["project"])
	})

	t.Run("GetPoliciesForPrincipals_Empty", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		policies, err := ps.GetPoliciesForPrincipals(ctx, nil)
		require.NoError(t, err)
		assert.Nil(t, policies)
	})

	t.Run("PolicyWithConditions", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		p := &store.Policy{
			ID:           uuid.New().String(),
			Name:         "Conditional Policy",
			ScopeType:    "hub",
			ResourceType: "agent",
			Actions:      []string{"read"},
			Effect:       "allow",
			Conditions: &store.PolicyConditions{
				Labels: map[string]string{"env": "production"},
				DelegatedFrom: &store.DelegatedFromCondition{
					PrincipalType: "user",
					PrincipalID:   policyTestUserUID.String(),
				},
				DelegatedFromGroup: policyTestGroupUID.String(),
			},
		}

		require.NoError(t, ps.CreatePolicy(ctx, p))

		got, err := ps.GetPolicy(ctx, p.ID)
		require.NoError(t, err)
		require.NotNil(t, got.Conditions)
		assert.Equal(t, "production", got.Conditions.Labels["env"])
		require.NotNil(t, got.Conditions.DelegatedFrom)
		assert.Equal(t, "user", got.Conditions.DelegatedFrom.PrincipalType)
		assert.Equal(t, policyTestUserUID.String(), got.Conditions.DelegatedFrom.PrincipalID)
		assert.Equal(t, policyTestGroupUID.String(), got.Conditions.DelegatedFromGroup)
	})

	t.Run("DeletePolicy_RemovesBindings", func(t *testing.T) {
		ps := newStore(t)
		ctx := context.Background()

		p := &store.Policy{
			ID: uuid.New().String(), Name: "Test", ScopeType: "hub",
			ResourceType: "*", Actions: []string{"*"}, Effect: "allow",
		}
		require.NoError(t, ps.CreatePolicy(ctx, p))

		require.NoError(t, ps.AddPolicyBinding(ctx, &store.PolicyBinding{
			PolicyID: p.ID, PrincipalType: "user", PrincipalID: policyTestUserUID.String(),
		}))
		require.NoError(t, ps.AddPolicyBinding(ctx, &store.PolicyBinding{
			PolicyID: p.ID, PrincipalType: "group", PrincipalID: policyTestGroupUID.String(),
		}))

		err := ps.DeletePolicy(ctx, p.ID)
		require.NoError(t, err)

		_, err = ps.GetPolicy(ctx, p.ID)
		assert.ErrorIs(t, err, store.ErrNotFound)
	})
}
