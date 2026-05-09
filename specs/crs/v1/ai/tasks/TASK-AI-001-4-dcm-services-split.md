# TASK-AI-001-4: DCM Workflow Services Split

| Field | Value |
|-------|-------|
| Solution | SOL-AI-001 |
| Priority | P1 |
| Depends On | — |
| Status | ✅ DONE |
| Completed | 2025-05-10 |
| Est. | M (3 services × 2 files each) |

## Objective

Split 3 DCM workflow services into 2 files each. Zero functional change.

## Delivered

| File | Lines | Content |
|------|-------|---------|
| `plan_service.go` | 539 | Struct + CRUD (Get/List/Create/Update/PlanCheck) |
| `plan_service_spec.go` | 735 | validateSpecs + converters + plan check builders |
| `issue_service.go` | 547 | Struct + CRUD (Get/List/Search/Create) + filter parser |
| `issue_service_lifecycle.go` | 713 | ApproveIssue, RejectIssue, RequestIssue, UpdateIssue, BatchUpdate, Comments |
| `project_service.go` | 531 | Struct + CRUD (Get/List/Create/Update/Delete/Undelete/Batch) |
| `project_service_iam.go` | 762 | GetIamPolicy, SetIamPolicy, Add/Update/Remove/TestWebhook, IAM validation |

## Verification

```bash
go build ./backend/api/v1/...  # ✅ PASS
go vet ./backend/api/v1/...    # ✅ PASS
```

## Acceptance Criteria

- [x] Each original file reduced to ≤550 lines
- [x] Each new file ≤800 lines
- [x] `go build` passes
- [x] `go vet` passes
