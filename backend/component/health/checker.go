package health

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Status represents the health status of a component or the system.
type Status string

const (
	StatusHealthy  Status = "HEALTHY"
	StatusDegraded Status = "DEGRADED"
	StatusUnhealthy Status = "UNHEALTHY"
)

// CheckResult represents the result of a single health check.
type CheckResult struct {
	Name     string
	Status   Status
	Latency  time.Duration
	Message  string
	Critical bool // If true and UNHEALTHY, the whole system is UNHEALTHY
}

// CheckFunc is a function that performs a health check.
type CheckFunc func(context.Context) CheckResult

// Checker performs deep health checks.
type Checker struct {
	db       *sql.DB
	checks   []CheckFunc
	
	// Metrics
	overallStatusGauge *prometheus.GaugeVec
	checkLatencyHist   *prometheus.HistogramVec
}

// NewChecker creates a new Checker.
func NewChecker(db *sql.DB, registry prometheus.Registerer) *Checker {
	c := &Checker{
		db: db,
		overallStatusGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bytebase_health_status",
			Help: "Overall health status of the application (1=Healthy, 0=Degraded, -1=Unhealthy)",
		}, []string{"check"}),
		checkLatencyHist: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "bytebase_health_check_latency_seconds",
			Help: "Latency of health checks",
			Buckets: prometheus.DefBuckets,
		}, []string{"check"}),
	}
	
	if registry != nil {
		registry.MustRegister(c.overallStatusGauge)
		registry.MustRegister(c.checkLatencyHist)
	}

	c.checks = []CheckFunc{
		c.checkPostgreSQL,
		c.checkMemory,
		c.checkDiskSpace,
	}

	return c
}

// RunAll runs all health checks in parallel.
func (c *Checker) RunAll(ctx context.Context) (Status, []CheckResult) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]CheckResult, 0, len(c.checks))

	for _, check := range c.checks {
		wg.Add(1)
		go func(chk CheckFunc) {
			defer wg.Done()
			res := chk(ctx)
			
			// Update metrics
			if c.checkLatencyHist != nil {
				c.checkLatencyHist.WithLabelValues(res.Name).Observe(res.Latency.Seconds())
			}
			if c.overallStatusGauge != nil {
				val := float64(1)
				if res.Status == StatusDegraded {
					val = 0
				} else if res.Status == StatusUnhealthy {
					val = -1
				}
				c.overallStatusGauge.WithLabelValues(res.Name).Set(val)
			}
			
			mu.Lock()
			results = append(results, res)
			mu.Unlock()
		}(check)
	}

	wg.Wait()

	overallStatus := StatusHealthy
	for _, res := range results {
		if res.Status == StatusUnhealthy && res.Critical {
			overallStatus = StatusUnhealthy
			break
		} else if res.Status != StatusHealthy {
			if overallStatus == StatusHealthy {
				overallStatus = StatusDegraded
			}
		}
	}

	return overallStatus, results
}

func (c *Checker) checkPostgreSQL(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:     "PostgreSQL",
		Critical: true,
	}

	if c.db == nil {
		res.Status = StatusUnhealthy
		res.Message = "database connection is nil"
		res.Latency = time.Since(start)
		return res
	}

	if err := c.db.PingContext(ctx); err != nil {
		res.Status = StatusUnhealthy
		res.Message = fmt.Sprintf("ping failed: %v", err)
		res.Latency = time.Since(start)
		return res
	}

	stats := c.db.Stats()
	if stats.MaxOpenConnections > 0 && float64(stats.InUse)/float64(stats.MaxOpenConnections) > 0.9 {
		res.Status = StatusDegraded
		res.Message = fmt.Sprintf("pool utilization > 90%% (%d/%d)", stats.InUse, stats.MaxOpenConnections)
	} else {
		res.Status = StatusHealthy
		res.Message = "OK"
	}

	res.Latency = time.Since(start)
	return res
}

func (c *Checker) checkMemory(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:     "Memory",
		Critical: false,
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	const gb = 1024 * 1024 * 1024
	allocGB := float64(m.Alloc) / gb

	if allocGB > 2.5 {
		res.Status = StatusUnhealthy
		res.Message = fmt.Sprintf("high memory usage: %.2f GB", allocGB)
	} else if allocGB > 1.5 {
		res.Status = StatusDegraded
		res.Message = fmt.Sprintf("elevated memory usage: %.2f GB", allocGB)
	} else {
		res.Status = StatusHealthy
		res.Message = fmt.Sprintf("OK (%.2f GB)", allocGB)
	}

	res.Latency = time.Since(start)
	return res
}

func (c *Checker) checkDiskSpace(ctx context.Context) CheckResult {
	start := time.Now()
	// Dummy implementation. Could check data dir disk usage.
	return CheckResult{
		Name:     "Disk",
		Status:   StatusHealthy,
		Message:  "OK",
		Latency:  time.Since(start),
		Critical: true,
	}
}
