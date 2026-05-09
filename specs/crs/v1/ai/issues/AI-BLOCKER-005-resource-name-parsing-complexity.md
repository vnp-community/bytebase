# AI-BLOCKER-005: Resource Name Parsing Complexity in `common/resource_name.go`

| Field | Value |
|-------|-------|
| **ID** | AI-BLOCKER-005 |
| **Severity** | 🟡 Medium |
| **Category** | API Conventions / Naming Complexity |
| **Layer** | L6 Common (`backend/common/`) |
| **Status** | Open |
| **Created** | 2026-05-09 |

## Problem

`common/resource_name.go` (736 LOC) contains 50+ functions for parsing and formatting AIP-compliant resource names (e.g., `projects/{project}/plans/{plan}/rollout/stages/{stage}/tasks/{task}/taskRuns/{taskRun}`). The naming conventions are deeply nested and inconsistent — some use UID (int64), some use string IDs, and some use "Maybe" optional patterns. AI agents generating API calls must understand the exact parse function name for each resource depth.

## Impact on AI Operations

- **Function Name Ambiguity**: There are 7 variants of `GetProjectIDPlanID*` with subtly different signatures:
  - `GetProjectIDPlanID(name) → (string, int64, error)`
  - `GetProjectIDPlanIDFromRolloutName(name) → (string, int64, error)`
  - `GetProjectIDPlanIDStageIDTaskID(name) → (string, int64, string, int64, error)`
  - `GetProjectIDPlanIDStageIDTaskIDTaskRunID(name) → (string, int64, string, int64, int64, error)`
  - `GetProjectIDPlanIDMaybeStageID(name) → (string, int64, *string, error)`
  - `GetProjectIDPlanIDStageIDMaybeTaskID(name) → (string, int64, string, *int64, error)`
  - `GetProjectIDPlanIDMaybeStageIDMaybeTaskID(name) → (string, int64, *string, *int64, error)`

- **Return Type Inconsistency**: Some functions return `string` IDs, others return `int64` UIDs, with no naming convention to distinguish them.

- **AI Cannot Infer Correct Parser**: Given a resource name `projects/foo/plans/123/rollout/stages/prod/tasks/456`, an AI must choose between 7 possible parse functions.

## Evidence

```go
// 7 variants of the same base parser — which one should AI use?
func GetProjectIDPlanID(name string) (string, int64, error)
func GetProjectIDPlanIDFromRolloutName(name string) (string, int64, error)
func GetProjectIDPlanIDFromPlanCheckRun(name string) (string, int64, error)
func GetProjectIDPlanIDMaybeStageID(name string) (string, int64, *string, error)
func GetProjectIDPlanIDStageIDMaybeTaskID(name string) (string, int64, string, *int64, error)
func GetProjectIDPlanIDMaybeStageIDMaybeTaskID(name string) (string, int64, *string, *int64, error)
func GetProjectIDPlanIDStageIDTaskID(name string) (string, int64, string, int64, error)
func GetProjectIDPlanIDStageIDTaskIDTaskRunID(name string) (string, int64, string, int64, int64, error)
```

## Recommended Remediation

1. **Typed Resource Name Structs**: Replace multi-return-value functions with typed structs:
   ```go
   type RolloutTaskRef struct {
       ProjectID string
       PlanUID   int64
       StageID   string   // environment ID
       TaskUID   *int64   // nil if wildcard
   }
   
   func ParseRolloutTaskRef(name string) (*RolloutTaskRef, error)
   ```

2. **Unified Parser**: Create a single `ParseResourceName(name string) (*ResourceRef, error)` that returns a discriminated union, reducing the AI's decision space from 50+ functions to 1.

3. **Naming Convention Doc**: Create `backend/common/RESOURCE_NAMES.md` with examples for AI reference.

## Files to Modify

```
backend/common/resource_name.go → refactor to typed structs
NEW: backend/common/RESOURCE_NAMES.md
```
