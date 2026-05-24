# SQLite Store — Code & Data Model

## Architecture Overview

```
pkg/store/
├── store.go            — Store interface (21 sub-interfaces) + filter/result types
├── models.go           — Persistence models (Go structs)
├── sqlite/
│   ├── driver.go       — Registers modernc.org/sqlite (pure-Go, build tag: !no_sqlite)
│   └── sqlite.go       — SQLiteStore: migrations V1–V53, CRUD implementations
└── entadapter/
    ├── composite.go    — CompositeStore: SQLite base + Ent-backed Group/Policy overrides
    ├── group_store.go  — Ent-backed GroupStore
    └── policy_store.go — Ent-backed PolicyStore
```

### SQLiteStore

```go
type SQLiteStore struct { db *sql.DB }

// Connection settings
MaxOpenConns = 4
MaxIdleConns = 4

// DSN pragmas (applied per-connection)
busy_timeout = 5000ms
foreign_keys = ON
journal_mode = WAL       // concurrent readers + single writer
```

In-memory databases use `file:memdbN?mode=memory&cache=shared` with an atomic counter so parallel tests each get an isolated instance.

### CompositeStore (entadapter)

`CompositeStore` embeds `store.Store` (the SQLite base) and overrides Group and Policy operations with an Ent ORM client. This lets the permissions system use Ent's richer graph traversal while everything else stays on raw SQL. The `Close()` method shuts down both.

---

## Migration History (V1–V53)

| Version | Description |
|---------|-------------|
| V1 | Initial schema: `groves`, `runtime_brokers`, `grove_contributors`, `agents`, `templates`, `users` |
| V2 | Add `default_runtime_broker_id` to `groves` |
| V3 | Add `local_path` to `grove_contributors` |
| V4 | Add `env_vars` and `secrets` tables |
| V5 | Add `groups`, `group_members`, `policies`, `policy_bindings` tables |
| V6 | Extend `templates`: display_name, description, content_hash, scope_id, storage fields, files, base_template, locked, status |
| V7 | Add `api_keys` table (later superseded by `user_access_tokens` in V34) |
| V8 | Add `message` column to `agents` |
| V9 | Add `broker_secrets` and `broker_join_tokens` tables |
| V10 | Add `linked_by`/`linked_at` to `grove_contributors`; `created_by` to `runtime_brokers` |
| V11 | Add `auto_provide` to `runtime_brokers` |
| V12 | Add `injection_mode` and `secret` columns to `env_vars` |
| V13 | Add `secret_type` and `target` columns to `secrets` |
| V14 | Add `secret_ref` to `secrets` (external SM reference, e.g. GCP SM) |
| V15 | Merge legacy `session_status` into main `status` column; drop `session_status` |
| V16 | Add `harness_configs` table |
| V17 | Add `deleted_at` to `agents` (soft-delete support) |
| V18 | Add `notification_subscriptions` and `notifications` tables |
| V19 | Add `scheduled_events` table |
| V20 | Add `phase`, `activity`, `tool_name` to `agents`; backfill from `status`; add `idx_agents_phase` |
| V21 | Drop legacy `status` column from `agents` (replaced by phase/activity) |
| V22 | Rename `trigger_statuses` → `trigger_activities` in `notification_subscriptions` |
| V23 | Add `injection_mode` to `secrets` |
| V24 | Add `last_activity_event` to `agents` for stalled detection |
| V25 | Add `stalled_from_activity` to `agents` |
| V26 | Add `current_turns`, `current_model_calls`, `started_at` to `agents` (limits tracking) |
| V27 | Add `last_seen` to `users` |
| V28 | Add `shared_dirs` to `groves` |
| V29 | Add `group_type` and `grove_id` to `groups` |
| V30 | Add `gcp_service_accounts` table |
| V31 | Recreate `notification_subscriptions` to make `agent_id` nullable, add `scope` column, add unique index |
| V32 | Add `schedules` table; add `schedule_id` FK to `scheduled_events` |
| V33 | Add `subscription_templates` table |
| V34 | Add `user_access_tokens` table (scoped PATs) |
| V35 | Add `github_installations` table; add GitHub columns to `groves` |
| V36 | Add `git_identity` to `groves` |
| V37 | Add `ancestry` column to `agents` (JSON array for transitive ACL) |
| V38 | Backfill `ancestry` from `created_by` for existing agents |
| V39 | Add `messages` table (bidirectional human-agent messaging) |
| V40 | Recreate `groves`: drop UNIQUE on `git_remote`, add UNIQUE on `slug` (**requires FK off**) |
| V41 | Add `maintenance_operations` and `maintenance_operation_runs` tables; seed built-in ops |
| V42 | Add `grove_sync_state` table |
| V43 | Data fix: backfill `secret_type='internal'` for signing key secrets |
| V44 | Add `managed` and `managed_by` to `gcp_service_accounts` |
| V45 | Add `allow_progeny` to `secrets` |
| V46 | Add `default_harness_config` to `templates` |
| V47 | Seed `rebuild-container-binaries` maintenance operation |
| V48 | Add `allow_list` table |
| V49 | Add `invite_codes` table |
| V50 | **Rename** `groves`→`projects`, `grove_contributors`→`project_contributors`, `grove_sync_state`→`project_sync_state`; rename all `grove_id` columns to `project_id`; update scope enum values `grove`→`project` across many tables (programmatic migration, idempotent) |
| V51 | Add `group_id` to `messages` |
| V52 | Data rename: activity `idle`→`working` |
| V53 | Ensure `allow_list` and `invite_codes` exist (resilience for DBs that skipped V48/V49); add pagination index on `allow_list(created DESC, id DESC)` |

