# TASK-WEAK-007-1: Store Interface Extraction

| Field | Value |
|-------|-------|
| Solution | SOL-WEAK-007 |
| Priority | P0 |
| Depends On | — |
| Est. | M (~120 LoC) |

## Objective

Extract role-based interfaces from concrete `Store` struct to enable unit testing without database. Foundation for all service-level testing.

## Files

| Action | Path |
|--------|------|
| CREATE | `backend/store/interfaces.go` |

## Specification

Define role-based interfaces (Reader/Writer per entity):

```go
type UserReader interface {
    GetUser(ctx context.Context, find *FindUserMessage) (*UserMessage, error)
    GetUserByEmail(ctx context.Context, workspace, email string) (*UserMessage, error)
    ListUsers(ctx context.Context, find *FindUserMessage) ([]*UserMessage, error)
}

type UserWriter interface {
    CreateUser(ctx context.Context, create *UserMessage) (*UserMessage, error)
    UpdateUser(ctx context.Context, id int, patch *UpdateUserMessage) (*UserMessage, error)
}

type PlanReader interface { /* GetPlan, ListPlans */ }
type IssueReader interface { /* GetIssue, ListIssues */ }
type ChangelogWriter interface { /* CreateChangelog, UpdateChangelog */ }
type PolicyReader interface { /* GetPolicy, ListPolicies */ }
```

Compile-time verification:
```go
var _ UserReader = (*Store)(nil)
var _ UserWriter = (*Store)(nil)
// ...etc
```

**Design**: Small role interfaces > one massive StoreInterface. Services declare only what they need.

## Acceptance Criteria

- [ ] 6+ role interfaces defined
- [ ] Compile-time `var _` assertions pass
- [ ] Existing code unaffected (additive file)
- [ ] `go build ./backend/store/...` passes
