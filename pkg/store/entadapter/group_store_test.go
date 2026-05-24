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

//go:build !no_sqlite

package entadapter

import (
	"context"
	"testing"

	"github.com/GoogleCloudPlatform/scion/pkg/ent/entc"
	"github.com/GoogleCloudPlatform/scion/pkg/store"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestGroupStore(t *testing.T) *GroupStore {
	t.Helper()
	client, err := entc.OpenSQLite("file:" + t.Name() + "?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	require.NoError(t, entc.AutoMigrate(context.Background(), client))

	_, err = client.User.Create().
		SetID(testUserUID).
		SetEmail("test@example.com").
		SetDisplayName("Test User").
		Save(context.Background())
	require.NoError(t, err)

	project, err := client.Project.Create().
		SetID(testProjectUID).
		SetName("test-project").
		SetSlug("test-project").
		Save(context.Background())
	require.NoError(t, err)

	_, err = client.Agent.Create().
		SetID(testAgentUID).
		SetName("test-agent").
		SetSlug("test-agent").
		SetProject(project).
		Save(context.Background())
	require.NoError(t, err)

	return NewGroupStore(client)
}

func TestGroupStore(t *testing.T) {
	runGroupStoreSuite(t, newTestGroupStore)
}

func TestCompositeStoreDelegation(t *testing.T) {
	client, err := entc.OpenSQLite("file:" + t.Name() + "?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	require.NoError(t, entc.AutoMigrate(context.Background(), client))

	composite := NewCompositeStore(nil, client)

	ctx := context.Background()
	g := &store.Group{
		ID:   uuid.New().String(),
		Name: "Composite Test",
		Slug: "composite-test",
	}

	err = composite.CreateGroup(ctx, g)
	require.NoError(t, err)

	got, err := composite.GetGroup(ctx, g.ID)
	require.NoError(t, err)
	assert.Equal(t, "Composite Test", got.Name)

	got, err = composite.GetGroupBySlug(ctx, "composite-test")
	require.NoError(t, err)
	assert.Equal(t, g.ID, got.ID)
}
