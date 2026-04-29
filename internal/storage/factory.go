package storage

import "omniflow-go/internal/config"

// ProviderFactory 根据 provider 配置创建 ObjectStorage 实例。
// alias 用于日志和标识，cfg 包含连接参数。
// 返回实例、cleanup 函数和可能的错误。
type ProviderFactory func(alias string, cfg config.ProviderConfig) (ObjectStorage, func(), error)
