// Package service defines the common interface for all domain services
// in the Gateway + Services architecture.
package service

import (
	"context"
	"net"
	"net/http"
)

// DomainService is the lifecycle interface that all domain services must implement.
// Services are created, started (serving HTTP on bufconn), and stopped.
type DomainService interface {
	// Name returns the service identifier (e.g., "dcm", "sql", "admin").
	Name() string

	// Start begins serving HTTP on the internal listener (non-blocking).
	Start(ctx context.Context) error

	// Stop gracefully shuts down the service HTTP server.
	Stop(ctx context.Context) error

	// Listener returns the internal net.Listener (bufconn for single-binary).
	Listener() net.Listener

	// HTTPClient returns an HTTP client configured to call this service.
	HTTPClient() *http.Client

	// Healthy returns nil if the service is healthy, or an error describing the issue.
	Healthy(ctx context.Context) error
}

// ServiceRouter holds references to all domain services for gateway routing.
type ServiceRouter struct {
	DCM    DomainService
	SQL    DomainService
	Admin  DomainService
	Runner RunnerService
}

// AllDomainServices returns all domain services for iteration.
func (r *ServiceRouter) AllDomainServices() []DomainService {
	return []DomainService{r.DCM, r.SQL, r.Admin}
}

// StartAll starts all domain services.
func (r *ServiceRouter) StartAll(ctx context.Context) error {
	for _, svc := range r.AllDomainServices() {
		if err := svc.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// StopAll gracefully stops all domain services.
func (r *ServiceRouter) StopAll(ctx context.Context) error {
	var firstErr error
	for _, svc := range r.AllDomainServices() {
		if err := svc.Stop(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// RunnerService is the interface for the background runner service.
type RunnerService interface {
	// Run starts all background runners (non-blocking, spawns goroutines).
	Run(ctx context.Context)

	// Wait blocks until all runners have finished.
	Wait()
}
