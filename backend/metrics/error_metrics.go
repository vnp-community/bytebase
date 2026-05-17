package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// TASK-WEAK-003-3: Prometheus counters for migration executor warning conditions.
// These track non-fatal errors that are surfaced as warnings in TaskRunResult
// instead of being silently logged and discarded.

// SchemaSyncErrorsCounter tracks schema sync failures during post-migration schema dump.
var SchemaSyncErrorsCounter = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "bytebase",
	Name:      "schema_sync_errors_total",
	Help:      "Total schema sync failures during post-migration schema dump.",
})

// ChangelogUpdateErrorsCounter tracks changelog update failures after migration execution.
var ChangelogUpdateErrorsCounter = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "bytebase",
	Name:      "changelog_update_errors_total",
	Help:      "Total changelog update failures after migration execution.",
})
