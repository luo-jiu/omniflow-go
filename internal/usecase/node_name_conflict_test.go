package usecase

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	domainnode "omniflow-go/internal/domain/node"
)

func TestNormalizeNodeNameConflictPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   NodeNameConflictPolicy
		want    NodeNameConflictPolicy
		wantErr bool
	}{
		{name: "empty defaults to error", input: "", want: NodeNameConflictError},
		{name: "explicit error", input: "error", want: NodeNameConflictError},
		{name: "auto rename", input: "auto_rename", want: NodeNameConflictAutoRename},
		{name: "auto rename dash alias", input: "auto-rename", want: NodeNameConflictAutoRename},
		{name: "invalid policy", input: "replace", wantErr: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeNodeNameConflictPolicy(test.input)
			if test.wantErr {
				if !errors.Is(err, ErrInvalidArgument) {
					t.Fatalf("expected ErrInvalidArgument, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("unexpected policy: got %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveAutoRenamedNodeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		siblings []domainnode.Node
		want     string
	}{
		{
			name:  "no duplicate keeps original",
			input: "demo",
			siblings: []domainnode.Node{
				{Name: "other"},
			},
			want: "demo",
		},
		{
			name:  "uses first available numeric suffix",
			input: "demo",
			siblings: []domainnode.Node{
				{Name: "demo"},
				{Name: "demo (1)"},
			},
			want: "demo (2)",
		},
		{
			name:  "preserves literal requested base",
			input: "demo (1)",
			siblings: []domainnode.Node{
				{Name: "demo (1)"},
			},
			want: "demo (1) (1)",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveAutoRenamedNodeName(test.input, test.siblings)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("unexpected name: got %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveAutoRenamedNodeNameTruncatesBeforeSuffix(t *testing.T) {
	t.Parallel()

	longName := strings.Repeat("漫", maxNodeNameLength)
	got, err := resolveAutoRenamedNodeName(longName, []domainnode.Node{{Name: longName}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(got, " (1)") {
		t.Fatalf("expected suffix, got %q", got)
	}
	if utf8.RuneCountInString(got) != maxNodeNameLength {
		t.Fatalf("expected %d runes, got %d", maxNodeNameLength, utf8.RuneCountInString(got))
	}
}

func TestNodeNameConflictMessageKeepsConflictClassification(t *testing.T) {
	t.Parallel()

	if errNodeNameAlreadyExists.Error() != "同一目录下已存在同名节点" {
		t.Fatalf("unexpected client message: %q", errNodeNameAlreadyExists.Error())
	}
	if !errors.Is(errNodeNameAlreadyExists, ErrConflict) {
		t.Fatalf("expected errors.Is(err, ErrConflict) to be true")
	}
}
