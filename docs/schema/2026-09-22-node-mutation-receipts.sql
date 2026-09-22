-- 条件节点写入的持久回执；必须与节点更新在同一事务提交。
-- 不自动清理回执，避免旧 operation ID 被再次执行。
BEGIN;

CREATE TABLE IF NOT EXISTS node_mutation_receipts (
    actor_id text NOT NULL,
    operation_id uuid NOT NULL,
    library_id bigint NOT NULL,
    kind text NOT NULL,
    request_hash text NOT NULL,
    result jsonb NOT NULL,
    committed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (actor_id, operation_id),
    CONSTRAINT chk_node_mutation_receipt_library CHECK (library_id > 0),
    CONSTRAINT chk_node_mutation_receipt_kind CHECK (kind = 'rename_batch'),
    CONSTRAINT chk_node_mutation_receipt_hash CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT chk_node_mutation_receipt_result CHECK (jsonb_typeof(result) = 'object')
);

-- 统一历史安装中的索引形状；重复可见名称会令本迁移整体失败，不自动改写用户节点。
DROP INDEX IF EXISTS uq_nodes_live_sibling_visible_name;
CREATE UNIQUE INDEX uq_nodes_live_sibling_visible_name ON nodes (
    library_id,
    COALESCE(parent_id, 0),
    (CASE WHEN node_type = 1 AND COALESCE(ext, '') <> '' THEN name || '.' || ext ELSE name END)
) WHERE deleted_at IS NULL;

COMMIT;
