package store

import (
	"database/sql"
	"testing"
	"time"
)

func TestReadReplicaPool_NoReplica_ForReadReturnsPrimary(t *testing.T) {
	primary := &sql.DB{}
	pool := &ReadReplicaPool{
		primary:      primary,
		lagThreshold: 5 * time.Second,
	}

	got := pool.ForRead()
	if got != primary {
		t.Error("ForRead() should return primary when no replica is configured")
	}
}

func TestReadReplicaPool_NoReplica_ForWriteReturnsPrimary(t *testing.T) {
	primary := &sql.DB{}
	pool := &ReadReplicaPool{
		primary:      primary,
		lagThreshold: 5 * time.Second,
	}

	got := pool.ForWrite()
	if got != primary {
		t.Error("ForWrite() should always return primary")
	}
}

func TestReadReplicaPool_ReplicaLagBelowThreshold(t *testing.T) {
	primary := &sql.DB{}
	replica := &sql.DB{}
	pool := &ReadReplicaPool{
		primary:      primary,
		replica:      replica,
		lagThreshold: 5 * time.Second,
	}
	// Set lag to 1 second (below 5s threshold)
	pool.replicaLag.Store(1_000_000) // 1 second in microseconds

	got := pool.ForRead()
	if got != replica {
		t.Error("ForRead() should return replica when lag is below threshold")
	}
}

func TestReadReplicaPool_ReplicaLagAboveThreshold(t *testing.T) {
	primary := &sql.DB{}
	replica := &sql.DB{}
	pool := &ReadReplicaPool{
		primary:      primary,
		replica:      replica,
		lagThreshold: 5 * time.Second,
	}
	// Set lag to 10 seconds (above 5s threshold)
	pool.replicaLag.Store(10_000_000) // 10 seconds in microseconds

	got := pool.ForRead()
	if got != primary {
		t.Error("ForRead() should return primary when lag exceeds threshold")
	}
}

func TestReadReplicaPool_ForWriteAlwaysPrimary(t *testing.T) {
	primary := &sql.DB{}
	replica := &sql.DB{}
	pool := &ReadReplicaPool{
		primary:      primary,
		replica:      replica,
		lagThreshold: 5 * time.Second,
	}
	// Even with healthy replica, ForWrite should always return primary
	pool.replicaLag.Store(0)

	got := pool.ForWrite()
	if got != primary {
		t.Error("ForWrite() should always return primary, even with healthy replica")
	}
}

func TestReadReplicaPool_ReplicaLag(t *testing.T) {
	pool := &ReadReplicaPool{
		primary:      &sql.DB{},
		lagThreshold: 5 * time.Second,
	}

	pool.replicaLag.Store(2_500_000) // 2.5 seconds
	lag := pool.ReplicaLag()

	expected := 2500 * time.Millisecond
	if lag != expected {
		t.Errorf("ReplicaLag() = %v, want %v", lag, expected)
	}
}
