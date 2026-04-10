package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"omniflow-go/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	var (
		configPath       string
		yes              bool
		dbOnly           bool
		minioOnly        bool
		allowNonLocal    bool
		includeLibraries bool
		timeout          time.Duration
	)

	flag.StringVar(&configPath, "config", "./configs/config.yaml", "配置文件路径")
	flag.BoolVar(&yes, "yes", false, "确认执行重置（必须显式传入）")
	flag.BoolVar(&dbOnly, "db-only", false, "仅清理目录树相关表")
	flag.BoolVar(&minioOnly, "minio-only", false, "仅清空 MinIO 桶对象")
	flag.BoolVar(&allowNonLocal, "allow-nonlocal", false, "允许在非 local/dev/test 环境执行重置（高风险）")
	flag.BoolVar(&includeLibraries, "include-libraries", false, "同时清理 libraries 表")
	flag.DurationVar(&timeout, "timeout", 90*time.Second, "单次重置超时时间")
	flag.Parse()

	if dbOnly && minioOnly {
		log.Fatal("参数冲突：--db-only 与 --minio-only 不能同时传入")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	if !allowNonLocal && !isSafeResetEnv(cfg.App.Env) {
		log.Fatalf(
			"已停止：当前 app.env=%q，不允许执行破坏性重置。仅支持 local/dev/test；如确认需执行请追加 --allow-nonlocal",
			strings.TrimSpace(cfg.App.Env),
		)
	}

	targetTables := []string{"node_files", "storage_objects", "nodes"}
	if !minioOnly {
		probeCtx, probeCancel := context.WithTimeout(context.Background(), timeout)
		tables, err := resolveExistingTables(probeCtx, cfg.Database.DSN, candidateTreeTables(includeLibraries))
		probeCancel()
		if err != nil {
			log.Fatalf("检测目录树表失败: %v", err)
		}
		targetTables = tables
	}

	printPlan(configPath, cfg, targetTables, dbOnly, minioOnly)
	if !yes {
		log.Fatal("已停止：这是破坏性操作，请加 --yes 确认执行")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if !minioOnly && len(targetTables) > 0 {
		if err := resetTreeTables(ctx, cfg.Database.DSN, targetTables); err != nil {
			log.Fatalf("清理目录树相关表失败: %v", err)
		}
		log.Printf("目录树相关表已重置: %s", strings.Join(targetTables, ", "))
	} else if !minioOnly {
		log.Printf("未检测到可重置的目录树相关表，跳过 PostgreSQL 清理")
	}

	if !dbOnly {
		deleted, err := resetMinIOBucket(ctx, cfg.MinIO)
		if err != nil {
			log.Fatalf("清理 MinIO 失败: %v", err)
		}
		log.Printf("MinIO 清理完成: bucket=%s, deleted_objects=%d", cfg.MinIO.Bucket, deleted)
	}

	log.Println("重置完成")
}

func candidateTreeTables(includeLibraries bool) []string {
	tables := []string{
		"file_ancestors",
		"file_metadata",
		"file_path_cache",
		"node_tag_rel",
		"node_files",
		"storage_objects",
		"nodes",
	}
	if includeLibraries {
		tables = append(tables, "libraries")
	}
	return tables
}

func printPlan(configPath string, cfg *config.Config, tables []string, dbOnly, minioOnly bool) {
	log.Printf("重置计划: config=%s", configPath)
	if !minioOnly {
		log.Printf("  - PostgreSQL: %s", safeDSN(cfg.Database.DSN))
		if len(tables) == 0 {
			log.Printf("  - Truncate tables: (none)")
		} else {
			log.Printf("  - Truncate tables: %s", strings.Join(tables, ", "))
		}
	}
	if !dbOnly {
		log.Printf("  - MinIO endpoint: %s", cfg.MinIO.Endpoint)
		log.Printf("  - MinIO bucket: %s", cfg.MinIO.Bucket)
	}
}

func resolveExistingTables(ctx context.Context, dsn string, candidates []string) ([]string, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}

	result := make([]string, 0, len(candidates))
	for _, table := range candidates {
		var reg sql.NullString
		if err := db.WithContext(ctx).
			Raw("SELECT to_regclass(?)", "public."+table).
			Scan(&reg).Error; err != nil {
			return nil, err
		}
		if reg.Valid && strings.TrimSpace(reg.String) != "" {
			result = append(result, table)
		}
	}
	return result, nil
}

func resetTreeTables(ctx context.Context, dsn string, tables []string) error {
	if strings.TrimSpace(dsn) == "" {
		return fmt.Errorf("database dsn 为空")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}

	sql := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(tables, ", "))
	if err := db.WithContext(ctx).Exec(sql).Error; err != nil {
		return err
	}
	return nil
}

func resetMinIOBucket(ctx context.Context, cfg config.MinIO) (int, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	bucket := strings.TrimSpace(cfg.Bucket)
	if endpoint == "" {
		return 0, fmt.Errorf("minio endpoint 为空")
	}
	if bucket == "" {
		return 0, fmt.Errorf("minio bucket 为空")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(strings.TrimSpace(cfg.AccessKey), strings.TrimSpace(cfg.SecretKey), ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return 0, err
	}

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}

	deleted := 0
	for obj := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
		if obj.Err != nil {
			return deleted, obj.Err
		}
		if err := client.RemoveObject(ctx, bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func safeDSN(dsn string) string {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return ""
	}

	i := strings.Index(trimmed, "://")
	if i < 0 {
		return trimmed
	}
	prefix := trimmed[:i+3]
	rest := trimmed[i+3:]

	at := strings.Index(rest, "@")
	if at < 0 {
		return trimmed
	}
	return prefix + "***:***@" + rest[at+1:]
}

func isSafeResetEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "", "local", "dev", "test":
		return true
	default:
		return false
	}
}
