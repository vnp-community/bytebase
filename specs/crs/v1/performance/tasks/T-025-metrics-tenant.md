# T-025: Metrics — Tenant Metrics

| Field | Value |
|-------|-------|
| **Task ID** | T-025 |
| **Solution** | SOL-PERF-004 |
| **Type** | New file |
| **Priority** | P2 |
| **Depends on** | None |
| **Blocks** | None |

## Target File

`backend/metrics/tenant_metrics.go` (new)

## Implementation

```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
    TenantAPIRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "bytebase_tenant_api_requests_total",
        Help: "Total API requests per workspace",
    }, []string{"workspace", "method", "status"})

    TenantDatabaseCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "bytebase_tenant_databases_total",
        Help: "Total databases per workspace",
    }, []string{"workspace"})

    TenantQuotaUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "bytebase_tenant_quota_usage_ratio",
        Help: "Quota usage ratio per workspace per resource",
    }, []string{"workspace", "resource"})

    TenantRateLimitRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "bytebase_tenant_rate_limit_rejected_total",
        Help: "Total rate-limited requests per workspace",
    }, []string{"workspace"})
)

func init() {
    prometheus.MustRegister(TenantAPIRequests, TenantDatabaseCount,
        TenantQuotaUsage, TenantRateLimitRejected)
}
```
