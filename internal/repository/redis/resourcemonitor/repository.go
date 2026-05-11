package resourcemonitor

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

// Repository 提供资源监测所需的 Redis 只读探针。
type Repository struct {
	client *redis.Client
}

// NewRepository 创建 Redis 探针仓储。
func NewRepository(client *redis.Client) *Repository {
	return &Repository{client: client}
}

// Ping 执行 Redis 只读连通性检查。
func (r *Repository) Ping(ctx context.Context) error {
	if r.client == nil {
		return errors.New("resource monitor redis repository: client is nil")
	}
	return r.client.Ping(ctx).Err()
}
