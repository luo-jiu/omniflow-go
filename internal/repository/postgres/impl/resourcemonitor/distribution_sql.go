package repository

const storageDistributionSQL = `
	WITH scoped_objects AS (
		SELECT
			so.id,
			so.library_id,
			so.provider,
			so.bucket,
			so.content_length
		FROM storage_objects so
		JOIN libraries l
		  ON l.id = so.library_id
		 AND l.deleted_at IS NULL
		WHERE so.deleted_at IS NULL
		  AND l.user_id = ?
		  AND (? = 0 OR so.library_id = ?)
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
	)
	SELECT
		so.provider AS provider,
		so.bucket AS bucket,
		COUNT(*) AS object_count,
		COALESCE(SUM(ref.file_ref_count), 0) AS file_ref_count,
		COALESCE(SUM(so.content_length), 0) AS physical_bytes,
		COUNT(*) FILTER (WHERE COALESCE(ref.has_visible_ref, FALSE)) AS visible_object_count,
		COALESCE(SUM(ref.visible_file_ref_count), 0) AS visible_file_ref_count,
		COALESCE(
			SUM(so.content_length) FILTER (WHERE COALESCE(ref.has_visible_ref, FALSE)),
			0
		) AS visible_bytes,
		COUNT(*) FILTER (
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
		COUNT(*) FILTER (WHERE COALESCE(ref.file_ref_count, 0) = 0) AS orphan_object_count,
		COALESCE(
			SUM(so.content_length) FILTER (WHERE COALESCE(ref.file_ref_count, 0) = 0),
			0
		) AS orphan_bytes
	FROM scoped_objects so
	LEFT JOIN object_refs ref
	  ON ref.storage_object_id = so.id
	GROUP BY so.provider, so.bucket
	ORDER BY physical_bytes DESC, object_count DESC, provider ASC, bucket ASC
`
