package repository

import (
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

func TestArchiveSubtitleNodeMatching(t *testing.T) {
	t.Parallel()

	ext := "srt"
	node := &pgmodel.Node{
		ID:       12,
		NodeType: nodeTypeFile,
		Name:     "clip",
		Ext:      &ext,
	}
	if !isArchiveSubtitleNode(node) {
		t.Fatalf("expected srt extension to match subtitle node")
	}
}

func TestSortAndPaginateArchiveUnits(t *testing.T) {
	t.Parallel()

	units := []ArchiveUnitRow{
		{ID: 3, SortOrder: 20},
		{ID: 2, SortOrder: 10},
		{ID: 1, SortOrder: 10},
	}
	sortArchiveUnits(units)
	page := paginateArchiveUnits(units, 1, 2)
	if len(page) != 2 || page[0].ID != 2 || page[1].ID != 3 {
		t.Fatalf("unexpected archive page: %#v", page)
	}
}
