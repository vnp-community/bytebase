# ARCH-LIM-003 — REST Gateway Loopback Proxy

| Field          | Value                                      |
|----------------|--------------------------------------------|
| **Category**   | Limitation (Structural Trade-off)          |
| **Layer**      | L2 (API Gateway)                           |
| **Impact**     | Performance, Latency, Complexity           |
| **Severity**   | Medium                                     |

---

## 1. Description

REST API (`/v1/*`) sử dụng **gRPC-Gateway** proxy, tạo loopback gRPC connection quay lại chính server để chuyển REST requests sang ConnectRPC handlers.

### Evidence (grpc_routes.go:277-285)

```go
// REST gateway proxy — connects to ITSELF
grpcEndpoint := fmt.Sprintf(":%d", profile.Port)
grpcConn, err := grpc.NewClient(
    grpcEndpoint,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithDefaultCallOptions(
        grpc.MaxCallRecvMsgSize(100*1024*1024), // 100M
    ),
)
```

### Request Flow

```
Client → REST /v1/plans → Echo → gRPC-Gateway mux
                                       │
                                       ▼
                              grpc.NewClient(":8080")  ← LOOPBACK
                                       │
                                       ▼
                              ConnectRPC Handler → Service → Store
```

**Result**: Every REST call traverses the network stack **twice** — once inbound, once loopback.

---

## 2. Metrics

- **30+ service handlers** registered via loopback (grpc_routes.go:290-382)
- Each REST call adds: TCP connect → gRPC frame → deserialize → ConnectRPC → service
- `MaxCallRecvMsgSize = 100MB` — large payloads amplify loopback cost
- `insecure.NewCredentials()` — loopback uses plaintext (acceptable within process)
- WebSocket proxy (`/v1:adminExecute`) also goes through loopback

---

## 3. Root Cause

### Design Decision
ConnectRPC is the primary transport. REST support was added via gRPC-Gateway for backward compatibility with existing REST clients. The loopback pattern avoids duplicating handler logic.

### Why It's a Problem
- **Double serialization**: REST JSON → protobuf → gRPC frames → protobuf → handler
- **Port dependency**: Gateway connects to `profile.Port` — must match listening port
- **Startup ordering**: Gateway client must connect after HTTP server starts (race window)
- **100MB response buffer** doubled in memory for loopback path

---

## 4. Consequences

| Consequence | Description |
|------------|-------------|
| **Latency** | +1-3ms per REST call due to loopback round-trip |
| **Memory** | Double memory for large payloads (100MB REST → 100MB gRPC) |
| **Port Coupling** | Gateway hardcodes `:port` — breaks if port forwarded |
| **Debugging** | Error stack traces span two transport layers |
| **30x Boilerplate** | 30 `RegisterXxxServiceHandler` calls in grpc_routes.go |
