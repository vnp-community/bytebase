package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// cspViolationCounter tracks CSP violations reported by browsers.
var cspViolationCounter = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "bytebase",
		Name:      "csp_violations_total",
		Help:      "Total number of Content-Security-Policy violations reported by browsers.",
	},
	[]string{"directive", "blocked_uri"},
)

// cspReportBody represents the JSON body sent by browsers when a CSP violation
// occurs. The format follows the CSP Level 2 reporting spec (RFC).
type cspReportBody struct {
	CSPReport struct {
		DocumentURI        string `json:"document-uri"`
		Referrer           string `json:"referrer"`
		BlockedURI         string `json:"blocked-uri"`
		ViolatedDirective  string `json:"violated-directive"`
		EffectiveDirective string `json:"effective-directive"`
		OriginalPolicy     string `json:"original-policy"`
		StatusCode         int    `json:"status-code"`
	} `json:"csp-report"`
}

// registerCSPReportRoute registers the POST /api/csp-report endpoint.
func registerCSPReportRoute(e *echo.Echo) {
	e.POST("/api/csp-report", handleCSPReport)
}

// handleCSPReport processes CSP violation reports from browsers.
// Returns 204 No Content on success, 400 on invalid JSON.
func handleCSPReport(c *echo.Context) error {
	body, err := io.ReadAll((*c).Request().Body)
	if err != nil {
		return (*c).NoContent(http.StatusBadRequest)
	}
	defer (*c).Request().Body.Close()

	var report cspReportBody
	if err := json.Unmarshal(body, &report); err != nil {
		return (*c).NoContent(http.StatusBadRequest)
	}

	r := report.CSPReport
	if r.ViolatedDirective == "" && r.EffectiveDirective == "" {
		return (*c).NoContent(http.StatusBadRequest)
	}

	// Use effective-directive if available (CSP Level 3), fall back to violated-directive
	directive := r.EffectiveDirective
	if directive == "" {
		directive = r.ViolatedDirective
	}

	// Truncate blocked_uri to prevent high-cardinality metric labels
	blockedURI := r.BlockedURI
	if len(blockedURI) > 128 {
		blockedURI = blockedURI[:128]
	}

	slog.Warn("CSP violation",
		"document_uri", r.DocumentURI,
		"blocked_uri", r.BlockedURI,
		"violated_directive", r.ViolatedDirective,
		"effective_directive", r.EffectiveDirective,
		"referrer", r.Referrer,
	)

	cspViolationCounter.WithLabelValues(directive, blockedURI).Inc()

	return (*c).NoContent(http.StatusNoContent)
}
