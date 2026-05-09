# Change Request: Background Runner Scalability & Job Orchestration

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PERF-006                                              |
| **Title**          | Runner Orchestration — Job Queue & Distributed Scheduling |
| **Category**       | Performance / Background Processing                      |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | DCM-01, DCM-06, DCM-10, SEC-09                          |

---

## 1. Tổng quan

### 1.1 Mô tả
Hiện có 6+ background runners (TaskScheduler, PlanCheckScheduler, ApprovalRunner, SchemaSyncer, AnomalyScanner, MailRunner) chạy as goroutines trong single process. Với 200K databases, runners cạnh tranh CPU/memory/connections — task execution chậm, approval check bị delay, plan check timeout.

### 1.2 Bối cảnh
- Runners chạy trên cùng process, share connection pool
- TaskRun executor mở connection tới target database — 200K potential connections
- PlanCheckScheduler scan ALL pending plans mỗi 10s
- ApprovalRunner scan ALL pending approvals mỗi 10s
- No job deduplication — same task có thể execute multiple times
- No job priority — infrastructure task cùng queue với user task

### 1.3 Mục tiêu
- Task execution throughput ≥ 500 tasks/minute
- Plan check response time ≤ 30s
- Approval check response time ≤ 10s
- Zero duplicate job execution
- Per-tenant fair scheduling

---

## 2. Yêu cầu chức năng

### FR-001: PostgreSQL-Based Job Queue
- **Mô tả**: Replace goroutine polling với persistent job queue
- **Logic**:
  ```sql
  CREATE TABLE job_queue (
      id BIGSERIAL PRIMARY KEY,
      job_type TEXT NOT NULL,
      workspace TEXT NOT NULL,
      payload JSONB NOT NULL,
      priority INT NOT NULL DEFAULT 0,
      status TEXT NOT NULL DEFAULT 'pending',
      locked_by TEXT,
      locked_at TIMESTAMPTZ,
      attempts INT DEFAULT 0,
      max_attempts INT DEFAULT 3,
      created_at TIMESTAMPTZ DEFAULT NOW(),
      scheduled_at TIMESTAMPTZ DEFAULT NOW(),
      completed_at TIMESTAMPTZ
  );
  CREATE INDEX idx_job_queue_pending ON job_queue(priority DESC, scheduled_at)
      WHERE status = 'pending';
  ```
- **AC**:
  - AC-1: Jobs persisted across restarts
  - AC-2: SELECT FOR UPDATE SKIP LOCKED — no duplicate execution
  - AC-3: Dead letter queue cho failed jobs (attempts > max)
  - AC-4: Job deduplication key prevents duplicate enqueue

### FR-002: Fair Scheduling Across Tenants
- **Mô tả**: Round-robin scheduling ensures each tenant gets fair share
- **Logic**: Weighted fair queue — tenant with 50K DBs gets proportionally more but doesn't starve others
- **AC**:
  - AC-1: Each tenant processes ≥ 1 job per scheduling round
  - AC-2: No tenant waits >60s for job processing
  - AC-3: Priority override for critical jobs (production rollout)

### FR-003: Runner Resource Isolation
- **Mô tả**: Separate resource pools per runner type
- **Allocation**:
  | Runner           | Max Goroutines | Max Connections | Priority |
  |------------------|---------------|-----------------|----------|
  | TaskRun          | 200           | 50              | P0       |
  | PlanCheck        | 100           | 30              | P1       |
  | Approval         | 50            | 10              | P1       |
  | SchemaSync       | 300           | 50              | P2       |
  | Anomaly          | 50            | 10              | P3       |
- **AC**:
  - AC-1: TaskRun executor cannot exhaust PlanCheck connections
  - AC-2: Runner resource usage visible in metrics
  - AC-3: Resource limits configurable per deployment

### FR-004: Distributed Runner Coordination
- **Mô tả**: Multiple Bytebase replicas share job queue via PostgreSQL
- **Logic**: Advisory locks per runner type + SKIP LOCKED for job dequeue
- **AC**:
  - AC-1: 3 replicas process 3x throughput (near-linear scaling)
  - AC-2: Replica failure — jobs redistributed within 30s
  - AC-3: No job processed twice across replicas

