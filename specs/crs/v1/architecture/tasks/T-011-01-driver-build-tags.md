# T-011-01: Build Tags per Driver

| Field | Value |
|---|---|
| **Task ID** | T-011-01 |
| **Solution** | SOL-ARCH-011 |
| **Priority** | P3 |
| **Depends On** | None |
| **Target Files** | `backend/plugin/db/*/driver.go` (23 files) |
| **Type** | Modify existing |

---

## Objective

Add `//go:build` constraints to each DB driver file, enabling selective compilation. Default build includes all drivers via `plugin_all`.

## Implementation

Add to top of each driver file:

```go
// backend/plugin/db/pg/driver.go
//go:build plugin_all || plugin_pg

package pg
// ... existing init() and implementation unchanged
```

```go
// backend/plugin/db/mysql/driver.go
//go:build plugin_all || plugin_mysql

package mysql
// ... existing init() and implementation unchanged
```

Apply same pattern to all 23 drivers + matching advisor + parser files.

Also create `backend/plugin/db/all/all.go`:
```go
//go:build plugin_all
package all
// blank import of all drivers
```

## Acceptance Criteria

- [ ] All 23 `driver.go` files have `//go:build` constraint
- [ ] `go build ./backend/...` (no tags) — still includes all (via `plugin_all` default)
- [ ] `go build -tags plugin_pg ./backend/...` — only PG driver
- [ ] Advisor/parser files get matching build tags
