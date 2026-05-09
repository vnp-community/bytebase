# Solution: REST Gateway Direct Dispatch — CR-ARCH-003

| Field              | Value                                                    |
|--------------------|----------------------------------------------------------|
| **Solution ID**    | SOL-ARCH-003                                             |
| **CR Reference**   | CR-ARCH-003                                              |
| **Title**          | ConnectRPC Direct Handler Mount — Eliminate Loopback     |
| **Affected Layers**| L2 (API Gateway)                                         |
| **Status**         | Draft                                                    |
| **Created**        | 2026-05-09                                               |
| **Author**         | VNP AI Ops Team                                          |

---

## 1. Architecture Context

Per [architecture.md](../../architecture.md) §2 (L2 — API Gateway):
- ConnectRPC handlers serve both gRPC (HTTP/2) and Connect (HTTP/1.1) natively
- gRPC-Gateway proxy creates REST endpoints by connecting to self via loopback gRPC

Per [TDD.md](../../TDD.md) §3 (API Layer):
- REST support via gRPC-Gateway auto-generated from `.proto` files
- 30+ `RegisterXxxServiceHandler` calls in `grpc_routes.go`

---

## 2. Current Implementation Analysis

### 2.1 Loopback Connection (grpc_routes.go:277-285)

```go
grpcEndpoint := fmt.Sprintf(":%d", profile.Port)
grpcConn, err := grpc.NewClient(
    grpcEndpoint,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithDefaultCallOptions(
        grpc.MaxCallRecvMsgSize(100*1024*1024),
    ),
)
// REST calls go: Client → Echo → gRPC-Gateway → TCP loopback → ConnectRPC handler
```

### 2.2 Request Flow Problem

```
REST /v1/plans → Echo → gRPC-Gateway mux → grpc.NewClient(:8080) → TCP → ConnectRPC
                                                     LOOPBACK ↗
```

**Cost**: +1-3ms per call, double serialization, double memory for large payloads.

---

## 3. Solution Design

### 3.1 Strategy: In-Process gRPC Server for Gateway

Replace TCP loopback with in-process `bufconn` listener — gRPC-Gateway connects to an in-memory buffer instead of TCP.

**Modified file**: `backend/server/grpc_routes.go`

```go
import (
    "google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024  // 1MB buffer

func configureGrpcRouters(ctx context.Context, e *echo.Echo, ...) error {
    // ... existing ConnectRPC handler setup ...

    // OPTION 1: bufconn — in-process gRPC for Gateway (RECOMMENDED)
    //
    // Instead of: grpc.NewClient(":port") → TCP loopback
    // Use:        grpc.NewClient("bufconn") → in-memory pipe
    lis := bufconn.Listen(bufSize)
    grpcServer := grpc.NewServer()

    // Register gRPC services on the in-process server
    v1pb.RegisterAuthServiceServer(grpcServer, authServiceGRPC)
    v1pb.RegisterPlanServiceServer(grpcServer, planServiceGRPC)
    // ... all services ...

    go grpcServer.Serve(lis)

    // Gateway connects via bufconn — NO TCP, NO port dependency
    grpcConn, err := grpc.NewClient(
        "passthrough:///bufconn",
        grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
            return lis.DialContext(ctx)
        }),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithDefaultCallOptions(
            grpc.MaxCallRecvMsgSize(100*1024*1024),
        ),
    )
    if err != nil {
        return errors.Wrapf(err, "failed to dial bufconn")
    }

    // REST gateway uses in-process connection
    mux := runtime.NewServeMux(...)
    v1pb.RegisterAuthServiceHandler(ctx, mux, grpcConn)
    v1pb.RegisterPlanServiceHandler(ctx, mux, grpcConn)
    // ... remaining handlers ...

    e.Any("/v1/*", echo.WrapHandler(mux))
}
```

### 3.2 Benefits vs Current

| Aspect | Current (TCP Loopback) | Solution (bufconn) |
|--------|----------------------|---------------------|
| Transport | TCP socket `:port` | In-memory buffer |
| Latency | +1-3ms (TCP round-trip) | ~0.01ms (memory copy) |
| Port dependency | Must know `:port` | None |
| Startup race | Gateway must wait for TCP listen | No race (bufconn is sync) |
| Memory | Double (REST + gRPC buffers) | Shared buffer |
| Code changes | None | Replace dialer only |

### 3.3 WebSocket Proxy Preservation

The WebSocket proxy for `/v1:adminExecute` needs special handling:

```go
// WebSocket proxy must still use HTTP transport (not bufconn)
// Keep the wsproxy route pointing to the HTTP server
e.GET("/v1:adminExecute", func(c *echo.Context) error {
    // WebSocket proxied directly to ConnectRPC HTTP handler
    // No change needed — it uses HTTP, not gRPC transport
    wsproxy.WebsocketProxy(c.Response(), c.Request(), ...)
})
```

---

## 4. File Change Manifest

| File | Layer | Action | Impact |
|------|-------|--------|--------|
| `backend/server/grpc_routes.go` | L2 | **MODIFY** | Replace TCP dialer with bufconn |

---

## 5. Dependency Direction Validation

```
L2 (grpc_routes.go) → google.golang.org/grpc/test/bufconn (existing grpc dep)
```

**No new dependencies** — `bufconn` is part of the existing `google.golang.org/grpc` module.

---

## 6. Migration Strategy

1. Replace dialer in `configureGrpcRouters` — single file change
2. Run REST API integration tests
3. Benchmark: compare latency before/after with `wrk`

---

## 7. Rollback Plan

1. Revert dialer change → back to TCP loopback
2. Single-file revert, no migration, no data changes
