package repository

import (
	domain "omniflow-go/internal/domain/resourcemonitor"
	resourcemonitorpg "omniflow-go/internal/repository/postgres/impl/resourcemonitor"
	resourcemonitorredis "omniflow-go/internal/repository/redis/resourcemonitor"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// NewResourceMonitorRepository 创建资源监测只读仓储。
func NewResourceMonitorRepository(db *gorm.DB) domain.Repository {
	return resourcemonitorpg.NewRepository(db)
}

// NewResourceMonitorRedisProbeRepository 创建资源监测 Redis 只读探针仓储。
func NewResourceMonitorRedisProbeRepository(client *redis.Client) domain.RedisProbeRepository {
	return resourcemonitorredis.NewRepository(client)
}