### FR-005: Job Observability
- **Mô tả**: Comprehensive metrics and tracing for background jobs
- **Metrics**:
  - `bytebase_job_queue_depth{type, workspace, priority}`
  - `bytebase_job_processing_duration_seconds{type, workspace}`
  - `bytebase_job_failure_total{type, workspace, error}`
  - `bytebase_runner_utilization{runner_type}`
- **AC**:
  - AC-1: Job latency histogram per type per tenant
  - AC-2: Alert when queue depth > threshold
  - AC-3: Trace ID propagation from API → job execution

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component          | File                                     | Thay đổi                              |
|--------------------|------------------------------------------|----------------------------------------|
| Job Queue          | `backend/component/jobqueue/queue.go`    | New: PG-based job queue                |
| Fair Scheduler     | `backend/component/jobqueue/scheduler.go`| New: weighted fair scheduling          |
| Runner Wrapper     | `backend/runner/runner.go`               | Refactor: consume from job queue       |
| Task Scheduler     | `backend/runner/taskrun/scheduler.go`    | Migrate to job queue                   |
| Plan Check         | `backend/runner/plancheck/scheduler.go`  | Migrate to job queue                   |
| Approval Runner    | `backend/runner/approval/runner.go`      | Migrate to job queue                   |
| Metrics            | `backend/metrics/job_metrics.go`         | New: job queue metrics                 |

### 3.2 Configuration

| Environment Variable        | Default | Mô tả                                |
|-----------------------------|---------|----------------------------------------|
| `JOB_QUEUE_POLL_INTERVAL`   | `1s`    | Job queue polling interval             |
| `JOB_MAX_RETRY`             | `3`     | Max retry attempts                     |
| `JOB_LOCK_TTL`              | `300s`  | Job lock timeout                       |
| `JOB_DEAD_LETTER_ENABLED`   | `true`  | Enable dead letter queue               |
| `RUNNER_TASKRUN_WORKERS`    | `200`   | TaskRun worker pool size               |
| `RUNNER_PLANCHECK_WORKERS`  | `100`   | PlanCheck worker pool size             |

---

## 4. Performance Targets

| Metric                       | Current       | Target           |
|------------------------------|---------------|------------------|
| Task throughput              | ~50/min       | ≥ 500/min        |
| Plan check latency          | ~60s          | ≤ 30s            |
| Approval check latency      | ~30s          | ≤ 10s            |
| Job duplicate rate           | >0%           | 0%               |
| Runner scaling (3 replicas)  | 1x            | ~2.5x            |

---

## 5. Test Cases

| Test ID | Mô tả                                         | Expected Result                |
|---------|------------------------------------------------|--------------------------------|
| TC-001  | Enqueue 10K jobs, 3 replicas consuming         | All processed, zero duplicates |
| TC-002  | Replica crash mid-job                          | Job released, re-processed     |
| TC-003  | 100 tenants each submit 100 tasks              | Fair distribution observed     |
| TC-004  | Priority P0 job inserted during heavy load     | Processed within 10s           |
| TC-005  | Job fails 3 times                              | Moved to dead letter queue     |
| TC-006  | TaskRun pool exhausted                         | PlanCheck unaffected           |

---

## 6. Rollout Plan

| Phase   | Mô tả                              | Timeline   |
|---------|--------------------------------------|------------|
| Phase 1 | Job queue tables + core library     | Sprint 1   |
| Phase 2 | Migrate PlanCheck + Approval        | Sprint 2   |
| Phase 3 | Migrate TaskRun executor            | Sprint 3   |
| Phase 4 | Fair scheduling + resource isolation | Sprint 3  |
| Phase 5 | Distributed coordination (multi-replica) | Sprint 4 |
| Phase 6 | Observability + load testing        | Sprint 4   |

---

## 7. Risks & Mitigations

| Risk                           | Impact | Mitigation                              |
|--------------------------------|--------|-----------------------------------------|
| PG job queue as bottleneck     | MEDIUM | Index optimization, SKIP LOCKED         |
| Job lock timeout too short     | HIGH   | Heartbeat-based lock renewal            |
| Migration complexity           | HIGH   | Dual-mode: old runners + new queue      |
| Fair scheduling overhead       | LOW    | Lightweight round-robin implementation  |
