package mcp

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"
)

// NoopServer is a disabled MCP server that returns 503 for all requests.
// Used as a fallback when the MCP server fails to initialize.
type NoopServer struct{}

// RegisterRoutes registers a placeholder route that returns 503 Service Unavailable.
func (n *NoopServer) RegisterRoutes(e *echo.Echo) {
	slog.Warn("MCP server is disabled — registering noop handler")
	e.Any("/mcp", func(c *echo.Context) error {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error":   "MCP server is not available",
			"message": "The Model Context Protocol service failed to initialize. Check server logs for details.",
		})
	})
}
