# TASK-305: Phase 3 Full Verification

| Field | Value |
|-------|-------|
| Task ID | TASK-305 |
| Phase | 3 |
| Dependencies | TASK-304 |
| Status | ✅ DONE |

## Steps

```bash
# 1. Build
go build ./backend/...

# 2. Tests
go test ./backend/... -count=1

# 3. Start server
go run backend/bin/server/main.go --port 18080 &

# 4. Verify endpoints
curl -s http://localhost:18080/healthz            # → OK
curl -s http://localhost:18080/metrics | head -5  # → Prometheus
curl -s -X POST http://localhost:18080/bytebase.v1.ActuatorService/GetActuatorInfo \
  -H "Content-Type: application/json" -d '{}'    # → JSON
curl -s http://localhost:18080/v1/actuator/info   # → REST response

# 5. Compare baseline (TASK-000)
# All handler counts, runner counts must match
```

## Acceptance Criteria

- [ ] All tests pass
- [ ] Server starts, healthz OK
- [ ] ConnectRPC routing works
- [ ] REST gateway works
- [ ] Handler counts match baseline
- [ ] Git commit: "phase-3: gateway + server refactor"
