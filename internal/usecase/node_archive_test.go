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