---

## Final Schema (post V53)

### `projects` (renamed from `groves` in V40/V50)

```sql
id                      TEXT PRIMARY KEY
name                    TEXT NOT NULL
slug                    TEXT NOT NULL UNIQUE
git_remote              TEXT                        -- nullable; multiple projects may share a remote
default_runtime_broker_id TEXT → runtime_brokers(id) ON DELETE SET NULL
labels                  TEXT  -- JSON map[string]string
annotations             TEXT  -- JSON map[string]string
shared_dirs             TEXT  -- JSON []api.SharedDir
github_installation_id  INTEGER → github_installations(installation_id)
github_permissions      TEXT  -- JSON GitHubTokenPermissions
github_app_status       TEXT  -- JSON GitHubAppProjectStatus
git_identity            TEXT  -- JSON GitIdentityConfig
created_by              TEXT
owner_id                TEXT
visibility              TEXT NOT NULL DEFAULT 'private'
created_at              TIMESTAMP
updated_at              TIMESTAMP

-- Indexes
idx_projects_slug           UNIQUE
idx_projects_git_remote
idx_projects_owner
idx_projects_default_runtime_broker
```

### `agents`

```sql
id                      TEXT PRIMARY KEY
agent_id                TEXT NOT NULL       -- slug (unique per project)
name                    TEXT NOT NULL
template                TEXT NOT NULL
project_id              TEXT NOT NULL → projects(id) ON DELETE CASCADE

labels                  TEXT  -- JSON
annotations             TEXT  -- JSON

-- State (phase/activity replaces the old status column as of V21)
phase                   TEXT NOT NULL DEFAULT 'created'
activity                TEXT DEFAULT ''
tool_name               TEXT DEFAULT ''
connection_state        TEXT DEFAULT 'unknown'
container_status        TEXT
runtime_state           TEXT
stalled_from_activity   TEXT DEFAULT ''     -- non-empty when agent is stalled

-- Limits (reported by sciontool inside container)
current_turns           INTEGER DEFAULT 0
current_model_calls     INTEGER DEFAULT 0
started_at              TIMESTAMP

-- Runtime
image                   TEXT
detached                INTEGER NOT NULL DEFAULT 1
runtime                 TEXT               -- docker, kubernetes, apple
runtime_broker_id       TEXT → runtime_brokers(id) ON DELETE SET NULL
web_pty_enabled         INTEGER NOT NULL DEFAULT 0
task_summary            TEXT
message                 TEXT
applied_config          TEXT  -- JSON AgentAppliedConfig

-- Timestamps
created_at              TIMESTAMP NOT NULL
updated_at              TIMESTAMP NOT NULL
last_seen               TIMESTAMP          -- last heartbeat from agent
last_activity_event     TIMESTAMP          -- last activity change (stall detection)
deleted_at              TIMESTAMP          -- non-null = soft-deleted

-- Ownership / ACL
created_by              TEXT
owner_id                TEXT
visibility              TEXT NOT NULL DEFAULT 'private'
ancestry                TEXT  -- JSON []string: [root, ..., parent] — denormalized, immutable
state_version           INTEGER NOT NULL DEFAULT 1   -- optimistic lock

-- Indexes
idx_agents_project_slug   UNIQUE (agent_id, project_id)
idx_agents_project
idx_agents_phase
idx_agents_runtime_broker
idx_agents_deleted        WHERE deleted_at IS NOT NULL
```

