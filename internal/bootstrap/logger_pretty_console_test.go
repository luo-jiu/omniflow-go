package bootstrap

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestPrettyConsoleHandler_GORMLogStripsCommonMeta(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(newPrettyConsoleHandler(&output, prettyConsoleOptions{
		Level: slog.LevelDebug,
		Color: false,
	})).With(
		"service", "omniflow-go",
		"env", "local",
		"version", "0.1.0",
		"component", "gorm",
	)

	logger.Info("[1.200ms] [rows:1] SELECT 1")
	text := output.String()

	for _, key := range []string{`"component"`, `"service"`, `"env"`, `"version"`} {
		if strings.Contains(text, key) {
			t.Fatalf("output contains %s, got: %s", key, text)
		}
	}
	if !strings.Contains(text, "SELECT 1") {
		t.Fatalf("output does not contain SQL, got: %s", text)
	}
}

func TestPrettyConsoleHandler_NonGORMLogKeepsCommonMeta(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(newPrettyConsoleHandler(&output, prettyConsoleOptions{
		Level: slog.LevelDebug,
		Color: false,
	})).With(
		"service", "omniflow-go",
		"env", "local",
		"version", "0.1.0",
		"component", "http",
	)

	logger.Info("http request")
	text := output.String()

	for _, key := range []string{`"component":"http"`, `"service":"omniflow-go"`, `"env":"local"`, `"version":"0.1.0"`} {
		if !strings.Contains(text, key) {
			t.Fatalf("output missing %s, got: %s", key, text)
		}
	}
}

func TestPrettyConsoleHandler_GORMLogColorizesSQLMessage(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(newPrettyConsoleHandler(&output, prettyConsoleOptions{
		Level: slog.LevelDebug,
		Color: true,
	})).With(
		"component", "gorm",
	)

	logger.Info("SELECT 1")
	text := output.String()

	expected := ansiGreen + "SELECT 1" + ansiReset
	if !strings.Contains(text, expected) {
		t.Fatalf("output does not contain colorized SQL message, want %q, got: %s", expected, text)
	}
}
