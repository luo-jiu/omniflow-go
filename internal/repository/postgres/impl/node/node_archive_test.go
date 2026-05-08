package repository

import (
	"strings"
	"testing"

	pgmodel "omniflow-go/internal/repository/postgres/model"
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

func TestArchiveAudioExtensionsIncludesExpectedFormats(t *testing.T) {
	t.Parallel()

	expected := []string{"mp3", "wav", "m4a", "flac", "oga", "opus"}
	seen := make(map[string]struct{}, len(archiveAudioExtensions))
	for _, ext := range archiveAudioExtensions {
		if _, duplicated := seen[ext]; duplicated {
			t.Fatalf("archiveAudioExtensions contains duplicate ext %q", ext)
		}
		seen[ext] = struct{}{}
	}

	for _, ext := range expected {
		if _, ok := seen[ext]; !ok {
			t.Fatalf("archiveAudioExtensions missing %q", ext)
		}
	}
}

func TestArchiveMediaNodeMatching(t *testing.T) {
	t.Parallel()

	ext := "bin"
	node := &pgmodel.Node{
		ID:       10,
		NodeType: nodeTypeFile,
		Name:     "clip",
		Ext:      &ext,
	}
	if !isArchiveMediaNode(node, map[uint64]string{10: "video/mp4"}, archiveMediaKindVideo) {
		t.Fatalf("expected mime type to match video media")
	}
	if isArchiveMediaNode(node, map[uint64]string{10: "audio/mpeg"}, archiveMediaKindVideo) {
		t.Fatalf("audio mime should not match video media")
	}

	videoExt := "mkv"
	node.Ext = &videoExt
	if !isArchiveMediaNode(node, map[uint64]string{}, archiveMediaKindVideo) {
		t.Fatalf("expected extension to match video media")
	}
}

func TestArchiveMediaNodeIgnoresHiddenFiles(t *testing.T) {
	t.Parallel()

	ext := "mp4"
	node := &pgmodel.Node{
		ID:       11,
		NodeType: nodeTypeFile,
		Name:     ".hidden",
		Ext:      &ext,
	}
	if isArchiveMediaNode(node, map[uint64]string{11: "video/mp4"}, archiveMediaKindVideo) {
		t.Fatalf("hidden file should not match archive media")
	}

	node.Name = ""
	if isArchiveMediaNode(node, map[uint64]string{11: "video/mp4"}, archiveMediaKindVideo) {
		t.Fatalf("extension-only file should not match archive media")
	}
}

func TestArchiveUnitFromNodeDefaultsToMediaCard(t *testing.T) {
	t.Parallel()

	got := archiveUnitFromNode(&pgmodel.Node{
		ID:        21,
		NodeType:  nodeTypeDirectory,
		Name:      "video",
		SortOrder: 7,
	})
	if got.CardKind != archiveCardKindMedia {
		t.Fatalf("archiveUnitFromNode().CardKind = %q, want %q", got.CardKind, archiveCardKindMedia)
	}
}

func TestArchivePagedMediaUnitsSQLArgumentShape(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name               string
		includeCollections bool
		builtInType        string
		mediaKind          archiveMediaKind
	}{
		{
			name:               "audio",
			includeCollections: false,
			builtInType:        "AUDIO",
			mediaKind:          archiveMediaKindAudio,
		},
		{
			name:               "video with collections",
			includeCollections: true,
			builtInType:        "VIDEO",
			mediaKind:          archiveMediaKindVideo,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			candidateSQL := archivePagedMediaUnitCandidatesSQL(tt.includeCollections)
			candidateArgs := archivePagedMediaUnitCandidateArgs(
				10,
				20,
				tt.builtInType,
				tt.mediaKind,
				tt.includeCollections,
			)
			if placeholders := strings.Count(candidateSQL, "?"); placeholders != len(candidateArgs) {
				t.Fatalf("candidate placeholder count = %d, args = %d", placeholders, len(candidateArgs))
			}

			pageSQL := archivePagedMediaUnitPageSQL(tt.includeCollections)
			pageArgs := append(append([]any{}, candidateArgs...), 0, 24)
			pageArgs = append(pageArgs, archivePagedMediaUnitDetailArgs(tt.mediaKind)...)
			if placeholders := strings.Count(pageSQL, "?"); placeholders != len(pageArgs) {
				t.Fatalf("page placeholder count = %d, args = %d", placeholders, len(pageArgs))
			}
			pagedIndex := strings.Index(pageSQL, "paged_units as")
			detailIndex := strings.Index(pageSQL, "left join lateral")
			if pagedIndex < 0 || detailIndex < 0 || pagedIndex > detailIndex {
				t.Fatalf("page sql must page candidates before running lateral detail queries")
			}
		})
	}
}
