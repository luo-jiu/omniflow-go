package repository

const resourceBreakdownLibrarySQL = `
	WITH scoped_libraries AS (
		SELECT
			l.id,
			l.name
		FROM libraries l
		WHERE l.deleted_at IS NULL
		  AND l.user_id = ?
		  AND (? = 0 OR l.id = ?)
	),
	scoped_objects AS (
		SELECT
			so.id,
			so.library_id,
			so.provider,
			so.bucket,
			so.content_length
		FROM storage_objects so
		JOIN scoped_libraries l
		  ON l.id = so.library_id
		WHERE so.deleted_at IS NULL
	),
	object_refs AS (
		SELECT
			so.id AS storage_object_id,
			COUNT(nf.file_id) AS file_ref_count,
			COUNT(nf.file_id) FILTER (
				WHERE n.id IS NOT NULL AND n.deleted_at IS NULL
			) AS visible_file_ref_count,
			COUNT(nf.file_id) FILTER (
				WHERE n.id IS NOT NULL AND n.deleted_at IS NOT NULL
			) AS recycle_file_ref_count,
			BOOL_OR(n.id IS NOT NULL AND n.deleted_at IS NULL) AS has_visible_ref,
			BOOL_OR(n.id IS NOT NULL AND n.deleted_at IS NOT NULL) AS has_recycle_ref
		FROM scoped_objects so
		LEFT JOIN node_files nf
		  ON nf.storage_object_id = so.id
		 AND nf.library_id = so.library_id
		LEFT JOIN nodes n
		  ON n.id = nf.file_id
		 AND n.library_id = nf.library_id
		GROUP BY so.id
	),
	archive_dirs AS (
		SELECT
			n.library_id,
			COUNT(*) AS archive_directory_count
		FROM nodes n
		JOIN scoped_libraries l
		  ON l.id = n.library_id
		WHERE n.deleted_at IS NULL
		  AND n.node_type = 0
		  AND n.archive_mode = TRUE
		GROUP BY n.library_id
	),
	provider_rank AS (
		SELECT
			so.library_id,
			so.provider,
			so.bucket,
			ROW_NUMBER() OVER (
				PARTITION BY so.library_id
				ORDER BY SUM(so.content_length) DESC, COUNT(*) DESC, so.provider ASC, so.bucket ASC
			) AS rank
		FROM scoped_objects so
		GROUP BY so.library_id, so.provider, so.bucket
	)
	SELECT
		l.id AS library_id,
		l.name AS library_name,
		COUNT(so.id) AS object_count,
		COALESCE(SUM(ref.file_ref_count), 0) AS file_ref_count,
		COALESCE(SUM(so.content_length), 0) AS physical_bytes,
		COALESCE(SUM(so.content_length * COALESCE(ref.file_ref_count, 0)), 0) AS referenced_bytes,
		COUNT(so.id) FILTER (WHERE COALESCE(ref.has_visible_ref, FALSE)) AS visible_object_count,
		COALESCE(SUM(ref.visible_file_ref_count), 0) AS visible_file_ref_count,
		COALESCE(
			SUM(so.content_length) FILTER (WHERE COALESCE(ref.has_visible_ref, FALSE)),
			0
		) AS visible_bytes,
		COUNT(so.id) FILTER (
			WHERE NOT COALESCE(ref.has_visible_ref, FALSE)
			  AND COALESCE(ref.has_recycle_ref, FALSE)
		) AS recycle_object_count,
		COALESCE(
			SUM(ref.recycle_file_ref_count) FILTER (
				WHERE NOT COALESCE(ref.has_visible_ref, FALSE)
				  AND COALESCE(ref.has_recycle_ref, FALSE)
			),
			0
		) AS recycle_file_ref_count,
		COALESCE(
			SUM(so.content_length) FILTER (
				WHERE NOT COALESCE(ref.has_visible_ref, FALSE)
				  AND COALESCE(ref.has_recycle_ref, FALSE)
			),
			0
		) AS recycle_bytes,
		COUNT(so.id) FILTER (WHERE COALESCE(ref.file_ref_count, 0) = 0) AS orphan_object_count,
		COALESCE(
			SUM(so.content_length) FILTER (WHERE COALESCE(ref.file_ref_count, 0) = 0),
			0
		) AS orphan_bytes,
		COALESCE(MAX(ad.archive_directory_count), 0) AS archive_directory_count,
		COUNT(so.id) FILTER (WHERE COALESCE(ref.file_ref_count, 0) > 1) AS multi_ref_object_count,
		COALESCE(
			SUM(so.content_length) FILTER (WHERE COALESCE(ref.file_ref_count, 0) > 1),
			0
		) AS multi_ref_physical_bytes,
		COALESCE(MAX(pr.provider), '') AS top_provider,
		COALESCE(MAX(pr.bucket), '') AS top_bucket
	FROM scoped_libraries l
	LEFT JOIN scoped_objects so
	  ON so.library_id = l.id
	LEFT JOIN object_refs ref
	  ON ref.storage_object_id = so.id
	LEFT JOIN archive_dirs ad
	  ON ad.library_id = l.id
	LEFT JOIN provider_rank pr
	  ON pr.library_id = l.id
	 AND pr.rank = 1
	GROUP BY l.id, l.name
	ORDER BY physical_bytes DESC, object_count DESC, l.name ASC
`

