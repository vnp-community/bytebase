# TASK-103: Define Service Interfaces

| Field | Value |
|-------|-------|
| Task ID | TASK-103 |
| Phase | 1 |
| Estimated | 0.5 day |
| Dependencies | TASK-000 |
| Status | ✅ DONE |

## Objective

Tạo `backend/service/service.go` — common interface cho tất cả domain services.

## File: `backend/service/service.go`

```go
package service

import (
    "google.golang.org/grpc/test/bufconn"
)

// DomainService is the interface all domain services must implement.
type DomainService interface {
    // Start starts the service's internal HTTP server on bufconn.
    Start() error

    // Stop gracefully stops the service.
    Stop()

    // Listener returns the bufconn listener for Gateway to connect.
    Listener() *bufconn.Listener

    // Name returns the service name for logging/metrics.
    Name() string
}
```

## Acceptance Criteria

- [ ] `backend/service/service.go` created
- [ ] `go build ./backend/service/` compiles
- [ ] No imports of `api/v1` or business logic packages
