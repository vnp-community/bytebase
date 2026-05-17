// Package config includes all the server configurations in a component.
package config

import (
	"sync/atomic"

	"github.com/bytebase/bytebase/backend/common"
)

type RegionRole string

const (
	RegionRolePrimary RegionRole = "PRIMARY"
	RegionRoleStandby RegionRole = "STANDBY"
	RegionRoleDR      RegionRole = "DR"
)

// Profile is the configuration to start main server.
// Profile must not be copied, its fields must not be modified unless mentioned otherwise.
type Profile struct {
	// Mode can be "prod" or "dev"
	Mode common.ReleaseMode
	// ExternalURL is the URL user visits Bytebase.
	ExternalURL string
	// DatastorePort is the binding port for database instance for storing Bytebase metadata.
	// Only applicable when using embedded PG (PgURL is empty).
	DatastorePort int
	// Port is the binding port for the server.
	Port int
	// When we are running in SaaS mode, some features are not allowed to edit by users.
	SaaS bool
	// Stripe configuration for SaaS subscription purchase. Only used when SaaS is true.
	StripeAPISecret     string
	StripeWebhookSecret string
	// LicensePrivateKey is the PEM-encoded RSA private key for signing license JWTs (SaaS only).
	LicensePrivateKey string
	// DataDir is the directory stores the data including Bytebase's own database, backups, etc.
	DataDir string
	// Demo mode.
	Demo bool
	// HA replica mode.
	HA bool
	// Enable debug level. Only works when SaaS is true.
	Debug bool

	// Version is the bytebase's server version
	Version string
	// Git commit hash of the build
	GitCommit string
	// PgURL is the optional external PostgreSQL instance connection url
	PgURL string

	// LastActiveTS is the service last active timestamp, any API calls will refresh this value.
	LastActiveTS atomic.Int64
	// Unique ID per Bytebase replica run.
	ReplicaID string
	// Whether the server is running in a docker container.
	IsDocker bool

	// can be set in runtime
	RuntimeDebug atomic.Bool
	// RuntimeMemoryProfileThreshold is the memory threshold in bytes for the server to trigger a pprof memory profile.
	// can be set in runtime
	// 0 means no threshold.
	RuntimeMemoryProfileThreshold atomic.Uint64
	// RuntimeEnableAuditLogStdout enables audit logging to stdout in structured JSON format.
	// can be set in runtime via workspace setting
	RuntimeEnableAuditLogStdout atomic.Bool

	// --- Enterprise Feature Flags ---

	// DurableBus enables the PG-backed durable message bus instead of in-memory channels.
	// Requires bus_queue table (migration 3.18/0001). Default: false (in-memory bus).
	DurableBus bool
	// CacheBackend selects the cache implementation: "lru" (default), "redis", "noop".
	CacheBackend string
	// CacheRedisURL is the Redis/Valkey connection URL when CacheBackend is "redis".
	CacheRedisURL string
	// DualPool enables API/Runner connection pool isolation. Default: false (single pool).
	DualPool bool

	// --- Backup Feature Flags ---

	// BackupEnabled enables the automated backup scheduler.
	BackupEnabled bool
	// BackupSchedule is the cron schedule for full backups.
	BackupSchedule string
	// BackupPath is the directory to store backups.
	BackupPath string
	// TargetRPOMinutes is the target RPO for monitoring.
	TargetRPOMinutes int
	// BackupEncryptionKey is the AES-256-GCM key for encrypting backups.
	BackupEncryptionKey string

	// --- Multi-Region Feature Flags ---
	RegionName       string     // env: REGION_NAME
	RegionRole       RegionRole // env: REGION_ROLE
	PrimaryRegionURL string     // env: PRIMARY_REGION_URL
}

func (p *Profile) IsStandby() bool {
	return p.RegionRole == RegionRoleStandby
}

func (p *Profile) IsPrimary() bool {
	return p.RegionRole == "" || p.RegionRole == RegionRolePrimary
}

// UseEmbedDB returns whether to use embedDB.
func (prof *Profile) UseEmbedDB() bool {
	return len(prof.PgURL) == 0
}

var saasFeatureControlMap = map[string]bool{}

// IsFeatureUnavailable returns if the feature is unavailable in SaaS mode.
func (prof *Profile) IsFeatureUnavailable(feature string) bool {
	return prof.SaaS && saasFeatureControlMap[feature]
}
