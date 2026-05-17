package service

import (
	"context"
	"log/slog"
	"net"
)

// Registry holds references to all services and provides lifecycle management.
type Registry struct {
	services map[string]DomainService
	runner   RunnerService
}

// NewRegistry creates a new service registry.
func NewRegistry() *Registry {
	return &Registry{
		services: make(map[string]DomainService),
	}
}

// Register adds a domain service to the registry.
func (r *Registry) Register(svc DomainService) {
	r.services[svc.Name()] = svc
}

// SetRunner sets the runner service.
func (r *Registry) SetRunner(runner RunnerService) {
	r.runner = runner
}

// Get returns a domain service by name.
func (r *Registry) Get(name string) (DomainService, bool) {
	svc, ok := r.services[name]
	return svc, ok
}

// Runner returns the runner service.
func (r *Registry) Runner() RunnerService {
	return r.runner
}

// AllDomainServices returns all registered domain services.
func (r *Registry) AllDomainServices() []DomainService {
	svcs := make([]DomainService, 0, len(r.services))
	for _, svc := range r.services {
		svcs = append(svcs, svc)
	}
	return svcs
}

// Listeners returns a map of service name → net.Listener for gateway routing.
func (r *Registry) Listeners() map[string]net.Listener {
	m := make(map[string]net.Listener)
	for name, svc := range r.services {
		m[name] = svc.Listener()
	}
	return m
}

// StartAll starts all domain services.
func (r *Registry) StartAll(ctx context.Context) error {
	for _, svc := range r.services {
		slog.Info("Starting domain service", "service", svc.Name())
		if err := svc.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// StopAll gracefully stops all domain services.
func (r *Registry) StopAll(ctx context.Context) error {
	var firstErr error
	for _, svc := range r.services {
		slog.Info("Stopping domain service", "service", svc.Name())
		if err := svc.Stop(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// HealthCheckAll runs health checks on all domain services.
func (r *Registry) HealthCheckAll(ctx context.Context) map[string]error {
	results := make(map[string]error)
	for name, svc := range r.services {
		results[name] = svc.Healthy(ctx)
	}
	return results
}
