# TASK-205: Create ServiceRouter

| Field | Value |
|-------|-------|
| Task ID | TASK-205 |
| Phase | 2 |
| Dependencies | TASK-201, TASK-202, TASK-203, TASK-204 |
| Status | ✅ DONE |

## Objective

Tạo ServiceRouter tổng hợp 3 domain services + runner service.

## File: `backend/service/registry.go`

```go
package service

import "google.golang.org/grpc/test/bufconn"

type ServiceRouter struct {
    DCM    DomainService
    SQL    DomainService
    Admin  DomainService
    Runner interface {
        Run(ctx context.Context)
        Wait()
    }
}

// AllServices returns all domain services.
func (r *ServiceRouter) AllServices() []DomainService {
    return []DomainService{r.DCM, r.SQL, r.Admin}
}

// StartAll starts all domain service HTTP servers.
func (r *ServiceRouter) StartAll() error {
    for _, svc := range r.AllServices() {
        go svc.Start()
    }
    return nil
}

// StopAll gracefully stops all domain services.
func (r *ServiceRouter) StopAll() {
    for _, svc := range r.AllServices() {
        svc.Stop()
    }
}

// Listeners returns bufconn listeners for Gateway to connect.
func (r *ServiceRouter) Listeners() map[string]*bufconn.Listener {
    m := make(map[string]*bufconn.Listener)
    for _, svc := range r.AllServices() {
        m[svc.Name()] = svc.Listener()
    }
    return m
}
```

## Acceptance Criteria

- [ ] `backend/service/registry.go` created
- [ ] `go build ./backend/service/...` compiles (all packages)
- [ ] Zero changes to existing files
