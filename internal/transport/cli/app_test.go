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
