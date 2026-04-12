package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	domainbrowserfilemapping "omniflow-go/internal/domain/browserfilemapping"
	"omniflow-go/internal/repository"
)

var (
	errBrowserFileMappingRepositoryNotConfigured = errors.New("browser file mapping repository is not configured")
	browserFileExtPattern                        = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
)

type ListBrowserFileMappingsQuery struct {
	Actor actor.Actor
}

type ResolveBrowserFileMappingQuery struct {
	Actor   actor.Actor
	FileExt string
}

type CreateBrowserFileMappingCommand struct {
	Actor   actor.Actor
	FileExt string
	SiteURL string
	DryRun  bool
}

type UpdateBrowserFileMappingCommand struct {
	Actor   actor.Actor
	FileExt string
	SiteURL string
	DryRun  bool
}

type DeleteBrowserFileMappingCommand struct {
	Actor     actor.Actor
	MappingID uint64
	DryRun    bool
}

type BrowserFileMappingUseCase struct {
	mappings *repository.BrowserFileMappingRepository
	tx       repository.Transactor
	auditLog audit.Sink
}

func NewBrowserFileMappingUseCase(
	mappings *repository.BrowserFileMappingRepository,
	tx repository.Transactor,
	auditLog audit.Sink,
) *BrowserFileMappingUseCase {
	return &BrowserFileMappingUseCase{
		mappings: mappings,
		tx:       tx,
		auditLog: auditLog,
	}
}

func (u *BrowserFileMappingUseCase) List(
	ctx context.Context,
	query ListBrowserFileMappingsQuery,
) ([]domainbrowserfilemapping.BrowserFileMapping, error) {
	if err := u.ensureConfigured(); err != nil {
		return nil, err
	}
	ownerUserID, err := actorIDToUint64(query.Actor)
	if err != nil {
		return nil, err
	}

	rows, err := u.mappings.ListByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	slog.DebugContext(ctx, "browser_file_mapping.list.completed",
		"owner_user_id", ownerUserID,
		"result_count", len(rows),
	)
	return rows, nil
}

func (u *BrowserFileMappingUseCase) Resolve(
	ctx context.Context,
	query ResolveBrowserFileMappingQuery,
) (domainbrowserfilemapping.BrowserFileMapping, error) {
	if err := u.ensureConfigured(); err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}
	ownerUserID, err := actorIDToUint64(query.Actor)
	if err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}
	fileExt, err := normalizeBrowserFileExt(query.FileExt)
	if err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}

	row, err := u.mappings.FindByOwnerAndExt(ctx, ownerUserID, fileExt)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domainbrowserfilemapping.BrowserFileMapping{}, ErrNotFound
		}
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}
	return row, nil
}

func (u *BrowserFileMappingUseCase) Create(
	ctx context.Context,
	cmd CreateBrowserFileMappingCommand,
) (domainbrowserfilemapping.BrowserFileMapping, error) {
	if err := u.ensureConfigured(); err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}
	ownerUserID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}
	fileExt, err := normalizeBrowserFileExt(cmd.FileExt)
	if err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}
	siteURL, err := normalizeBrowserSiteURL(cmd.SiteURL)
	if err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}

	var created domainbrowserfilemapping.BrowserFileMapping
	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		exists, err := u.mappings.ExistsFileExt(txCtx, ownerUserID, fileExt, 0)
		if err != nil {
			return err
		}
		if exists {
			return ErrConflict
		}

		row, err := u.mappings.Create(txCtx, repository.CreateBrowserFileMappingInput{
			FileExt:     fileExt,
			SiteURL:     siteURL,
			OwnerUserID: ownerUserID,
		})
		if err != nil {
			if errors.Is(err, repository.ErrConflict) {
				return ErrConflict
			}
			return err
		}
		created = row
		return nil
	}); err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "browser_file_mapping.create", true, map[string]any{
		"mapping_id":    created.ID,
		"owner_user_id": ownerUserID,
		"file_ext":      created.FileExt,
		"site_url":      created.SiteURL,
		"mode":          resolveMutationMode(cmd.DryRun),
		"dry_run":       cmd.DryRun,
	})
	slog.InfoContext(ctx, "browser_file_mapping.created",
		"mapping_id", created.ID,
		"owner_user_id", ownerUserID,
		"file_ext", created.FileExt,
		"dry_run", cmd.DryRun,
	)
	return created, nil
}

