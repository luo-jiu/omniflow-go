package cli

import "testing"

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
