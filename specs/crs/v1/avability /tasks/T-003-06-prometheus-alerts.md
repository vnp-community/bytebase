# T-003-06: Prometheus Alert Rules

| Field | Value |
|---|---|
| **Task ID** | T-003-06 |
| **Solution** | SOL-AVAIL-003 |
| **Depends On** | T-003-01, T-003-03 |
| **Target File** | `deploy/alerts/availability.yml` (NEW) |

---

## Objective

Tạo Prometheus alert rules cho health status, circuit breaker, và memory.

## Implementation

```yaml
groups:
  - name: bytebase-health
    rules:
      - alert: BytebaseUnhealthy
        expr: bytebase_health_status{component="postgresql"} == 0
        for: 1m
        labels: { severity: critical }
        annotations: { summary: "PostgreSQL connection unhealthy" }

      - alert: BytebaseCircuitOpen
        expr: bytebase_circuit_breaker_state > 0
        for: 2m
        labels: { severity: warning }
        annotations: { summary: "Circuit breaker {{ $labels.breaker }} is open" }

      - alert: BytebaseMemoryHigh
        expr: bytebase_health_status{component="memory"} < 2
        for: 5m
        labels: { severity: warning }
        annotations: { summary: "Memory pressure detected" }

      - alert: BytebasePoolExhaustion
        expr: bytebase_db_pool_in_use_connections / bytebase_db_pool_max_open_connections > 0.9
        for: 2m
        labels: { severity: warning }
        annotations: { summary: "DB pool > 90% utilization" }
```

## Acceptance Criteria

- [x] 4 alert rules covering health, CB, memory, pool
- [x] Valid Prometheus alerting rule YAML syntax
