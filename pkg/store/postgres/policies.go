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

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/store"
)

func (s *PostgresStore) CreatePolicy(ctx context.Context, policy *store.Policy) error {
	now := time.Now()
	policy.Created = now
	policy.Updated = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO policies (id, name, description, scope_type, scope_id, resource_type, resource_id, actions, effect, conditions, priority, labels, annotations, created_at, updated_at, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`,
		policy.ID, policy.Name, policy.Description, policy.ScopeType, policy.ScopeID,
		policy.ResourceType, policy.ResourceID,
		marshalJSON(policy.Actions), policy.Effect, marshalJSON(policy.Conditions),
		policy.Priority, marshalJSON(policy.Labels), marshalJSON(policy.Annotations),
		policy.Created, policy.Updated, policy.CreatedBy,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return store.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (s *PostgresStore) GetPolicy(ctx context.Context, id string) (*store.Policy, error) {
	policy := &store.Policy{}
	var actions, conditions, labels, annotations string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, scope_type, scope_id, resource_type, resource_id, actions, effect, conditions, priority, labels, annotations, created_at, updated_at, created_by
		FROM policies WHERE id = $1
	`, id).Scan(
		&policy.ID, &policy.Name, &policy.Description, &policy.ScopeType, &policy.ScopeID,
		&policy.ResourceType, &policy.ResourceID,
		&actions, &policy.Effect, &conditions,
		&policy.Priority, &labels, &annotations,
		&policy.Created, &policy.Updated, &policy.CreatedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	unmarshalJSON(actions, &policy.Actions)
	unmarshalJSON(conditions, &policy.Conditions)
	unmarshalJSON(labels, &policy.Labels)
	unmarshalJSON(annotations, &policy.Annotations)
	return policy, nil
}

func (s *PostgresStore) UpdatePolicy(ctx context.Context, policy *store.Policy) error {
	policy.Updated = time.Now()

	result, err := s.db.ExecContext(ctx, `
		UPDATE policies SET
			name = $1, description = $2, scope_type = $3, scope_id = $4,
			resource_type = $5, resource_id = $6,
			actions = $7, effect = $8, conditions = $9,
			priority = $10, labels = $11, annotations = $12,
			updated_at = $13
		WHERE id = $14
	`,
		policy.Name, policy.Description, policy.ScopeType, policy.ScopeID,
		policy.ResourceType, policy.ResourceID,
		marshalJSON(policy.Actions), policy.Effect, marshalJSON(policy.Conditions),
		policy.Priority, marshalJSON(policy.Labels), marshalJSON(policy.Annotations),
		policy.Updated,
		policy.ID,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) DeletePolicy(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM policies WHERE id = $1", id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) ListPolicies(ctx context.Context, filter store.PolicyFilter, opts store.ListOptions) (*store.ListResult[store.Policy], error) {
	var conditions []string
	var args []interface{}
	idx := 1

	if filter.Name != "" {
		conditions = append(conditions, fmt.Sprintf("name = $%d", idx))
		args = append(args, filter.Name)
		idx++
	}
	if filter.ScopeType != "" {
		conditions = append(conditions, fmt.Sprintf("scope_type = $%d", idx))
		args = append(args, filter.ScopeType)
		idx++
	}
	if filter.ScopeID != "" {
		conditions = append(conditions, fmt.Sprintf("scope_id = $%d", idx))
		args = append(args, filter.ScopeID)
		idx++
	}
	if filter.ResourceType != "" {
		conditions = append(conditions, fmt.Sprintf("resource_type = $%d", idx))
		args = append(args, filter.ResourceType)
		idx++
	}
	if filter.Effect != "" {
		conditions = append(conditions, fmt.Sprintf("effect = $%d", idx))
		args = append(args, filter.Effect)
		idx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var totalCount int
	if err := s.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM policies %s", where), args...,
	).Scan(&totalCount); err != nil {
		return nil, err
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, name, description, scope_type, scope_id, resource_type, resource_id, actions, effect, conditions, priority, labels, annotations, created_at, updated_at, created_by
		FROM policies %s ORDER BY priority DESC, created_at DESC LIMIT $%d
	`, where, idx), append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []store.Policy
	for rows.Next() {
		var policy store.Policy
		var actions, conditions, labels, annotations string

		if err := rows.Scan(
			&policy.ID, &policy.Name, &policy.Description, &policy.ScopeType, &policy.ScopeID,
			&policy.ResourceType, &policy.ResourceID,
			&actions, &policy.Effect, &conditions,
			&policy.Priority, &labels, &annotations,
			&policy.Created, &policy.Updated, &policy.CreatedBy,
		); err != nil {
			return nil, err
		}

		unmarshalJSON(actions, &policy.Actions)
		unmarshalJSON(conditions, &policy.Conditions)
		unmarshalJSON(labels, &policy.Labels)
		unmarshalJSON(annotations, &policy.Annotations)
		policies = append(policies, policy)
	}

	return &store.ListResult[store.Policy]{
		Items:      policies,
		TotalCount: totalCount,
	}, nil
}

func (s *PostgresStore) AddPolicyBinding(ctx context.Context, binding *store.PolicyBinding) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO policy_bindings (policy_id, principal_type, principal_id)
		VALUES ($1,$2,$3)
	`,
		binding.PolicyID, binding.PrincipalType, binding.PrincipalID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return store.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (s *PostgresStore) RemovePolicyBinding(ctx context.Context, policyID, principalType, principalID string) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM policy_bindings WHERE policy_id = $1 AND principal_type = $2 AND principal_id = $3",
		policyID, principalType, principalID,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) GetPolicyBindings(ctx context.Context, policyID string) ([]store.PolicyBinding, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT policy_id, principal_type, principal_id
		FROM policy_bindings WHERE policy_id = $1
	`, policyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bindings []store.PolicyBinding
	for rows.Next() {
		var binding store.PolicyBinding
		if err := rows.Scan(&binding.PolicyID, &binding.PrincipalType, &binding.PrincipalID); err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

func (s *PostgresStore) GetPoliciesForPrincipal(ctx context.Context, principalType, principalID string) ([]store.Policy, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.name, p.description, p.scope_type, p.scope_id, p.resource_type, p.resource_id, p.actions, p.effect, p.conditions, p.priority, p.labels, p.annotations, p.created_at, p.updated_at, p.created_by
		FROM policies p
		INNER JOIN policy_bindings pb ON p.id = pb.policy_id
		WHERE pb.principal_type = $1 AND pb.principal_id = $2
		ORDER BY p.priority DESC, p.created_at DESC
	`, principalType, principalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPolicies(rows)
}

func (s *PostgresStore) GetPoliciesForPrincipals(ctx context.Context, principals []store.PrincipalRef) ([]store.Policy, error) {
	if len(principals) == 0 {
		return nil, nil
	}

	var clauses []string
	var args []interface{}
	for i, p := range principals {
		clauses = append(clauses, fmt.Sprintf("(pb.principal_type = $%d AND pb.principal_id = $%d)", i*2+1, i*2+2))
		args = append(args, p.Type, p.ID)
	}

	query := `
		SELECT DISTINCT p.id, p.name, p.description, p.scope_type, p.scope_id, p.resource_type, p.resource_id, p.actions, p.effect, p.conditions, p.priority, p.labels, p.annotations, p.created_at, p.updated_at, p.created_by
		FROM policies p
		INNER JOIN policy_bindings pb ON p.id = pb.policy_id
		WHERE ` + strings.Join(clauses, " OR ") + `
		ORDER BY
			CASE p.scope_type WHEN 'hub' THEN 0 WHEN 'project' THEN 1 WHEN 'resource' THEN 2 END,
			p.priority ASC
	`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPolicies(rows)
}

func scanPolicies(rows *sql.Rows) ([]store.Policy, error) {
	var policies []store.Policy
	for rows.Next() {
		var policy store.Policy
		var actions, conditions, labels, annotations string

		if err := rows.Scan(
			&policy.ID, &policy.Name, &policy.Description, &policy.ScopeType, &policy.ScopeID,
			&policy.ResourceType, &policy.ResourceID,
			&actions, &policy.Effect, &conditions,
			&policy.Priority, &labels, &annotations,
			&policy.Created, &policy.Updated, &policy.CreatedBy,
		); err != nil {
			return nil, err
		}

		unmarshalJSON(actions, &policy.Actions)
		unmarshalJSON(conditions, &policy.Conditions)
		unmarshalJSON(labels, &policy.Labels)
		unmarshalJSON(annotations, &policy.Annotations)
		policies = append(policies, policy)
	}
	return policies, rows.Err()
}
