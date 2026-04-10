package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	"omniflow-go/internal/authz"
	domainlibrary "omniflow-go/internal/domain/library"
	"omniflow-go/internal/repository"
)

type ListLibrariesQuery struct {
	Actor  actor.Actor
	LastID uint64
	Size   int
}

type ScrollLibrariesResult struct {
	Items   []domainlibrary.Library `json:"items"`
	HasMore bool                    `json:"hasMore"`
}

type CreateLibraryCommand struct {
	Actor  actor.Actor
	Name   string
	DryRun bool
}

type UpdateLibraryCommand struct {
	Actor   actor.Actor
	Name    *string
	Starred *int
	DryRun  bool
}

type DeleteLibraryCommand struct {
	Actor  actor.Actor
	ID     uint64
	DryRun bool
}

type LibraryUseCase struct {
	libraries  *repository.LibraryRepository
	tx         repository.Transactor
	authorizer authz.Authorizer
	auditLog   audit.Sink
}

func NewLibraryUseCase(
	libraries *repository.LibraryRepository,
	tx repository.Transactor,
	authorizer authz.Authorizer,
	auditLog audit.Sink,
) *LibraryUseCase {
	return &LibraryUseCase{
		libraries:  libraries,
		tx:         tx,
		authorizer: authorizer,
		auditLog:   auditLog,
	}
}

func (u *LibraryUseCase) Scroll(ctx context.Context, query ListLibrariesQuery) (ScrollLibrariesResult, error) {
	if err := u.ensureLibrariesConfigured(); err != nil {
		return ScrollLibrariesResult{}, err
	}

	userID, err := actorIDToUint64(query.Actor)
	if err != nil {
		return ScrollLibrariesResult{}, err
	}

	size := query.Size
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	records, err := u.libraries.ScrollByUser(ctx, userID, query.LastID, size)
	if err != nil {
		return ScrollLibrariesResult{}, err
	}

	result := make([]domainlibrary.Library, 0, len(records))
	for _, item := range records {
		if err := u.AuthorizeRead(ctx, query.Actor, item.ID); err != nil {
			return ScrollLibrariesResult{}, err
		}
		result = append(result, item)
	}

	hasMore := len(result) == size
	slog.DebugContext(ctx, "library.scroll.completed",
		"user_id", userID,
		"last_id", query.LastID,
		"size", size,
		"result_count", len(result),
		"has_more", hasMore,
	)

	return ScrollLibrariesResult{
		Items:   result,
		HasMore: hasMore,
	}, nil
}

func (u *LibraryUseCase) Create(ctx context.Context, cmd CreateLibraryCommand) (domainlibrary.Library, error) {
	if err := u.ensureLibrariesConfigured(); err != nil {
		return domainlibrary.Library{}, err
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return domainlibrary.Library{}, fmt.Errorf("%w: library name is required", ErrInvalidArgument)
	}

	userID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return domainlibrary.Library{}, err
	}

	var record domainlibrary.Library
	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		created, err := u.libraries.Create(txCtx, userID, name)
		if err != nil {
			return err
		}
		record = created
		return nil
	}); err != nil {
		return domainlibrary.Library{}, err
	}

	_ = u.RecordCreateIntent(ctx, cmd)
	_ = u.writeAudit(ctx, cmd.Actor, "library.create", true, map[string]any{
		"library_id": record.ID,
		"user_id":    userID,
		"name":       record.Name,
		"mode":       resolveMutationMode(cmd.DryRun),
		"dry_run":    cmd.DryRun,
	})
	slog.InfoContext(ctx, "library.created",
		"library_id", record.ID,
		"user_id", userID,
		"dry_run", cmd.DryRun,
	)
	return record, nil
}

