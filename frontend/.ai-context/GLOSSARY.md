# Bytebase Domain Glossary

> Reference for all domain-specific objects, relationships, and conventions.

---

## Core Workflow Objects

### Plan
- **What**: A database change request containing one or more SQL scripts (Specs).
- **Resource name**: `projects/{project}/plans/{plan}`
- **Lifecycle**: `OPEN` → `DONE`
- **Relations**: 1 Plan → 1 Issue (auto-created), 1 Plan contains N Specs
- **Key fields**: name, title, description, creator, specs[]
- **API client**: `planServiceClientConnect`

### Issue
- **What**: Workflow tracking wrapper — approvals, comments, status. Created automatically when a Plan is created.
- **Resource name**: `projects/{project}/issues/{issue}`
- **Lifecycle**: `OPEN` → `DONE` | `CANCELED`
- **Relations**: 1 Issue ← 1 Plan, 1 Issue → 1 Rollout
- **Key fields**: name, title, status, type, creator, assignee, approvalStatus
- **API client**: `issueServiceClientConnect`

### Rollout
- **What**: Execution plan for a set of database changes, organized by environment stages.
- **Resource name**: `projects/{project}/rollouts/{rollout}`
- **Relations**: 1 Rollout ← 1 Issue, 1 Rollout contains N Stages
- **Key fields**: name, plan, stages[]
- **API client**: `rolloutServiceClientConnect`

### Stage
- **What**: One environment stage within a Rollout (e.g., "Dev", "Staging", "Prod").
- **Belongs to**: Rollout
- **Relations**: 1 Stage contains N Tasks
- **Key fields**: name, environment, tasks[]

### Task
- **What**: A single database operation within a Stage (e.g., run DDL on `prod-db-1`).
- **Types**: `SCHEMA_UPDATE` (DDL), `DATA_UPDATE` (DML), `DATABASE_CREATE`, `DATABASE_RESTORE`
- **Relations**: 1 Task → N TaskRuns (execution history)
- **Key fields**: name, type, status, database, statement

### TaskRun
- **What**: A single execution attempt of a Task.
- **Lifecycle**: `PENDING` → `RUNNING` → `DONE` | `FAILED` | `CANCELED`
- **Key fields**: name, status, result, executionLog, startTime, endTime

### Release
- **What**: A versioned package of change files for deployment pipelines.
- **Resource name**: `projects/{project}/releases/{release}`
- **Differs from Plan/Issue**: No approval workflow — just a versioned snapshot.
- **API client**: `releaseServiceClientConnect`

### Sheet
- **What**: A SQL script file. Can be anonymous (temporary) or named (persisted).
- **Resource name**: `projects/{project}/sheets/{sheet}`
- **Types**: `SHEET_TYPE_SQL`, `SHEET_TYPE_SCHEMA_DESIGN`
- **API client**: `sheetServiceClientConnect`

### Worksheet
- **What**: A named, persisted Sheet in SQL Editor. Has folder organization and sharing.
- **Differs from Sheet**: Worksheet = persisted + named + folder-organized; Sheet = raw SQL content.
- **API client**: `worksheetServiceClientConnect`

---

## Object Hierarchy

```
Workspace
  ├── Environment[] (Dev, Staging, Prod — ordered by pipeline position)
  ├── Instance[] (database server connections)
  │   └── Database[] (managed databases on that instance)
  │       ├── Schema (tables, columns, indexes, views)
  │       └── ChangeHistory[] (DDL/DML audit trail)
  ├── Project[] (logical grouping of databases + workflows)
  │   ├── Database[] (databases assigned to this project)
  │   ├── Plan[] (change requests)
  │   │   └── Spec[] (individual SQL scripts within the plan)
  │   ├── Issue[] (workflow tracking for plans)
  │   │   └── Rollout (execution plan)
  │   │       └── Stage[] (per environment)
  │   │           └── Task[] (per database)
  │   │               └── TaskRun[] (execution history)
  │   ├── Release[] (versioned change packages)
  │   ├── DatabaseGroup[] (dynamic sets defined by CEL expressions)
  │   ├── Branch[] (schema versioning branches)
  │   └── Members/IAM (project-level RBAC)
  ├── Setting[] (workspace configuration)
  ├── SQLReviewConfig[] (review rule sets)
  ├── User[] / Group[] / Role[]
  └── Subscription (plan tier: FREE / TEAM / ENTERPRISE)
```

