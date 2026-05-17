// Package store — cache invalidation via PG NOTIFY.
//
// CacheInvalidator listens on the "cache_invalidation" PG NOTIFY channel and
// routes invalidation events to the appropriate store caches.
// This ensures cache coherence across multiple replicas when using Redis or LRU.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/bytebase/bytebase/backend/common/log"
)

const (
	// cacheInvalidationChannel is the PG NOTIFY channel for cache invalidation events.
	cacheInvalidationChannel = "cache_invalidation"
	// invalidatorReconnectBackoff is the delay before reconnecting after a PG connection error.
	invalidatorReconnectBackoff = 5 * time.Second
)

// cacheInvalidationPayload is the JSON structure sent via PG NOTIFY.
type cacheInvalidationPayload struct {
	Table  string `json:"table"`
	Action string `json:"action"`
	ID     string `json:"id"`
}

// CacheInvalidator listens for PG NOTIFY events on the cache_invalidation
// channel and invalidates the corresponding cache entries.
type CacheInvalidator struct {
	store *Store
	db    *sql.DB
}

// NewCacheInvalidator creates a new CacheInvalidator.
func NewCacheInvalidator(store *Store, db *sql.DB) *CacheInvalidator {
	return &CacheInvalidator{
		store: store,
		db:    db,
	}
}

// Run starts the cache invalidation listener loop.
// It reconnects automatically after PG connection drops.
func (ci *CacheInvalidator) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	slog.Info("Cache invalidator started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("Cache invalidator stopped")
			return
		default:
			if err := ci.listen(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("cache invalidator error, reconnecting",
					log.BBError(err),
					"backoff", invalidatorReconnectBackoff,
				)
				time.Sleep(invalidatorReconnectBackoff)
			}
		}
	}
}

// listen acquires a dedicated PG connection, subscribes to the cache_invalidation
// channel, and processes notifications until error or context cancellation.
func (ci *CacheInvalidator) listen(ctx context.Context) error {
	conn, err := ci.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Raw(func(driverConn any) error {
		pgxConn := driverConn.(*stdlib.Conn).Conn()

		_, err := pgxConn.Exec(ctx, "LISTEN "+cacheInvalidationChannel)
		if err != nil {
			return err
		}

		slog.Debug("Cache invalidator listening on PG channel", "channel", cacheInvalidationChannel)

		for {
			notification, err := pgxConn.WaitForNotification(ctx)
			if err != nil {
				return err
			}
			ci.handleNotification(ctx, notification.Payload)
		}
	})
}

// handleNotification parses the JSON payload and routes invalidation to the
// appropriate cache based on the source table.
func (ci *CacheInvalidator) handleNotification(_ context.Context, payload string) {
	var p cacheInvalidationPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		slog.Warn("cache invalidator: invalid payload", "payload", payload, log.BBError(err))
		return
	}

	slog.Debug("cache invalidation event",
		"table", p.Table,
		"action", p.Action,
		"id", p.ID,
	)

	switch p.Table {
	case "principal":
		ci.store.userEmailCache.Remove(p.ID)
	case "instance":
		ci.store.instanceCache.Remove(p.ID)
	case "db":
		ci.store.databaseCache.Remove(p.ID)
	case "project":
		ci.store.projectCache.Remove(p.ID)
	case "policy":
		ci.store.policyCache.Remove(p.ID)
	case "setting":
		ci.store.settingCache.Remove(p.ID)
	default:
		slog.Debug("cache invalidator: unhandled table", "table", p.Table)
	}
}
