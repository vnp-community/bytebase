package store

import "github.com/prometheus/client_golang/prometheus"

// CacheMetrics exposes Prometheus counters and gauges for cache observability.
type CacheMetrics struct {
	hits   *prometheus.CounterVec
	misses *prometheus.CounterVec
	size   *prometheus.GaugeVec
}

// NewCacheMetrics creates and registers cache metrics with the default Prometheus registry.
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

// Hit records a cache hit for the given cache name and tier.
func (m *CacheMetrics) Hit(cache, tier string) { m.hits.WithLabelValues(cache, tier).Inc() }

// Miss records a cache miss for the given cache name.
func (m *CacheMetrics) Miss(cache string) { m.misses.WithLabelValues(cache).Inc() }

// SetSize records the current number of entries in a cache tier.
func (m *CacheMetrics) SetSize(cache, tier string, n int) {
	m.size.WithLabelValues(cache, tier).Set(float64(n))
}
