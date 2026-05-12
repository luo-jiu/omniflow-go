-- OmniFlow PostgreSQL schema patch.
--
-- Purpose:
-- 1. 引入 resource_monitor_samples，承载资源监测控制台的历史采样基座。
-- 2. 第一版只写入显式采样结果，不启动后台定时任务。
-- 3. summary 字段用于趋势查询，snapshot_json 保留完整快照，便于后续告警和详情回放。

BEGIN;

CREATE TABLE IF NOT EXISTS resource_monitor_samples (
    id bigserial PRIMARY KEY,
    actor_id text NOT NULL,
    scope text NOT NULL,
    library_id bigint NOT NULL DEFAULT 0,
    generated_at timestamptz NOT NULL,
    provider_count integer NOT NULL DEFAULT 0,
    bucket_count integer NOT NULL DEFAULT 0,
    object_count bigint NOT NULL DEFAULT 0,
    file_ref_count bigint NOT NULL DEFAULT 0,
    physical_bytes bigint NOT NULL DEFAULT 0,
    visible_object_count bigint NOT NULL DEFAULT 0,
    visible_file_ref_count bigint NOT NULL DEFAULT 0,
    visible_bytes bigint NOT NULL DEFAULT 0,
    recycle_object_count bigint NOT NULL DEFAULT 0,
    recycle_file_ref_count bigint NOT NULL DEFAULT 0,
    recycle_bytes bigint NOT NULL DEFAULT 0,
    orphan_object_count bigint NOT NULL DEFAULT 0,
    orphan_bytes bigint NOT NULL DEFAULT 0,
    unmatched_count integer NOT NULL DEFAULT 0,
    legacy_provider_count integer NOT NULL DEFAULT 0,
    probe_total integer NOT NULL DEFAULT 0,
    probe_ok integer NOT NULL DEFAULT 0,
    probe_error integer NOT NULL DEFAULT 0,
    probe_unknown integer NOT NULL DEFAULT 0,
    distribution_error text NOT NULL DEFAULT '',
    snapshot_json jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_resource_monitor_samples_scope
        CHECK (scope IN ('global', 'library')),
    CONSTRAINT chk_resource_monitor_samples_library_scope
        CHECK ((scope = 'global' AND library_id = 0) OR (scope = 'library' AND library_id > 0)),
    CONSTRAINT chk_resource_monitor_samples_counts_nonneg
        CHECK (
            provider_count >= 0
            AND bucket_count >= 0
            AND object_count >= 0
            AND file_ref_count >= 0
            AND physical_bytes >= 0
            AND visible_object_count >= 0
            AND visible_file_ref_count >= 0
            AND visible_bytes >= 0
            AND recycle_object_count >= 0
            AND recycle_file_ref_count >= 0
            AND recycle_bytes >= 0
            AND orphan_object_count >= 0
            AND orphan_bytes >= 0
            AND unmatched_count >= 0
            AND legacy_provider_count >= 0
            AND probe_total >= 0
            AND probe_ok >= 0
            AND probe_error >= 0
            AND probe_unknown >= 0
        )
);

CREATE INDEX IF NOT EXISTS idx_resource_monitor_samples_actor_scope_time
    ON resource_monitor_samples (actor_id, scope, library_id, generated_at DESC);

CREATE INDEX IF NOT EXISTS idx_resource_monitor_samples_created_at
    ON resource_monitor_samples (created_at DESC);

COMMIT;
