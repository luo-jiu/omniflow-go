-- OmniFlow PostgreSQL schema patch.
--
-- Purpose:
-- 1. Introduce upload_sessions to back the direct-to-MinIO upload pipeline
--    (S3 multipart + presigned PUT). Backend issues uploadId and storageKey,
--    client streams bytes straight to object storage.
-- 2. Two-layer TTL: row-level expires_at acts as a refreshable lease,
--    while presigned URL signatures stay short-lived and re-issuable.
-- 3. Janitor sweeps rows past expires_at, aborting MinIO multipart leftovers.

BEGIN;

CREATE TABLE IF NOT EXISTS upload_sessions (
    id text PRIMARY KEY,
    library_id bigint NOT NULL,
    parent_id bigint,
    actor_id text NOT NULL,
    storage_key text NOT NULL,
    file_name text NOT NULL,
    file_size bigint NOT NULL,
    content_type text NOT NULL,
    storage_provider text NOT NULL,
    mode text NOT NULL,
    minio_upload_id text,
    part_size bigint NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_upload_sessions_mode CHECK (mode IN ('single', 'multipart')),
    CONSTRAINT chk_upload_sessions_file_size_nonneg CHECK (file_size >= 0),
    CONSTRAINT chk_upload_sessions_part_size_pos CHECK (part_size > 0)
);

CREATE INDEX IF NOT EXISTS idx_upload_sessions_expires_at
    ON upload_sessions (expires_at);

CREATE INDEX IF NOT EXISTS idx_upload_sessions_actor
    ON upload_sessions (actor_id);

-- updated_at 由仓储层（gorm）在写入时自动维护，不依赖数据库触发器，
-- 与本仓库其他表的现状保持一致（pg_trigger 当前为空）。

COMMIT;
