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

package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/scion/pkg/store"
	"github.com/GoogleCloudPlatform/scion/pkg/store/postgres"
	"github.com/GoogleCloudPlatform/scion/pkg/store/storetest"
	"github.com/stretchr/testify/require"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestConformance(t *testing.T) {
	dsn := os.Getenv("SCION_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set SCION_TEST_POSTGRES_DSN to run postgres conformance tests")
	}

	storetest.RunAll(t, func(t *testing.T) store.Store {
		s, err := postgres.New(dsn)
		require.NoError(t, err)
		require.NoError(t, s.Migrate(context.Background()))
		t.Cleanup(func() { s.Close() })
		return s
	})
}
