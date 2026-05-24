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
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

// truncateEntTables removes all rows from every Ent-managed table in the "ent"
// schema so each Postgres test starts with a clean slate. CASCADE handles FK ordering.
func truncateEntTables(t *testing.T, dsn string) {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.ExecContext(context.Background(), `
		TRUNCATE TABLE
			ent.group_memberships,
			ent.policy_bindings,
			ent.group_child_groups,
			ent.groups,
			ent.access_policies,
			ent.agents,
			ent.users,
			ent.projects
		CASCADE
	`)
	require.NoError(t, err)
}
