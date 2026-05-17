package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/component/config"
)

// cspNonceContextKey is the context key for the CSP nonce.
type cspNonceContextKey struct{}

// generateNonce generates a cryptographically random 128-bit nonce encoded as
// unpadded base64. The result is safe for use in CSP 'nonce-...' directives.
func generateNonce() (string, error) {
	b := make([]byte, 16) // 128 bits
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("csp: generate nonce: %w", err)
	}
	return base64.RawStdEncoding.EncodeToString(b), nil
}

// GetCSPNonce retrieves the CSP nonce from the request context.
// Returns empty string when nonce is not present (feature flag off).
func GetCSPNonce(c *echo.Context) string {
	if v, ok := (*c).Request().Context().Value(cspNonceContextKey{}).(string); ok {
		return v
	}
	return ""
}

// cspNonceEnabled returns true when CSP_NONCE_ENABLED env var is set to a
// truthy value ("true", "1", "yes"). Default is false for gradual rollout.
func cspNonceEnabled() bool {
	v := os.Getenv("CSP_NONCE_ENABLED")
	switch strings.ToLower(v) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}

// buildCSP constructs a nonce-based Content-Security-Policy header for
// production. In this mode style-src uses 'nonce-{nonce}' instead of
// 'unsafe-inline', and connect-src removes ws: and data: (TASK-WEAK-001-4).
func buildCSP(nonce string, scriptHashes []string) string {
	parts := []string{
		"default-src 'self'",
		"script-src 'self' " + strings.Join(scriptHashes, " ") + " 'wasm-unsafe-eval'",
		fmt.Sprintf("style-src 'self' 'nonce-%s'", nonce),
		"img-src 'self' data: blob: discordapp.com",
		// Production: no ws:, no data: — only wss: for encrypted WebSocket (TASK-WEAK-001-4)
		"connect-src 'self' wss: https://api.github.com https://hub.bytebase.com",
		"font-src 'self'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'self'",
		"report-uri /api/csp-report",
	}
	return strings.Join(parts, "; ")
}

// buildCSPDev constructs a nonce-based CSP header for development mode.
// Same as buildCSP but includes ws: for localhost dev server HMR (TASK-WEAK-001-4).
func buildCSPDev(nonce string, scriptHashes []string) string {
	parts := []string{
		"default-src 'self'",
		"script-src 'self' " + strings.Join(scriptHashes, " ") + " 'wasm-unsafe-eval'",
		fmt.Sprintf("style-src 'self' 'nonce-%s'", nonce),
		"img-src 'self' data: blob: discordapp.com",
		// Dev: ws: allowed for HMR/localhost, wss: for production WebSocket (TASK-WEAK-001-4)
		"connect-src 'self' ws: wss: https://api.github.com https://hub.bytebase.com",
		"font-src 'self'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'self'",
		"report-uri /api/csp-report",
	}
	return strings.Join(parts, "; ")
}

// buildCSPLegacy returns the original CSP string with 'unsafe-inline' for
// style-src. Used as fallback when CSP_NONCE_ENABLED=false.
func buildCSPLegacy(scriptHashes []string) string {
	return "default-src 'self'; " +
		"script-src 'self' " + strings.Join(scriptHashes, " ") + " 'wasm-unsafe-eval'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob: discordapp.com; " +
		"connect-src 'self' data: ws: wss: https://api.github.com https://hub.bytebase.com; " +
		"font-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"frame-ancestors 'self'"
}

// newSecurityHeadersMiddleware creates the CSP middleware. When CSP nonce is
// enabled, each request gets a unique 128-bit nonce injected into the request
// context and the CSP header. Otherwise, the legacy CSP is used.
func newSecurityHeadersMiddleware(profile *config.Profile) echo.MiddlewareFunc {
	scriptHashes := loadCSPHashes()
	nonceEnabled := cspNonceEnabled()
	isDev := profile.Mode == common.ReleaseModeDev

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			// Allow popups to maintain window.opener for OAuth flows
			(*c).Response().Header().Set("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
			// Prevent being embedded in iframes from different origins
			(*c).Response().Header().Set("X-Frame-Options", "SAMEORIGIN")
			// Prevent MIME-type sniffing
			(*c).Response().Header().Set("X-Content-Type-Options", "nosniff")
			// Force HTTPS in production (only if request is already HTTPS)
			if (*c).Request().TLS != nil || (*c).Request().Header.Get("X-Forwarded-Proto") == "https" {
				(*c).Response().Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			var csp string
			if nonceEnabled {
				nonce, err := generateNonce()
				if err != nil {
					// Fallback to legacy CSP if nonce generation fails
					csp = buildCSPLegacy(scriptHashes)
				} else {
					// Store nonce in request context for HTML injection (TASK-WEAK-001-3)
					ctx := context.WithValue((*c).Request().Context(), cspNonceContextKey{}, nonce)
					(*c).SetRequest((*c).Request().WithContext(ctx))

					if isDev {
						csp = buildCSPDev(nonce, scriptHashes)
					} else {
						csp = buildCSP(nonce, scriptHashes)
					}
				}
			} else {
				csp = buildCSPLegacy(scriptHashes)
			}

			(*c).Response().Header().Set("Content-Security-Policy", csp)
			return next(c)
		}
	}
}
