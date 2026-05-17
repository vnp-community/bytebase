package cmd

import (
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/bytebase/bytebase/backend/component/config"
)

func getBaseProfile(dataDir string) *config.Profile {
	config := &config.Profile{
		ExternalURL:         flags.externalURL,
		Port:                flags.port,     // Using flags.port as our gRPC server port.
		DatastorePort:       flags.port + 2, // Using flags.port + 2 as our datastore port.
		HA:                  flags.ha,
		SaaS:                flags.saas,
		Debug:               flags.debug,
		IsDocker:            isDocker(),
		DataDir:             dataDir,
		Demo:                flags.demo,
		Version:             version,
		GitCommit:           gitcommit,
		PgURL:               os.Getenv("PG_URL"),
		ReplicaID:           uuid.NewString(),
		StripeAPISecret:     os.Getenv("STRIPE_API_SECRET"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		LicensePrivateKey:   os.Getenv("LICENSE_PRIVATE_KEY"),

		BackupEnabled:       os.Getenv("BB_BACKUP_ENABLED") == "true",
		BackupSchedule:      getEnvOrDefault("BB_BACKUP_SCHEDULE", "0 2 * * *"),
		BackupPath:          getEnvOrDefault("BB_BACKUP_PATH", "/data/backups"),
		TargetRPOMinutes:    getEnvOrDefaultInt("BB_TARGET_RPO_MINUTES", 15),
		BackupEncryptionKey: os.Getenv("BB_BACKUP_ENCRYPTION_KEY"),

		RegionName:       os.Getenv("REGION_NAME"),
		RegionRole:       config.RegionRole(os.Getenv("REGION_ROLE")),
		PrimaryRegionURL: os.Getenv("PRIMARY_REGION_URL"),
	}

	config.LastActiveTS.Store(time.Now().Unix())
	config.RuntimeMemoryProfileThreshold.Store(flags.memoryProfileThreshold)
	return config
}

func isDocker() bool {
	if _, err := os.Stat("/etc/bb.env"); err == nil {
		return true
	}
	return false
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvOrDefaultInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return defaultVal
}
