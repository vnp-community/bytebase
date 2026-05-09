# Change Request: API Performance — Batch Operations & Query Optimization

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-PERF-005                                              |
| **Title**          | API Performance — Batch Ops, Lazy Loading & Streaming    |
| **Category**       | Performance / API Layer                                  |
| **Priority**       | P1 — High                                                |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-08                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | DCM-09, SQL-12, ADM-08                                  |

---

## 1. Tổng quan

### 1.1 Mô tả
Tối ưu API layer để handle high-throughput operations trên 200K+ databases. Hiện tại `BatchUpdateDatabases` loop từng request, `ListDatabases` deserialize protobuf trên mỗi row, và `convertToDatabase` thực hiện N+1 queries cho environment/project lookup.

### 1.2 Bối cảnh
- `BatchUpdateDatabases` (line 421-431 database_service.go): Loop gọi `UpdateDatabase` — N API calls
- `ListDatabases` deserializes `protojson.Unmarshal` trên mỗi row (line 242-244 database.go) — CPU intensive
- `convertToDatabase` gọi `GetInstance`, `GetProject` per database — N+1 query pattern
- `BatchGetDatabases` (line 77-131): Sequential permission checks per database
- Page size max 1000 — insufficient cho bulk operations

### 1.3 Mục tiêu
- Batch operations 10x faster qua single-query execution
- Eliminate N+1 queries trong list/batch operations
- Support streaming responses cho large result sets
- Reduce protobuf deserialization overhead 50%+

---

## 2. Yêu cầu chức năng

### FR-001: True Batch Database Operations
- **Mô tả**: Single SQL statement cho batch update/create thay vì loop
- **Current (N queries)**:
  ```go
  for _, updateReq := range req.Msg.GetRequests() {
      updated, err := s.UpdateDatabase(ctx, connect.NewRequest(updateReq))
  }
  ```
- **Target (1 query)**:
  ```go
  func (s *Store) BatchUpdateDatabasesOptimized(ctx context.Context, updates []UpdateDatabaseMessage) error {
      // Single UPDATE ... FROM unnest(...) query
  }
  ```
- **AC**:
  - AC-1: Batch update 1000 databases < 500ms (vs current ~10s)
  - AC-2: Atomic: all succeed or all fail
  - AC-3: Backward compatible API contract

### FR-002: Lazy Loading & Field Masks
- **Mô tả**: Only load requested fields qua FieldMask support
- **Logic**: `ListDatabases(fields=["name","project","environment"])` → skip metadata protobuf deserialization
- **AC**:
  - AC-1: List with minimal fields 3x faster than full load
  - AC-2: Metadata field skipped unless explicitly requested
  - AC-3: `view` parameter: BASIC (name/project/env) vs FULL (include metadata)

### FR-003: Batch Permission Check
- **Mô tả**: Single IAM check cho batch of databases thay vì per-item
- **Logic**: Group databases by project → 1 permission check per project
- **AC**:
  - AC-1: BatchGet 100 databases: 3-5 permission checks vs 100
  - AC-2: Permission cache per-project per-user (TTL 30s)
  - AC-3: No security regression — same authorization result

### FR-004: Server Streaming cho Large Lists
- **Mô tả**: ConnectRPC server streaming cho list >10K results
- **Logic**: Stream database records in chunks of 1000
- **AC**:
  - AC-1: Stream 200K databases without OOM
  - AC-2: Client-side cancellation stops server processing
  - AC-3: Fallback to pagination for non-streaming clients

### FR-005: Protobuf Deserialization Optimization
- **Mô tả**: Lazy deserialize metadata — store as raw bytes, only parse when accessed
- **Logic**: `DatabaseMessage.Metadata` becomes lazy — parsed on first access
- **AC**:
  - AC-1: ListDatabases (BASIC view) skips protobuf parse entirely
  - AC-2: 50%+ CPU reduction for list operations
  - AC-3: Metadata access still works transparently

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component          | File                                  | Thay đổi                              |
|--------------------|---------------------------------------|----------------------------------------|
| Database Service   | `backend/api/v1/database_service.go`  | Batch ops, streaming, field masks      |
| Store — Database   | `backend/store/database.go`           | True batch SQL, lazy metadata          |
| IAM Manager        | `backend/component/iam/manager.go`    | Batch permission check, caching        |
| Proto — Database   | `proto/v1/database_service.proto`     | Add view enum, streaming RPC           |
| Converter          | `backend/api/v1/database_converter.go`| Lazy metadata conversion               |

### 3.2 API Changes

```protobuf
// Thêm view parameter
enum DatabaseView {
    DATABASE_VIEW_UNSPECIFIED = 0;
    BASIC = 1;    // name, project, environment, instance
    FULL = 2;     // include metadata, labels, config
}

message ListDatabasesRequest {
    // existing fields...
    DatabaseView view = 6;
}

// Streaming RPC
service DatabaseService {
    rpc StreamDatabases(StreamDatabasesRequest) returns (stream Database);
}
```

---

## 4. Performance Targets

| Metric                        | Current       | Target           |
|-------------------------------|---------------|------------------|
| BatchUpdate 1000 DBs          | ~10s          | ≤ 500ms         |
| ListDatabases (BASIC, 1K)     | ~100ms        | ≤ 30ms          |
| BatchGet 100 DBs permissions  | 100 checks    | 5-10 checks     |
| Protobuf parse (per row)      | ~0.1ms        | 0ms (lazy)       |
| Stream 200K databases         | OOM           | ~30s streaming   |

---

## 5. Test Cases

| Test ID | Mô tả                                         | Expected Result                |
|---------|------------------------------------------------|--------------------------------|
| TC-001  | BatchUpdate 1000 databases                     | < 500ms, atomic                |
| TC-002  | ListDatabases BASIC view                       | 3x faster, no metadata parse  |
| TC-003  | BatchGet 100 DBs permission grouping           | ≤ 10 IAM checks               |
| TC-004  | Stream 200K databases                          | Complete without OOM           |
| TC-005  | Lazy metadata: access .Metadata after list     | Transparent deserialization    |
| TC-006  | BatchUpdate rollback on partial failure        | All or nothing                 |

---

## 6. Rollout Plan

| Phase   | Mô tả                              | Timeline   |
|---------|--------------------------------------|------------|
| Phase 1 | True batch SQL operations           | Sprint 1   |
| Phase 2 | Lazy metadata + field masks          | Sprint 2   |
| Phase 3 | Batch permission checks              | Sprint 2   |
| Phase 4 | Streaming RPC                        | Sprint 3   |
| Phase 5 | Performance benchmarks               | Sprint 3   |

---

## 7. Risks & Mitigations

| Risk                           | Impact | Mitigation                              |
|--------------------------------|--------|-----------------------------------------|
| Batch atomicity vs performance | MEDIUM | Transaction wrapping for consistency    |
| Lazy metadata thread safety    | HIGH   | sync.Once per DatabaseMessage           |
| Streaming backpressure         | MEDIUM | Server-side flow control, timeout       |
| Proto backward compatibility   | LOW    | Additive changes only, default = FULL   |