---

## IAM Model

### Workspace Roles

| Role | Description | Permissions |
|---|---|---|
| `OWNER` | Full admin | All `bb.*` permissions |
| `DBA` | Database administrator | Database + Instance operations |
| `DEVELOPER` | Limited access | View + limited create |

### Project Roles (override workspace for project scope)

| Role | Description |
|---|---|
| `PROJECT_OWNER` | Full project admin |
| `PROJECT_DEVELOPER` | Create plans/issues, query databases |
| `PROJECT_VIEWER` | Read-only access |
| Custom roles | Defined via `roleServiceClientConnect` |

### Permission Scopes

```typescript
// Workspace-level permissions
"bb.users.list", "bb.users.create", "bb.users.update", "bb.users.delete"
"bb.instances.list", "bb.instances.create", "bb.instances.update"
"bb.settings.get", "bb.settings.set"
"bb.roles.list", "bb.roles.create"

// Project-level permissions
"bb.databases.list", "bb.databases.get", "bb.databases.update"
"bb.issues.list", "bb.issues.create", "bb.issues.update"
"bb.plans.list", "bb.plans.create"
"bb.projects.getIamPolicy", "bb.projects.setIamPolicy"

// Check patterns in code:
useWorkspacePermission("bb.users.list")     // → boolean
useProjectPermission(project, "bb.databases.list")  // → boolean
```

---

## SQL Review Rules

```
Categories: NAMING, STATEMENT, TABLE, COLUMN, INDEX, SCHEMA, DATABASE, SYSTEM
Severity: ERROR (blocks pipeline), WARNING (warns only), DISABLED
Engines: MYSQL, POSTGRESQL, TIDB, ORACLE, MSSQL, SNOWFLAKE, OCEANBASE, etc.

Rule format: {category}.{subcategory}.{rule-name}
Examples:
  naming.column.no-keyword
  statement.select.no-select-star
  table.require-pk
  column.no-null-default

Source: src/types/sql-review-schema.yaml (~46KB)
```

---

## CEL Expressions

```
Used in: Database Groups (dynamic set matching), Data Masking conditions
Syntax: Google Common Expression Language (CEL)

Examples:
  resource.database == "my-db"
  resource.environment == "environments/prod"
  resource.labels["app"] == "backend" && resource.labels["tier"] == "critical"

Parser: src/plugins/cel/
Validator: celServiceClientConnect.batchParseExpressions()
```

---

## Database Engines

| Engine | Enum Value | Notes |
|---|---|---|
| MySQL | `Engine.MYSQL` | Most common |
| PostgreSQL | `Engine.POSTGRES` | |
| TiDB | `Engine.TIDB` | MySQL-compatible |
| Oracle | `Engine.ORACLE` | Enterprise |
| SQL Server | `Engine.MSSQL` | |
| MongoDB | `Engine.MONGODB` | NoSQL |
| Redis | `Engine.REDIS` | |
| ClickHouse | `Engine.CLICKHOUSE` | |
| Snowflake | `Engine.SNOWFLAKE` | |
| Spanner | `Engine.SPANNER` | Google Cloud |
| MariaDB | `Engine.MARIADB` | MySQL fork |
| OceanBase | `Engine.OCEANBASE` | |
| DM | `Engine.DM` | Dameng |
| RisingWave | `Engine.RISINGWAVE` | |
| Hive | `Engine.HIVE` | |

---

## Data Masking

```
Masking Levels: NONE → PARTIAL → FULL
Column classification: semantic types + classification labels
Exemption: per-user + per-project access grants (accessGrantServiceClientConnect)
CEL condition: scopes masking rules to specific databases/tables
Policy: orgPolicyServiceClientConnect → MASKING type policies
```

---

## Commonly Confused Terms

| ❌ Wrong | ✅ Correct |
|---|---|
| "Plan" and "Issue" are the same | Plan = what to change; Issue = workflow to track/approve |
| "Task" and "TaskRun" are the same | Task = definition; TaskRun = one execution attempt |
| "Sheet" and "Worksheet" are the same | Sheet = raw SQL content; Worksheet = named + persisted + organized |
| "Environment" = database | Environment = deployment stage (Dev/Staging/Prod) that groups databases |
