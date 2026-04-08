package repository

import (
	"fmt"
	"strings"

	"omniflow-go/internal/config"
	objectminio "omniflow-go/internal/repository/object/minio"
	"omniflow-go/internal/storage"
)

const providerMinIO = "minio"

// NewObjectStorage 根据配置选择对象存储实现。
// 当前默认 provider 为 minio，其他 provider 仅预留扩展入口。
func NewObjectStorage(cfg *config.Config) (storage.ObjectStorage, func(), error) {
	provider := strings.TrimSpace(strings.ToLower(cfg.Storage.Provider))
	if provider == "" {
		provider = providerMinIO
	}

	switch provider {
	case providerMinIO:
		return objectminio.NewStore(cfg)
	case "s3", "oss", "cos", "obs":
		return nil, func() {}, fmt.Errorf("%w: %s", storage.ErrProviderNotImplemented, provider)
	default:
		return nil, func() {}, fmt.Errorf("%w: %s", storage.ErrProviderUnknown, provider)
	}
}