### `runtime_brokers`

```sql
id                      TEXT PRIMARY KEY
name                    TEXT NOT NULL
slug                    TEXT NOT NULL UNIQUE
type                    TEXT NOT NULL
mode                    TEXT NOT NULL DEFAULT 'connected'
version                 TEXT
status                  TEXT NOT NULL DEFAULT 'offline'   -- online, offline, degraded
connection_state        TEXT DEFAULT 'disconnected'
last_heartbeat          TIMESTAMP
capabilities            TEXT  -- JSON BrokerCapabilities
supported_harnesses     TEXT  -- JSON (legacy)
resources               TEXT  -- JSON (legacy)
runtimes                TEXT  -- JSON (legacy)
profiles                TEXT  -- JSON []BrokerProfile (stored on the contributor row)
labels                  TEXT  -- JSON
annotations             TEXT  -- JSON
endpoint                TEXT
auto_provide            INTEGER NOT NULL DEFAULT 0
created_by              TEXT
created_at              TIMESTAMP
updated_at              TIMESTAMP

-- Indexes
idx_runtime_brokers_slug
idx_runtime_brokers_status
```

### `project_contributors` (join: projects ↔ runtime_brokers)

```sql
project_id              TEXT NOT NULL → projects(id) ON DELETE CASCADE
broker_id               TEXT NOT NULL → runtime_brokers(id) ON DELETE CASCADE
broker_name             TEXT NOT NULL
mode                    TEXT NOT NULL DEFAULT 'connected'
status                  TEXT NOT NULL DEFAULT 'offline'   -- online, offline
profiles                TEXT  -- JSON []BrokerProfile
local_path              TEXT  -- filesystem path to project root on broker
last_seen               TIMESTAMP
linked_by               TEXT  -- user ID who linked
linked_at               TIMESTAMP
PRIMARY KEY (project_id, broker_id)
```

### `templates`

```sql
id                      TEXT PRIMARY KEY
name                    TEXT NOT NULL
slug                    TEXT NOT NULL
display_name            TEXT
description             TEXT
harness                 TEXT NOT NULL      -- claude, gemini, opencode, codex, generic
image                   TEXT
config                  TEXT  -- JSON TemplateConfig
default_harness_config  TEXT
content_hash            TEXT
scope                   TEXT NOT NULL DEFAULT 'global'  -- global, project, user
scope_id                TEXT               -- projectId or userId (null for global)
project_id              TEXT → projects(id) ON DELETE CASCADE  -- deprecated alias for scope_id
storage_uri             TEXT
storage_bucket          TEXT
storage_path            TEXT
files                   TEXT  -- JSON []TemplateFile
base_template           TEXT  -- parent template ID
locked                  INTEGER NOT NULL DEFAULT 0
status                  TEXT NOT NULL DEFAULT 'active'   -- pending, active, archived
owner_id                TEXT
created_by              TEXT
updated_by              TEXT
visibility              TEXT NOT NULL DEFAULT 'private'
created_at              TIMESTAMP
updated_at              TIMESTAMP

-- Indexes
idx_templates_slug_scope    (slug, scope)
idx_templates_harness
idx_templates_status
idx_templates_content_hash
idx_templates_scope_id      (scope, scope_id)
```

### `harness_configs`

```sql
id                      TEXT PRIMARY KEY
name                    TEXT NOT NULL
slug                    TEXT NOT NULL
display_name            TEXT
description             TEXT
harness                 TEXT NOT NULL
config                  TEXT  -- JSON HarnessConfigData
content_hash            TEXT
scope                   TEXT NOT NULL DEFAULT 'global'
scope_id                TEXT
storage_uri / storage_bucket / storage_path  TEXT
files                   TEXT  -- JSON []TemplateFile
locked                  INTEGER NOT NULL DEFAULT 0
status                  TEXT NOT NULL DEFAULT 'active'
owner_id / created_by / updated_by  TEXT
visibility              TEXT NOT NULL DEFAULT 'private'
created_at / updated_at TIMESTAMP

-- Indexes
idx_harness_configs_slug_scope
idx_harness_configs_harness
idx_harness_configs_status
idx_harness_configs_content_hash
idx_harness_configs_scope_id
```