func (u *LibraryUseCase) Update(ctx context.Context, id uint64, cmd UpdateLibraryCommand) error {
	if err := u.ensureLibrariesConfigured(); err != nil {
		return err
	}

	if id == 0 {
		return fmt.Errorf("%w: id is required", ErrInvalidArgument)
	}
	if cmd.Name == nil && cmd.Starred == nil {
		return fmt.Errorf("%w: name or starred is required", ErrInvalidArgument)
	}
	if err := u.AuthorizeRead(ctx, cmd.Actor, id); err != nil {
		return err
	}

	userID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return err
	}

	updates := map[string]any{
		"updated_at": time.Now().UTC(),
	}
	if cmd.Name != nil {
		name := strings.TrimSpace(*cmd.Name)
		if name == "" {
			return fmt.Errorf("%w: name cannot be empty", ErrInvalidArgument)
		}
		updates["name"] = name
	}
	if cmd.Starred != nil {
		if *cmd.Starred != 0 && *cmd.Starred != 1 {
			return fmt.Errorf("%w: starred only supports 0 or 1", ErrInvalidArgument)
		}
		updates["starred"] = (*cmd.Starred == 1)
	}

	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		updated, err := u.libraries.UpdateFields(txCtx, id, userID, updates)
		if err != nil {
			return err
		}
		if !updated {
			return ErrNotFound
		}
		return nil
	}); err != nil {
		return err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "library.update", true, map[string]any{
		"library_id": id,
		"name":       updates["name"],
		"starred":    updates["starred"],
		"mode":       resolveMutationMode(cmd.DryRun),
		"dry_run":    cmd.DryRun,
	})
	slog.InfoContext(ctx, "library.updated",
		"library_id", id,
		"user_id", userID,
		"name_updated", cmd.Name != nil,
		"starred_updated", cmd.Starred != nil,
		"dry_run", cmd.DryRun,
	)
	return nil
}

func (u *LibraryUseCase) Delete(ctx context.Context, cmd DeleteLibraryCommand) error {
	if err := u.ensureLibrariesConfigured(); err != nil {
		return err
	}

	if cmd.ID == 0 {
		return fmt.Errorf("%w: id is required", ErrInvalidArgument)
	}
	if err := u.AuthorizeRead(ctx, cmd.Actor, cmd.ID); err != nil {
		return err
	}

	userID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return err
	}

	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		deleted, err := u.libraries.SoftDelete(txCtx, cmd.ID, userID, time.Now().UTC())
		if err != nil {
			return err
		}
		if !deleted {
			return ErrNotFound
		}
		return nil
	}); err != nil {
		return err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "library.delete", true, map[string]any{
		"library_id": cmd.ID,
		"mode":       resolveMutationMode(cmd.DryRun),
		"dry_run":    cmd.DryRun,
	})
	slog.InfoContext(ctx, "library.deleted",
		"library_id", cmd.ID,
		"user_id", userID,
		"dry_run", cmd.DryRun,
	)
	return nil
}

func (u *LibraryUseCase) HasPermission(ctx context.Context, principal actor.Actor, libraryID uint64) (bool, error) {
	if err := u.ensureLibrariesConfigured(); err != nil {
		return false, err
	}

	if libraryID == 0 {
		return false, fmt.Errorf("%w: library id is required", ErrInvalidArgument)
	}

	lib, err := u.libraries.FindByID(ctx, libraryID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, ErrNotFound
		}
		return false, err
	}

	userID, err := actorIDToUint64(principal)
	if err != nil {
		return false, err
	}
	if userID != lib.UserID {
		return false, authz.ErrPermissionDenied
	}
	return true, nil
}

func (u *LibraryUseCase) AuthorizeRead(ctx context.Context, principal actor.Actor, libraryID uint64) error {
	if u.authorizer == nil {
		return nil
	}

	return u.authorizer.Authorize(ctx, principal, authz.Resource{
		Kind: "library",
		ID:   fmt.Sprintf("%d", libraryID),
	}, authz.ActionRead)
}

func (u *LibraryUseCase) RecordCreateIntent(ctx context.Context, cmd CreateLibraryCommand) error {
	if u.auditLog == nil {
		return nil
	}

	return u.auditLog.Write(ctx, audit.Event{
		Actor:      cmd.Actor,
		Action:     "library.create.intent",
		Resource:   "library",
		Success:    true,
		OccurredAt: time.Now().UTC(),
		Metadata: map[string]any{
			"name":    cmd.Name,
			"mode":    resolveMutationMode(cmd.DryRun),
			"dry_run": cmd.DryRun,
		},
	})
}

func (u *LibraryUseCase) withinMutationTx(ctx context.Context, dryRun bool, fn func(ctx context.Context) error) error {
	if !dryRun {
		if u.tx == nil {
			return fn(ctx)
		}
		return u.tx.WithinTx(ctx, fn)
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

func (u *LibraryUseCase) writeAudit(ctx context.Context, principal actor.Actor, action string, success bool, metadata map[string]any) error {
	if u.auditLog == nil {
		return nil
	}
	return u.auditLog.Write(ctx, audit.Event{
		Actor:      principal,
		Action:     action,
		Resource:   "library",
		Success:    success,
		OccurredAt: time.Now().UTC(),
		Metadata:   metadata,
	})
}

func (u *LibraryUseCase) ensureLibrariesConfigured() error {
	if u.libraries != nil {
		return nil
	}
	return fmt.Errorf("%w: library repository not configured", ErrInvalidArgument)
}
