// Package metrics provides Prometheus metrics for multi-tenant observability.
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// TenantAPIRequests tracks the total API requests per workspace, method, and status.
	TenantAPIRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bytebase_tenant_api_requests_total",
		Help: "Total API requests per workspace",
	}, []string{"workspace", "method", "status"})

	// TenantDatabaseCount tracks the total databases per workspace.
	TenantDatabaseCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bytebase_tenant_databases_total",
		Help: "Total databases per workspace",
	}, []string{"workspace"})

	// TenantQuotaUsage tracks the quota usage ratio per workspace per resource.
	TenantQuotaUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bytebase_tenant_quota_usage_ratio",
		Help: "Quota usage ratio per workspace per resource",
	}, []string{"workspace", "resource"})

	// TenantRateLimitRejected tracks the total rate-limited requests per workspace.
	TenantRateLimitRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bytebase_tenant_rate_limit_rejected_total",
		Help: "Total rate-limited requests per workspace",
	}, []string{"workspace"})
)

func init() {
	prometheus.MustRegister(TenantAPIRequests, TenantDatabaseCount,
		TenantQuotaUsage, TenantRateLimitRejected)
}
