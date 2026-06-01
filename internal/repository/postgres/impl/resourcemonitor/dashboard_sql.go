package repository

const resourceDashboardMatrixSQL = `
	WITH RECURSIVE scoped_libraries AS (
		SELECT
			l.id
		FROM libraries l
		WHERE l.deleted_at IS NULL
		  AND l.user_id = ?
		  AND (? = 0 OR l.id = ?)
	),
	scoped_objects AS (
		SELECT
			so.id,
			so.library_id,
			so.content_length,
			LOWER(TRIM(COALESCE(so.content_type, ''))) AS object_content_type
		FROM storage_objects so
		JOIN scoped_libraries l
		  ON l.id = so.library_id
		WHERE so.deleted_at IS NULL
	),
	ref_nodes AS (
		SELECT
			so.id AS storage_object_id,
			so.library_id,
			so.content_length,
			n.id AS node_id,
			n.parent_id AS node_parent_id,
			n.deleted_at AS node_deleted_at,
			LOWER(TRIM(COALESCE(nf.mime_type, so.object_content_type, ''))) AS mime_type,
			LOWER(TRIM(COALESCE(n.ext, ''))) AS node_ext
		FROM scoped_objects so
		LEFT JOIN node_files nf
		  ON nf.storage_object_id = so.id
		 AND nf.library_id = so.library_id
		LEFT JOIN nodes n
		  ON n.id = nf.file_id
		 AND n.library_id = nf.library_id
	),
	ref_ancestors AS (
		SELECT
			rn.storage_object_id,
			rn.library_id,
			rn.node_id,
			rn.node_deleted_at,
			n.id AS ancestor_id,
			n.parent_id AS next_parent_id,
			UPPER(TRIM(COALESCE(n.built_in_type, ''))) AS ancestor_built_in_type,
			0 AS depth
		FROM ref_nodes rn
		JOIN nodes n
		  ON n.id = rn.node_id
		 AND n.library_id = rn.library_id
		WHERE rn.node_id IS NOT NULL

		UNION ALL

		SELECT
			ra.storage_object_id,
			ra.library_id,
			ra.node_id,
			ra.node_deleted_at,
			p.id AS ancestor_id,
			p.parent_id AS next_parent_id,
			UPPER(TRIM(COALESCE(p.built_in_type, ''))) AS ancestor_built_in_type,
			ra.depth + 1 AS depth
		FROM ref_ancestors ra
		JOIN nodes p
		  ON p.id = ra.next_parent_id
		 AND p.library_id = ra.library_id
		WHERE ra.next_parent_id IS NOT NULL
		  AND ra.next_parent_id <> 0
		  AND ra.depth < 64
	),
	ref_effective_collections AS (
		SELECT
			ranked.storage_object_id,
			ranked.node_id,
			CASE
				WHEN ranked.ancestor_built_in_type IN ('COMIC', 'ASMR', 'VIDEO', 'AUDIO', 'GALLERY') THEN
					ranked.ancestor_built_in_type
				ELSE 'UNKNOWN'
			END AS collection_key,
			ranked.ancestor_built_in_type AS built_in_type
		FROM (
			SELECT
				ra.storage_object_id,
				ra.node_id,
				ra.ancestor_built_in_type,
				ROW_NUMBER() OVER (
					PARTITION BY ra.storage_object_id, ra.node_id
					ORDER BY ra.depth DESC, ra.ancestor_id ASC
				) AS rank
			FROM ref_ancestors ra
			WHERE ra.ancestor_built_in_type NOT IN ('', 'DEF')
		) ranked
		WHERE ranked.rank = 1
	),
	ref_dimensions AS (
		SELECT
			rn.storage_object_id,
			rn.content_length,
			rn.node_id,
			rn.node_deleted_at AS deleted_at,
			CASE
				WHEN rn.node_id IS NULL THEN 'UNCLASSIFIED'
				WHEN ec.collection_key IS NOT NULL THEN ec.collection_key
				ELSE 'DEF'
			END AS collection_key,
			COALESCE(
				ec.built_in_type,
				CASE WHEN rn.node_id IS NULL THEN '' ELSE 'DEF' END
			) AS collection_built_in_type,
			CASE
				WHEN rn.mime_type LIKE 'video/%' THEN 'video'
				WHEN rn.mime_type LIKE 'image/%' THEN 'image'
				WHEN rn.mime_type LIKE 'audio/%' THEN 'audio'
				WHEN rn.mime_type IN (
					'application/zip',
					'application/x-7z-compressed',
					'application/x-rar-compressed',
					'application/vnd.rar',
					'application/x-tar',
					'application/gzip',
					'application/x-gzip',
					'application/x-bzip2',
					'application/x-xz'
				) THEN 'archive'
				WHEN rn.mime_type LIKE 'text/%'
				  OR rn.mime_type IN (
					'application/json',
					'application/xml',
					'application/javascript',
					'application/typescript',
					'application/x-typescript',
					'application/x-javascript',
					'application/yaml',
					'application/x-yaml'
				  ) THEN 'text'
				WHEN rn.node_ext IN ('mp4', 'm4v', 'mkv', 'webm', 'mov', 'avi', 'flv', 'wmv', 'mpeg', 'mpg', 'm2ts') THEN 'video'
				WHEN rn.node_ext IN ('jpg', 'jpeg', 'png', 'gif', 'webp', 'avif', 'bmp', 'svg', 'heic', 'heif') THEN 'image'
				WHEN rn.node_ext IN ('mp3', 'm4a', 'aac', 'flac', 'wav', 'ogg', 'opus', 'wma') THEN 'audio'
				WHEN rn.node_ext IN ('zip', 'rar', '7z', 'tar', 'gz', 'bz2', 'xz') THEN 'archive'
				WHEN rn.node_ext IN (
					'txt', 'md', 'json', 'xml', 'yaml', 'yml', 'csv', 'log',
					'ts', 'tsx', 'js', 'jsx', 'css', 'html', 'go', 'py', 'rs',
					'java', 'c', 'cpp', 'h', 'hpp', 'sql'
				) THEN 'text'
				ELSE 'unknown'
			END AS file_type_key
		FROM ref_nodes rn
		LEFT JOIN ref_effective_collections ec
		  ON ec.storage_object_id = rn.storage_object_id
		 AND ec.node_id = rn.node_id
	),
	object_main_dimensions AS (
		SELECT
			r.storage_object_id,
			r.collection_key,
			r.collection_built_in_type,
			r.file_type_key,
			ROW_NUMBER() OVER (
				PARTITION BY r.storage_object_id
				ORDER BY
					CASE
						WHEN r.node_id IS NOT NULL AND r.deleted_at IS NULL THEN 0
						WHEN r.node_id IS NOT NULL AND r.deleted_at IS NOT NULL THEN 1
						ELSE 2
					END,
					CASE WHEN r.collection_key NOT IN ('DEF', 'UNCLASSIFIED') THEN 0 ELSE 1 END,
					CASE WHEN r.file_type_key <> 'unknown' THEN 0 ELSE 1 END,
					COALESCE(r.node_id, 9223372036854775807)
			) AS rank
		FROM ref_dimensions r
	),
	physical_by_matrix AS (
		SELECT
			main.collection_key,
			MAX(main.collection_built_in_type) AS collection_built_in_type,
			main.file_type_key,
			COUNT(so.id) AS object_count,
			COALESCE(SUM(so.content_length), 0) AS physical_bytes
		FROM scoped_objects so
		JOIN object_main_dimensions main
		  ON main.storage_object_id = so.id
		 AND main.rank = 1
		GROUP BY main.collection_key, main.file_type_key
	),
	referenced_by_matrix AS (
		SELECT
			r.collection_key,
			MAX(r.collection_built_in_type) AS collection_built_in_type,
			r.file_type_key,
			COUNT(r.node_id) AS file_ref_count,
			COALESCE(SUM(r.content_length) FILTER (WHERE r.node_id IS NOT NULL), 0) AS referenced_bytes
		FROM ref_dimensions r
		GROUP BY r.collection_key, r.file_type_key
	),
	matrix_keys AS (
		SELECT collection_key, file_type_key FROM physical_by_matrix
		UNION
		SELECT collection_key, file_type_key FROM referenced_by_matrix
	)
	SELECT
		k.collection_key,
		COALESCE(p.collection_built_in_type, r.collection_built_in_type, k.collection_key) AS collection_built_in_type,
		k.file_type_key,
		COALESCE(p.object_count, 0) AS object_count,
		COALESCE(r.file_ref_count, 0) AS file_ref_count,
		COALESCE(p.physical_bytes, 0) AS physical_bytes,
		COALESCE(r.referenced_bytes, 0) AS referenced_bytes
	FROM matrix_keys k
	LEFT JOIN physical_by_matrix p
	  ON p.collection_key = k.collection_key
	 AND p.file_type_key = k.file_type_key
	LEFT JOIN referenced_by_matrix r
	  ON r.collection_key = k.collection_key
	 AND r.file_type_key = k.file_type_key
	ORDER BY physical_bytes DESC, object_count DESC, collection_key ASC, file_type_key ASC
`
