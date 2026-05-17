# T-006-01: Region Configuration

| Field | Value |
|---|---|
| **Task ID** | T-006-01 |
| **Solution** | SOL-AVAIL-006 |
| **Priority** | P2 |
| **Depends On** | Phase 1 + Phase 2 complete |
| **Target File** | `backend/component/config/profile.go` (Modify) |

---

## Objective

Thêm `RegionRole`, `RegionName`, `PrimaryRegionURL` vào Profile. Helper methods `IsStandby()`, `IsPrimary()`.

## Implementation

```go
type RegionRole string
const (
    RegionRolePrimary RegionRole = "PRIMARY"
    RegionRoleStandby RegionRole = "STANDBY"
    RegionRoleDR      RegionRole = "DR"
)

// Add to Profile struct:
RegionName       string     // env: REGION_NAME
RegionRole       RegionRole // env: REGION_ROLE
PrimaryRegionURL string     // env: PRIMARY_REGION_URL

func (p *Profile) IsStandby() bool {
    return p.RegionRole == RegionRoleStandby
}

func (p *Profile) IsPrimary() bool {
    return p.RegionRole == "" || p.RegionRole == RegionRolePrimary
}
```

Parse from env in profile initialization (existing pattern).

## Acceptance Criteria

- [x] 3 new Profile fields
- [x] `RegionRole` type with 3 constants
- [x] `IsStandby()` / `IsPrimary()` helpers
- [x] Default (empty) → PRIMARY behavior
- [x] `go build ./backend/component/config/...` passes
