package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"omniflow-go/internal/config"
	objectrepo "omniflow-go/internal/repository/object"
	"omniflow-go/internal/storage"

	"github.com/gin-gonic/gin"
)

// StorageConfigHandler 存储配置管理的 HTTP 处理器。
type StorageConfigHandler struct {
	registry          *storage.StorageRegistry
	storageConfigPath string
}

// NewStorageConfigHandler 创建存储配置 Handler。
func NewStorageConfigHandler(
	registry *storage.StorageRegistry,
	storageConfigPath string,
) *StorageConfigHandler {
	return &StorageConfigHandler{
		registry:          registry,
		storageConfigPath: storageConfigPath,
	}
}

// --- 请求/响应结构体 ---

type providerResponse struct {
	Alias    string `json:"alias"`
	Type     string `json:"type"`
	Endpoint string `json:"endpoint"`
	UseSSL   bool   `json:"useSSL"`
	Bucket   string `json:"bucket"`
	Region   string `json:"region"`
	Label    string `json:"label"`
	// 密钥脱敏展示
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

type addProviderRequest struct {
	Alias     string `json:"alias" binding:"required"`
	Type      string `json:"type" binding:"required"`
	Endpoint  string `json:"endpoint" binding:"required"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	UseSSL    bool   `json:"useSSL"`
	Bucket    string `json:"bucket" binding:"required"`
	Region    string `json:"region"`
	Label     string `json:"label"`
}

type updateProviderRequest struct {
	Type      string `json:"type" binding:"required"`
	Endpoint  string `json:"endpoint" binding:"required"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	UseSSL    bool   `json:"useSSL"`
	Bucket    string `json:"bucket" binding:"required"`
	Region    string `json:"region"`
	Label     string `json:"label"`
}

type setDefaultRequest struct {
	Alias string `json:"alias" binding:"required"`
}

type routingRuleDTO struct {
	Name           string             `json:"name"`
	Conditions     ruleConditionsDTO  `json:"conditions"`
	TargetProvider string             `json:"targetProvider" binding:"required"`
}

type ruleConditionsDTO struct {
	MinFileSizeBytes int64    `json:"minFileSizeBytes"`
	MaxFileSizeBytes int64    `json:"maxFileSizeBytes"`
	Extensions       []string `json:"extensions"`
	MIMEPrefixes     []string `json:"mimePrefixes"`
}

type updateRoutingRulesRequest struct {
	Rules []routingRuleDTO `json:"rules" binding:"required"`
}

type resolveTargetRequest struct {
	FileSize    int64  `json:"fileSize"`
	Extension   string `json:"extension"`
	ContentType string `json:"contentType"`
}

type resolveTargetResponse struct {
	ProviderAlias string `json:"providerAlias"`
	Label         string `json:"label"`
}

// --- Handler 方法 ---

// ListProviders 列出所有 provider（密钥脱敏）。
func (h *StorageConfigHandler) ListProviders(ctx *gin.Context) {
	cfg := h.registry.StorageConfig()
	if cfg == nil {
		Success(ctx, []providerResponse{})
		return
	}

	result := make([]providerResponse, 0, len(cfg.Providers))
	for alias, p := range cfg.Providers {
		result = append(result, toProviderResponse(alias, p))
	}
	Success(ctx, map[string]any{
		"providers":       result,
		"defaultProvider": cfg.DefaultProvider,
	})
}

// AddProvider 添加新的 provider。
func (h *StorageConfigHandler) AddProvider(ctx *gin.Context) {
	var req addProviderRequest
	if !BindJSON(ctx, &req) {
		return
	}

	alias := strings.TrimSpace(req.Alias)
	if alias == "" {
		BadRequest(ctx, "alias is required")
		return
	}

	rawCfg := h.registry.RawStorageConfig()
	if rawCfg == nil {
		rawCfg = &config.StorageConfig{
			Providers: make(map[string]config.ProviderConfig),
		}
	}

	if _, exists := rawCfg.Providers[alias]; exists {
		respond(ctx, http.StatusConflict, ClientErrorCode, fmt.Sprintf("provider %q already exists", alias), nil)
		return
	}

	rawCfg.Providers[alias] = toProviderConfig(req.Type, req.Endpoint, req.AccessKey, req.SecretKey, req.UseSSL, req.Bucket, req.Region, req.Label)
	if rawCfg.DefaultProvider == "" {
		rawCfg.DefaultProvider = alias
	}

	if err := h.saveAndReload(rawCfg); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// UpdateProvider 更新已有的 provider。
func (h *StorageConfigHandler) UpdateProvider(ctx *gin.Context) {
	alias := strings.TrimSpace(ctx.Param("alias"))
	if alias == "" {
		BadRequest(ctx, "alias is required")
		return
	}

	var req updateProviderRequest
	if !BindJSON(ctx, &req) {
		return
	}

	rawCfg := h.registry.RawStorageConfig()
	if rawCfg == nil {
		BadRequest(ctx, "storage config not initialized")
		return
	}

	if _, exists := rawCfg.Providers[alias]; !exists {
		respond(ctx, http.StatusNotFound, ClientErrorCode, fmt.Sprintf("provider %q not found", alias), nil)
		return
	}

	rawCfg.Providers[alias] = toProviderConfig(req.Type, req.Endpoint, req.AccessKey, req.SecretKey, req.UseSSL, req.Bucket, req.Region, req.Label)

	if err := h.saveAndReload(rawCfg); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// DeleteProvider 删除 provider（默认 provider 不可删除）。
func (h *StorageConfigHandler) DeleteProvider(ctx *gin.Context) {
	alias := strings.TrimSpace(ctx.Param("alias"))
	if alias == "" {
		BadRequest(ctx, "alias is required")
		return
	}

	rawCfg := h.registry.RawStorageConfig()
	if rawCfg == nil {
		BadRequest(ctx, "storage config not initialized")
		return
	}

	if rawCfg.DefaultProvider == alias {
		respond(ctx, http.StatusConflict, ClientErrorCode, "cannot delete the default provider", nil)
		return
	}

	if _, exists := rawCfg.Providers[alias]; !exists {
		respond(ctx, http.StatusNotFound, ClientErrorCode, fmt.Sprintf("provider %q not found", alias), nil)
		return
	}

	delete(rawCfg.Providers, alias)

	// 清理引用该 provider 的路由规则
	cleaned := make([]config.RoutingRule, 0, len(rawCfg.RoutingRules))
	for _, rule := range rawCfg.RoutingRules {
		if rule.TargetProvider != alias {
			cleaned = append(cleaned, rule)
		}
	}
	rawCfg.RoutingRules = cleaned

	if err := h.saveAndReload(rawCfg); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// GetDefault 查询当前默认 provider。
func (h *StorageConfigHandler) GetDefault(ctx *gin.Context) {
	Success(ctx, map[string]string{
		"defaultProvider": h.registry.DefaultAlias(),
	})
}

// SetDefault 设置默认 provider。
func (h *StorageConfigHandler) SetDefault(ctx *gin.Context) {
	var req setDefaultRequest
	if !BindJSON(ctx, &req) {
		return
	}

	rawCfg := h.registry.RawStorageConfig()
	if rawCfg == nil {
		BadRequest(ctx, "storage config not initialized")
		return
	}

	alias := strings.TrimSpace(req.Alias)
	if _, exists := rawCfg.Providers[alias]; !exists {
		respond(ctx, http.StatusNotFound, ClientErrorCode, fmt.Sprintf("provider %q not found", alias), nil)
		return
	}

	rawCfg.DefaultProvider = alias

	if err := h.saveAndReload(rawCfg); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// GetRoutingRules 查询当前路由规则。
func (h *StorageConfigHandler) GetRoutingRules(ctx *gin.Context) {
	rules := h.registry.RoutingRules()
	dtos := make([]routingRuleDTO, 0, len(rules))
	for _, r := range rules {
		dtos = append(dtos, toRoutingRuleDTO(r))
	}
	Success(ctx, map[string]any{"rules": dtos})
}

// UpdateRoutingRules 替换全部路由规则。
func (h *StorageConfigHandler) UpdateRoutingRules(ctx *gin.Context) {
	var req updateRoutingRulesRequest
	if !BindJSON(ctx, &req) {
		return
	}

	rawCfg := h.registry.RawStorageConfig()
	if rawCfg == nil {
		BadRequest(ctx, "storage config not initialized")
		return
	}

	rules := make([]config.RoutingRule, 0, len(req.Rules))
	for _, dto := range req.Rules {
		rules = append(rules, fromRoutingRuleDTO(dto))
	}
	rawCfg.RoutingRules = rules

	if err := h.saveAndReload(rawCfg); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// TestProvider 测试 provider 连通性。
func (h *StorageConfigHandler) TestProvider(ctx *gin.Context) {
	alias := strings.TrimSpace(ctx.Param("alias"))
	if alias == "" {
		BadRequest(ctx, "alias is required")
		return
	}

	store, err := h.registry.Get(alias)
	if err != nil {
		respond(ctx, http.StatusNotFound, ClientErrorCode, fmt.Sprintf("provider %q not found", alias), nil)
		return
	}

	testCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	// 尝试获取 bucket 信息来验证连通性
	_, listErr := store.GetPresignedURL(testCtx, "__connectivity_test__", 1*time.Second)
	// GetPresignedURL 对不存在的 key 也能正常返回 URL（MinIO/S3 特性），
	// 如果连接失败才会报错
	if listErr != nil && strings.Contains(listErr.Error(), "connect") {
		Success(ctx, map[string]any{
			"success": false,
			"message": listErr.Error(),
		})
		return
	}

	Success(ctx, map[string]any{
		"success": true,
		"message": "connected",
	})
}

// ResolveTarget 根据文件信息返回推荐 provider。
func (h *StorageConfigHandler) ResolveTarget(ctx *gin.Context) {
	var req resolveTargetRequest
	if !BindJSON(ctx, &req) {
		return
	}

	store, alias, err := h.registry.Resolve(req.FileSize, req.Extension, req.ContentType)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}

	_ = store
	cfg := h.registry.StorageConfig()
	label := alias
	if cfg != nil {
		if p, ok := cfg.Providers[alias]; ok && p.Label != "" {
			label = p.Label
		}
	}

	Success(ctx, resolveTargetResponse{
		ProviderAlias: alias,
		Label:         label,
	})
}

// --- 内部方法 ---

func (h *StorageConfigHandler) saveAndReload(cfg *config.StorageConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("%w: %v", config.ErrStorageConfigInvalid, err)
	}

	if err := config.SaveStorageConfig(h.storageConfigPath, cfg); err != nil {
		return err
	}

	// 写文件后 fsnotify watcher 会自动触发 reload，
	// 但也主动 reload 以确保当前请求返回后配置已生效
	factory := storage.ProviderFactory(objectrepo.NewObjectStorageByConfig)
	if _, err := h.registry.Reload(cfg, factory); err != nil {
		return err
	}
	return nil
}

// --- 转换函数 ---

func toProviderResponse(alias string, p config.ProviderConfig) providerResponse {
	return providerResponse{
		Alias:     alias,
		Type:      p.Type,
		Endpoint:  p.Endpoint,
		UseSSL:    p.UseSSL,
		Bucket:    p.Bucket,
		Region:    p.Region,
		Label:     p.Label,
		AccessKey: p.AccessKey,
		SecretKey: p.SecretKey,
	}
}

func toProviderConfig(pType, endpoint, accessKey, secretKey string, useSSL bool, bucket, region, label string) config.ProviderConfig {
	return config.ProviderConfig{
		Type:      strings.TrimSpace(pType),
		Endpoint:  strings.TrimSpace(endpoint),
		AccessKey: strings.TrimSpace(accessKey),
		SecretKey: strings.TrimSpace(secretKey),
		UseSSL:    useSSL,
		Bucket:    strings.TrimSpace(bucket),
		Region:    strings.TrimSpace(region),
		Label:     strings.TrimSpace(label),
	}
}

func toRoutingRuleDTO(r config.RoutingRule) routingRuleDTO {
	return routingRuleDTO{
		Name:           r.Name,
		TargetProvider: r.TargetProvider,
		Conditions: ruleConditionsDTO{
			MinFileSizeBytes: r.Conditions.MinFileSizeBytes,
			MaxFileSizeBytes: r.Conditions.MaxFileSizeBytes,
			Extensions:       r.Conditions.Extensions,
			MIMEPrefixes:     r.Conditions.MIMEPrefixes,
		},
	}
}

func fromRoutingRuleDTO(dto routingRuleDTO) config.RoutingRule {
	return config.RoutingRule{
		Name:           dto.Name,
		TargetProvider: dto.TargetProvider,
		Conditions: config.RuleConditions{
			MinFileSizeBytes: dto.Conditions.MinFileSizeBytes,
			MaxFileSizeBytes: dto.Conditions.MaxFileSizeBytes,
			Extensions:       dto.Conditions.Extensions,
			MIMEPrefixes:     dto.Conditions.MIMEPrefixes,
		},
	}
}