### `users`

```sql
id                      TEXT PRIMARY KEY
email                   TEXT UNIQUE NOT NULL
display_name            TEXT NOT NULL
avatar_url              TEXT
role                    TEXT NOT NULL DEFAULT 'member'   -- admin, member, viewer
status                  TEXT NOT NULL DEFAULT 'active'   -- active, suspended
preferences             TEXT  -- JSON UserPreferences
created_at              TIMESTAMP
last_login              TIMESTAMP
last_seen               TIMESTAMP

-- Indexes
idx_users_email
```

### `groups` (Ent-backed in CompositeStore)

```sql
id                      TEXT PRIMARY KEY
name                    TEXT NOT NULL
slug                    TEXT UNIQUE NOT NULL
description             TEXT
group_type              TEXT NOT NULL DEFAULT 'explicit'  -- explicit, project_agents
project_id              TEXT DEFAULT ''  -- FK for project_agents groups
parent_id               TEXT → groups(id) ON DELETE SET NULL
labels / annotations    TEXT  -- JSON
created_at / updated_at TIMESTAMP
created_by / owner_id   TEXT

-- Indexes
idx_groups_slug
idx_groups_parent
idx_groups_owner
idx_groups_project
```

### `group_members`

```sql
group_id                TEXT NOT NULL → groups(id) ON DELETE CASCADE
member_type             TEXT NOT NULL   -- user, group, agent
member_id               TEXT NOT NULL
role                    TEXT NOT NULL DEFAULT 'member'   -- member, admin, owner
added_at                TIMESTAMP
added_by                TEXT
PRIMARY KEY (group_id, member_type, member_id)

-- Indexes
idx_group_members_member  (member_type, member_id)
```

### `policies` (Ent-backed in CompositeStore)

```sql
id                      TEXT PRIMARY KEY
name                    TEXT NOT NULL
description             TEXT
scope_type              TEXT NOT NULL   -- hub, project, resource
scope_id                TEXT
resource_type           TEXT NOT NULL DEFAULT '*'
resource_id             TEXT
actions                 TEXT NOT NULL   -- JSON []string
effect                  TEXT NOT NULL   -- allow, deny
conditions              TEXT            -- JSON PolicyConditions
priority                INTEGER NOT NULL DEFAULT 0   -- higher = evaluated first
labels / annotations    TEXT  -- JSON
created_at / updated_at TIMESTAMP
created_by              TEXT

-- Indexes
idx_policies_scope    (scope_type, scope_id)
idx_policies_effect
idx_policies_priority DESC
```

### `policy_bindings`

```sql
policy_id               TEXT NOT NULL → policies(id) ON DELETE CASCADE
principal_type          TEXT NOT NULL   -- user, group
principal_id            TEXT NOT NULL
PRIMARY KEY (policy_id, principal_type, principal_id)

-- Indexes
idx_policy_bindings_principal  (principal_type, principal_id)
```

### `env_vars`

```sql
id                      TEXT PRIMARY KEY
key                     TEXT NOT NULL
value                   TEXT NOT NULL
scope                   TEXT NOT NULL   -- user, project, runtime_broker
scope_id                TEXT NOT NULL
description             TEXT
sensitive               INTEGER NOT NULL DEFAULT 0
injection_mode          TEXT NOT NULL DEFAULT 'as_needed'  -- always, as_needed
secret                  INTEGER NOT NULL DEFAULT 0
created_at / updated_at TIMESTAMP
created_by              TEXT

UNIQUE (key, scope, scope_id)
idx_env_vars_scope  (scope, scope_id)
```

### `secrets`

```sql
id                      TEXT PRIMARY KEY
key                     TEXT NOT NULL
encrypted_value         TEXT NOT NULL   -- never returned via API
secret_ref              TEXT            -- external reference (e.g. gcpsm:…)
secret_type             TEXT NOT NULL DEFAULT 'environment'  -- environment, variable, file, internal
target                  TEXT
scope                   TEXT NOT NULL   -- user, project, runtime_broker
scope_id                TEXT NOT NULL
description             TEXT
injection_mode          TEXT NOT NULL DEFAULT 'as_needed'
allow_progeny           INTEGER NOT NULL DEFAULT 0  -- user-scope only
version                 INTEGER NOT NULL DEFAULT 1
created_at / updated_at TIMESTAMP
created_by / updated_by TEXT

UNIQUE (key, scope, scope_id)
idx_secrets_scope  (scope, scope_id)
```

