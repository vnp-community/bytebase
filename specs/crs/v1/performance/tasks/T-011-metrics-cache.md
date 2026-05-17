# T-011: Metrics — Cache Metrics

| Field | Value |
|-------|-------|
| **Task ID** | T-011 |
| **Solution** | SOL-PERF-002 |
| **Type** | New file |
| **Priority** | P2 |
| **Depends on** | None |
| **Blocks** | None |
| **Status** | DONE |

## Target File

`backend/store/cache_metrics.go` (new)

## Implementation

```go
package store

import "github.com/prometheus/client_golang/prometheus"

type CacheMetrics struct {
    hits   *prometheus.CounterVec
    misses *prometheus.CounterVec
    size   *prometheus.GaugeVec
}

func NewCacheMetrics() *CacheMetrics {
    m := &CacheMetrics{
        hits: prometheus.NewCounterVec(prometheus.CounterOpts{
            Name: "bytebase_cache_hits_total",
            Help: "Total cache hits by cache name and tier",
        }, []string{"cache", "tier"}),
        misses: prometheus.NewCounterVec(prometheus.CounterOpts{
            Name: "bytebase_cache_misses_total",
            Help: "Total cache misses by cache name",
        }, []string{"cache"}),
        size: prometheus.NewGaugeVec(prometheus.GaugeOpts{
            Name: "bytebase_cache_entries",
            Help: "Current entries in cache",
        }, []string{"cache", "tier"}),
    }
    prometheus.MustRegister(m.hits, m.misses, m.size)
    return m
}

func (m *CacheMetrics) Hit(cache, tier string)  { m.hits.WithLabelValues(cache, tier).Inc() }
func (m *CacheMetrics) Miss(cache string)       { m.misses.WithLabelValues(cache).Inc() }
func (m *CacheMetrics) SetSize(cache, tier string, n int) {
    m.size.WithLabelValues(cache, tier).Set(float64(n))
}
```
