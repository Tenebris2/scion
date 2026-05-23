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

// migrationV1 is the squashed post-V53 schema in Postgres dialect.
// Translation rules applied from the SQLite source:
//   - TIMESTAMP DEFAULT CURRENT_TIMESTAMP → TIMESTAMPTZ DEFAULT NOW()
//   - INTEGER DEFAULT 0/1 (boolean columns) → SMALLINT (scan sites unchanged)
//   - BLOB → BYTEA
//   - INTEGER PRIMARY KEY on installation_id/app_id → BIGINT (Go int64)
//   - agents.ancestry, labels, annotations → JSONB (required for jsonb operators)
//   - email UNIQUE COLLATE NOCASE → lower(email) unique index (avoids citext)
//   - INSERT OR IGNORE → ON CONFLICT DO NOTHING (applied at query time, not here)
//   - json_each → jsonb_array_elements_text (applied at query time)
//   - Ent-managed tables (groups, policies, users, projects) may alias with Ent
//     schema; verify on real Postgres before first deployment.
const migrationV1 = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TIMESTAMPTZ DEFAULT NOW()
);

-- Projects (renamed from groves in SQLite V50)
CREATE TABLE IF NOT EXISTS projects (
    id                       TEXT        NOT NULL PRIMARY KEY,
    name                     TEXT        NOT NULL,
    slug                     TEXT        NOT NULL,
    git_remote               TEXT,
    labels                   JSONB,
    annotations              JSONB,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by               TEXT,
    owner_id                 TEXT,
    visibility               TEXT        NOT NULL DEFAULT 'private',
    default_runtime_broker_id TEXT,
    shared_dirs              TEXT,
    github_installation_id   BIGINT,
    github_permissions       TEXT,
    github_app_status        TEXT,
    git_identity             TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_slug ON projects(slug);
CREATE INDEX IF NOT EXISTS idx_projects_git_remote ON projects(git_remote);
CREATE INDEX IF NOT EXISTS idx_projects_owner ON projects(owner_id);
CREATE INDEX IF NOT EXISTS idx_projects_default_runtime_broker ON projects(default_runtime_broker_id);

-- Runtime Brokers
CREATE TABLE IF NOT EXISTS runtime_brokers (
    id                TEXT        NOT NULL PRIMARY KEY,
    name              TEXT        NOT NULL,
    slug              TEXT        NOT NULL,
    type              TEXT        NOT NULL,
    mode              TEXT        NOT NULL DEFAULT 'connected',
    version           TEXT,
    status            TEXT        NOT NULL DEFAULT 'offline',
    connection_state  TEXT                 DEFAULT 'disconnected',
    last_heartbeat    TIMESTAMPTZ,
    capabilities      TEXT,
    supported_harnesses TEXT,
    resources         TEXT,
    runtimes          TEXT,
    labels            JSONB,
    annotations       JSONB,
    endpoint          TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by        TEXT,
    auto_provide      SMALLINT    NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_runtime_brokers_slug ON runtime_brokers(slug);
CREATE INDEX IF NOT EXISTS idx_runtime_brokers_status ON runtime_brokers(status);

-- FK from projects to runtime_brokers (deferred; runtime_brokers created first)
ALTER TABLE projects
    ADD CONSTRAINT fk_projects_default_broker
    FOREIGN KEY (default_runtime_broker_id) REFERENCES runtime_brokers(id) ON DELETE SET NULL
    DEFERRABLE INITIALLY DEFERRED;

-- GitHub App Installations (referenced by projects)
CREATE TABLE IF NOT EXISTS github_installations (
    installation_id BIGINT      NOT NULL PRIMARY KEY,
    account_login   TEXT        NOT NULL,
    account_type    TEXT        NOT NULL DEFAULT 'Organization',
    app_id          BIGINT      NOT NULL,
    repositories    JSONB       NOT NULL DEFAULT '[]',
    status          TEXT        NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_github_installations_account ON github_installations(account_login);
CREATE INDEX IF NOT EXISTS idx_github_installations_status ON github_installations(status);

ALTER TABLE projects
    ADD CONSTRAINT fk_projects_github_installation
    FOREIGN KEY (github_installation_id) REFERENCES github_installations(installation_id)
    DEFERRABLE INITIALLY DEFERRED;

-- Project Contributors (renamed from grove_contributors)
CREATE TABLE IF NOT EXISTS project_contributors (
    project_id  TEXT        NOT NULL,
    broker_id   TEXT        NOT NULL,
    broker_name TEXT        NOT NULL,
    mode        TEXT        NOT NULL DEFAULT 'connected',
    status      TEXT        NOT NULL DEFAULT 'offline',
    profiles    TEXT,
    last_seen   TIMESTAMPTZ,
    local_path  TEXT,
    linked_by   TEXT,
    linked_at   TIMESTAMPTZ,
    PRIMARY KEY (project_id, broker_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (broker_id) REFERENCES runtime_brokers(id) ON DELETE CASCADE
);

-- Agents
CREATE TABLE IF NOT EXISTS agents (
    id                     TEXT        NOT NULL PRIMARY KEY,
    agent_id               TEXT        NOT NULL,
    name                   TEXT        NOT NULL,
    template               TEXT        NOT NULL,
    project_id             TEXT        NOT NULL,
    labels                 JSONB,
    annotations            JSONB,
    phase                  TEXT        NOT NULL DEFAULT 'created',
    activity               TEXT                 DEFAULT '',
    tool_name              TEXT                 DEFAULT '',
    connection_state       TEXT                 DEFAULT 'unknown',
    container_status       TEXT,
    runtime_state          TEXT,
    image                  TEXT,
    detached               SMALLINT    NOT NULL DEFAULT 1,
    runtime                TEXT,
    runtime_broker_id      TEXT,
    web_pty_enabled        SMALLINT    NOT NULL DEFAULT 0,
    task_summary           TEXT,
    message                TEXT,
    applied_config         TEXT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen              TIMESTAMPTZ,
    last_activity_event    TIMESTAMPTZ,
    created_by             TEXT,
    owner_id               TEXT,
    visibility             TEXT        NOT NULL DEFAULT 'private',
    state_version          INTEGER     NOT NULL DEFAULT 1,
    stalled_from_activity  TEXT                 DEFAULT '',
    current_turns          INTEGER              DEFAULT 0,
    current_model_calls    INTEGER              DEFAULT 0,
    started_at             TIMESTAMPTZ,
    deleted_at             TIMESTAMPTZ,
    ancestry               JSONB,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (runtime_broker_id) REFERENCES runtime_brokers(id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_project_slug ON agents(agent_id, project_id);
CREATE INDEX IF NOT EXISTS idx_agents_project ON agents(project_id);
CREATE INDEX IF NOT EXISTS idx_agents_phase ON agents(phase);
CREATE INDEX IF NOT EXISTS idx_agents_runtime_broker ON agents(runtime_broker_id);
CREATE INDEX IF NOT EXISTS idx_agents_deleted ON agents(deleted_at) WHERE deleted_at IS NOT NULL;

-- Templates
CREATE TABLE IF NOT EXISTS templates (
    id                   TEXT        NOT NULL PRIMARY KEY,
    name                 TEXT        NOT NULL,
    slug                 TEXT        NOT NULL,
    harness              TEXT        NOT NULL,
    image                TEXT,
    config               TEXT,
    scope                TEXT        NOT NULL DEFAULT 'global',
    project_id           TEXT,
    storage_uri          TEXT,
    owner_id             TEXT,
    visibility           TEXT        NOT NULL DEFAULT 'private',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    display_name         TEXT,
    description          TEXT,
    content_hash         TEXT,
    scope_id             TEXT,
    storage_bucket       TEXT,
    storage_path         TEXT,
    files                TEXT,
    base_template        TEXT,
    locked               SMALLINT    NOT NULL DEFAULT 0,
    status               TEXT        NOT NULL DEFAULT 'active',
    created_by           TEXT,
    updated_by           TEXT,
    default_harness_config TEXT,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_templates_slug_scope ON templates(slug, scope);
CREATE INDEX IF NOT EXISTS idx_templates_harness ON templates(harness);
CREATE INDEX IF NOT EXISTS idx_templates_status ON templates(status);
CREATE INDEX IF NOT EXISTS idx_templates_content_hash ON templates(content_hash);
CREATE INDEX IF NOT EXISTS idx_templates_scope_id ON templates(scope, scope_id);

-- Harness Configs
CREATE TABLE IF NOT EXISTS harness_configs (
    id           TEXT        NOT NULL PRIMARY KEY,
    name         TEXT        NOT NULL,
    slug         TEXT        NOT NULL,
    display_name TEXT,
    description  TEXT,
    harness      TEXT        NOT NULL,
    config       TEXT,
    content_hash TEXT,
    scope        TEXT        NOT NULL DEFAULT 'global',
    scope_id     TEXT,
    storage_uri  TEXT,
    storage_bucket TEXT,
    storage_path TEXT,
    files        TEXT,
    locked       SMALLINT    NOT NULL DEFAULT 0,
    status       TEXT        NOT NULL DEFAULT 'active',
    owner_id     TEXT,
    created_by   TEXT,
    updated_by   TEXT,
    visibility   TEXT        NOT NULL DEFAULT 'private',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_harness_configs_slug_scope ON harness_configs(slug, scope);
CREATE INDEX IF NOT EXISTS idx_harness_configs_harness ON harness_configs(harness);
CREATE INDEX IF NOT EXISTS idx_harness_configs_status ON harness_configs(status);
CREATE INDEX IF NOT EXISTS idx_harness_configs_content_hash ON harness_configs(content_hash);
CREATE INDEX IF NOT EXISTS idx_harness_configs_scope_id ON harness_configs(scope, scope_id);

-- Users
CREATE TABLE IF NOT EXISTS users (
    id           TEXT        NOT NULL PRIMARY KEY,
    email        TEXT        NOT NULL,
    display_name TEXT        NOT NULL,
    avatar_url   TEXT,
    role         TEXT        NOT NULL DEFAULT 'member',
    status       TEXT        NOT NULL DEFAULT 'active',
    preferences  TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login   TIMESTAMPTZ,
    last_seen    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
-- Case-insensitive uniqueness via functional index (avoids citext extension dep)
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users(lower(email));

-- API Keys (legacy, replaced by user_access_tokens; retained for schema completeness)
CREATE TABLE IF NOT EXISTS api_keys (
    id         TEXT        NOT NULL PRIMARY KEY,
    user_id    TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    prefix     TEXT        NOT NULL,
    key_hash   TEXT        NOT NULL UNIQUE,
    scopes     TEXT,
    revoked    SMALLINT    NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    last_used  TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);

-- Environment Variables
CREATE TABLE IF NOT EXISTS env_vars (
    id             TEXT        NOT NULL PRIMARY KEY,
    key            TEXT        NOT NULL,
    value          TEXT        NOT NULL,
    scope          TEXT        NOT NULL,
    scope_id       TEXT        NOT NULL,
    description    TEXT,
    sensitive      SMALLINT    NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by     TEXT,
    injection_mode TEXT        NOT NULL DEFAULT 'as_needed',
    secret         SMALLINT    NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_env_vars_key_scope ON env_vars(key, scope, scope_id);
CREATE INDEX IF NOT EXISTS idx_env_vars_scope ON env_vars(scope, scope_id);

-- Secrets
CREATE TABLE IF NOT EXISTS secrets (
    id             TEXT        NOT NULL PRIMARY KEY,
    key            TEXT        NOT NULL,
    encrypted_value TEXT       NOT NULL,
    scope          TEXT        NOT NULL,
    scope_id       TEXT        NOT NULL,
    description    TEXT,
    version        INTEGER     NOT NULL DEFAULT 1,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by     TEXT,
    updated_by     TEXT,
    secret_type    TEXT        NOT NULL DEFAULT 'environment',
    target         TEXT,
    secret_ref     TEXT,
    injection_mode TEXT        NOT NULL DEFAULT 'as_needed',
    allow_progeny  SMALLINT    NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_secrets_key_scope ON secrets(key, scope, scope_id);
CREATE INDEX IF NOT EXISTS idx_secrets_scope ON secrets(scope, scope_id);

-- Groups
CREATE TABLE IF NOT EXISTS groups (
    id          TEXT        NOT NULL PRIMARY KEY,
    name        TEXT        NOT NULL,
    slug        TEXT        NOT NULL UNIQUE,
    description TEXT,
    parent_id   TEXT        REFERENCES groups(id) ON DELETE SET NULL,
    labels      JSONB,
    annotations JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  TEXT,
    owner_id    TEXT,
    group_type  TEXT        NOT NULL DEFAULT 'explicit',
    project_id  TEXT                 DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_groups_slug ON groups(slug);
CREATE INDEX IF NOT EXISTS idx_groups_parent ON groups(parent_id);
CREATE INDEX IF NOT EXISTS idx_groups_owner ON groups(owner_id);
CREATE INDEX IF NOT EXISTS idx_groups_project ON groups(project_id);

-- Group Members
CREATE TABLE IF NOT EXISTS group_members (
    group_id    TEXT        NOT NULL,
    member_type TEXT        NOT NULL,
    member_id   TEXT        NOT NULL,
    role        TEXT        NOT NULL DEFAULT 'member',
    added_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    added_by    TEXT,
    PRIMARY KEY (group_id, member_type, member_id),
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_group_members_member ON group_members(member_type, member_id);

-- Policies
CREATE TABLE IF NOT EXISTS policies (
    id            TEXT        NOT NULL PRIMARY KEY,
    name          TEXT        NOT NULL,
    description   TEXT,
    scope_type    TEXT        NOT NULL,
    scope_id      TEXT,
    resource_type TEXT        NOT NULL DEFAULT '*',
    resource_id   TEXT,
    actions       JSONB       NOT NULL,
    effect        TEXT        NOT NULL,
    conditions    JSONB,
    priority      INTEGER     NOT NULL DEFAULT 0,
    labels        JSONB,
    annotations   JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    TEXT
);
CREATE INDEX IF NOT EXISTS idx_policies_scope ON policies(scope_type, scope_id);
CREATE INDEX IF NOT EXISTS idx_policies_effect ON policies(effect);
CREATE INDEX IF NOT EXISTS idx_policies_priority ON policies(priority DESC);

-- Policy Bindings
CREATE TABLE IF NOT EXISTS policy_bindings (
    policy_id      TEXT NOT NULL,
    principal_type TEXT NOT NULL,
    principal_id   TEXT NOT NULL,
    PRIMARY KEY (policy_id, principal_type, principal_id),
    FOREIGN KEY (policy_id) REFERENCES policies(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_policy_bindings_principal ON policy_bindings(principal_type, principal_id);

-- Broker Secrets
CREATE TABLE IF NOT EXISTS broker_secrets (
    broker_id  TEXT        NOT NULL PRIMARY KEY,
    secret_key BYTEA       NOT NULL,
    algorithm  TEXT        NOT NULL DEFAULT 'hmac-sha256',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rotated_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    status     TEXT        NOT NULL DEFAULT 'active',
    FOREIGN KEY (broker_id) REFERENCES runtime_brokers(id) ON DELETE CASCADE
);

-- Broker Join Tokens
CREATE TABLE IF NOT EXISTS broker_join_tokens (
    broker_id  TEXT        NOT NULL PRIMARY KEY,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT        NOT NULL,
    FOREIGN KEY (broker_id) REFERENCES runtime_brokers(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_broker_join_tokens_hash ON broker_join_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_broker_join_tokens_expires ON broker_join_tokens(expires_at);

-- Notification Subscriptions
CREATE TABLE IF NOT EXISTS notification_subscriptions (
    id               TEXT        NOT NULL PRIMARY KEY,
    scope            TEXT        NOT NULL DEFAULT 'agent',
    agent_id         TEXT,
    subscriber_type  TEXT        NOT NULL DEFAULT 'agent',
    subscriber_id    TEXT        NOT NULL,
    project_id       TEXT        NOT NULL,
    trigger_activities TEXT      NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by       TEXT        NOT NULL,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_notification_subs_agent ON notification_subscriptions(agent_id);
CREATE INDEX IF NOT EXISTS idx_notification_subs_project ON notification_subscriptions(project_id);
CREATE INDEX IF NOT EXISTS idx_notification_subs_subscriber ON notification_subscriptions(subscriber_type, subscriber_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_subs_unique
    ON notification_subscriptions(scope, COALESCE(agent_id, ''), subscriber_type, subscriber_id, project_id);

-- Notifications
CREATE TABLE IF NOT EXISTS notifications (
    id              TEXT        NOT NULL PRIMARY KEY,
    subscription_id TEXT        NOT NULL,
    agent_id        TEXT        NOT NULL,
    project_id      TEXT        NOT NULL,
    subscriber_type TEXT        NOT NULL,
    subscriber_id   TEXT        NOT NULL,
    status          TEXT        NOT NULL,
    message         TEXT        NOT NULL,
    dispatched      SMALLINT    NOT NULL DEFAULT 0,
    acknowledged    SMALLINT    NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (subscription_id) REFERENCES notification_subscriptions(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_notifications_subscriber ON notifications(subscriber_type, subscriber_id);
CREATE INDEX IF NOT EXISTS idx_notifications_project ON notifications(project_id);

-- Scheduled Events
CREATE TABLE IF NOT EXISTS scheduled_events (
    id          TEXT        NOT NULL PRIMARY KEY,
    project_id  TEXT        NOT NULL,
    event_type  TEXT        NOT NULL,
    fire_at     TIMESTAMPTZ NOT NULL,
    payload     TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  TEXT,
    fired_at    TIMESTAMPTZ,
    error       TEXT,
    schedule_id TEXT                 DEFAULT '',
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_scheduled_events_status ON scheduled_events(status);
CREATE INDEX IF NOT EXISTS idx_scheduled_events_fire_at ON scheduled_events(fire_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_scheduled_events_project ON scheduled_events(project_id);

-- Schedules
CREATE TABLE IF NOT EXISTS schedules (
    id              TEXT        NOT NULL PRIMARY KEY,
    project_id      TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    cron_expr       TEXT        NOT NULL,
    event_type      TEXT        NOT NULL,
    payload         TEXT        NOT NULL DEFAULT '{}',
    status          TEXT        NOT NULL DEFAULT 'active',
    next_run_at     TIMESTAMPTZ,
    last_run_at     TIMESTAMPTZ,
    last_run_status TEXT,
    last_run_error  TEXT,
    run_count       INTEGER     NOT NULL DEFAULT 0,
    error_count     INTEGER     NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, name),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_schedules_project ON schedules(project_id);
CREATE INDEX IF NOT EXISTS idx_schedules_next_run ON schedules(next_run_at) WHERE status = 'active';

-- Subscription Templates
CREATE TABLE IF NOT EXISTS subscription_templates (
    id                 TEXT NOT NULL PRIMARY KEY,
    name               TEXT NOT NULL,
    scope              TEXT NOT NULL DEFAULT 'project',
    trigger_activities TEXT NOT NULL,
    project_id         TEXT NOT NULL DEFAULT '',
    created_by         TEXT NOT NULL,
    UNIQUE(project_id, name)
);
CREATE INDEX IF NOT EXISTS idx_sub_templates_project ON subscription_templates(project_id);

-- User Access Tokens
CREATE TABLE IF NOT EXISTS user_access_tokens (
    id         TEXT        NOT NULL PRIMARY KEY,
    user_id    TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    prefix     TEXT        NOT NULL,
    key_hash   TEXT        NOT NULL UNIQUE,
    project_id TEXT        NOT NULL,
    scopes     TEXT        NOT NULL,
    revoked    SMALLINT    NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    last_used  TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_uat_user_id ON user_access_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_uat_key_hash ON user_access_tokens(key_hash);

-- GCP Service Accounts
CREATE TABLE IF NOT EXISTS gcp_service_accounts (
    id             TEXT        NOT NULL PRIMARY KEY,
    scope          TEXT        NOT NULL,
    scope_id       TEXT        NOT NULL,
    email          TEXT        NOT NULL,
    project_id     TEXT        NOT NULL,
    display_name   TEXT        NOT NULL DEFAULT '',
    default_scopes TEXT        NOT NULL DEFAULT '',
    verified       SMALLINT    NOT NULL DEFAULT 0,
    verified_at    TIMESTAMPTZ,
    created_by     TEXT        NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    managed        SMALLINT    NOT NULL DEFAULT 0,
    managed_by     TEXT        NOT NULL DEFAULT '',
    UNIQUE(email, scope, scope_id)
);
CREATE INDEX IF NOT EXISTS idx_gcp_sa_scope ON gcp_service_accounts(scope, scope_id);
CREATE INDEX IF NOT EXISTS idx_gcp_sa_project ON gcp_service_accounts(project_id);

-- Messages
CREATE TABLE IF NOT EXISTS messages (
    id           TEXT        NOT NULL PRIMARY KEY,
    project_id   TEXT        NOT NULL,
    sender       TEXT        NOT NULL,
    sender_id    TEXT        NOT NULL DEFAULT '',
    recipient    TEXT        NOT NULL,
    recipient_id TEXT        NOT NULL DEFAULT '',
    msg          TEXT        NOT NULL,
    type         TEXT        NOT NULL DEFAULT 'instruction',
    urgent       SMALLINT    NOT NULL DEFAULT 0,
    broadcasted  SMALLINT    NOT NULL DEFAULT 0,
    read         SMALLINT    NOT NULL DEFAULT 0,
    agent_id     TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    group_id     TEXT        NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_messages_project ON messages(project_id);
CREATE INDEX IF NOT EXISTS idx_messages_recipient ON messages(recipient_id, read);
CREATE INDEX IF NOT EXISTS idx_messages_agent ON messages(agent_id);
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender_id);
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at DESC);

-- Maintenance Operations
CREATE TABLE IF NOT EXISTS maintenance_operations (
    id           TEXT        NOT NULL PRIMARY KEY,
    key          TEXT        NOT NULL UNIQUE,
    title        TEXT        NOT NULL,
    description  TEXT        NOT NULL DEFAULT '',
    category     TEXT        NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'pending',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    started_by   TEXT,
    result       TEXT,
    metadata     TEXT        NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_maintenance_ops_category ON maintenance_operations(category);
CREATE INDEX IF NOT EXISTS idx_maintenance_ops_status ON maintenance_operations(status);

CREATE TABLE IF NOT EXISTS maintenance_operation_runs (
    id            TEXT        NOT NULL PRIMARY KEY,
    operation_key TEXT        NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'running',
    started_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ,
    started_by    TEXT,
    result        TEXT,
    log           TEXT        NOT NULL DEFAULT '',
    FOREIGN KEY (operation_key) REFERENCES maintenance_operations(key)
);
CREATE INDEX IF NOT EXISTS idx_maintenance_runs_key ON maintenance_operation_runs(operation_key);
CREATE INDEX IF NOT EXISTS idx_maintenance_runs_started ON maintenance_operation_runs(started_at DESC);

-- Seed maintenance operations
INSERT INTO maintenance_operations (id, key, title, description, category, status)
VALUES
    (gen_random_uuid()::text, 'secret-hub-id-migration',
     'Secret Hub ID Namespace Migration',
     'Migrates hub-scoped secrets from the legacy fixed "hub" scope ID to the per-instance hub ID.',
     'migration', 'pending'),
    (gen_random_uuid()::text, 'pull-images',
     'Pull Container Images',
     'Pulls the latest container images for all configured harnesses from the image registry.',
     'operation', 'pending'),
    (gen_random_uuid()::text, 'rebuild-server',
     'Rebuild Server from Git',
     'Pulls latest code from the repository, rebuilds the server binary and web assets, then restarts the hub service.',
     'operation', 'pending'),
    (gen_random_uuid()::text, 'rebuild-web',
     'Rebuild Web Frontend',
     'Rebuilds only the web frontend assets from source without restarting the server binary.',
     'operation', 'pending'),
    (gen_random_uuid()::text, 'rebuild-container-binaries',
     'Rebuild Container Binaries',
     'Rebuilds scion and sciontool binaries for Linux containers (make container-binaries).',
     'operation', 'pending')
ON CONFLICT (key) DO NOTHING;

-- Project Sync State (renamed from grove_sync_state)
CREATE TABLE IF NOT EXISTS project_sync_state (
    project_id      TEXT        NOT NULL,
    broker_id       TEXT        NOT NULL DEFAULT '',
    last_sync_time  TIMESTAMPTZ,
    last_commit_sha TEXT,
    file_count      INTEGER     NOT NULL DEFAULT 0,
    total_bytes     INTEGER     NOT NULL DEFAULT 0,
    PRIMARY KEY (project_id, broker_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_project_sync_state_project ON project_sync_state(project_id);

-- Allow List
CREATE TABLE IF NOT EXISTS allow_list (
    id        TEXT        NOT NULL PRIMARY KEY,
    email     TEXT        NOT NULL,
    note      TEXT        NOT NULL DEFAULT '',
    added_by  TEXT        NOT NULL,
    invite_id TEXT        NOT NULL DEFAULT '',
    created   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_allow_list_email_lower ON allow_list(lower(email));
CREATE INDEX IF NOT EXISTS idx_allow_list_created_id ON allow_list(created DESC, id DESC);

-- Invite Codes
CREATE TABLE IF NOT EXISTS invite_codes (
    id          TEXT        NOT NULL PRIMARY KEY,
    code_hash   TEXT        NOT NULL UNIQUE,
    code_prefix TEXT        NOT NULL,
    max_uses    INTEGER     NOT NULL DEFAULT 1,
    use_count   INTEGER     NOT NULL DEFAULT 0,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked     SMALLINT    NOT NULL DEFAULT 0,
    created_by  TEXT        NOT NULL,
    note        TEXT        NOT NULL DEFAULT '',
    created     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_invite_codes_expires ON invite_codes(expires_at);
`
