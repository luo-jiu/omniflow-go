-- 条件目标目录语义：仅 Agent 产物等明确要求固定父目录的上传会话使用。
-- 普通上传默认 false，继续保留历史 parent 缺失时回退资料库根目录的兼容行为。
BEGIN;

ALTER TABLE upload_sessions
    ADD COLUMN IF NOT EXISTS strict_parent boolean NOT NULL DEFAULT false;

COMMIT;
