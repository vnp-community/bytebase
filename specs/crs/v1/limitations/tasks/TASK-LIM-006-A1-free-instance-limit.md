# TASK-LIM-006-A1: FREE Instance Limit + Smart Counting

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-006 |
| Phase | A — Quick Wins |
| Priority | P0 |
| Depends On | — |
| Est. | S (~60 LoC) |

## Objective

Tăng FREE plan instance limit từ 10 → 25. Đếm chỉ ACTIVE instances (loại ARCHIVED/DELETED khỏi quota).

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/enterprise/plan.yaml` — maxInstances: 25 |
| MODIFY | `backend/enterprise/license.go` — GetInstanceLimit |
| CREATE | `backend/store/instance_count.go` — CountActiveInstances |
| MODIFY | `backend/api/v1/instance_service.go` — use active count |

## Specification

### plan.yaml

```yaml
FREE:
  maxInstances: 25  # was 10
```

### Store helper

```go
func (s *Store) CountActiveInstances(ctx context.Context) (int, error) {
    var count int
    err := s.GetDB().QueryRowContext(ctx,
        "SELECT COUNT(*) FROM instance WHERE deleted = false AND archived = false",
    ).Scan(&count)
    return count, err
}
```

### Instance service quota check

Replace existing `CountInstances()` with `CountActiveInstances()` in `CreateInstance`.
Error message: `"instance limit reached (%d/%d active). Archive unused instances or upgrade."`

## Acceptance Criteria

- [ ] FREE plan allows up to 25 instances
- [ ] Only ACTIVE instances count toward quota
- [ ] Archived instances don't block new creation
- [ ] Error message guides user to archive or upgrade