### `broker_secrets`

```sql
broker_id               TEXT PRIMARY KEY → runtime_brokers(id) ON DELETE CASCADE
secret_key              BLOB NOT NULL    -- raw HMAC key, never serialized
algorithm               TEXT NOT NULL DEFAULT 'hmac-sha256'
created_at              TIMESTAMP
rotated_at              TIMESTAMP
expires_at              TIMESTAMP
status                  TEXT NOT NULL DEFAULT 'active'  -- active, deprecated, revoked
```

### `broker_join_tokens`

```sql
broker_id               TEXT PRIMARY KEY → runtime_brokers(id) ON DELETE CASCADE
token_hash              TEXT NOT NULL UNIQUE
expires_at              TIMESTAMP NOT NULL
created_at              TIMESTAMP
created_by              TEXT NOT NULL

-- Indexes
idx_broker_join_tokens_hash
idx_broker_join_tokens_expires
```

### `user_access_tokens`

```sql
id                      TEXT PRIMARY KEY
user_id                 TEXT NOT NULL → users(id) ON DELETE CASCADE
name                    TEXT NOT NULL
prefix                  TEXT NOT NULL
key_hash                TEXT NOT NULL UNIQUE
project_id              TEXT NOT NULL → projects(id) ON DELETE CASCADE
scopes                  TEXT NOT NULL   -- JSON []string
revoked                 INTEGER NOT NULL DEFAULT 0
expires_at              TIMESTAMP
last_used               TIMESTAMP
created_at              TIMESTAMP NOT NULL

-- Indexes
idx_uat_user_id
idx_uat_key_hash
```

### `notification_subscriptions`

```sql
id                      TEXT PRIMARY KEY
scope                   TEXT NOT NULL DEFAULT 'agent'   -- agent, project
agent_id                TEXT  (nullable) → agents(id) ON DELETE CASCADE
subscriber_type         TEXT NOT NULL DEFAULT 'agent'   -- agent, user
subscriber_id           TEXT NOT NULL
project_id              TEXT NOT NULL
trigger_activities      TEXT NOT NULL   -- JSON []string
created_at              TIMESTAMP
created_by              TEXT NOT NULL

UNIQUE (scope, COALESCE(agent_id,''), subscriber_type, subscriber_id, project_id)
idx_notification_subs_agent
idx_notification_subs_project
idx_notification_subs_subscriber  (subscriber_type, subscriber_id)
```

### `notifications`

```sql
id                      TEXT PRIMARY KEY
subscription_id         TEXT NOT NULL → notification_subscriptions(id) ON DELETE CASCADE
agent_id                TEXT NOT NULL
project_id              TEXT NOT NULL
subscriber_type         TEXT NOT NULL
subscriber_id           TEXT NOT NULL
status                  TEXT NOT NULL   -- trigger activity in UPPER CASE
message                 TEXT NOT NULL
dispatched              INTEGER NOT NULL DEFAULT 0
acknowledged            INTEGER NOT NULL DEFAULT 0
created_at              TIMESTAMP

-- Indexes
idx_notifications_subscriber  (subscriber_type, subscriber_id)
idx_notifications_project
```

### `subscription_templates`

```sql
id                      TEXT PRIMARY KEY
name                    TEXT NOT NULL
scope                   TEXT NOT NULL DEFAULT 'project'
trigger_activities      TEXT NOT NULL   -- JSON []string
project_id              TEXT NOT NULL DEFAULT ''
created_by              TEXT NOT NULL
UNIQUE (project_id, name)
idx_sub_templates_project
```

### `scheduled_events`

```sql
id                      TEXT PRIMARY KEY
project_id              TEXT NOT NULL → projects(id) ON DELETE CASCADE
event_type              TEXT NOT NULL   -- message, status_update
fire_at                 TIMESTAMP NOT NULL
payload                 TEXT NOT NULL   -- JSON (handler-specific)
status                  TEXT NOT NULL DEFAULT 'pending'  -- pending, fired, cancelled, expired
schedule_id             TEXT DEFAULT ''  -- FK to schedules.id (for recurring-generated events)
created_at              TIMESTAMP
created_by              TEXT
fired_at                TIMESTAMP
error                   TEXT

-- Indexes
idx_scheduled_events_status
idx_scheduled_events_fire_at  WHERE status = 'pending'
idx_scheduled_events_project
```

