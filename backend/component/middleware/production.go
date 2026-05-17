// Package middleware provides production-grade HTTP middleware for Bytebase services.
package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/bytebase/bytebase/backend/component/metrics"
	"go.opentelemetry.io/otel/trace"
)

// InternalAuthHeader is the header name for internal service authentication.
const InternalAuthHeader = "X-Bytebase-Internal-Auth"

// PanicRecoveryMiddleware recovers from panics and returns 500.
func PanicRecoveryMiddleware(serviceName string, m *metrics.ServiceMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					stack := debug.Stack()
					slog.Error("panic recovered",
						"service", serviceName,
						"method", r.URL.Path,
						"error", fmt.Sprintf("%v", err),
						"stack", string(stack),
					)
					if m != nil {
						m.PanicTotal.WithLabelValues(serviceName).Inc()
					}
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// MetricsMiddleware records request count, duration, and active requests.
func MetricsMiddleware(m *metrics.ServiceMetrics, serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m == nil {
				next.ServeHTTP(w, r)
				return
			}

			m.ActiveRequests.WithLabelValues(serviceName).Inc()
			start := time.Now()

			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			m.ActiveRequests.WithLabelValues(serviceName).Dec()
			m.RequestDuration.WithLabelValues(serviceName, r.URL.Path).Observe(duration.Seconds())
			m.RequestTotal.WithLabelValues(serviceName, r.URL.Path, strconv.Itoa(rw.statusCode)).Inc()
		})
	}
}

// LoggingMiddleware logs every request with structured fields including trace_id.
func LoggingMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r)

			traceID := ""
			if span := trace.SpanFromContext(r.Context()); span.SpanContext().IsValid() {
				traceID = span.SpanContext().TraceID().String()
			}

			slog.Info("request",
				"service", serviceName,
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.statusCode,
				"duration_ms", time.Since(start).Milliseconds(),
				"trace_id", traceID,
			)
		})
	}
}

// InternalAuthMiddleware validates internal HMAC tokens on requests.
func InternalAuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secret == "" {
				// No secret configured — skip auth (development mode).
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get(InternalAuthHeader)
			if token == "" {
				http.Error(w, "missing internal auth token", http.StatusForbidden)
				return
			}

			if !ValidateHMAC(secret, token) {
				http.Error(w, "invalid internal auth token", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TimeoutMiddleware enforces a maximum request duration.
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if timeout <= 0 {
			return next
		}
		return http.TimeoutHandler(next, timeout, "request timeout")
	}
}

// --- HMAC Helpers ---

// SignHMAC creates an HMAC token with a timestamp.
func SignHMAC(secret string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d", timestamp)))
	return fmt.Sprintf("%d:%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}

// ValidateHMAC checks an HMAC token. Accepts tokens within 60 seconds of current time.
func ValidateHMAC(secret, token string) bool {
	now := time.Now().Unix()
	// Try timestamps within a 60-second window.
	for delta := int64(-30); delta <= 30; delta++ {
		expected := SignHMAC(secret, now+delta)
		if hmac.Equal([]byte(expected), []byte(token)) {
			return true
		}
	}
	return false
}

// --- responseWriter wrapper ---

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
