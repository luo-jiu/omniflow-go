package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
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
	if !strings.Contains(stdout.String(), "archive") {
		t.Fatalf("expected archive in fs help, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "path") {
		t.Fatalf("expected path in fs help, got: %s", stdout.String())
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
	if !strings.Contains(out, "clear") {
		t.Fatalf("expected clear in recycle help, got: %s", out)
	}
	if !strings.Contains(out, "restore") {
		t.Fatalf("expected restore in recycle help, got: %s", out)
	}
	if !strings.Contains(out, "hard") {
		t.Fatalf("expected hard in recycle help, got: %s", out)
	}
}

func TestRunHelpFSPathContainsResolve(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"help", "fs", "path"})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %s", stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "resolve") {
		t.Fatalf("expected resolve in fs path help, got: %s", out)
	}
}

func TestRunHelpFSArchiveContainsSubcommands(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"help", "fs", "archive"})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %s", stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "batch-set-built-in-type") {
		t.Fatalf("expected batch-set-built-in-type in fs archive help, got: %s", out)
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

func TestRunHelpFSMkdirContainsDryRunFlag(t *testing.T) {
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
	if !strings.Contains(stdout.String(), "--dry-run") {
		t.Fatalf("expected --dry-run in mkdir help, got: %s", stdout.String())
	}
}

func TestRunHelpAuthLogoutContainsDryRunFlag(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"help", "auth", "logout"})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "--dry-run") {
		t.Fatalf("expected --dry-run in auth logout help, got: %s", stdout.String())
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

func TestRunFSMkdirRejectsMixedParentInputs(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"fs", "mkdir", "--library-id", "1", "--name", "docs", "--parent-id", "10", "--parent-path", "/docs"})
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "`--parent-id` and `--parent-path` cannot be used together") {
		t.Fatalf("expected mixed parent input error, got: %s", stderr.String())
	}
}

func TestRunFSMvRequiresNodeInput(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"fs", "mv", "--library-id", "1", "--new-parent-id", "2"})
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "one of `--node-id` or `--node-path` is required") {
		t.Fatalf("expected missing node input error, got: %s", stderr.String())
	}
}

func TestRunFSRmRequiresNodeInput(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"fs", "rm", "--library-id", "1"})
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "one of `--node-id` or `--path` is required") {
		t.Fatalf("expected missing rm node input error, got: %s", stderr.String())
	}
}

func TestRunFSRecycleRestoreRequiresNodeInput(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"fs", "recycle", "restore", "--library-id", "1"})
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "`--node-id` is required") {
		t.Fatalf("expected missing restore node input error, got: %s", stderr.String())
	}
}

func TestRunFSRecycleClearRequiresLibraryID(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"fs", "recycle", "clear"})
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "`--library-id` is required") {
		t.Fatalf("expected missing library id error, got: %s", stderr.String())
	}
}

func TestRunFSRecycleHardRequiresNodeInput(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"fs", "recycle", "hard", "--library-id", "1"})
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "`--node-id` is required") {
		t.Fatalf("expected missing hard node input error, got: %s", stderr.String())
	}
}

func TestRunFSArchiveBatchSetBuiltInTypeRequiresNodeID(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	exitCode := app.Run([]string{"fs", "archive", "batch-set-built-in-type"})
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "`--node-id` is required and must be greater than 0") {
		t.Fatalf("expected missing node id error, got: %s", stderr.String())
	}
}

func TestRunFSArchiveBatchSetBuiltInTypeSuccessJSON(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	t.Setenv(envUsername, "tester")
	t.Setenv(envToken, "token-123")

	originTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/nodes/123/archive/built-in-type/batch-set" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("dryRun"); got != "true" {
			t.Fatalf("expected dryRun=true, got %q", got)
		}
		if got := r.Header.Get("username"); got != "tester" {
			t.Fatalf("expected username header to be set, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("expected authorization header to be set, got %q", got)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"nodeId":123,"libraryId":1,"builtInType":"COMIC","totalChildren":5,"dirChildren":3,"updatedCount":2},"request_id":"req-1"}`)),
		}, nil
	})
	defer func() {
		http.DefaultTransport = originTransport
	}()

	exitCode := app.Run([]string{
		"fs", "archive", "batch-set-built-in-type",
		"--base-url", "http://example.test",
		"--node-id", "123",
		"--dry-run",
		"--json",
	})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %s", stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("expected valid JSON output, got err=%v output=%s", err, stdout.String())
	}
	if got["dryRun"] != true {
		t.Fatalf("expected dryRun=true, got %#v", got["dryRun"])
	}
	if got["nodeId"] != float64(123) {
		t.Fatalf("expected nodeId=123, got %#v", got["nodeId"])
	}
	if got["libraryId"] != float64(1) {
		t.Fatalf("expected libraryId=1, got %#v", got["libraryId"])
	}
	if got["builtInType"] != "COMIC" {
		t.Fatalf("expected builtInType=COMIC, got %#v", got["builtInType"])
	}
	if got["totalChildren"] != float64(5) {
		t.Fatalf("expected totalChildren=5, got %#v", got["totalChildren"])
	}
	if got["dirChildren"] != float64(3) {
		t.Fatalf("expected dirChildren=3, got %#v", got["dirChildren"])
	}
	if got["updatedCount"] != float64(2) {
		t.Fatalf("expected updatedCount=2, got %#v", got["updatedCount"])
	}
}

func TestRunFSRecycleClearSuccessJSON(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := NewApp(stdout, stderr)

	t.Setenv(envUsername, "tester")
	t.Setenv(envToken, "token-123")

	originTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/nodes/recycle/library/7/clear" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("dryRun"); got != "true" {
			t.Fatalf("expected dryRun=true, got %q", got)
		}
		if got := r.Header.Get("username"); got != "tester" {
			t.Fatalf("expected username header to be set, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("expected authorization header to be set, got %q", got)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(strings.NewReader(`{"code":"0","message":"ok","data":{"clearedCount":4},"request_id":"req-clear"}`)),
		}, nil
	})
	defer func() {
		http.DefaultTransport = originTransport
	}()

	exitCode := app.Run([]string{
		"fs", "recycle", "clear",
		"--base-url", "http://example.test",
		"--library-id", "7",
		"--dry-run",
		"--json",
	})
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got: %s", stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("expected valid JSON output, got err=%v output=%s", err, stdout.String())
	}
	if got["dryRun"] != true {
		t.Fatalf("expected dryRun=true, got %#v", got["dryRun"])
	}
	if got["libraryId"] != float64(7) {
		t.Fatalf("expected libraryId=7, got %#v", got["libraryId"])
	}
	if got["clearedCount"] != float64(4) {
		t.Fatalf("expected clearedCount=4, got %#v", got["clearedCount"])
	}
}
