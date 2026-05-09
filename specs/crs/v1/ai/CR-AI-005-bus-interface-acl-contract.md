# Change Request: Event Bus Interface & ACL Contract Extraction

| Field              | Value                                                               |
|--------------------|---------------------------------------------------------------------|
| **CR ID**          | CR-AI-005                                                           |
| **Issue IDs**      | AI-BLOCKER-007, AI-BLOCKER-009                                      |
| **Title**          | Typed Event Bus Interface & Explicit ACL Resource Contract          |
| **Category**       | Architecture / Security (ARCH)                                      |
| **Priority**       | P2 — Medium                                                         |
| **Status**         | Draft                                                               |
| **Created**        | 2026-05-09                                                          |
| **Author**         | VNP AI Ops Team                                                     |
| **PRD Refs**       | DCM-01 (Issue/Plan/Rollout), SEC-01 (IAM), SEC-09 (Approval Workflow), ADM-08 (API Integration) |

---

## 1. Tổng quan

### 1.1 Mô tả
Giải quyết hai vấn đề coupling cấp hệ thống:
1. **Event Bus**: `component/bus/bus.go` sử dụng raw `chan int` không interface — AI không thể trace event flows
2. **ACL Interceptor**: `api/v1/acl.go` sử dụng protobuf reflection để extract resources — contract ẩn trong naming convention

### 1.2 Bối cảnh
- **Bus**: Bytebase Runner Layer (PRD §2 — TaskRun, PlanCheck, SchemaSync, Approval) communicate qua in-process channels. AI agents thêm runner mới không biết phải listen trên channel nào.
- **ACL**: IAM system (PRD SEC-01, SEC-14) sử dụng CEL-based authorization (PRD §7 — CEL). ACL interceptor probe proto fields (`parent`, `name`, `resource`, `project`) bằng reflection — AI sửa ACL có thể tạo authorization bypass.

### 1.3 Mục tiêu
- Event bus có testable interface + typed events
- ACL resource extraction có explicit contract (static map thay vì reflection)
- AI agents có event flow documentation

---

## 2. Yêu cầu chức năng

### FR-001: EventBus Interface Extraction
- **Mô tả**: Extract interface từ concrete `Bus` struct
- **Logic**:
  ```go
  // EventBus defines the contract for internal event communication.
  type EventBus interface {
      // Task/Plan scheduling
      TicklePlanCheck()
      TickleTaskRun()

      // Approval workflow
      RequestApprovalCheck(ref IssueRef)

      // Rollout lifecycle
      RequestRolloutCreation(ref PlanRef)
      RequestPlanCompletionCheck(ref PlanRef)

      // Cancellation
      RegisterTaskRunCancel(ref TaskRunRef, cancel context.CancelFunc)
      CancelTaskRun(ref TaskRunRef) bool
      RegisterPlanCheckCancel(ref PlanCheckRunRef, cancel context.CancelFunc)
      CancelPlanCheck(ref PlanCheckRunRef) bool
  }
  ```
- **PRD Alignment**: DCM-01 (Rollout workflow), SEC-09 (Approval workflow)
- **Acceptance Criteria**:
  - AC-1: `Bus` struct implements `EventBus` interface (compile-time check)
  - AC-2: All runners use `EventBus` interface instead of direct channel access
  - AC-3: `chan int` replaced with typed method calls

### FR-002: Typed Event Structs
- **Mô tả**: Replace `chan int` tickle signals with typed events
- **Logic**:
  ```go
  // TickleEvent carries context about why a scheduler was triggered.
  type TickleEvent struct {
      Source    string    // component that triggered (e.g., "plan_service")
      Reason   string    // why (e.g., "new_plan_created")
      EmittedAt time.Time
  }
  ```
- **Acceptance Criteria**:
  - AC-1: No `chan int` in bus.go
  - AC-2: Event types carry diagnostic context

### FR-003: Event Flow Documentation
- **Mô tả**: Tạo `EVENT_FLOWS.md` mapping producers → bus → consumers
- **Content**:
  ```
  Plan Created → bus.TicklePlanCheck() → PlanCheckRunner
  Plan Check Done → bus.RequestApprovalCheck() → ApprovalRunner
  Approval Done → bus.TickleTaskRun() → TaskRunRunner
  Task Run Done → bus.RequestPlanCompletionCheck() → WebhookRunner
  ```
- **Acceptance Criteria**:
  - AC-1: All 6 bus channels documented with producer/consumer pairs
  - AC-2: Diagram shows event lifecycle for DCM-01 workflow

### FR-004: ACL Static Resource Extractor Map
- **Mô tả**: Replace reflection-based resource extraction with code-generated static map
- **Logic**:
  ```go
  // aclResourceExtractor defines how to extract resource names from each RPC method.
  // This replaces the reflection-based probing in getResourceFromSingleRequest().
  type ResourceExtractorFunc func(proto.Message) []string

  var aclResourceExtractors = map[string]ResourceExtractorFunc{
      // Auth Service
      "/bytebase.v1.AuthService/Login":              extractNone,
      "/bytebase.v1.AuthService/GetUser":             extractFromName,

      // Database Service
      "/bytebase.v1.DatabaseService/GetDatabase":     extractFromName,
      "/bytebase.v1.DatabaseService/ListDatabases":   extractFromParent,
      "/bytebase.v1.DatabaseService/UpdateDatabase":  extractFromDatabaseUpdate,

      // Rollout Service
      "/bytebase.v1.RolloutService/CreateRollout":    extractFromParent,
      "/bytebase.v1.RolloutService/GetRollout":       extractFromName,
      // ... all 30+ services × methods
  }

  // Helper extractors
  func extractFromName(msg proto.Message) []string   { /* use "name" field */ }
  func extractFromParent(msg proto.Message) []string  { /* use "parent" field */ }
  func extractNone(msg proto.Message) []string        { return nil }
  ```
