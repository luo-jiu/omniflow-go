-- OmniFlow PostgreSQL schema patch.
--
-- Purpose:
-- 1. Introduce upload_sessions to back the direct-to-MinIO upload pipeline
--    (S3 multipart + presigned PUT). Backend issues uploadId and storageKey,
--    client streams bytes straight to object storage.
-- 2. Two-layer TTL: row-level expires_at acts as a refreshable lease,
--    while presigned URL signatures stay short-lived and re-issuable.
-- 3. Janitor sweeps rows past expires_at, aborting MinIO multipart leftovers.
-- 4. Completed rows become short-lived receipts so complete can be retried safely.

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
    status text NOT NULL DEFAULT 'pending',
    client_operation_id text,
    completed_node_id bigint,
    completion_result jsonb,
    completed_at timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_upload_sessions_mode CHECK (mode IN ('single', 'multipart')),
    CONSTRAINT chk_upload_sessions_status CHECK (status IN ('pending', 'committed')),
    CONSTRAINT chk_upload_sessions_completion CHECK (
        (status = 'pending' AND completed_node_id IS NULL AND completion_result IS NULL AND completed_at IS NULL)
        OR
        (
            status = 'committed'
            AND client_operation_id IS NOT NULL
            AND completed_node_id IS NOT NULL
            AND jsonb_typeof(completion_result) = 'object'
            AND completed_at IS NOT NULL
        )
    ),
    CONSTRAINT chk_upload_sessions_operation_length CHECK (
        client_operation_id IS NULL OR char_length(client_operation_id) <= 128
    ),
    CONSTRAINT chk_upload_sessions_file_size_nonneg CHECK (file_size >= 0),
    CONSTRAINT chk_upload_sessions_part_size_pos CHECK (part_size > 0)
);

-- Existing installations may already have the original table. Keep this patch
-- rerunnable so local development and deployment use the same migration file.
ALTER TABLE upload_sessions
    ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS client_operation_id text,
    ADD COLUMN IF NOT EXISTS completed_node_id bigint,
    ADD COLUMN IF NOT EXISTS completion_result jsonb,
    ADD COLUMN IF NOT EXISTS completed_at timestamptz;

-- Recreate this constraint so rerunning the patch also upgrades installations
-- that briefly used the node-id-only completion receipt.
ALTER TABLE upload_sessions
    DROP CONSTRAINT IF EXISTS chk_upload_sessions_completion;

ALTER TABLE upload_sessions
    ADD CONSTRAINT chk_upload_sessions_completion CHECK (
        (status = 'pending' AND completed_node_id IS NULL AND completion_result IS NULL AND completed_at IS NULL)
        OR
        (
            status = 'committed'
            AND client_operation_id IS NOT NULL
            AND completed_node_id IS NOT NULL
            AND jsonb_typeof(completion_result) = 'object'
            AND completed_at IS NOT NULL
        )
    );

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'chk_upload_sessions_status'
          AND conrelid = 'upload_sessions'::regclass
    ) THEN
        ALTER TABLE upload_sessions
            ADD CONSTRAINT chk_upload_sessions_status
            CHECK (status IN ('pending', 'committed'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'chk_upload_sessions_operation_length'
          AND conrelid = 'upload_sessions'::regclass
    ) THEN
        ALTER TABLE upload_sessions
            ADD CONSTRAINT chk_upload_sessions_operation_length
            CHECK (client_operation_id IS NULL OR char_length(client_operation_id) <= 128);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_upload_sessions_expires_at
    ON upload_sessions (expires_at);

CREATE INDEX IF NOT EXISTS idx_upload_sessions_actor
    ON upload_sessions (actor_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_upload_sessions_actor_operation
    ON upload_sessions (actor_id, client_operation_id)
    WHERE client_operation_id IS NOT NULL;

-- updated_at 由仓储层（gorm）在写入时自动维护，不依赖数据库触发器，
-- 与本仓库其他表的现状保持一致（pg_trigger 当前为空）。

COMMIT;
