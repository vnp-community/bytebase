# ACL Security Contract

> This document defines the ACL (Access Control List) security contract for all gRPC/ConnectRPC methods in the Bytebase API.

## Two-Level Permission Model

Bytebase uses a **workspace + project** permission model:

1. **Workspace-level**: Role-based (`OWNER`, `DBA`, `DEVELOPER`). Controls instance, environment, and global settings.
2. **Project-level**: IAM policy-based (`roles/projectOwner`, `roles/projectDeveloper`, `roles/projectViewer`). Controls project resources (databases, issues, plans, rollouts).

## How ACL Works

```
Request → ACLInterceptor → lookupExtractor(method) → extract resource names
                                                   → resolve to project/workspace
                                                   → check IAM policy
                                                   → allow or deny
```

### Static Extractor Map (`acl_extractors.go`)

Every RPC method has a static entry in `aclResourceExtractors`:

| Extraction Pattern | Function | Example |
|---|---|---|
| No resource needed | `extractNone` | `Login`, `ListUsers` |
| From `name` field | `extractFromName` | `GetProject`, `GetDatabase` |
| From `parent` field | `extractFromParent` | `ListIssues`, `ListPlans` |
| From `resource` field | `extractFromResource` | `GetIamPolicy`, `SetIamPolicy` |
| From `project` field | `extractFromProject` | `AddWebhook`, `TestWebhook` |
| Custom extractor | typed function | `UpdateDatabase` (project transfer check) |

### Fail-Closed Behavior

If a method is NOT in the extractor map:
1. A **warning log** is emitted (`slog.Warn`)
2. The request falls back to **workspace-level** permission check
3. This prevents unauthorized access probing

## Adding a New RPC Method

When adding a new gRPC method, you MUST:

1. **Add an entry** to `aclResourceExtractors` in `acl_extractors.go`
2. **Choose the correct extractor** based on your request message structure
3. **Run the coverage test**: `go test -run TestACLExtractorMap_Exhaustive`
4. **Add the method name** to the `knownMethods` list in `acl_extractors_test.go`

### Quick Decision Tree

```
Does the request have a `name` field (resource identifier)?
  → YES: Use `extractFromName`
Does the request have a `parent` field (listing)?
  → YES: Use `extractFromParent`
Is it a public/workspace-level endpoint?
  → YES: Use `extractNone`
Does it need custom logic (e.g., checking both old and new project)?
  → YES: Write a custom extractor
```

## Batch Methods

Batch methods (e.g., `BatchRunTasks`, `BatchSkipTasks`) are handled by the `lookupExtractor` function which strips the `Batch` prefix and `s` suffix to find the singular form.

Example: `BatchRunTasks` → strips to `RunTask` → looks up `RunTask` in the map.

## Coverage Guarantee

The `TestACLExtractorMap_Exhaustive` test in `acl_extractors_test.go` ensures every known RPC method has a corresponding extractor. **If you add a method without an extractor, CI will fail.**
