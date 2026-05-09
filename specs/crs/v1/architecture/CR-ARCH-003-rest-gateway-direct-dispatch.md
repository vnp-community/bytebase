# Change Request: REST Gateway Direct Dispatch

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **CR ID**          | CR-ARCH-003                                              |
| **Source ID**      | ARCH-LIM-003                                             |
| **Title**          | REST Gateway Direct Dispatch — Eliminate Loopback Proxy  |
| **Category**       | Architecture (Performance)                               |
| **Priority**       | P2 — Medium                                              |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |
| **PRD Refs**       | ADM-08 (API Integration REST+gRPC)                       |

---

## 1. Tổng quan

### 1.1 Mô tả
Loại bỏ gRPC loopback connection trong REST gateway. REST requests hiện tại đi qua `grpc.NewClient(":port")` → loopback TCP → ConnectRPC handler, tạo double serialization và +1-3ms latency mỗi request.

### 1.2 Bối cảnh
- REST `/v1/*` sử dụng gRPC-Gateway proxy qua loopback connection (grpc_routes.go:279)
- 30 `RegisterXxxServiceHandler` calls tạo proxy cho mỗi service
- `insecure.NewCredentials()` + `MaxCallRecvMsgSize(100MB)` trên loopback
- Double memory cho large payloads (100MB REST → 100MB gRPC)
- Port coupling: gateway hardcodes `:profile.Port`

### 1.3 Mục tiêu
- Loại bỏ loopback gRPC connection
- REST requests dispatch trực tiếp tới ConnectRPC handlers
- Giảm latency 1-3ms per REST call
- Giảm memory usage cho large payload responses
- Giảm boilerplate code (30x Register calls)

---

## 2. Yêu cầu chức năng

### FR-001: Direct Handler Dispatch
- **Mô tả**: REST gateway gọi ConnectRPC handlers trực tiếp in-process thay vì qua TCP loopback.
- **Logic**:
  ```go
  // BEFORE: grpc_routes.go:279-285
  grpcConn, err := grpc.NewClient(grpcEndpoint, ...)
  v1pb.RegisterAuthServiceHandler(ctx, mux, grpcConn)

  // AFTER: Direct HTTP handler routing
  // gRPC-Gateway mux routes REST → ConnectRPC HTTP handler (in-process)
  // No TCP round-trip
  ```
- **Acceptance Criteria**:
  - AC-1: REST API responses identical to current behavior
  - AC-2: No loopback TCP connection opened
  - AC-3: Latency reduced ≥ 1ms per REST call (benchmarked)
  - AC-4: 100MB payload responses use single buffer

### FR-002: Unified Handler Map
- **Mô tả**: Merge ConnectRPC handlers và REST gateway routing vào single dispatch table.
- **Acceptance Criteria**:
  - AC-1: Giảm 30 `RegisterXxxServiceHandler` calls
  - AC-2: Adding new service = 1 registration point thay vì 2

### FR-003: WebSocket Proxy Preservation
- **Mô tả**: `/v1:adminExecute` WebSocket proxy vẫn hoạt động.
- **Acceptance Criteria**:
  - AC-1: `wsproxy.WebsocketProxy` continues to work
  - AC-2: Streaming SQL execution functional

---

## 3. Yêu cầu kỹ thuật

### 3.1 Backend Changes

| Component              | File                                  | Thay đổi                                    |
|------------------------|---------------------------------------|----------------------------------------------|
| Gateway routing        | `backend/server/grpc_routes.go`       | Replace loopback with direct dispatch        |
| Handler registration   | `backend/server/grpc_routes.go`       | Unified registration table                   |
| WebSocket preservation | `backend/server/grpc_routes.go:384`   | Verify wsproxy compatibility                 |

### 3.2 Database/Frontend Changes
Không có.

---

## 4. Test Cases

| Test ID    | Mô tả                                                        | Expected Result                          |
|------------|---------------------------------------------------------------|------------------------------------------|
| TC-001     | REST `POST /v1/plans` returns same response as ConnectRPC    | Response body identical                  |
| TC-002     | REST large payload (50MB export) → single buffer             | Memory usage reduced                     |
| TC-003     | WebSocket `/v1:adminExecute` streaming works                 | SQL streaming functional                 |
| TC-004     | Benchmark: 1000 REST calls latency comparison                | ≥1ms improvement per call                |
| TC-005     | Auth cookies pass through REST gateway correctly             | Authentication works                     |
| TC-006     | REST error responses maintain same gRPC status codes         | Error format unchanged                   |

---

## 5. Rollout Plan

| Phase   | Mô tả                                             | Timeline     |
|---------|----------------------------------------------------|--------------| 
| Phase 1 | Prototype direct dispatch for 3 services (POC)     | Sprint 1     |
| Phase 2 | Benchmark latency + memory improvement             | Sprint 1     |
| Phase 3 | Migrate all 30 services to direct dispatch         | Sprint 2     |
| Phase 4 | Remove loopback gRPC client code                   | Sprint 2     |
| Phase 5 | Integration testing + WebSocket verification       | Sprint 3     |

---

## 6. Risks & Mitigations

| Risk                                          | Impact | Mitigation                                         |
|-----------------------------------------------|--------|-----------------------------------------------------|
| gRPC-Gateway routing behavior change          | HIGH   | Extensive REST API compatibility testing             |
| WebSocket proxy depends on gRPC connection    | HIGH   | May need separate WebSocket handling                 |
| Header passing (cookies, auth) may differ     | MEDIUM | Test all auth methods via REST gateway               |
