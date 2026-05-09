package lsp

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"
)

// NoopServer is a disabled LSP server that returns 503 for all requests.
// Used as a fallback when the LSP server fails to initialize.
type NoopServer struct{}

// RegisterRoutes registers a placeholder route that returns 503 Service Unavailable.
func (n *NoopServer) RegisterRoutes(e *echo.Echo) {
	slog.Warn("LSP server is disabled — registering noop handler")
	e.Any("/lsp", func(c *echo.Context) error {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error":   "LSP server is not available",
			"message": "The Language Server Protocol service failed to initialize. Check server logs for details.",
		})
	})
}
