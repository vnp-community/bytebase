# TASK-LIM-006-A2: TEAM Audit Log Retention 90 Days

| Field | Value |
|-------|-------|
| Solution | SOL-LIM-006 |
| Phase | A — Quick Wins |
| Priority | P0 |
| Depends On | — |
| Est. | S (~30 LoC) |

## Objective

Tăng TEAM audit log retention từ 7 → 90 ngày cho compliance (ISO 27001, SBV Circular 09).

## Files

| Action | Path |
|--------|------|
| MODIFY | `backend/runner/cleaner/data_cleaner.go` — audit retention config |

## Specification

```go
func (c *DataCleaner) getAuditLogRetention(ctx context.Context) time.Duration {
    plan := c.licenseService.GetCurrentPlan(ctx)
    switch plan {
    case api.FREE:      return 0           // No audit access
    case api.TEAM:      return 90 * 24 * time.Hour  // was 7 days
    case api.ENTERPRISE: return 0          // Unlimited
    }
    return 7 * 24 * time.Hour
}
```

Update `cleanAuditLogs()` to use this configurable retention.

## Acceptance Criteria

- [ ] TEAM plan retains 90 days of audit logs
- [ ] FREE plan: no audit log access (unchanged)
- [ ] ENTERPRISE: unlimited retention (unchanged)
- [ ] DataCleaner correctly uses new retention period
