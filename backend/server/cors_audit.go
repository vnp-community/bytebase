package server

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	connectcors "connectrpc.com/cors"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/component/config"
)

// TASK-WEAK-002-2: Prometheus counter for rejected CORS requests.
var corsRejectedCounter = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "bytebase",
		Name:      "cors_rejected_total",
		Help:      "CORS requests rejected by origin validation.",
	},
	[]string{"origin"},
)

// TASK-WEAK-002-2: Gauge that indicates whether the server is running in dev mode.
// 1 = development (wildcard CORS), 0 = release (restricted CORS).
var devModeGauge = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: "bytebase",
	Name:      "dev_mode",
	Help:      "1 if server running in development mode, 0 otherwise.",
})

// configureCORS sets up the CORS middleware based on the runtime profile.
//
// TASK-WEAK-002-1: Extracted from configureEchoRouters for clarity.
//   - Dev mode: wildcard CORS (any origin allowed) + warning log + devModeGauge=1.
//   - Production + CORS_ALLOWED_ORIGINS set: only configured origins allowed.
//   - Production + CORS_ALLOWED_ORIGINS contains "*": rejected, no CORS installed.
//   - Production + no CORS_ALLOWED_ORIGINS: same-origin only (no CORS middleware).
func configureCORS(e *echo.Echo, profile *config.Profile) {
	corsMaxAge := getCORSMaxAge()

	if profile.Mode == common.ReleaseModeDev {
		// Dev mode: wildcard CORS for local development convenience.
		devModeGauge.Set(1)
		slog.Warn("⚠️ SERVER RUNNING IN DEVELOPMENT MODE",
			slog.Bool("cors_wildcard", true),
			slog.String("mode", string(profile.Mode)))

		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			UnsafeAllowOriginFunc: func(_ *echo.Context, origin string) (string, bool, error) {
				return origin, true, nil
			},
			AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions},
			AllowHeaders:     connectcors.AllowedHeaders(),
			ExposeHeaders:    connectcors.ExposedHeaders(),
			AllowCredentials: true,
		}))
		return
	}

	// Production mode.
	devModeGauge.Set(0)

	corsOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if corsOrigins == "" {
		// No CORS_ALLOWED_ORIGINS → same-origin only. No CORS middleware installed.
		slog.Info("CORS: same-origin mode (no CORS_ALLOWED_ORIGINS configured)")
		return
	}

	origins := strings.Split(corsOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	// TASK-WEAK-002-1: Reject wildcard "*" in production — it's a security risk.
	for _, o := range origins {
		if o == "*" {
			slog.Error("CORS wildcard '*' not allowed in production — CORS middleware NOT installed",
				slog.String("env", "CORS_ALLOWED_ORIGINS"),
				slog.String("value", corsOrigins))
			return
		}
	}

	// Install production CORS with configured origins.
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     origins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions},
		AllowHeaders:     append(connectcors.AllowedHeaders(), "Authorization", "X-Auth-Mode"),
		ExposeHeaders:    append(connectcors.ExposedHeaders(), "X-Refresh-Token"),
		AllowCredentials: true,
		MaxAge:           corsMaxAge,
	}))
	slog.Info("CORS enabled for standalone frontend",
		slog.Any("origins", origins),
		slog.Int("max_age", corsMaxAge))
}

// getCORSMaxAge reads CORS_MAX_AGE from environment. Default: 86400 (24h).
func getCORSMaxAge() int {
	if v := os.Getenv("CORS_MAX_AGE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
		slog.Warn("Invalid CORS_MAX_AGE value, using default",
			slog.String("value", v),
			slog.Int("default", 86400))
	}
	return 86400
}
