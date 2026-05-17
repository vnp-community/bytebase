// Package config provides service-level configuration and feature flags.
package config

import (
	"os"
	"strconv"
	"time"
)

// ServiceConfig holds configuration for internal services.
type ServiceConfig struct {
	// DefaultTimeout is the default request timeout for service handlers.
	DefaultTimeout time.Duration

	// CircuitBreaker settings.
	CBMaxRequests uint32
	CBInterval    time.Duration
	CBTimeout     time.Duration
	CBFailureRate float64

	// NATS settings.
	NATSMaxReconnects  int
	NATSReconnectWait  time.Duration
	NATSMaxPendingMsgs int

	// Observability.
	TraceSampleRate float64
	MetricsEnabled  bool
	LogLevel        string

	// Internal auth.
	InternalSecret string
}

// DefaultServiceConfig returns production-safe defaults.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		DefaultTimeout:     30 * time.Second,
		CBMaxRequests:      5,
		CBInterval:         30 * time.Second,
		CBTimeout:          10 * time.Second,
		CBFailureRate:      0.5,
		NATSMaxReconnects:  10,
		NATSReconnectWait:  2 * time.Second,
		NATSMaxPendingMsgs: 65536,
		TraceSampleRate:    0.1,
		MetricsEnabled:     true,
		LogLevel:           "info",
		InternalSecret:     "",
	}
}

// LoadFromEnv overrides defaults with environment variables.
func (c *ServiceConfig) LoadFromEnv() {
	if v := os.Getenv("BB_DEFAULT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.DefaultTimeout = d
		}
	}
	if v := os.Getenv("BB_TRACE_SAMPLE_RATE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.TraceSampleRate = f
		}
	}
	if v := os.Getenv("BB_METRICS_ENABLED"); v != "" {
		c.MetricsEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("BB_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
	if v := os.Getenv("BB_INTERNAL_SECRET"); v != "" {
		c.InternalSecret = v
	}
}

// FeatureFlags controls gradual rollout of new features.
type FeatureFlags struct {
	// UseNATSBus enables NATSBus instead of Go channels bus.
	UseNATSBus bool

	// EnableTracing enables OpenTelemetry distributed tracing.
	EnableTracing bool

	// EnableCircuitBreaker enables circuit breaker on gateway→service calls.
	EnableCircuitBreaker bool

	// InternalAuthEnabled enables HMAC auth for internal service calls.
	InternalAuthEnabled bool
}

// DefaultFeatureFlags returns conservative defaults (all disabled for safe rollout).
func DefaultFeatureFlags() FeatureFlags {
	return FeatureFlags{
		UseNATSBus:           false,
		EnableTracing:        false,
		EnableCircuitBreaker: false,
		InternalAuthEnabled:  false,
	}
}

// LoadFromEnv overrides feature flags from environment variables.
func (f *FeatureFlags) LoadFromEnv() {
	f.UseNATSBus = envBool("BB_USE_NATS_BUS")
	f.EnableTracing = envBool("BB_ENABLE_TRACING")
	f.EnableCircuitBreaker = envBool("BB_ENABLE_CIRCUIT_BREAKER")
	f.InternalAuthEnabled = envBool("BB_INTERNAL_AUTH_ENABLED")
}

func envBool(key string) bool {
	v := os.Getenv(key)
	return v == "true" || v == "1"
}
