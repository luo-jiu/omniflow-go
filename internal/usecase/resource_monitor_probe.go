package usecase

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"omniflow-go/internal/config"
	domain "omniflow-go/internal/domain/resourcemonitor"
)

const (
	resourceMonitorProbeTimeout      = 2 * time.Second
	resourceMonitorProbeParallelism  = 4
	resourceMonitorProbeErrorMaxSize = 180
)

var (
	probeURIUserInfoPattern = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)([^/\s@]+)@`)
	probeSecretKVPattern    = regexp.MustCompile(
		`(?i)\b(password|passwd|pwd|secret|secret_key|access_key|accesskey|` +
			`access_key_id|accesskeyid|token|credential)(\s*[:=]\s*)([^\s&;,]+)`,
	)
	probeAWSKeyPattern = regexp.MustCompile(`\b(AKIA|ASIA)[A-Z0-9]{12,}\b`)
)

func (u *ResourceMonitorUseCase) probeTargets(
	ctx context.Context,
	checkedAt time.Time,
	defaultProvider string,
) []domain.ProbeTarget {
	targets := make([]domain.ProbeTarget, 0, 4)
	targets = append(targets, u.objectStorageProbeTargets(ctx, checkedAt, defaultProvider)...)
	targets = append(targets, u.runProbe(ctx, domain.ProbeTarget{
		Key:       "postgres:primary",
		Kind:      "postgres",
		Label:     "PostgreSQL",
		Status:    domain.ProbeStatusUnknown,
		CheckedAt: checkedAt,
	}, func(probeCtx context.Context) error {
		return u.repo.Ping(probeCtx)
	}))
	targets = append(targets, u.runProbe(ctx, domain.ProbeTarget{
		Key:       "redis:primary",
		Kind:      "redis",
		Label:     "Redis",
		Status:    domain.ProbeStatusUnknown,
		CheckedAt: checkedAt,
	}, func(probeCtx context.Context) error {
		if u.redisRepo == nil {
			return errors.New("redis probe repository is not configured")
		}
		return u.redisRepo.Ping(probeCtx)
	}))
	return targets
}

func (u *ResourceMonitorUseCase) objectStorageProbeTargets(
	ctx context.Context,
	checkedAt time.Time,
	defaultProvider string,
) []domain.ProbeTarget {
	if u.registry == nil {
		return nil
	}
	cfg := u.registry.StorageConfig()
	if cfg == nil {
		return nil
	}
	aliases := make([]string, 0, len(cfg.Providers))
	for alias := range cfg.Providers {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	targets := make([]domain.ProbeTarget, 0, len(aliases))
	if len(aliases) == 0 {
		return targets
	}

	results := make([]domain.ProbeTarget, len(aliases))
	sem := make(chan struct{}, resourceMonitorProbeParallelism)
	var wg sync.WaitGroup
	for index, alias := range aliases {
		index, alias := index, alias
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[index] = u.objectStorageProbeTarget(
				ctx,
				checkedAt,
				defaultProvider,
				alias,
				cfg.Providers[alias],
			)
		}()
	}
	wg.Wait()
	return results
}

func (u *ResourceMonitorUseCase) objectStorageProbeTarget(
	ctx context.Context,
	checkedAt time.Time,
	defaultProvider string,
	alias string,
	pcfg config.ProviderConfig,
) domain.ProbeTarget {
	target := domain.ProbeTarget{
		Key:          "object-storage:" + alias,
		Kind:         "object_storage",
		Label:        probeLabel(alias, pcfg.Label),
		Provider:     alias,
		ProviderType: strings.TrimSpace(pcfg.Type),
		Endpoint:     strings.TrimSpace(pcfg.Endpoint),
		Bucket:       strings.TrimSpace(pcfg.Bucket),
		IsDefault:    defaultProvider != "" && strings.EqualFold(alias, defaultProvider),
		Status:       domain.ProbeStatusUnknown,
		CheckedAt:    checkedAt,
	}
	store, err := u.registry.Get(alias)
	if err != nil || store == nil {
		target.Status = domain.ProbeStatusError
		if err != nil {
			target.Error = sanitizeProbeError(err)
		} else {
			target.Error = "storage provider is not available"
		}
		return target
	}
	return u.runProbe(ctx, target, store.Probe)
}

func (u *ResourceMonitorUseCase) runProbe(
	ctx context.Context,
	target domain.ProbeTarget,
	fn func(context.Context) error,
) domain.ProbeTarget {
	start := time.Now()
	probeCtx, cancel := context.WithTimeout(ctx, resourceMonitorProbeTimeout)
	defer cancel()

	if err := fn(probeCtx); err != nil {
		target.Status = domain.ProbeStatusError
		target.Error = sanitizeProbeError(err)
	} else {
		target.Status = domain.ProbeStatusOK
	}
	target.LatencyMs = time.Since(start).Milliseconds()
	return target
}

func probeLabel(alias string, label string) string {
	label = strings.TrimSpace(label)
	if label != "" {
		return label
	}
	alias = strings.TrimSpace(alias)
	if alias != "" {
		return alias
	}
	return "未命名存储"
}

func sanitizeProbeError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.Join(strings.Fields(err.Error()), " ")
	message = probeURIUserInfoPattern.ReplaceAllString(message, `${1}***@`)
	message = probeSecretKVPattern.ReplaceAllString(message, `${1}${2}***`)
	message = probeAWSKeyPattern.ReplaceAllString(message, `***`)
	if len(message) > resourceMonitorProbeErrorMaxSize {
		return message[:resourceMonitorProbeErrorMaxSize] + "..."
	}
	return message
}

func summarizeProbes(targets []domain.ProbeTarget) domain.ProbeSummary {
	summary := domain.ProbeSummary{Total: len(targets)}
	for _, target := range targets {
		switch target.Status {
		case domain.ProbeStatusOK:
			summary.OK++
		case domain.ProbeStatusError:
			summary.Error++
		default:
			summary.Unknown++
		}
	}
	return summary
}
