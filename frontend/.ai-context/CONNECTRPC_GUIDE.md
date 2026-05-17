# ConnectRPC Usage Guide

> **All API calls use ConnectRPC (gRPC-Web).** Never use `fetch()` for `/v1/` endpoints.

---

## Basic Pattern

```typescript
import { databaseServiceClientConnect } from "@/connect";

// GET single entity
const database = await databaseServiceClientConnect.getDatabase({
  name: "instances/prod/databases/mydb",
});

// LIST with pagination
const { databases, nextPageToken } = await databaseServiceClientConnect.listDatabases({
  parent: "instances/prod",
  pageSize: 100,
  filter: 'environment == "environments/prod"',
});

// UPDATE — updateMask is REQUIRED
await databaseServiceClientConnect.updateDatabase({
  database: { ...existing, labels: newLabels },
  updateMask: ["labels"],  // ← ONLY the changed fields
});

// CREATE
const newDb = await databaseServiceClientConnect.createDatabase({
  parent: "instances/prod",
  database: create(DatabaseSchema, { title: "My DB" }),
  databaseId: "mydb",
});

// DELETE
await databaseServiceClientConnect.deleteDatabase({
  name: "instances/prod/databases/mydb",
});
```

---

## Service Client Lookup Table

All clients are exported from `src/connect/index.ts`.

```typescript
import { <clientName> } from "@/connect";
```

| Domain | Client Name | Common Methods |
|---|---|---|
| **Database** | `databaseServiceClientConnect` | getDatabase, listDatabases, updateDatabase, searchDatabases |
| **Database Catalog** | `databaseCatalogServiceClientConnect` | getDatabaseCatalog, updateDatabaseCatalog |
| **Database Group** | `databaseGroupServiceClientConnect` | getDatabaseGroup, listDatabaseGroups, createDatabaseGroup |
| **Project** | `projectServiceClientConnect` | getProject, listProjects, updateProject, getIamPolicy, setIamPolicy |
| **Instance** | `instanceServiceClientConnect` | getInstance, listInstances, createInstance, updateInstance, deleteInstance |
| **Instance Role** | `instanceRoleServiceClientConnect` | listInstanceRoles |
| **Issue** | `issueServiceClientConnect` | getIssue, listIssues, createIssue, updateIssue, approveIssue, rejectIssue |
| **Plan** | `planServiceClientConnect` | getPlan, listPlans, createPlan, updatePlan |
| **Rollout** | `rolloutServiceClientConnect` | getRollout, listRollouts, createRollout, listStages, listTasks, runTasks |
| **User** | `userServiceClientConnect` | getUser, listUsers, createUser, updateUser, deleteUser |
| **Service Account** | `serviceAccountServiceClientConnect` | listServiceAccounts, createServiceAccount, deleteServiceAccount |
| **Workload Identity** | `workloadIdentityServiceClientConnect` | listWorkloadIdentities, createWorkloadIdentity |
| **Auth** | `authServiceClientConnect` | login, logout, refresh, createUser (signup) |
| **Setting** | `settingServiceClientConnect` | getSetting, updateSetting, listSettings |
| **SQL** | `sqlServiceClientConnect` | query, export, searchQueryHistories |
| **Sheet** | `sheetServiceClientConnect` | getSheet, createSheet, updateSheet |
| **Worksheet** | `worksheetServiceClientConnect` | getWorksheet, listWorksheets, createWorksheet, updateWorksheet |
| **Subscription** | `subscriptionServiceClientConnect` | getSubscription, updateSubscription |
| **Role** | `roleServiceClientConnect` | listRoles, createRole, updateRole, deleteRole |
| **Group** | `groupServiceClientConnect` | getGroup, listGroups, createGroup, updateGroup, deleteGroup |
| **Policy** | `orgPolicyServiceClientConnect` | getPolicy, listPolicies, createPolicy, updatePolicy, deletePolicy |
| **Review Config** | `reviewConfigServiceClientConnect` | getReviewConfig, updateReviewConfig, listReviewConfigs |
| **IDP** | `identityProviderServiceClientConnect` | getIdentityProvider, listIdentityProviders, createIdentityProvider, updateIdentityProvider, deleteIdentityProvider, testIdentityProvider |
| **Audit Log** | `auditLogServiceClientConnect` | searchAuditLogs, exportAuditLogs |
| **Access Grant** | `accessGrantServiceClientConnect` | listAccessGrants, createAccessGrant |
| **Revision** | `revisionServiceClientConnect` | listRevisions, getRevision |
| **Release** | `releaseServiceClientConnect` | listReleases, getRelease, createRelease |
| **CEL** | `celServiceClientConnect` | batchParseExpressions, batchDecomposeExpressions |
| **AI** | `aiServiceClientConnect` | chat |
| **Actuator** | `actuatorServiceClientConnect` | getActuatorInfo, updateActuatorInfo |
| **Workspace** | `workspaceServiceClientConnect` | getWorkspace |

---

## Proto-ES Type Construction

```typescript
// ✅ CORRECT: Use create() with Schema
import { create } from "@bufbuild/protobuf";
import { DatabaseSchema } from "@/types/proto-es/v1/database_service_pb";

const db = create(DatabaseSchema, {
  title: "My Database",
  labels: { env: "prod" },
});

// ❌ WRONG: new Constructor()
const db = new Database({ title: "My Database" });

// ❌ WRONG: plain object (missing protobuf metadata)
const db = { title: "My Database" };
```

---

## Anti-Patterns — NEVER Do These

```typescript
// ❌ NEVER: raw fetch for gRPC endpoints
const res = await fetch("/v1/databases/mydb");

// ❌ NEVER: update without updateMask
await databaseServiceClientConnect.updateDatabase({
  database: modifiedDatabase,
  // MISSING updateMask → server may ignore changes or overwrite all fields
});

// ❌ NEVER: wrong client for domain
await projectServiceClientConnect.getDatabase(...)  // Database ≠ Project

// ❌ NEVER: construct resource name manually with wrong format
const name = `databases/${id}`;  // WRONG — should be instances/{instance}/databases/{db}
```

---

## Resource Name Formats

| Resource | Format | Example |
|---|---|---|
| Database | `instances/{instance}/databases/{db}` | `instances/prod/databases/mydb` |
| Project | `projects/{project}` | `projects/my-project` |
| Instance | `instances/{instance}` | `instances/prod` |
| Issue | `projects/{project}/issues/{issue}` | `projects/hr/issues/101` |
| Plan | `projects/{project}/plans/{plan}` | `projects/hr/plans/42` |
| Rollout | `projects/{project}/rollouts/{rollout}` | `projects/hr/rollouts/42` |
| User | `users/{email}` | `users/admin@example.com` |
| Setting | `settings/{setting}` | `settings/bb.workspace.profile` |
| Environment | `environments/{env}` | `environments/prod` |
| Role | `roles/{role}` | `roles/projectDeveloper` |
| Group | `groups/{email}` | `groups/dba-team@example.com` |
| Policy | `{parent}/policies/{policy}` | `projects/hr/policies/masking` |