func (u *BrowserFileMappingUseCase) Update(
	ctx context.Context,
	mappingID uint64,
	cmd UpdateBrowserFileMappingCommand,
) (domainbrowserfilemapping.BrowserFileMapping, error) {
	if err := u.ensureConfigured(); err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}
	if mappingID == 0 {
		return domainbrowserfilemapping.BrowserFileMapping{}, fmt.Errorf("%w: mappingId is required", ErrInvalidArgument)
	}

	ownerUserID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}
	fileExt, err := normalizeBrowserFileExt(cmd.FileExt)
	if err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}
	siteURL, err := normalizeBrowserSiteURL(cmd.SiteURL)
	if err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}

	var updated domainbrowserfilemapping.BrowserFileMapping
	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		if _, err := u.mappings.FindOwnerByID(txCtx, mappingID, ownerUserID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}

		exists, err := u.mappings.ExistsFileExt(txCtx, ownerUserID, fileExt, mappingID)
		if err != nil {
			return err
		}
		if exists {
			return ErrConflict
		}

		row, err := u.mappings.UpdateOwnerByID(txCtx, mappingID, ownerUserID, repository.UpdateBrowserFileMappingInput{
			FileExt: fileExt,
			SiteURL: siteURL,
		})
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			if errors.Is(err, repository.ErrConflict) {
				return ErrConflict
			}
			return err
		}
		updated = row
		return nil
	}); err != nil {
		return domainbrowserfilemapping.BrowserFileMapping{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "browser_file_mapping.update", true, map[string]any{
		"mapping_id":    updated.ID,
		"owner_user_id": ownerUserID,
		"file_ext":      updated.FileExt,
		"site_url":      updated.SiteURL,
		"mode":          resolveMutationMode(cmd.DryRun),
		"dry_run":       cmd.DryRun,
	})
	slog.InfoContext(ctx, "browser_file_mapping.updated",
		"mapping_id", updated.ID,
		"owner_user_id", ownerUserID,
		"file_ext", updated.FileExt,
		"dry_run", cmd.DryRun,
	)
	return updated, nil
}

func (u *BrowserFileMappingUseCase) Delete(ctx context.Context, cmd DeleteBrowserFileMappingCommand) error {
	if err := u.ensureConfigured(); err != nil {
		return err
	}
	if cmd.MappingID == 0 {
		return fmt.Errorf("%w: mappingId is required", ErrInvalidArgument)
	}

	ownerUserID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return err
	}

	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		ok, err := u.mappings.SoftDeleteOwnerByID(txCtx, cmd.MappingID, ownerUserID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		return nil
	}); err != nil {
		return err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "browser_file_mapping.delete", true, map[string]any{
		"mapping_id":    cmd.MappingID,
		"owner_user_id": ownerUserID,
		"mode":          resolveMutationMode(cmd.DryRun),
		"dry_run":       cmd.DryRun,
	})
	slog.InfoContext(ctx, "browser_file_mapping.deleted",
		"mapping_id", cmd.MappingID,
		"owner_user_id", ownerUserID,
		"dry_run", cmd.DryRun,
	)
	return nil
}

func (u *BrowserFileMappingUseCase) ensureConfigured() error {
	if u == nil || u.mappings == nil {
		return errBrowserFileMappingRepositoryNotConfigured
	}
	return nil
}

func normalizeBrowserFileExt(raw string) (string, error) {
	fileExt := strings.ToLower(strings.TrimSpace(raw))
	fileExt = strings.TrimPrefix(fileExt, ".")
	if fileExt == "" {
		return "", fmt.Errorf("%w: fileExt is required", ErrInvalidArgument)
	}
	if !browserFileExtPattern.MatchString(fileExt) {
		return "", fmt.Errorf("%w: fileExt format is invalid", ErrInvalidArgument)
	}
	return fileExt, nil
}

func normalizeBrowserSiteURL(raw string) (string, error) {
	siteURL := strings.TrimSpace(raw)
	if siteURL == "" {
		return "", fmt.Errorf("%w: siteUrl is required", ErrInvalidArgument)
	}
	if len(siteURL) > 2048 {
		return "", fmt.Errorf("%w: siteUrl length must be <= 2048", ErrInvalidArgument)
	}

	parsed, err := url.Parse(siteURL)
	if err != nil || parsed == nil {
		return "", fmt.Errorf("%w: siteUrl is invalid", ErrInvalidArgument)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%w: siteUrl must use http or https", ErrInvalidArgument)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("%w: siteUrl host is required", ErrInvalidArgument)
	}
	return parsed.String(), nil
}

func (u *BrowserFileMappingUseCase) withinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if u.tx == nil {
		return fn(ctx)
	}
	return u.tx.WithinTx(ctx, fn)
}

func (u *BrowserFileMappingUseCase) withinMutationTx(ctx context.Context, dryRun bool, fn func(ctx context.Context) error) error {
	if !dryRun {
		return u.withinTx(ctx, fn)
	}
	if u.tx == nil {
		return fmt.Errorf("%w: dry-run requires transaction manager", ErrInvalidArgument)
	}

	err := u.tx.WithinTx(ctx, func(txCtx context.Context) error {
		if err := fn(txCtx); err != nil {
			return err
		}
		return errUsecaseDryRunRollback
	})
	if err != nil && !errors.Is(err, errUsecaseDryRunRollback) {
		return err
	}
	return nil
}

func (u *BrowserFileMappingUseCase) writeAudit(ctx context.Context, principal actor.Actor, action string, success bool, metadata map[string]any) error {
	if u.auditLog == nil {
		return nil
	}
	return u.auditLog.Write(ctx, audit.Event{
		Actor:      principal,
		Action:     action,
		Resource:   "browser_file_mapping",
		Success:    success,
		OccurredAt: time.Now().UTC(),
		Metadata:   metadata,
	})
}
