// Package errors provides standardized error types for Bytebase services.
package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ServiceError is a standardized error type for cross-service communication.
type ServiceError struct {
	Code       string `json:"code"`                  // e.g., "DCM_PLAN_NOT_FOUND"
	Message    string `json:"message"`               // Human-readable description
	Service    string `json:"service"`               // Service that generated the error
	TraceID    string `json:"trace_id,omitempty"`     // OTel trace ID for debugging
	RetryAfter int    `json:"retry_after,omitempty"`  // Seconds until retry is safe (0 = not retryable)
	HTTPStatus int    `json:"-"`                      // HTTP status code (not serialized)
}

// Error implements the error interface.
func (e *ServiceError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Service, e.Code, e.Message)
}

// WriteHTTP writes the error as a JSON response.
func (e *ServiceError) WriteHTTP(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if e.RetryAfter > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", e.RetryAfter))
	}
	status := e.HTTPStatus
	if status == 0 {
		status = http.StatusInternalServerError
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(e)
}

// --- Error constructors ---

// NotFound creates a 404 error.
func NotFound(service, code, msg string) *ServiceError {
	return &ServiceError{
		Code:       code,
		Message:    msg,
		Service:    service,
		HTTPStatus: http.StatusNotFound,
	}
}

// Internal creates a 500 error.
func Internal(service, code, msg string) *ServiceError {
	return &ServiceError{
		Code:       code,
		Message:    msg,
		Service:    service,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// Unavailable creates a 503 error with retry-after.
func Unavailable(service, code, msg string, retryAfter int) *ServiceError {
	return &ServiceError{
		Code:       code,
		Message:    msg,
		Service:    service,
		HTTPStatus: http.StatusServiceUnavailable,
		RetryAfter: retryAfter,
	}
}

// Timeout creates a 504 error.
func Timeout(service, code, msg string, retryAfter int) *ServiceError {
	return &ServiceError{
		Code:       code,
		Message:    msg,
		Service:    service,
		HTTPStatus: http.StatusGatewayTimeout,
		RetryAfter: retryAfter,
	}
}

// WrapError wraps a Go error into a ServiceError.
func WrapError(err error, service, traceID string) *ServiceError {
	return &ServiceError{
		Code:       service + "_INTERNAL_ERROR",
		Message:    err.Error(),
		Service:    service,
		TraceID:    traceID,
		HTTPStatus: http.StatusInternalServerError,
	}
}
