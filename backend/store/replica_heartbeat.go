package store

import (
	"context"
	"time"

	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/common/qb"
	"github.com/bytebase/bytebase/backend/store/model"
)

// UpsertReplicaHeartbeat updates or inserts a replica heartbeat.
func (s *Store) UpsertReplicaHeartbeat(ctx context.Context, node *model.ReplicaNode) error {
	q := qb.Q().Space(`
		INSERT INTO replica_heartbeat (replica_id, endpoint_url, version, status, capabilities, metadata, last_heartbeat)
		VALUES (?, ?, ?, ?, ?, ?, now())
		ON CONFLICT (replica_id)
		DO UPDATE SET
			endpoint_url = EXCLUDED.endpoint_url,
			version = EXCLUDED.version,
			status = EXCLUDED.status,
			capabilities = EXCLUDED.capabilities,
			metadata = EXCLUDED.metadata,
			last_heartbeat = now()
	`, node.ReplicaID, node.EndpointURL, node.Version, node.Status, pq.Array(node.Capabilities), node.Metadata)

	query, args, err := q.ToSQL()
	if err != nil {
		return errors.Wrapf(err, "failed to build sql")
	}

	if _, err := s.GetDB().ExecContext(ctx, query, args...); err != nil {
		return errors.Wrapf(err, "failed to upsert replica heartbeat")
	}
	return nil
}

// DeleteStaleReplicaHeartbeats deletes heartbeat rows older than the given duration.
func (s *Store) DeleteStaleReplicaHeartbeats(ctx context.Context, olderThan time.Duration) (int64, error) {
	q := qb.Q().Space(`
		DELETE FROM replica_heartbeat
		WHERE last_heartbeat < now() - ?::INTERVAL
	`, olderThan.String())

	query, args, err := q.ToSQL()
	if err != nil {
		return 0, errors.Wrapf(err, "failed to build sql")
	}

	result, err := s.GetDB().ExecContext(ctx, query, args...)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to delete stale replica heartbeats")
	}
	return result.RowsAffected()
}

// CountActiveReplicas returns the count of replicas with recent heartbeats.
// The `within` parameter defines the time window for considering a replica active.
// Replicas without heartbeats within this window are considered inactive.
func (s *Store) CountActiveReplicas(ctx context.Context, within time.Duration) (int, error) {
	q := qb.Q().Space(`
		SELECT COUNT(*) FROM replica_heartbeat
		WHERE last_heartbeat > now() - ?::INTERVAL
	`, within.String())

	query, args, err := q.ToSQL()
	if err != nil {
		return 0, errors.Wrapf(err, "failed to build sql")
	}

	var count int
	if err := s.GetDB().QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, errors.Wrapf(err, "failed to count active replicas")
	}
	return count, nil
}

// MarkStaleReplicas marks replicas as UNHEALTHY if their last heartbeat is older than threshold.
func (s *Store) MarkStaleReplicas(ctx context.Context, threshold time.Duration) (int64, error) {
	q := qb.Q().Space(`
		UPDATE replica_heartbeat
		SET status = 'UNHEALTHY'
		WHERE status IN ('STARTING', 'READY') AND last_heartbeat < now() - ?::INTERVAL
	`, threshold.String())

	query, args, err := q.ToSQL()
	if err != nil {
		return 0, errors.Wrapf(err, "failed to build sql")
	}

	result, err := s.GetDB().ExecContext(ctx, query, args...)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to mark stale replicas")
	}
	return result.RowsAffected()
}

// ListActiveReplicas returns a list of replicas with recent heartbeats.
func (s *Store) ListActiveReplicas(ctx context.Context, within time.Duration) ([]*model.ReplicaNode, error) {
	q := qb.Q().Space(`
		SELECT replica_id, endpoint_url, version, status, capabilities, metadata, started_at, last_heartbeat
		FROM replica_heartbeat
		WHERE last_heartbeat > now() - ?::INTERVAL
		ORDER BY replica_id
	`, within.String())

	query, args, err := q.ToSQL()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build sql")
	}

	rows, err := s.GetDB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list active replicas")
	}
	defer rows.Close()

	var nodes []*model.ReplicaNode
	for rows.Next() {
		node := &model.ReplicaNode{}
		if err := rows.Scan(
			&node.ReplicaID,
			&node.EndpointURL,
			&node.Version,
			&node.Status,
			pq.Array(&node.Capabilities),
			&node.Metadata,
			&node.StartedAt,
			&node.LastHeartbeat,
		); err != nil {
			return nil, errors.Wrapf(err, "failed to scan replica node")
		}
		nodes = append(nodes, node)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "failed to iterate active replicas")
	}

	return nodes, nil
}
