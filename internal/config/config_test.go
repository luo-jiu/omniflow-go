package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_DefaultsWhenConfigFileMissing(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.yaml")

	cfg, err := Load(missingPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Mode != "debug" {
		t.Fatalf("Server.Mode = %q, want %q", cfg.Server.Mode, "debug")
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
	if cfg.Log.Format != "text" {
		t.Fatalf("Log.Format = %q, want %q", cfg.Log.Format, "text")
	}
	if !cfg.Log.Console.Enabled {
		t.Fatal("Log.Console.Enabled = false, want true")
	}
	if !cfg.Log.Console.Color {
		t.Fatal("Log.Console.Color = false, want true")
	}
	if cfg.Database.LogLevel != "warn" {
		t.Fatalf("Database.LogLevel = %q, want %q", cfg.Database.LogLevel, "warn")
	}
}

func TestLoad_ReleaseModeUsesReleaseLogDefaults(t *testing.T) {
	path := writeTempConfig(t, `
server:
  mode: release
log: {}
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Mode != "release" {
		t.Fatalf("Server.Mode = %q, want %q", cfg.Server.Mode, "release")
	}
	if cfg.Log.Level != "info" {
		t.Fatalf("Log.Level = %q, want %q", cfg.Log.Level, "info")
	}
	if cfg.Log.Format != "json" {
		t.Fatalf("Log.Format = %q, want %q", cfg.Log.Format, "json")
	}
}

func TestLoad_InvalidServerModeReturnsError(t *testing.T) {
	path := writeTempConfig(t, `
server:
  mode: prod
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "invalid server.mode") {
		t.Fatalf("error = %v, want contains %q", err, "invalid server.mode")
	}
}

func TestLoad_InvalidLogLevelReturnsError(t *testing.T) {
	path := writeTempConfig(t, `
server:
  mode: release
log:
  level: verbose
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "invalid log.level") {
		t.Fatalf("error = %v, want contains %q", err, "invalid log.level")
	}
}

func TestLoad_InvalidLogFormatReturnsError(t *testing.T) {
	path := writeTempConfig(t, `
server:
  mode: release
log:
  format: pretty
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "invalid log.format") {
		t.Fatalf("error = %v, want contains %q", err, "invalid log.format")
	}
}

func TestLoad_DatabaseDebugSQLForcesInfoLogLevel(t *testing.T) {
	path := writeTempConfig(t, `
database:
  log_level: error
  debug_sql: true
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Database.DebugSQL {
		t.Fatal("Database.DebugSQL = false, want true")
	}
	if cfg.Database.LogLevel != "info" {
		t.Fatalf("Database.LogLevel = %q, want %q", cfg.Database.LogLevel, "info")
	}
}

func TestLoad_InvalidDatabaseLogLevelReturnsError(t *testing.T) {
	path := writeTempConfig(t, `
database:
  log_level: trace
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "invalid database.log_level") {
		t.Fatalf("error = %v, want contains %q", err, "invalid database.log_level")
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
