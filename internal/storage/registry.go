package storage

import (
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"sync"

	"omniflow-go/internal/config"
)

// StorageRegistry 管理多个命名存储 provider 实例，支持热加载。
type StorageRegistry struct {
	mu           sync.RWMutex
	providers    map[string]ObjectStorage
	cleanups     map[string]func()
	defaultAlias string
	routingRules []config.RoutingRule
	storageCfg   *config.StorageConfig
}

// NewStorageRegistry 创建空的存储注册表。
func NewStorageRegistry() *StorageRegistry {
	return &StorageRegistry{
		providers: make(map[string]ObjectStorage),
		cleanups:  make(map[string]func()),
	}
}

// Get 获取指定别名的 provider。精确匹配失败时做大小写不敏感回退。
func (r *StorageRegistry) Get(alias string) (ObjectStorage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if store, ok := r.providers[alias]; ok {
		return store, nil
	}

	lower := strings.ToLower(alias)
	for k, store := range r.providers {
		if strings.ToLower(k) == lower {
			slog.Warn("storage provider alias case-insensitive fallback (consider migrating DB data)",
				"requested", alias, "matched", k)
			return store, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrProviderUnknown, alias)
}

// Default 获取默认 provider 及其别名。
func (r *StorageRegistry) Default() (ObjectStorage, string, error) {
	r.mu.RLock()
	alias := r.defaultAlias
	r.mu.RUnlock()

	if alias == "" {
		return nil, "", fmt.Errorf("%w: no default provider configured", ErrProviderUnknown)
	}
	store, err := r.Get(alias)
	if err != nil {
		return nil, alias, err
	}
	return store, alias, nil
}

// Resolve 根据文件元信息匹配路由规则，返回目标 provider 和别名。
// 未命中任何规则时回退到默认 provider。
// 若指定了 overrideAlias 则直接使用。
func (r *StorageRegistry) Resolve(fileSize int64, ext string, mimeType string) (ObjectStorage, string, error) {
	r.mu.RLock()
	rules := r.routingRules
	defaultAlias := r.defaultAlias
	r.mu.RUnlock()

	alias := MatchRoutingRules(rules, fileSize, ext, mimeType)
	if alias == "" {
		alias = defaultAlias
	}
	if alias == "" {
		return nil, "", fmt.Errorf("%w: no default provider configured", ErrProviderUnknown)
	}

	store, err := r.Get(alias)
	if err != nil {
		return nil, alias, err
	}
	return store, alias, nil
}

// Reload 使用新配置重新加载 provider 实例。
// 同名且配置未变的 provider 复用现有实例。
// 返回被移除的旧 provider 别名列表。
func (r *StorageRegistry) Reload(cfg *config.StorageConfig, factory ProviderFactory) ([]string, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	newProviders := make(map[string]ObjectStorage, len(cfg.Providers))
	newCleanups := make(map[string]func(), len(cfg.Providers))

	r.mu.RLock()
	oldProviders := r.providers
	oldCleanups := r.cleanups
	oldCfg := r.storageCfg
	r.mu.RUnlock()

	for alias, pcfg := range cfg.Providers {
		if oldCfg != nil {
			if oldPcfg, ok := oldCfg.Providers[alias]; ok && providerConfigEqual(oldPcfg, pcfg) {
				if store, ok := oldProviders[alias]; ok {
					newProviders[alias] = store
					newCleanups[alias] = oldCleanups[alias]
					continue
				}
			}
		}

		store, cleanup, err := factory(alias, pcfg)
		if err != nil {
			for _, c := range newCleanups {
				if c != nil {
					c()
				}
			}
			return nil, fmt.Errorf("create provider %q: %w", alias, err)
		}
		newProviders[alias] = store
		newCleanups[alias] = cleanup
	}

	var removed []string
	for alias, cleanup := range oldCleanups {
		if _, exists := newProviders[alias]; !exists {
			removed = append(removed, alias)
			if cleanup != nil {
				cleanup()
			}
		}
	}

	r.mu.Lock()
	r.providers = newProviders
	r.cleanups = newCleanups
	r.defaultAlias = cfg.DefaultProvider
	r.routingRules = cfg.RoutingRules
	r.storageCfg = cfg
	r.mu.Unlock()

	if len(removed) > 0 {
		slog.Info("storage providers removed during reload", "aliases", removed)
	}

	return removed, nil
}

// Aliases 返回当前所有活跃的 provider 别名。
func (r *StorageRegistry) Aliases() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	aliases := make([]string, 0, len(r.providers))
	for k := range r.providers {
		aliases = append(aliases, k)
	}
	return aliases
}

// DefaultAlias 返回当前默认 provider 的别名。
func (r *StorageRegistry) DefaultAlias() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultAlias
}

// RoutingRules 返回当前路由规则的副本。
func (r *StorageRegistry) RoutingRules() []config.RoutingRule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rules := make([]config.RoutingRule, len(r.routingRules))
	copy(rules, r.routingRules)
	return rules
}

// StorageConfig 返回当前配置快照（密钥脱敏）。
func (r *StorageRegistry) StorageConfig() *config.StorageConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.storageCfg == nil {
		return nil
	}
	masked := r.storageCfg.MaskSecrets()
	return &masked
}

// RawStorageConfig 返回当前配置快照（含密钥，仅限内部写回文件使用）。
func (r *StorageRegistry) RawStorageConfig() *config.StorageConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.storageCfg == nil {
		return nil
	}
	cp := *r.storageCfg
	providers := make(map[string]config.ProviderConfig, len(cp.Providers))
	maps.Copy(providers, cp.Providers)
	cp.Providers = providers
	return &cp
}

// Close 关闭所有 provider 连接。
func (r *StorageRegistry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, cleanup := range r.cleanups {
		if cleanup != nil {
			cleanup()
		}
	}
	r.providers = make(map[string]ObjectStorage)
	r.cleanups = make(map[string]func())
}

func providerConfigEqual(a, b config.ProviderConfig) bool {
	return a.Type == b.Type &&
		a.Endpoint == b.Endpoint &&
		a.AccessKey == b.AccessKey &&
		a.SecretKey == b.SecretKey &&
		a.UseSSL == b.UseSSL &&
		a.Bucket == b.Bucket &&
		a.Region == b.Region
}
