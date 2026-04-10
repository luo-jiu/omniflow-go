package bootstrap

import (
	"testing"
	"time"

	gormlogger "gorm.io/gorm/logger"
)

func TestResolveGORMLogLevel(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		debugSQL  bool
		wantLevel gormlogger.LogLevel
	}{
		{name: "debug sql overrides", raw: "silent", debugSQL: true, wantLevel: gormlogger.Info},
		{name: "silent", raw: "silent", wantLevel: gormlogger.Silent},
		{name: "error", raw: "error", wantLevel: gormlogger.Error},
		{name: "info", raw: "info", wantLevel: gormlogger.Info},
		{name: "default warn", raw: "warn", wantLevel: gormlogger.Warn},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := resolveGORMLogLevel(test.raw, test.debugSQL)
			if got != test.wantLevel {
				t.Fatalf("resolveGORMLogLevel(%q, %v) = %v, want %v", test.raw, test.debugSQL, got, test.wantLevel)
			}
		})
	}
}

func TestFormatGORMSQLLine(t *testing.T) {
	got := formatGORMSQLLine(1500*time.Microsecond, 2, "  SELECT 1  ")
	want := "[1.500ms] [rows:2] SELECT 1"
	if got != want {
		t.Fatalf("formatGORMSQLLine() = %q, want %q", got, want)
	}
}
