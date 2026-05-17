// Package readiness provides production readiness checks for embedded PG usage.
// When Bytebase runs with embedded PostgreSQL and the workload exceeds typical
// development thresholds, the checker flags the deployment as needing an external
// PostgreSQL instance for production reliability.
package readiness

import (
	"context"
	"database/sql"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/component/config"
	"github.com/bytebase/bytebase/backend/store"
)

// Criterion represents a single production-readiness criterion.
type Criterion struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Met         bool   `json:"met"`
	Value       string `json:"value"`
	Threshold   string `json:"threshold"`
}

// ReadinessReport is the result of a production readiness check.
type ReadinessReport struct {
	// IsEmbedded indicates whether the server is using embedded PG.
	IsEmbedded bool `json:"isEmbedded"`
	// CriteriaMet is the number of production criteria that were met.
	CriteriaMet int `json:"criteriaMet"`
	// ShowWarning is true when ≥2 criteria are met and embedded PG is in use.
	ShowWarning bool `json:"showWarning"`
	// Criteria is the full list of evaluated criteria.
	Criteria []Criterion `json:"criteria"`
}

// Thresholds for production usage detection.
const (
	thresholdInstances  = 5
	thresholdUsers      = 10
	thresholdChangelogs = 100
	thresholdUptime     = 30 * 24 * time.Hour // 30 days
	criteriaThreshold   = 2                   // number of criteria that must be met to trigger warning
)

// Checker evaluates whether an embedded PG deployment has grown beyond
// typical dev/test usage and should migrate to an external PostgreSQL.
type Checker struct {
	store     *store.Store
	profile   *config.Profile
	startedAt time.Time
}

// NewChecker creates a new ReadinessChecker.
func NewChecker(store *store.Store, profile *config.Profile, startedAt time.Time) *Checker {
	return &Checker{
		store:     store,
		profile:   profile,
		startedAt: startedAt,
	}
}

// Check evaluates all 5 production readiness criteria and returns a report.
// For external PG deployments, returns immediately with IsEmbedded=false and no warning.
func (c *Checker) Check(ctx context.Context) *ReadinessReport {
	report := &ReadinessReport{
		IsEmbedded: c.profile.UseEmbedDB(),
	}

	if !report.IsEmbedded {
		// External PG — no warning needed.
		return report
	}

	report.Criteria = []Criterion{
		c.checkInstanceCount(ctx),
		c.checkUserCount(ctx),
		c.checkChangelogCount(ctx),
		c.checkUptime(),
		c.checkProductionEnv(ctx),
	}

	for _, cr := range report.Criteria {
		if cr.Met {
			report.CriteriaMet++
		}
	}
	report.ShowWarning = report.CriteriaMet >= criteriaThreshold

	return report
}

func (c *Checker) checkInstanceCount(ctx context.Context) Criterion {
	cr := Criterion{
		Name:        "instance_count",
		Description: "Number of database instances exceeds production threshold",
		Threshold:   ">5",
	}

	count, err := c.countAll(ctx, `SELECT COUNT(*) FROM instance WHERE deleted = FALSE`)
	if err != nil {
		slog.Debug("readiness: failed to count instances", log.BBError(err))
		return cr
	}
	cr.Value = intToStr(count)
	cr.Met = count > thresholdInstances
	return cr
}

func (c *Checker) checkUserCount(ctx context.Context) Criterion {
	cr := Criterion{
		Name:        "user_count",
		Description: "Number of active users exceeds production threshold",
		Threshold:   ">10",
	}

	count, err := c.store.CountActivePrincipals(ctx)
	if err != nil {
		slog.Debug("readiness: failed to count users", log.BBError(err))
		return cr
	}
	cr.Value = intToStr(count)
	cr.Met = count > thresholdUsers
	return cr
}

func (c *Checker) checkChangelogCount(ctx context.Context) Criterion {
	cr := Criterion{
		Name:        "changelog_count",
		Description: "Number of schema changes exceeds production threshold",
		Threshold:   ">100",
	}

	count, err := c.countAll(ctx, `SELECT COUNT(*) FROM changelog`)
	if err != nil {
		slog.Debug("readiness: failed to count changelogs", log.BBError(err))
		return cr
	}
	cr.Value = intToStr(count)
	cr.Met = count > thresholdChangelogs
	return cr
}

func (c *Checker) checkUptime() Criterion {
	uptime := time.Since(c.startedAt)
	cr := Criterion{
		Name:        "uptime",
		Description: "Server uptime exceeds 30 days",
		Threshold:   ">30d",
		Value:       uptime.Round(time.Hour).String(),
		Met:         uptime > thresholdUptime,
	}
	return cr
}

func (c *Checker) checkProductionEnv(ctx context.Context) Criterion {
	cr := Criterion{
		Name:        "production_env",
		Description: "Deployment has a production environment configured",
		Threshold:   "exists",
	}

	// Check if any environment has a name containing "prod" (case-insensitive).
	rows, err := c.store.GetDB().QueryContext(ctx,
		`SELECT value FROM setting WHERE name = 'bb.workspace.environment'`)
	if err != nil {
		slog.Debug("readiness: failed to query environments", log.BBError(err))
		return cr
	}
	defer rows.Close()

	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			continue
		}
		if containsProd(value) {
			cr.Met = true
			cr.Value = "true"
			return cr
		}
	}
	cr.Value = "false"
	return cr
}

// countAll executes a simple COUNT(*) query.
func (c *Checker) countAll(ctx context.Context, query string) (int, error) {
	var count int
	if err := c.store.GetDB().QueryRowContext(ctx, query).Scan(&count); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}

func containsProd(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "prod")
}

func intToStr(v int) string {
	return strconv.Itoa(v)
}