### `schedules`

```sql
id                      TEXT PRIMARY KEY
project_id              TEXT NOT NULL → projects(id) ON DELETE CASCADE
name                    TEXT NOT NULL
cron_expr               TEXT NOT NULL   -- standard 5-field cron (UTC)
event_type              TEXT NOT NULL
payload                 TEXT NOT NULL DEFAULT '{}'
status                  TEXT NOT NULL DEFAULT 'active'  -- active, paused, deleted
next_run_at             TIMESTAMP
last_run_at             TIMESTAMP
last_run_status         TEXT            -- success, error
last_run_error          TEXT
run_count               INTEGER NOT NULL DEFAULT 0
error_count             INTEGER NOT NULL DEFAULT 0
created_at / updated_at TIMESTAMP
created_by              TEXT
UNIQUE (project_id, name)

-- Indexes
idx_schedules_project
idx_schedules_next_run  WHERE status = 'active'
```

### `messages`

```sql
id                      TEXT PRIMARY KEY
project_id              TEXT NOT NULL
sender                  TEXT NOT NULL   -- "user:alice" or "agent:slug"
sender_id               TEXT NOT NULL DEFAULT ''
recipient               TEXT NOT NULL
recipient_id            TEXT NOT NULL DEFAULT ''
msg                     TEXT NOT NULL
type                    TEXT NOT NULL DEFAULT 'instruction'  -- instruction, input-needed, state-change
urgent                  INTEGER NOT NULL DEFAULT 0
broadcasted             INTEGER NOT NULL DEFAULT 0
read                    INTEGER NOT NULL DEFAULT 0
agent_id                TEXT NOT NULL DEFAULT ''
group_id                TEXT NOT NULL DEFAULT ''   -- correlates set[] deliveries
created_at              TIMESTAMP

-- Indexes
idx_messages_project
idx_messages_recipient  (recipient_id, read)
idx_messages_agent
idx_messages_sender
idx_messages_created    (created_at DESC)
```

### `gcp_service_accounts`

```sql
id                      TEXT PRIMARY KEY
scope                   TEXT NOT NULL   -- hub, project, user
scope_id                TEXT NOT NULL
project_id              TEXT NOT NULL   -- GCP project (not Scion project)
email                   TEXT NOT NULL
display_name            TEXT NOT NULL DEFAULT ''
default_scopes          TEXT NOT NULL DEFAULT ''  -- comma-separated OAuth scopes
verified                INTEGER NOT NULL DEFAULT 0
verified_at             TIMESTAMP
verification_status     TEXT
verification_error      TEXT
created_by              TEXT NOT NULL DEFAULT ''
created_at              TIMESTAMP
managed                 INTEGER NOT NULL DEFAULT 0  -- true = hub-minted SA
managed_by              TEXT NOT NULL DEFAULT ''
UNIQUE (email, scope, scope_id)

-- Indexes
idx_gcp_sa_scope    (scope, scope_id)
idx_gcp_sa_project
```

### `github_installations`

```sql
installation_id         INTEGER PRIMARY KEY
account_login           TEXT NOT NULL
account_type            TEXT NOT NULL DEFAULT 'Organization'
app_id                  INTEGER NOT NULL
repositories            TEXT NOT NULL DEFAULT '[]'  -- JSON []string
status                  TEXT NOT NULL DEFAULT 'active'  -- active, suspended, deleted
created_at / updated_at TIMESTAMP

-- Indexes
idx_github_installations_account
idx_github_installations_status
```

### `maintenance_operations`

```sql
id                      TEXT PRIMARY KEY
key                     TEXT NOT NULL UNIQUE
title                   TEXT NOT NULL
description             TEXT NOT NULL DEFAULT ''
category                TEXT NOT NULL   -- migration, operation
status                  TEXT NOT NULL DEFAULT 'pending'  -- pending, running, completed, failed
created_at / started_at / completed_at  TIMESTAMP
started_by              TEXT
result                  TEXT
metadata                TEXT NOT NULL DEFAULT '{}'

-- Indexes
idx_maintenance_ops_category
idx_maintenance_ops_status
```

