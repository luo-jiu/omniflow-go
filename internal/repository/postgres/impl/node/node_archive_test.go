package repository

import (
	"strings"
	"testing"
)

func TestArchiveVideoExtensionsIncludesExpectedFormats(t *testing.T) {
	t.Parallel()

	expected := []string{"mp4", "m4v", "ts", "flv", "mkv", "ogv"}
	seen := make(map[string]struct{}, len(archiveVideoExtensions))
	for _, ext := range archiveVideoExtensions {
		if _, duplicated := seen[ext]; duplicated {
			t.Fatalf("archiveVideoExtensions contains duplicate ext %q", ext)
		}
		seen[ext] = struct{}{}
	}

	for _, ext := range expected {
		if _, ok := seen[ext]; !ok {
			t.Fatalf("archiveVideoExtensions missing %q", ext)
		}
	}
}

func TestVideoArchiveSQLMatchesDirectChildVideoFiles(t *testing.T) {
	t.Parallel()

	assertContainsAll := func(t *testing.T, query string, fragments ...string) {
		t.Helper()
		for _, fragment := range fragments {
			if !strings.Contains(query, fragment) {
				t.Fatalf("query missing fragment %q\nquery:\n%s", fragment, query)
			}
		}
	}

	t.Run("count query filters direct child video files", func(t *testing.T) {
		t.Parallel()

		assertContainsAll(
			t,
			sqlCountVideoArchiveUnitsByParentID,
			"FROM nodes n",
			"LEFT JOIN node_files nf",
			"n.parent_id = ?",
			"n.node_type = 1",
			"n.deleted_at IS NULL",
			"n.name NOT LIKE '.%%'",
			"LOWER(COALESCE(nf.mime_type, '')) LIKE 'video/%%'",
			"LOWER(COALESCE(n.ext, '')) IN ?",
		)
	})

	t.Run("list query keeps stable ordering and pagination", func(t *testing.T) {
		t.Parallel()

		assertContainsAll(
			t,
			sqlListVideoArchiveUnitsByParentID,
			"SELECT",
			"n.id,",
			"n.name,",
			"n.sort_order,",
			"n.view_meta",
			"n.parent_id = ?",
			"n.node_type = 1",
			"ORDER BY n.sort_order ASC, n.id ASC",
			"OFFSET ?",
			"LIMIT ?",
		)
	})
}