const resourceBreakdownCategorySQL = `
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
			so.content_length
		FROM storage_objects so
		JOIN scoped_libraries l
		  ON l.id = so.library_id
		WHERE so.deleted_at IS NULL
	),
	object_refs AS (
		SELECT
			so.id AS storage_object_id,
			COUNT(nf.file_id) AS file_ref_count,
			COUNT(nf.file_id) FILTER (
				WHERE n.id IS NOT NULL AND n.deleted_at IS NULL
			) AS visible_file_ref_count,
			COUNT(nf.file_id) FILTER (
				WHERE n.id IS NOT NULL AND n.deleted_at IS NOT NULL
			) AS recycle_file_ref_count,
			BOOL_OR(n.id IS NOT NULL AND n.deleted_at IS NULL) AS has_visible_ref,
			BOOL_OR(n.id IS NOT NULL AND n.deleted_at IS NOT NULL) AS has_recycle_ref
		FROM scoped_objects so
		LEFT JOIN node_files nf
		  ON nf.storage_object_id = so.id
		 AND nf.library_id = so.library_id
		LEFT JOIN nodes n
		  ON n.id = nf.file_id
		 AND n.library_id = nf.library_id
		GROUP BY so.id
	),
	ref_nodes AS (
		SELECT
			so.id AS storage_object_id,
			so.library_id,
			so.content_length,
			n.id AS node_id,
			n.parent_id AS node_parent_id,
			n.deleted_at AS node_deleted_at
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
			rn.content_length,
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
			ra.content_length,
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
	ref_effective_categories AS (
		SELECT
			ranked.storage_object_id,
			ranked.node_id,
			CASE
				WHEN ranked.ancestor_built_in_type IN ('COMIC', 'ASMR', 'VIDEO', 'AUDIO', 'GALLERY') THEN
					ranked.ancestor_built_in_type
				ELSE 'UNKNOWN'
			END AS category_key,
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
	ref_categories AS (
		SELECT
			rn.storage_object_id,
			rn.content_length,
			rn.node_id,
			rn.node_deleted_at AS deleted_at,
			CASE
				WHEN rn.node_id IS NULL THEN 'UNCLASSIFIED'
				WHEN ec.category_key IS NOT NULL THEN ec.category_key
				ELSE 'DEF'
			END AS category_key,
			COALESCE(
				ec.built_in_type,
				CASE WHEN rn.node_id IS NULL THEN '' ELSE 'DEF' END
			) AS built_in_type
		FROM ref_nodes rn
		LEFT JOIN ref_effective_categories ec
		  ON ec.storage_object_id = rn.storage_object_id
		 AND ec.node_id = rn.node_id
	),
	object_main_category AS (
		SELECT
			r.storage_object_id,
			r.category_key,
			r.built_in_type,
			ROW_NUMBER() OVER (
				PARTITION BY r.storage_object_id
				ORDER BY
					CASE
						WHEN r.node_id IS NOT NULL AND r.deleted_at IS NULL THEN 0
						WHEN r.node_id IS NOT NULL AND r.deleted_at IS NOT NULL THEN 1
						ELSE 2
					END,
					CASE WHEN r.category_key NOT IN ('DEF', 'UNCLASSIFIED') THEN 0 ELSE 1 END,
					COALESCE(r.node_id, 9223372036854775807)
			) AS rank
		FROM ref_categories r
	),
	physical_by_category AS (
		SELECT
			main.category_key,
			MAX(main.built_in_type) AS built_in_type,
			COUNT(so.id) AS object_count,
			COALESCE(SUM(so.content_length), 0) AS physical_bytes,
			COUNT(so.id) FILTER (WHERE COALESCE(ref.has_visible_ref, FALSE)) AS visible_object_count,
			COALESCE(
				SUM(so.content_length) FILTER (WHERE COALESCE(ref.has_visible_ref, FALSE)),
				0
			) AS visible_bytes,
			COUNT(so.id) FILTER (
				WHERE NOT COALESCE(ref.has_visible_ref, FALSE)
				  AND COALESCE(ref.has_recycle_ref, FALSE)
			) AS recycle_object_count,
			COALESCE(
				SUM(so.content_length) FILTER (
					WHERE NOT COALESCE(ref.has_visible_ref, FALSE)
					  AND COALESCE(ref.has_recycle_ref, FALSE)
				),
				0
			) AS recycle_bytes,
			COUNT(so.id) FILTER (WHERE COALESCE(ref.file_ref_count, 0) = 0) AS orphan_object_count,
			COALESCE(
				SUM(so.content_length) FILTER (WHERE COALESCE(ref.file_ref_count, 0) = 0),
				0
			) AS orphan_bytes
		FROM scoped_objects so
		JOIN object_main_category main
		  ON main.storage_object_id = so.id
		 AND main.rank = 1
		LEFT JOIN object_refs ref
		  ON ref.storage_object_id = so.id
		GROUP BY main.category_key
	),
	referenced_by_category AS (
		SELECT
			r.category_key,
			COUNT(r.node_id) AS file_ref_count,
			COALESCE(SUM(r.content_length) FILTER (WHERE r.node_id IS NOT NULL), 0) AS referenced_bytes,
			COUNT(r.node_id) FILTER (WHERE r.node_id IS NOT NULL AND r.deleted_at IS NULL) AS visible_file_ref_count,
			COUNT(r.node_id) FILTER (WHERE r.node_id IS NOT NULL AND r.deleted_at IS NOT NULL) AS recycle_file_ref_count
		FROM ref_categories r
		GROUP BY r.category_key
	),
	category_keys AS (
		SELECT category_key FROM physical_by_category
		UNION
		SELECT category_key FROM referenced_by_category
	)
	SELECT
		k.category_key AS key,
		COALESCE(p.built_in_type, k.category_key) AS built_in_type,
		COALESCE(p.object_count, 0) AS object_count,
		COALESCE(r.file_ref_count, 0) AS file_ref_count,
		COALESCE(p.physical_bytes, 0) AS physical_bytes,
		COALESCE(r.referenced_bytes, 0) AS referenced_bytes,
		COALESCE(p.visible_object_count, 0) AS visible_object_count,
		COALESCE(r.visible_file_ref_count, 0) AS visible_file_ref_count,
		COALESCE(p.visible_bytes, 0) AS visible_bytes,
		COALESCE(p.recycle_object_count, 0) AS recycle_object_count,
		COALESCE(r.recycle_file_ref_count, 0) AS recycle_file_ref_count,
		COALESCE(p.recycle_bytes, 0) AS recycle_bytes,
		COALESCE(p.orphan_object_count, 0) AS orphan_object_count,
		COALESCE(p.orphan_bytes, 0) AS orphan_bytes,
		0 AS archive_directory_count
	FROM category_keys k
	LEFT JOIN physical_by_category p
	  ON p.category_key = k.category_key
	LEFT JOIN referenced_by_category r
	  ON r.category_key = k.category_key
	ORDER BY physical_bytes DESC, object_count DESC, key ASC
`
