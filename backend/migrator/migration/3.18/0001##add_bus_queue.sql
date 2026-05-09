-- Bus queue table for durable message persistence (HA mode).
-- Replaces volatile Go channels with a PG-backed queue that survives restarts
-- and supports multi-instance consumption via SELECT FOR UPDATE SKIP LOCKED.

CREATE TABLE IF NOT EXISTS bus_queue (
    id           BIGSERIAL     PRIMARY KEY,
    channel      TEXT          NOT NULL,
    payload      JSONB         NOT NULL,
    status       TEXT          NOT NULL DEFAULT 'pending',
    priority     INT           NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    claimed_by   TEXT,
    claimed_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    attempts     INT           NOT NULL DEFAULT 0,
    max_retries  INT           NOT NULL DEFAULT 3,
    error_msg    TEXT
);

-- Index for consumer polling: pending messages by priority (desc) then FIFO.
CREATE INDEX IF NOT EXISTS idx_bus_queue_pending
    ON bus_queue (channel, priority DESC, id ASC)
    WHERE status = 'pending';

-- Index for stale claim recovery: find processing messages by claim time.
CREATE INDEX IF NOT EXISTS idx_bus_queue_processing
    ON bus_queue (claimed_at)
    WHERE status = 'processing';

-- Index for completed message cleanup (periodic GC).
CREATE INDEX IF NOT EXISTS idx_bus_queue_completed
    ON bus_queue (completed_at)
    WHERE status IN ('done', 'failed');
