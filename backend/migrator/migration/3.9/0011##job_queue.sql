CREATE TABLE IF NOT EXISTS job_queue (
    id           BIGSERIAL PRIMARY KEY,
    workspace    TEXT NOT NULL,
    job_type     TEXT NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}',
    priority     INT NOT NULL DEFAULT 0,
    status       TEXT NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','running','done','failed','dead')),
    locked_by    TEXT,
    locked_at    TIMESTAMPTZ,
    attempts     INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error_msg    TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    dedup_key    TEXT,
    UNIQUE (dedup_key) WHERE dedup_key IS NOT NULL
);

CREATE INDEX idx_job_queue_pending
    ON job_queue (priority DESC, scheduled_at ASC)
    WHERE status = 'pending';

CREATE INDEX idx_job_queue_workspace_pending
    ON job_queue (workspace, status)
    WHERE status IN ('pending', 'running');

CREATE INDEX idx_job_queue_locked
    ON job_queue (locked_at) WHERE status = 'running';

-- NOTIFY trigger for worker wake-up
CREATE OR REPLACE FUNCTION notify_job_queue()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('bytebase:job_queue', NEW.job_type);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_job_queue_notify
    AFTER INSERT ON job_queue
    FOR EACH ROW EXECUTE FUNCTION notify_job_queue();
