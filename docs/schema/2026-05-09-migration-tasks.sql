-- OmniFlow PostgreSQL schema patch.
--
-- Purpose:
-- 1. 引入 migration_tasks / migration_task_items，承载"右键迁移文件物理位置"能力。
--    一个 task = 一次"把某个 root_node_id 子树下的所有 storage_objects 搬到 target_provider"的请求。
--    一个 item = 一次具体物理对象（storage_object_id）的搬运单元。
-- 2. 多 worker 并发抢占 items：使用 (status, id) 部分索引 + SELECT ... FOR UPDATE SKIP LOCKED。
-- 3. 任务失败不破坏 node：item 完成时在 usecase 内一次事务里 INSERT 新 storage_objects 行 → UPDATE
--    所有 node_files.storage_object_id 引用 → DELETE 旧 storage_objects 行；事务外再 best-effort 删源对象。
-- 4. status 取值集合（全部 text，不引 enum 类型，方便后续扩展）：
--    - migration_tasks.status: pending / running / completed / failed / canceled
--    - migration_task_items.status: pending / running / done / failed / skipped

BEGIN;

CREATE TABLE IF NOT EXISTS migration_tasks (
    id text PRIMARY KEY,
    actor_id text NOT NULL,
    library_id bigint NOT NULL,
    root_node_id bigint NOT NULL,
    target_provider text NOT NULL,
    status text NOT NULL,
    total_objects integer NOT NULL DEFAULT 0,
    completed_objects integer NOT NULL DEFAULT 0,
    failed_objects integer NOT NULL DEFAULT 0,
    skipped_objects integer NOT NULL DEFAULT 0,
    total_bytes bigint NOT NULL DEFAULT 0,
    transferred_bytes bigint NOT NULL DEFAULT 0,
    current_object_key text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz,
    CONSTRAINT chk_migration_tasks_status
        CHECK (status IN ('pending', 'running', 'completed', 'failed', 'canceled')),
    CONSTRAINT chk_migration_tasks_counts_nonneg
        CHECK (total_objects >= 0 AND completed_objects >= 0 AND failed_objects >= 0 AND skipped_objects >= 0),
    CONSTRAINT chk_migration_tasks_bytes_nonneg
        CHECK (total_bytes >= 0 AND transferred_bytes >= 0)
);

CREATE INDEX IF NOT EXISTS idx_migration_tasks_status
    ON migration_tasks (status);

CREATE INDEX IF NOT EXISTS idx_migration_tasks_actor_library
    ON migration_tasks (actor_id, library_id);

CREATE TABLE IF NOT EXISTS migration_task_items (
    id bigserial PRIMARY KEY,
    task_id text NOT NULL,
    storage_object_id bigint NOT NULL,
    source_provider text NOT NULL,
    source_bucket text NOT NULL,
    source_key text NOT NULL,
    target_storage_object_id bigint,
    target_key text NOT NULL,
    file_size bigint NOT NULL DEFAULT 0,
    status text NOT NULL,
    error_message text NOT NULL DEFAULT '',
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_migration_task_items_status
        CHECK (status IN ('pending', 'running', 'done', 'failed', 'skipped')),
    CONSTRAINT chk_migration_task_items_file_size_nonneg
        CHECK (file_size >= 0)
);

CREATE INDEX IF NOT EXISTS idx_migration_task_items_task
    ON migration_task_items (task_id);

CREATE INDEX IF NOT EXISTS idx_migration_task_items_status
    ON migration_task_items (status);

-- worker 抢占索引：只覆盖 status='pending' 行，避免大表全索引膨胀。
CREATE INDEX IF NOT EXISTS idx_migration_task_items_pending
    ON migration_task_items (status, id) WHERE status = 'pending';

COMMIT;
