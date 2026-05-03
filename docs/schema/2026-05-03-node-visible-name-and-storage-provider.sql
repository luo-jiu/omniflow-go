-- OmniFlow PostgreSQL schema patch.
--
-- Purpose:
-- 1. Enforce sibling node uniqueness by user-visible name:
--    directories use name, files use name.ext when ext is present.
-- 2. Allow storage_objects.provider to store provider aliases such as
--    local-minio or win-minio instead of only legacy provider type values.
--
-- This patch intentionally does not rewrite existing storage_objects.provider
-- values. Historical type values such as MINIO are handled by runtime fallback.
-- Rewrite them to aliases only after confirming the target storage location.

BEGIN;

DROP INDEX IF EXISTS uq_nodes_live_sibling_visible_name;

CREATE UNIQUE INDEX uq_nodes_live_sibling_visible_name
  ON nodes (
    library_id,
    COALESCE(parent_id, 0),
    (CASE
      WHEN node_type = 1 AND COALESCE(ext, '') <> '' THEN name || '.' || ext
      ELSE name
    END)
  )
  WHERE deleted_at IS NULL;

ALTER TABLE storage_objects
  DROP CONSTRAINT IF EXISTS chk_storage_provider;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conrelid = 'storage_objects'::regclass
      AND conname = 'chk_storage_provider_not_blank'
  ) THEN
    ALTER TABLE storage_objects
      ADD CONSTRAINT chk_storage_provider_not_blank CHECK (btrim(provider) <> '');
  END IF;
END $$;

COMMIT;
