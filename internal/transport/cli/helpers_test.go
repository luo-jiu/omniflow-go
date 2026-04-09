package cli

import (
	"strings"
	"testing"
)

func TestParseUint64CSV(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []uint64
		wantErr bool
	}{
		{
			name:  "empty",
			input: "",
			want:  nil,
		},
		{
			name:  "single value",
			input: "7",
			want:  []uint64{7},
		},
		{
			name:  "trim and skip empty",
			input: " 1, 2, ,3 ",
			want:  []uint64{1, 2, 3},
		},
		{
			name:    "invalid number",
			input:   "1,a,3",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseUint64CSV(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got=%d want=%d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("value mismatch at %d: got=%d want=%d", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestNormalizeNodePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantPath     string
		wantSegments []string
		wantErr      string
	}{
		{
			name:         "root",
			input:        "/",
			wantPath:     "/",
			wantSegments: []string{},
		},
		{
			name:         "relative path",
			input:        "docs/ch1",
			wantPath:     "/docs/ch1",
			wantSegments: []string{"docs", "ch1"},
		},
		{
			name:         "windows style and redundant slash",
			input:        "\\docs\\\\ch1\\",
			wantPath:     "/docs/ch1",
			wantSegments: []string{"docs", "ch1"},
		},
		{
			name:    "reject parent navigation",
			input:   "/docs/../secret",
			wantErr: "cannot contain `..`",
		},
		{
			name:    "empty",
			input:   "   ",
			wantErr: "`--path` is required",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotPath, gotSegments, err := normalizeNodePath(tc.input)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPath != tc.wantPath {
				t.Fatalf("path mismatch: got=%q want=%q", gotPath, tc.wantPath)
			}
			if len(gotSegments) != len(tc.wantSegments) {
				t.Fatalf("segment len mismatch: got=%d want=%d", len(gotSegments), len(tc.wantSegments))
			}
			for i := range gotSegments {
				if gotSegments[i] != tc.wantSegments[i] {
					t.Fatalf("segment mismatch at %d: got=%q want=%q", i, gotSegments[i], tc.wantSegments[i])
				}
			}
		})
	}
}