- **PRD Alignment**: SEC-01 (IAM), SEC-14 (Custom Roles) — authorization contract must be explicit
- **Acceptance Criteria**:
  - AC-1: No `protoreflect` usage for field probing in ACL path
  - AC-2: Every registered gRPC method has an explicit extractor entry
  - AC-3: Missing method → explicit `CodeInternal` error (no silent fallback)
  - AC-4: `BatchUpdateIssuesStatus` HACK removed

### FR-005: ACL Resource Contract Documentation
- **Mô tả**: Document ACL resource extraction rules
- **Content**: Method-to-resource mapping table, security implications
- **Acceptance Criteria**:
  - AC-1: All 30+ services have documented resource extraction patterns

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                        | Thay đổi                                         |
|------------------------|---------------------------------------------|---------------------------------------------------|
| Bus Interface          | `backend/component/bus/interface.go`        | NEW — `EventBus` interface definition             |
| Bus Implementation     | `backend/component/bus/bus.go`              | Implement `EventBus`, replace `chan int`           |
| Event Types            | `backend/component/bus/events.go`           | NEW — `TickleEvent` and other typed events        |
| Event Flows Doc        | `backend/component/bus/EVENT_FLOWS.md`      | NEW — producer/consumer documentation             |
| ACL Extractors         | `backend/api/v1/acl_extractors.go`          | NEW — static extractor map                        |
| ACL Interceptor        | `backend/api/v1/acl.go`                     | Replace reflection with static map lookup         |
| ACL Contract Doc       | `backend/api/v1/ACL_CONTRACT.md`            | NEW — method-to-resource mapping                  |
| Runner Updates         | `backend/runner/*.go`                       | Use `EventBus` interface instead of `*Bus`        |

### 3.2 Không có Database Changes
### 3.3 Không có API Contract Changes — internal refactoring only

---

## 4. Phụ thuộc

| Dependency             | Mô tả                                                       |
|------------------------|--------------------------------------------------------------|
| gRPC Service Registry  | Full list of registered methods needed for ACL map           |
| Runner components      | Must update to use `EventBus` interface                      |
| CR-AI-002              | Interface-based DI makes bus injection cleaner               |

---

## 5. Test Cases

| Test ID | Mô tả                                                            | Expected Result                     |
|---------|-------------------------------------------------------------------|-------------------------------------|
| TC-001  | `Bus` struct satisfies `EventBus` interface (compile-time)       | Compiles                            |
| TC-002  | `TicklePlanCheck()` triggers PlanCheckRunner                     | Runner processes event              |
| TC-003  | ACL: known method → correct resource extracted                   | Resource matches expected           |
| TC-004  | ACL: unknown method → `CodeInternal` error                       | Explicit error, not silent fallback |
| TC-005  | ACL: `BatchUpdateIssuesStatus` — explicit extractor (no HACK)   | Clean extraction                    |
| TC-006  | ACL: `UpdateDatabase` with `project` field mask → both projects checked | Both project resources verified |
| TC-007  | Event flow: Plan → PlanCheck → Approval → TaskRun               | Complete workflow triggers          |
| TC-008  | Mock `EventBus` in service tests                                 | Mockable, no channel leaks          |

---

## 6. Rollout Plan

| Phase   | Mô tả                                                   | Timeline  |
|---------|----------------------------------------------------------|-----------|
| Phase 1 | Extract `EventBus` interface + implement on `Bus`        | Sprint 1  |
| Phase 2 | Replace `chan int` with typed methods                    | Sprint 1  |
| Phase 3 | Create `EVENT_FLOWS.md`                                  | Sprint 1  |
| Phase 4 | Build ACL static extractor map                           | Sprint 2  |
| Phase 5 | Replace reflection in `acl.go`                           | Sprint 2  |
| Phase 6 | Create `ACL_CONTRACT.md`                                 | Sprint 2  |
| Phase 7 | Update all runners to use `EventBus` interface           | Sprint 3  |

---

## 7. Risks & Mitigations

| Risk                                            | Impact | Mitigation                                          |
|--------------------------------------------------|--------|------------------------------------------------------|
| ACL static map out-of-sync with proto changes   | HIGH   | CI check: compare registered methods vs map entries |
| Bus interface breaks runner implementations     | MEDIUM | Incremental migration, keep old channels temporarily |
| Missing ACL extractor → auth bypass             | HIGH   | Default to `CodeInternal` error for unregistered methods |
| Event type changes require runner updates       | LOW    | Typed events are additive, not breaking              |

---

## 8. Success Metrics

| Metric                              | Before  | Target      |
|--------------------------------------|---------|-------------|
| Bus `chan int` usage                 | 2       | 0           |
| Bus interface defined               | No      | Yes         |
| ACL `protoreflect` field probing    | 4 probes | 0          |
| ACL methods with explicit extractors | 0      | All (100+)  |
| Event flow documentation            | None    | Complete    |
| ACL contract documentation          | None    | Complete    |
