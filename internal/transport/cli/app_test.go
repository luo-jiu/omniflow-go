package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelpAuth(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"help", "auth"})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %s", stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Authentication commands") {
		t.Fatalf("expected auth help content, got: %s", output)
	}
	if !strings.Contains(output, "login") {
		t.Fatalf("expected auth subcommand list, got: %s", output)
	}
}

func TestRunUnknownTopCommand(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"unknown"})
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}

	output := stderr.String()
	if !strings.Contains(output, "unknown command") {
		t.Fatalf("expected unknown command message, got: %s", output)
	}
}

func TestRunConfigShowRejectsExtraArgs(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"config", "show", "unexpected"})
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}

	if !strings.Contains(stderr.String(), "unexpected arguments") {
		t.Fatalf("expected unexpected arguments error, got: %s", stderr.String())
	}
}

func TestRunHelpFSContainsMkdir(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"help", "fs"})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "mkdir") {
		t.Fatalf("expected mkdir in fs help, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "mv") {
		t.Fatalf("expected mv in fs help, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "rm") {
		t.Fatalf("expected rm in fs help, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "recycle") {
		t.Fatalf("expected recycle in fs help, got: %s", stdout.String())
	}
}

func TestRunHelpFSRecycleContainsSubcommands(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"help", "fs", "recycle"})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %s", stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "restore") {
		t.Fatalf("expected restore in recycle help, got: %s", out)
	}
	if !strings.Contains(out, "hard") {
		t.Fatalf("expected hard in recycle help, got: %s", out)
	}
}

func TestRunHelpLeafHidesExamplesByDefault(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"help", "fs", "mkdir"})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %s", stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Tip:") {
		t.Fatalf("expected examples tip in leaf help, got: %s", out)
	}
	if strings.Contains(out, "Examples:") {
		t.Fatalf("did not expect examples by default, got: %s", out)
	}
}

func TestRunHelpLeafShowsExamplesWithFlag(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"help", "fs", "mkdir", "--examples"})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %s", stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Examples:") {
		t.Fatalf("expected examples section, got: %s", out)
	}
	if !strings.Contains(out, "of fs mkdir --library-id") {
		t.Fatalf("expected mkdir example content, got: %s", out)
	}
}

func TestRunInlineHelpShowsExamplesWithFlag(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"fs", "rename", "--help", "--examples"})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Examples:") {
		t.Fatalf("expected examples section, got: %s", stdout.String())
	}
}
