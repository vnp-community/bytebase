CREATE TABLE IF NOT EXISTS backup_registry (
    id              BIGSERIAL PRIMARY KEY,
    backup_id       TEXT UNIQUE NOT NULL,
    backup_type     TEXT NOT NULL,          -- FULL, INCREMENTAL, WAL
    status          TEXT NOT NULL DEFAULT 'IN_PROGRESS',
    size_bytes      BIGINT NOT NULL DEFAULT 0,
    checksum_sha256 TEXT,
    storage_path    TEXT NOT NULL,
    storage_type    TEXT NOT NULL DEFAULT 'LOCAL',
    encryption      TEXT NOT NULL DEFAULT 'NONE',
    data_timestamp  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    verified_at     TIMESTAMPTZ,
    verify_result   TEXT,
    expires_at      TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '30 days',
    completed_at    TIMESTAMPTZ,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_backup_registry_status ON backup_registry (status);
CREATE INDEX idx_backup_registry_data_time ON backup_registry (data_timestamp DESC);
