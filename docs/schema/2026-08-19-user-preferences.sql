-- OmniFlow PostgreSQL schema patch.
--
-- Purpose:
-- 1. 引入 user_preferences，按用户和命名空间保存可跨设备同步的个人偏好。
-- 2. data 仅保存对象类型 JSON；schema_version 管理数据结构演进。
-- 3. revision 为乐观并发控制版本，避免设备间使用旧数据静默覆盖新设置。

BEGIN;

CREATE TABLE IF NOT EXISTS user_preferences (
    user_id bigint NOT NULL,
    namespace varchar(64) NOT NULL,
    data jsonb NOT NULL DEFAULT '{}'::jsonb,
    schema_version integer NOT NULL DEFAULT 1,
    revision bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_user_preferences PRIMARY KEY (user_id, namespace),
    CONSTRAINT fk_user_preferences_user
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT chk_user_preferences_namespace
        CHECK (namespace ~ '^[a-z][a-z0-9._:-]{0,63}$'),
    CONSTRAINT chk_user_preferences_data_object
        CHECK (jsonb_typeof(data) = 'object'),
    CONSTRAINT chk_user_preferences_schema_version
        CHECK (schema_version > 0),
    CONSTRAINT chk_user_preferences_revision
        CHECK (revision > 0)
);

COMMENT ON TABLE user_preferences IS '用户跨设备偏好，按 namespace 隔离';
COMMENT ON COLUMN user_preferences.namespace IS '偏好命名空间，例如 tool-workspace';
COMMENT ON COLUMN user_preferences.data IS '命名空间内的 JSON 对象数据';
COMMENT ON COLUMN user_preferences.schema_version IS 'data 结构版本';
COMMENT ON COLUMN user_preferences.revision IS '乐观并发控制版本';

COMMIT;
