# TASK-000: Pre-Migration Checklist & Baseline

| Field | Value |
|-------|-------|
| Task ID | TASK-000 |
| Phase | 0 — Pre-Migration |
| Priority | P0 (Blocker) |
| Estimated | 0.5 day |
| Dependencies | None |
| Status | ✅ DONE |

## Objective

Thiết lập baseline trước khi bắt đầu migration. Đảm bảo hệ thống hiện tại ổn định và có đủ dữ liệu để so sánh sau migration.

## Steps

### Step 1: Run Full Test Suite
```bash
cd /path/to/vnp-bytebase
go test ./backend/... 2>&1 | tee /tmp/pre-migration-test.log
echo "Exit code: $?"
```
**Acceptance**: Exit code = 0, tất cả tests pass.

### Step 2: Run Go Vet
```bash
go vet ./backend/...
```
**Acceptance**: No warnings, no errors.

### Step 3: Inventory ConnectRPC Handlers
```bash
# Count all ConnectRPC handler registrations in grpc_routes.go
grep -c "v1connect.New.*ServiceHandler" backend/server/grpc_routes.go
```
**Expected**: 31 handlers (30 services + 1 engine capability).
**Record this number** — must match after migration.

### Step 4: Inventory REST Gateway Registrations
```bash
grep -c "v1pb.Register.*ServiceHandler" backend/server/grpc_routes.go
```
**Expected**: ~30 registrations.
**Record this number** — must match after migration.

### Step 5: Inventory Runner Goroutines
```bash
# Count runner goroutines started in Run()
grep -c "go s\." backend/server/server.go
grep -c "go .*\.Run" backend/server/server.go
```
**Record these numbers** — must match after migration.

### Step 6: Inventory Service Names for Reflection
```bash
grep -c "v1connect\..*ServiceName" backend/server/grpc_routes.go
```
**Record this number** — must match after migration.

### Step 7: Create Git Tag
```bash
git add -A && git stash  # Stash any working changes
git tag -a pre-gateway-migration -m "Baseline before gateway+services migration"
git stash pop             # Restore working changes
```

### Step 8: Document Baseline Numbers

Create `specs/services/tasks/baseline.md`:
```markdown
| Metric | Count |
|--------|-------|
| ConnectRPC Handlers | __ |
| REST Gateway Registrations | __ |
| Runner Goroutines (single-node) | __ |
| Runner Goroutines (HA) | __ |
| Reflection Service Names | __ |
| Test Count (pass) | __ |
| Test Count (total) | __ |
```

## Acceptance Criteria

- [ ] `go test ./backend/...` passes 100%
- [ ] `go vet ./backend/...` clean
- [ ] Baseline numbers documented
- [ ] Git tag `pre-gateway-migration` created
- [ ] No uncommitted changes in working directory
