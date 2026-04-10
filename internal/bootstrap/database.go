package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"omniflow-go/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func NewDatabase(cfg *config.Config, logger *slog.Logger) (*gorm.DB, func(), error) {
	sqlDB, err := sql.Open("pgx", cfg.Database.DSN)
	if err != nil {
		return nil, nil, fmt.Errorf("open sql database: %w", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		Logger:               newGORMLogger(cfg, logger),
		DisableAutomaticPing: true,
	})
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err = gormDB.DB()
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("get sql db: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	logger.Info("database logger configured",
		"log_level", cfg.Database.LogLevel,
		"debug_sql", cfg.Database.DebugSQL,
		"slow_threshold_ms", 200,
		"colorful", cfg.Log.Console.Enabled && cfg.Log.Console.Color && strings.EqualFold(cfg.Log.Format, "text"),
	)

	cleanup := func() {
		if err := sqlDB.Close(); err != nil {
			logger.Error("close database", "error", err)
		}
	}

	return gormDB, cleanup, nil
}

func newGORMLogger(cfg *config.Config, logger *slog.Logger) gormlogger.Interface {
	level := resolveGORMLogLevel(cfg.Database.LogLevel, cfg.Database.DebugSQL)

	return &gormSlogLogger{
		logger:                    logger.With("component", "gorm"),
		level:                     level,
		slowThreshold:             200 * time.Millisecond,
		ignoreRecordNotFoundError: true,
	}
}

func resolveGORMLogLevel(raw string, debugSQL bool) gormlogger.LogLevel {
	if debugSQL {
		return gormlogger.Info
	}

	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "silent":
		return gormlogger.Silent
	case "error":
		return gormlogger.Error
	case "info":
		return gormlogger.Info
	default:
		return gormlogger.Warn
	}
}

type gormSlogLogger struct {
	logger                    *slog.Logger
	level                     gormlogger.LogLevel
	slowThreshold             time.Duration
	ignoreRecordNotFoundError bool
}

func (l *gormSlogLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	clone := *l
	clone.level = level
	return &clone
}

func (l *gormSlogLogger) Info(ctx context.Context, msg string, args ...any) {
	if l.level < gormlogger.Info {
		return
	}
	l.logger.InfoContext(ctx, fmt.Sprintf(msg, args...))
}

func (l *gormSlogLogger) Warn(ctx context.Context, msg string, args ...any) {
	if l.level < gormlogger.Warn {
		return
	}
	l.logger.WarnContext(ctx, fmt.Sprintf(msg, args...))
}

func (l *gormSlogLogger) Error(ctx context.Context, msg string, args ...any) {
	if l.level < gormlogger.Error {
		return
	}
	l.logger.ErrorContext(ctx, fmt.Sprintf(msg, args...))
}

func (l *gormSlogLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level == gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sqlText, rowsAffected := fc()
	line := formatGORMSQLLine(elapsed, rowsAffected, sqlText)

	if err != nil {
		if l.ignoreRecordNotFoundError && errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		if l.level >= gormlogger.Error {
			l.logger.ErrorContext(ctx, line, "error", err)
		}
		return
	}

	if l.slowThreshold > 0 && elapsed > l.slowThreshold {
		if l.level >= gormlogger.Warn {
			l.logger.WarnContext(ctx, line, "slow_threshold_ms", l.slowThreshold.Milliseconds())
		}
		return
	}

	if l.level >= gormlogger.Info {
		l.logger.InfoContext(ctx, line)
	}
}

func formatGORMSQLLine(elapsed time.Duration, rowsAffected int64, sqlText string) string {
	rowsText := "-"
	if rowsAffected >= 0 {
		rowsText = fmt.Sprintf("%d", rowsAffected)
	}

	return fmt.Sprintf(
		"[%.3fms] [rows:%s] %s",
		float64(elapsed.Microseconds())/1000,
		rowsText,
		strings.TrimSpace(sqlText),
	)
}
