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
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/scion/pkg/ent/entc"
	"github.com/GoogleCloudPlatform/scion/pkg/store/sqlite"
	"github.com/stretchr/testify/require"
)

// newTestCompositeStorePostgres creates a CompositeStore backed by an in-memory
// SQLite base store and a real Postgres Ent client. Requires SCION_TEST_POSTGRES_DSN.
func newTestCompositeStorePostgres(t *testing.T) *CompositeStore {
	t.Helper()
	dsn := os.Getenv("SCION_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SCION_TEST_POSTGRES_DSN not set")
	}

	base, err := sqlite.New(":memory:")
	require.NoError(t, err)
	require.NoError(t, base.Migrate(context.Background()))

	entClient, err := entc.OpenPostgresInSchema(context.Background(), dsn, "ent")
	require.NoError(t, err)
	require.NoError(t, entc.AutoMigrate(context.Background(), entClient))

	truncateEntTables(t, dsn)

	cs := NewCompositeStore(base, entClient)
	t.Cleanup(func() { cs.Close() })
	return cs
}

func TestCompositeStore_Postgres(t *testing.T) {
	runCompositeStoreSuite(t, newTestCompositeStorePostgres)
}
