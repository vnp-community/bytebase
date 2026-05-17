# TASK-102: Transport Abstraction Layer (bufconn)

| Field | Value |
|-------|-------|
| Task ID | TASK-102 |
| Phase | 1 |
| Estimated | 0.5 day |
| Dependencies | TASK-000 |
| Status | ✅ DONE |

## Objective

Tạo `backend/transport/` package cung cấp HTTP transport qua bufconn, cho phép services chạy HTTP server in-memory.

## File: `backend/transport/bufconn.go`

```go
package transport

import (
    "context"
    "net"
    "net/http"

    "google.golang.org/grpc/test/bufconn"
)

const DefaultBufSize = 1024 * 1024

// BufconnHTTPTransport creates an http.Transport that connects via bufconn.
func BufconnHTTPTransport(listener *bufconn.Listener) *http.Transport {
    return &http.Transport{
        DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
            return listener.DialContext(ctx)
        },
    }
}

// BufconnHTTPClient creates an http.Client using bufconn transport.
func BufconnHTTPClient(listener *bufconn.Listener) *http.Client {
    return &http.Client{
        Transport: BufconnHTTPTransport(listener),
    }
}

// NewBufconnListener creates a new bufconn listener.
func NewBufconnListener(size int) *bufconn.Listener {
    if size <= 0 {
        size = DefaultBufSize
    }
    return bufconn.Listen(size)
}
```

## Acceptance Criteria

- [ ] `backend/transport/bufconn.go` created
- [ ] `go build ./backend/transport/` compiles
- [ ] No new external dependencies (bufconn is part of google.golang.org/grpc)
