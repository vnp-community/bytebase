# Tasks — Availability Implementation

> **Total**: 24 tasks | **3 Phases** | Optimized for minimal token consumption per task

---

## Thiết kế tối ưu token

Mỗi task được thiết kế theo nguyên tắc:
1. **1-2 files per task** — Agent chỉ cần đọc tối thiểu context
2. **Self-contained** — Mỗi task có đủ thông tin: file target, code snippet, acceptance criteria
3. **No cross-references** — Không yêu cầu đọc solution doc; task chứa trực tiếp code cần viết
4. **Ordered by dependency** — Task sau có thể import kết quả task trước mà không cần re-read

---

## Phase 1 — Foundation (Sprint 1-2)

### SOL-004: Connection Resilience (standalone, 0 dependencies)
| Task | File(s) | Est. Tokens | Description |
|---|---|---|---|
| [T-004-01](./T-004-01-pool-config.md) | `store/db_connection.go` | ~800 | PoolConfig struct + env parsing |
| [T-004-02](./T-004-02-retry-engine.md) | `store/retry.go` | ~900 | RetryableExec + isRetryable |
| [T-004-03](./T-004-03-pool-metrics.md) | `store/db_metrics.go` | ~700 | PoolMetricsCollector |
| [T-004-04](./T-004-04-retry-driver.md) | `plugin/db/retry_wrapper.go` | ~700 | RetryDriver decorator |
| [T-004-05](./T-004-05-pool-monitor.md) | `runner/monitor/pool_monitor.go`, `server/server.go` | ~600 | Pool monitor runner + wiring |

### SOL-003: Health & Circuit Breaker (standalone)
| Task | File(s) | Est. Tokens | Description |
|---|---|---|---|
| [T-003-01](./T-003-01-health-checker.md) | `component/health/checker.go` | ~1000 | Deep health check system |
| [T-003-02](./T-003-02-health-endpoints.md) | `server/echo_routes.go` | ~500 | /healthz/deep, /readyz |
| [T-003-03](./T-003-03-circuit-breaker.md) | `component/circuitbreaker/breaker.go` | ~1000 | Circuit breaker pattern |
| [T-003-04](./T-003-04-dbfactory-cb.md) | `component/dbfactory/dbfactory.go` | ~600 | Wire CB into DBFactory |
| [T-003-05](./T-003-05-selfheal-runner.md) | `runner/selfheal/runner.go`, `server/server.go` | ~600 | Self-healing runner |
| [T-003-06](./T-003-06-prometheus-alerts.md) | `deploy/alerts/availability.yml` | ~300 | Alert rules |

### SOL-001: HA Clustering (core)
| Task | File(s) | Est. Tokens | Description |
|---|---|---|---|
| [T-001-01](./T-001-01-replica-model.md) | `store/model/replica.go`, migration SQL | ~500 | ReplicaNode struct + migration |
| [T-001-02](./T-001-02-heartbeat-enhanced.md) | `store/replica_heartbeat.go` | ~700 | Enhanced CRUD methods |
| [T-001-03](./T-001-03-heartbeat-runner.md) | `runner/heartbeat/runner.go` | ~600 | Cluster registration + DRAINING |
| [T-001-04](./T-001-04-graceful-shutdown.md) | `server/server.go` | ~600 | Enhanced shutdown sequence |
| [T-001-05](./T-001-05-k8s-deploy.md) | `deploy/k8s/deployment.yaml` | ~400 | K8s zero-downtime spec |

## Phase 2 — Resilience (Sprint 3-4)

### SOL-002: Failover & DR (depends on 001, 003)
| Task | File(s) | Est. Tokens | Description |
|---|---|---|---|
| [T-002-01](./T-002-01-advisory-lock-keys.md) | `store/advisory_lock.go` | ~200 | Add lock keys 2001-2003 |
| [T-002-02](./T-002-02-leader-runner.md) | `runner/leader/runner.go` | ~800 | Leader election runner |
| [T-002-03](./T-002-03-task-recovery.md) | `runner/leader/task_recovery.go`, migration | ~700 | Orphaned task recovery |
| [T-002-04](./T-002-04-persistent-bus.md) | `component/bus/persistent.go`, migration | ~700 | PG-backed bus durability |

### SOL-005: Backup & Recovery (depends on 004)
| Task | File(s) | Est. Tokens | Description |
|---|---|---|---|
| [T-005-01](./T-005-01-backup-registry.md) | `store/backup_registry.go`, migration | ~700 | Backup record CRUD + table |
| [T-005-02](./T-005-02-backup-executor.md) | `runner/backup/executor.go` | ~800 | pg_dump wrapper + encryption |
| [T-005-03](./T-005-03-backup-scheduler.md) | `runner/backup/scheduler.go`, `server/server.go` | ~700 | Cron scheduler + RPO check |

## Phase 3 — Geographic (Sprint 5-7)

### SOL-006: Multi-Region (depends on all)
| Task | File(s) | Est. Tokens | Description |
|---|---|---|---|
| [T-006-01](./T-006-01-region-config.md) | `component/config/profile.go` | ~400 | RegionRole + config fields |
| [T-006-02](./T-006-02-standby-interceptor.md) | `api/v1/standby_interceptor.go` | ~700 | Read-only enforcement |
| [T-006-03](./T-006-03-replication-monitor.md) | `runner/replication/monitor.go` | ~700 | Lag monitor + Prometheus |

---

## Token Budget Summary

| Phase | Tasks | Est. Total Tokens |
|---|---|---|
| Phase 1 | 16 | ~11,400 |
| Phase 2 | 4 | ~2,400 |
| Phase 3 | 3 | ~1,800 |
| **Total** | **24** | **~15,600** |

> Estimated token usage per task: **~650 tokens** (average)
> Each task is executable in a single agent turn with minimal context window consumption.
