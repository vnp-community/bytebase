package server

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo-contrib/v5/echoprometheus"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/pkg/errors"

	"time"
	"github.com/bytebase/bytebase/backend/component/health"
	directorysync "github.com/bytebase/bytebase/backend/api/directory-sync"
	"github.com/bytebase/bytebase/backend/api/lsp"
	"github.com/bytebase/bytebase/backend/api/mcp"
	"github.com/bytebase/bytebase/backend/api/oauth2"
	stripeapi "github.com/bytebase/bytebase/backend/api/stripe"
	"github.com/bytebase/bytebase/backend/common/log"
	"github.com/bytebase/bytebase/backend/component/config"
	stripeplugin "github.com/bytebase/bytebase/backend/plugin/stripe"
)

func configureEchoRouters(
	e *echo.Echo,
	s *Server,
	lspServer *lsp.Server,
	directorySyncServer *directorysync.Service,
	oauth2Service *oauth2.Service,
	mcpServer *mcp.Server,
	stripeWebhookHandler *stripeapi.WebhookHandler,
	profile *config.Profile,
) {
	e.Use(recoverMiddleware)
	e.Use(standbyRedirectMiddleware(profile))
	// CSP nonce middleware: when CSP_NONCE_ENABLED=true, each request gets a
	// unique 128-bit nonce in style-src, eliminating 'unsafe-inline' (TASK-WEAK-001-1).
	// When flag is off, the legacy CSP with 'unsafe-inline' is used (no behavior change).
	e.Use(newSecurityHeadersMiddleware(profile))

	// CSP violation report endpoint (TASK-WEAK-001-2).
	registerCSPReportRoute(e)

	// CORS configuration: extracted for clarity and hardened for production.
	// Dev mode: wildcard (any origin). Production: configurable via CORS_ALLOWED_ORIGINS.
	// See configureCORS() in cors_audit.go (TASK-WEAK-002-1, TASK-WEAK-002-2).
	configureCORS(e, profile)

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:    true,
		LogMethod: true,
		LogStatus: true,
		LogValuesFunc: func(_ *echo.Context, values middleware.RequestLoggerValues) error {
			if values.Error != nil {
				slog.Error("echo request logger", "method", values.Method, "uri", values.URI, "status", values.Status, log.BBError(values.Error))
			}
			return nil
		},
	}))

	registerPprof(e, &profile.RuntimeDebug)

	// Prometheus metrics - use custom registry to avoid duplicate registration in tests
	e.Use(echoprometheus.NewMiddlewareWithConfig(echoprometheus.MiddlewareConfig{
		Subsystem:  "api",
		Registerer: s.promRegistry,
	}))
	e.GET("/metrics", echoprometheus.NewHandlerWithConfig(echoprometheus.HandlerConfig{
		Gatherer: s.promRegistry,
	}))

	// Health check endpoints.
	e.GET("/healthz", func(c *echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Deep health check (monitoring/alerting)
	e.GET("/healthz/deep", func(c *echo.Context) error {
		overall, checks := s.healthChecker.RunAll(c.Request().Context())
		status := http.StatusOK
		if overall == health.StatusUnhealthy {
			status = http.StatusServiceUnavailable
		}
		return c.JSON(status, map[string]any{
			"status":    overall,
			"checks":    checks,
			"node":      s.profile.ReplicaID,
			"version":   s.profile.Version,
			"uptime":    time.Since(time.Unix(s.startedTS, 0)).String(),
			"timestamp": time.Now().UTC(),
		})
	})

	// Readiness probe (K8s)
	e.GET("/readyz", func(c *echo.Context) error {
		overall, checks := s.healthChecker.RunAll(c.Request().Context())
		status := http.StatusOK
		if overall == health.StatusUnhealthy {
			status = http.StatusServiceUnavailable
		}
		return c.JSON(status, map[string]any{"status": overall, "checks": checks})
	})

	// LSP server.
	e.GET(lspAPI, lspServer.Router)

	hookGroup := e.Group(webhookAPIPrefix)
	scimGroup := hookGroup.Group(scimAPIPrefix)
	directorySyncServer.RegisterDirectorySyncRoutes(scimGroup)

	// Stripe (SaaS only, requires both API key and webhook secret).
	if profile.SaaS && profile.StripeAPISecret != "" && profile.StripeWebhookSecret != "" {
		stripeplugin.Init(profile.StripeAPISecret)
		stripeGroup := hookGroup.Group("/stripe")
		stripeWebhookHandler.RegisterRoutes(stripeGroup)
	}

	// OAuth2 server.
	oauth2Service.RegisterRoutes(e)

	// MCP server.
	mcpServer.RegisterRoutes(e)

	// Embed frontend (must be last to serve as fallback for SPA routes).
	embedFrontend(e)
}

func recoverMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if !ok {
					err = errors.Errorf("%v", r)
				}
				slog.Error("Middleware PANIC RECOVER", log.BBError(err), log.BBStack("panic-stack"))

				// In Echo v5, send error response directly
				resp, unwrapErr := echo.UnwrapResponse(c.Response())
				if unwrapErr == nil && !resp.Committed {
					_ = c.JSON(http.StatusInternalServerError, map[string]string{
						"error": "Internal server error",
					})
				}
			}
		}()
		return next(c)
	}
}

func standbyRedirectMiddleware(profile *config.Profile) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if !profile.IsStandby() {
				return next(c)
			}
			method := c.Request().Method
			if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
				return next(c)
			}

			// It's a write request on a standby node
			primaryURL := profile.PrimaryRegionURL
			if primaryURL == "" {
				primaryURL = "PRIMARY_REGION_NOT_CONFIGURED"
			}
			c.Response().Header().Set("X-Bytebase-Primary", primaryURL)
			return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
				"error": "write operations are not allowed on standby node",
				"primary_url": primaryURL,
			})
		}
	}
}