### `maintenance_operation_runs`

```sql
id                      TEXT PRIMARY KEY
operation_key           TEXT NOT NULL → maintenance_operations(key)
status                  TEXT NOT NULL DEFAULT 'running'  -- running, completed, failed
started_at              TIMESTAMP
completed_at            TIMESTAMP
started_by              TEXT
result                  TEXT
log                     TEXT NOT NULL DEFAULT ''

-- Indexes
idx_maintenance_runs_key
idx_maintenance_runs_started  DESC
```

### `project_sync_state`

```sql
project_id              TEXT NOT NULL → projects(id) ON DELETE CASCADE
broker_id               TEXT NOT NULL DEFAULT ''  -- '' for hub-native
last_sync_time          TIMESTAMP
last_commit_sha         TEXT
file_count              INTEGER NOT NULL DEFAULT 0
total_bytes             INTEGER NOT NULL DEFAULT 0
PRIMARY KEY (project_id, broker_id)

-- Indexes
idx_project_sync_state_project
```

### `allow_list`

```sql
id                      TEXT PRIMARY KEY
email                   TEXT NOT NULL UNIQUE COLLATE NOCASE
note                    TEXT NOT NULL DEFAULT ''
added_by                TEXT NOT NULL
invite_id               TEXT NOT NULL DEFAULT ''  -- FK to invite_codes.id
created                 DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP

-- Indexes
idx_allow_list_created_id  (created DESC, id DESC)  -- keyset pagination
```

### `invite_codes`

```sql
id                      TEXT PRIMARY KEY
code_hash               TEXT NOT NULL UNIQUE   -- SHA-256, never exposed
code_prefix             TEXT NOT NULL          -- first 8 chars for identification
max_uses                INTEGER NOT NULL DEFAULT 1
use_count               INTEGER NOT NULL DEFAULT 0
expires_at              DATETIME NOT NULL
revoked                 INTEGER NOT NULL DEFAULT 0
created_by              TEXT NOT NULL
note                    TEXT NOT NULL DEFAULT ''
created                 DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP

-- Indexes
idx_invite_codes_expires
```

### `schema_migrations`

```sql
version                 INTEGER PRIMARY KEY
applied_at              TIMESTAMP DEFAULT CURRENT_TIMESTAMP
```

---

## Entity-Relationship Summary

```
projects ─┬─< agents (project_id) ─── applied_config (JSON)
          ├─< project_contributors >─ runtime_brokers
          ├─< templates (scope_id)
          ├─< harness_configs (scope_id)
          ├─< env_vars (scope=project)
          ├─< secrets (scope=project)
          ├─< notification_subscriptions
          ├─< scheduled_events
          ├─< schedules ──< scheduled_events (schedule_id)
          ├─< messages
          ├─< user_access_tokens
          ├─< project_sync_state
          ├─< groups (group_type=project_agents)
          ├─< gcp_service_accounts (scope=project)
          └── github_installation_id → github_installations

users ─┬─< env_vars (scope=user)
       ├─< secrets (scope=user)
       ├─< user_access_tokens
       └─< group_members (member_type=user)

runtime_brokers ─┬─< project_contributors
                 ├── broker_secrets (1:1)
                 ├── broker_join_tokens (1:1)
                 ├─< env_vars (scope=runtime_broker)
                 └─< secrets (scope=runtime_broker)

groups ─┬─< group_members
        ├── parent_id → groups (self-referential hierarchy)
        └─< policy_bindings (principal_type=group)

policies ─┬─< policy_bindings
          └── conditions (JSON: labels, time, IP, delegation)

agents ─┬── ancestry (JSON: denormalized creator chain for transitive ACL)
        └─< notification_subscriptions (agent_id, when scope=agent)

notification_subscriptions ─< notifications

allow_list ── invite_id → invite_codes
```

---

## Store Interface Design

The `store.Store` interface composes 21 sub-interfaces, one per domain:

