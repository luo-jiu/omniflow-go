package config

import (
	"net/url"
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

func TestLoad_EnvironmentOverridesYAML(t *testing.T) {
	path := writeTempConfig(t, `
app:
  env: local
server:
  port: 8850
  mode: debug
database:
  dsn: postgres://local
redis:
  addr: 127.0.0.1:6379
  password: local
`)
	t.Setenv("OMNIFLOW_APP_ENV", "production")
	t.Setenv("OMNIFLOW_SERVER_PORT", "9080")
	t.Setenv("OMNIFLOW_SERVER_MODE", "release")
	t.Setenv("OMNIFLOW_DATABASE_DSN", "postgres://production")
	t.Setenv("OMNIFLOW_REDIS_ADDR", "redis:6379")
	t.Setenv("OMNIFLOW_REDIS_PASSWORD", "secret")
	t.Setenv("OMNIFLOW_DATABASE_DEBUG_SQL", "true")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Env != "production" {
		t.Fatalf("App.Env = %q, want %q", cfg.App.Env, "production")
	}
	if cfg.Server.Port != 9080 || cfg.Server.Mode != "release" {
		t.Fatalf("Server = %+v, want port 9080 and release mode", cfg.Server)
	}
	if cfg.Database.DSN != "postgres://production" || !cfg.Database.DebugSQL {
		t.Fatalf("Database = %+v, want environment overrides", cfg.Database)
	}
	if cfg.Redis.Addr != "redis:6379" || cfg.Redis.Password != "secret" {
		t.Fatalf("Redis = %+v, want environment overrides", cfg.Redis)
	}
}

func TestLoad_InvalidEnvironmentOverrideReturnsError(t *testing.T) {
	t.Setenv("OMNIFLOW_SERVER_PORT", "not-a-port")

	_, err := Load("")
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "OMNIFLOW_SERVER_PORT must be an integer") {
		t.Fatalf("error = %v, want environment variable name", err)
	}
}

func TestLoad_EmptyEnvironmentOverrideIsIgnored(t *testing.T) {
	t.Setenv("OMNIFLOW_DATABASE_DSN", "  ")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Database.DSN != "postgres://postgres:123456@127.0.0.1:5432/omniflow?sslmode=disable" {
		t.Fatalf("Database.DSN = %q, want default", cfg.Database.DSN)
	}
}

func TestLoad_StructuredDatabaseEnvironmentEscapesCredentials(t *testing.T) {
	t.Setenv("OMNIFLOW_DATABASE_HOST", "postgres")
	t.Setenv("OMNIFLOW_DATABASE_PORT", "5432")
	t.Setenv("OMNIFLOW_DATABASE_USER", "omniflow")
	t.Setenv("OMNIFLOW_DATABASE_PASSWORD", "colon:at@slash/value")
	t.Setenv("OMNIFLOW_DATABASE_NAME", "omniflow")
	t.Setenv("OMNIFLOW_DATABASE_SSLMODE", "disable")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	parsed, err := url.Parse(cfg.Database.DSN)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	password, ok := parsed.User.Password()
	if !ok || password != "colon:at@slash/value" {
		t.Fatalf("parsed password = %q, %v, want original password", password, ok)
	}
	if parsed.Host != "postgres:5432" || parsed.User.Username() != "omniflow" || parsed.Path != "/omniflow" {
		t.Fatalf("parsed DSN = %#v, want production database fields", parsed)
	}
}

func TestLoad_StructuredDatabaseEnvironmentRequiresCompleteCredentials(t *testing.T) {
	t.Setenv("OMNIFLOW_DATABASE_HOST", "postgres")

	_, err := Load("")
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "OMNIFLOW_DATABASE_USER is required") {
		t.Fatalf("error = %v, want missing environment variable", err)
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
