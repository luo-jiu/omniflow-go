package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunUploadFileRequiresLibraryID(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"upload", "file", "--file", "/tmp/some-file"})
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "`--library-id` is required") {
		t.Fatalf("expected library-id required error, got: %s", stderr.String())
	}
}

func TestRunUploadFileRequiresFile(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"upload", "file", "--library-id", "1"})
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "`--file` is required") {
		t.Fatalf("expected file required error, got: %s", stderr.String())
	}
}

func TestRunUploadFileRejectsMissingFile(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{
		"upload", "file",
		"--library-id", "1",
		"--file", "/tmp/__definitely-does-not-exist__.bin",
	})
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "stat upload file") {
		t.Fatalf("expected stat error, got: %s", stderr.String())
	}
}

func TestRunUploadFileRejectsExtraArgs(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{
		"upload", "file",
		"--library-id", "1",
		"--file", "/tmp/x",
		"unexpected",
	})
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "unexpected arguments") {
		t.Fatalf("expected unexpected arguments error, got: %s", stderr.String())
	}
}

func TestUploadHelpListsFlags(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"help", "upload", "file"})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got: %s", stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"--library-id",
		"--file",
		"--parent-id",
		"--storage-provider",
		"--conflict-policy",
		"--content-type",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %s in upload file help, got: %s", want, out)
		}
	}
}

func TestBoundedReader(t *testing.T) {
	t.Parallel()

	src := strings.NewReader("hello world")
	br := &boundedReader{r: src, remaining: 5}

	buf := make([]byte, 100)
	n, err := br.Read(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected to read 5 bytes, got %d", n)
	}
	if string(buf[:n]) != "hello" {
		t.Fatalf("expected hello, got %q", string(buf[:n]))
	}

	n, err = br.Read(buf)
	if n != 0 {
		t.Fatalf("expected 0 bytes after limit, got %d", n)
	}
	if err == nil {
		t.Fatalf("expected EOF, got nil")
	}
}