| Sub-interface | Key operations |
|---|---|
| `AgentStore` | CRUD, `ListAgents`, `UpdateAgentStatus`, `MarkStaleAgentsOffline`, `MarkStalledAgents`, `PurgeDeletedAgents` |
| `ProjectStore` | CRUD, `GetProjectBySlug`, `GetProjectsByGitRemote`, `NextAvailableSlug` |
| `RuntimeBrokerStore` | CRUD, `UpdateRuntimeBrokerHeartbeat` |
| `TemplateStore` | CRUD, `GetTemplateBySlug`, `DeleteTemplatesByScope` |
| `HarnessConfigStore` | CRUD, `GetHarnessConfigBySlug`, `DeleteHarnessConfigsByScope` |
| `UserStore` | CRUD, `GetUserByEmail`, `UpdateUserLastSeen` |
| `ProjectProviderStore` | Add/Remove/Get/Update provider links |
| `EnvVarStore` | CRUD, `UpsertEnvVar`, `DeleteEnvVarsByScope` |
| `SecretStore` | CRUD, `UpsertSecret`, `GetSecretValue`, `ListProgenySecrets` |
| `GroupStore` | CRUD, members, cycles, effective groups, delegated access |
| `PolicyStore` | CRUD, bindings, `GetPoliciesForPrincipals` |
| `UserAccessTokenStore` | CRUD, `UpdateUserAccessTokenLastUsed`, `CountUserAccessTokens` |
| `BrokerSecretStore` | Secrets + join tokens for broker HMAC auth |
| `NotificationStore` | Subscriptions, notifications, subscription templates |
| `ScheduledEventStore` | One-shot timers: create/cancel/list-pending/purge |
| `ScheduleStore` | Recurring cron schedules: CRUD, `ListDueSchedules`, `UpdateScheduleAfterRun` |
| `GCPServiceAccountStore` | CRUD, `CountGCPServiceAccounts` |
| `GitHubInstallationStore` | CRUD, `GetInstallationForRepository` |
| `MessageStore` | CRUD, `MarkAllMessagesRead`, `PurgeOldMessages` |
| `MaintenanceStore` | Operations + runs, `AbortRunningMaintenanceOps` |
| `ProjectSyncStateStore` | Upsert/get/list/delete sync metadata |
| `AllowListStore` | Add/remove/list entries, bulk add, domain listing |
| `InviteCodeStore` | CRUD, `IncrementInviteUseCount`, `RevokeInviteCode`, `GetInviteStats` |

### Sentinel errors

```go
ErrNotFound        // 404 semantics
ErrAlreadyExists   // 409 semantics
ErrVersionConflict // 409 optimistic lock (agents only)
ErrInvalidInput    // 400 semantics
```

---

## Implementation Patterns

### JSON columns
Struct fields that are maps, slices, or nested structs are stored as JSON text. On read, `unmarshalJSON` is a no-op for empty strings, which avoids null-handling noise:
```go
func unmarshalJSON[T any](data string, v *T) {
    if data == "" { return }
    json.Unmarshal([]byte(data), v)
}
```

### Nullable helpers
SQLite requires explicit `sql.NullString` / `sql.NullTime` for nullable columns. Three helpers centralise this:
```go
nullableString(s string) sql.NullString  // "" → NULL (guards FK/UNIQUE)
nullableTime(t time.Time) sql.NullTime   // zero → NULL
nullableInt64(v *int64) sql.NullInt64    // nil → NULL
```

### Optimistic locking (`agents` only)
`UpdateAgent` appends `AND state_version = ?` to the WHERE clause and increments on success. Zero `RowsAffected` → existence check → `ErrNotFound` or `ErrVersionConflict`.

### Soft delete (`agents` only)
`deleted_at IS NOT NULL` marks soft-deleted agents. `AgentFilter.IncludeDeleted` controls visibility. `PurgeDeletedAgents(cutoff)` hard-deletes them after retention.

### Stale/stalled agent detection
Two bulk-update methods work at the store level for efficiency:
- `MarkStaleAgentsOffline(threshold)` — sets `activity='offline'` for running agents whose `last_seen < threshold` (not already terminal)
- `MarkStalledAgents(activityThreshold, heartbeatRecency)` — sets `stalled_from_activity = activity` for agents with recent heartbeats but no activity change

### Migration runner
Migrations are a `[]any` slice (1-indexed by position). Each entry is either:
- `string` — plain SQL, run in a transaction
- `func(ctx, tx) error` — programmatic migration (V50)

V40 is in `foreignKeysOffMigrations` — it pins a single connection, disables FK enforcement, runs the migration, then restores FK enforcement.

### In-memory test isolation
Each `:memory:` call gets a unique `memdbN` URI via an atomic counter, so parallel test suites don't share state.
