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
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/scion/pkg/ent/entc"
	"github.com/stretchr/testify/require"
)

func newTestGroupStorePostgres(t *testing.T) *GroupStore {
	t.Helper()
	dsn := os.Getenv("SCION_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SCION_TEST_POSTGRES_DSN not set")
	}

	client, err := entc.OpenPostgresInSchema(context.Background(), dsn, "ent")
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	require.NoError(t, entc.AutoMigrate(context.Background(), client))

	truncateEntTables(t, dsn)

	ctx := context.Background()

	_, err = client.User.Create().
		SetID(testUserUID).
		SetEmail("test@example.com").
		SetDisplayName("Test User").
		Save(ctx)
	require.NoError(t, err)

	project, err := client.Project.Create().
		SetID(testProjectUID).
		SetName("test-project").
		SetSlug("test-project").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Agent.Create().
		SetID(testAgentUID).
		SetName("test-agent").
		SetSlug("test-agent").
		SetProject(project).
		Save(ctx)
	require.NoError(t, err)

	return NewGroupStore(client)
}

func TestGroupStore_Postgres(t *testing.T) {
	runGroupStoreSuite(t, newTestGroupStorePostgres)
}
