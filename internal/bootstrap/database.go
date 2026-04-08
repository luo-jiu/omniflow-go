package bootstrap

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
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
		Logger:               newGORMLogger(cfg),
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

	cleanup := func() {
		if err := sqlDB.Close(); err != nil {
			logger.Error("close database", "error", err)
		}
	}

	return gormDB, cleanup, nil
}

func newGORMLogger(cfg *config.Config) gormlogger.Interface {
	level := gormlogger.Warn

	switch cfg.Database.LogLevel {
	case "silent":
		level = gormlogger.Silent
	case "error":
		level = gormlogger.Error
	case "info":
		level = gormlogger.Info
	}

	return gormlogger.New(
		log.New(os.Stdout, "[gorm] ", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  level,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
}
