package usecase

import (
	"strings"
	"testing"
)

func TestNormalizeArchiveCardBuiltInType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "comic", input: "comic", want: "COMIC"},
		{name: "asmr", input: " ASMR ", want: "ASMR"},
		{name: "video", input: "video", want: "VIDEO"},
		{name: "audio", input: "audio", want: "AUDIO"},
		{name: "unsupported", input: "def", want: ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeArchiveCardBuiltInType(tc.input)
			if got != tc.want {
				t.Fatalf("normalizeArchiveCardBuiltInType(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestResolveArchiveCoverNodeIDFromMetaSupportsVideoTopLevelCover(t *testing.T) {
	t.Parallel()

	got := resolveArchiveCoverNodeIDFromMeta(`{"coverNodeId":321}`, "VIDEO")
	if got != 321 {
		t.Fatalf("resolveArchiveCoverNodeIDFromMeta() = %d, want 321", got)
	}
}

func TestApplyArchiveCoverNodeIDToMetaSupportsVideoTopLevelCover(t *testing.T) {
	t.Parallel()

	got, changed := applyArchiveCoverNodeIDToMeta(`{"tag":"movie"}`, "VIDEO", 456)
	if !changed {
		t.Fatalf("applyArchiveCoverNodeIDToMeta() changed = false, want true")
	}
	if !strings.Contains(got, `"coverNodeId":456`) {
		t.Fatalf("applyArchiveCoverNodeIDToMeta() = %s, want coverNodeId in payload", got)
	}
}

func TestResolveNodeMediaDurationFromMeta(t *testing.T) {
	t.Parallel()

	got := resolveNodeMediaDurationFromMeta(`{"__omniflowNodeMetadataV1":{"media":{"durationSeconds":95.25}}}`)
	if got != 95.25 {
		t.Fatalf("resolveNodeMediaDurationFromMeta() = %v, want 95.25", got)
	}
}

func TestApplyNodeMediaDurationToMetaPreservesExistingFields(t *testing.T) {
	t.Parallel()

	got, changed := applyNodeMediaDurationToMeta(`{"tagIds":[1,2]}`, 120.5)
	if !changed {
		t.Fatalf("applyNodeMediaDurationToMeta() changed = false, want true")
	}
	if !strings.Contains(got, `"tagIds":[1,2]`) {
		t.Fatalf("applyNodeMediaDurationToMeta() = %s, want tagIds preserved", got)
	}
	if !strings.Contains(got, `"durationSeconds":120.5`) {
		t.Fatalf("applyNodeMediaDurationToMeta() = %s, want durationSeconds", got)
	}
	if !strings.Contains(got, `"source":"ffprobe"`) {
		t.Fatalf("applyNodeMediaDurationToMeta() = %s, want source", got)
	}
}
