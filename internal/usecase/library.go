package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	"omniflow-go/internal/authz"
	domainlibrary "omniflow-go/internal/domain/library"
	"omniflow-go/internal/repository"

	"gorm.io/gorm"
)

type ListLibrariesQuery struct {
	Actor  actor.Actor
	LastID uint64
	Size   int
}

type ScrollLibrariesResult struct {
	Items   []domainlibrary.Library
	HasMore bool
}

type CreateLibraryCommand struct {
	Actor actor.Actor
	Name  string
}

type UpdateLibraryCommand struct {
	Actor actor.Actor
	Name  string
}

type DeleteLibraryCommand struct {
	Actor actor.Actor
	ID    uint64
}

type LibraryUseCase struct {
	libraries  *repository.LibraryRepository
	authorizer authz.Authorizer
	auditLog   audit.Sink
}

func NewLibraryUseCase(
	libraries *repository.LibraryRepository,
	authorizer authz.Authorizer,
	auditLog audit.Sink,
) *LibraryUseCase {
	return &LibraryUseCase{
		libraries:  libraries,
		authorizer: authorizer,
		auditLog:   auditLog,
	}
}

func (u *LibraryUseCase) Scroll(ctx context.Context, query ListLibrariesQuery) (ScrollLibrariesResult, error) {
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

	db, err := dbFromRepository(u.libraries)
	if err != nil {
		return ScrollLibrariesResult{}, err
	}

	tx := db.WithContext(ctx).Model(&libraryRecord{}).Where("user_id = ?", userID).Order("id ASC").Limit(size)
	if query.LastID > 0 {
		tx = tx.Where("id > ?", query.LastID)
	}

	var records []libraryRecord
	if err := tx.Find(&records).Error; err != nil {
		return ScrollLibrariesResult{}, err
	}

	result := make([]domainlibrary.Library, 0, len(records))
	for _, item := range records {
		if err := u.AuthorizeRead(ctx, query.Actor, item.ID); err != nil {
			return ScrollLibrariesResult{}, err
		}
		result = append(result, item.toDomain())
	}

	return ScrollLibrariesResult{
		Items:   result,
		HasMore: len(result) == size,
	}, nil
}

func (u *LibraryUseCase) Create(ctx context.Context, cmd CreateLibraryCommand) (domainlibrary.Library, error) {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return domainlibrary.Library{}, fmt.Errorf("%w: library name is required", ErrInvalidArgument)
	}

	userID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return domainlibrary.Library{}, err
	}

	record := libraryRecord{
		UserID: userID,
		Name:   name,
	}

	db, err := dbFromRepository(u.libraries)
	if err != nil {
		return domainlibrary.Library{}, err
	}
	if err := db.WithContext(ctx).Create(&record).Error; err != nil {
		return domainlibrary.Library{}, err
	}

	_ = u.RecordCreateIntent(ctx, cmd)
	_ = u.writeAudit(ctx, cmd.Actor, "library.create", true, map[string]any{
		"library_id": record.ID,
		"user_id":    userID,
		"name":       record.Name,
	})
	return record.toDomain(), nil
}

func (u *LibraryUseCase) Update(ctx context.Context, id uint64, cmd UpdateLibraryCommand) error {
	name := strings.TrimSpace(cmd.Name)
	if id == 0 || name == "" {
		return fmt.Errorf("%w: id and name are required", ErrInvalidArgument)
	}
	if err := u.AuthorizeRead(ctx, cmd.Actor, id); err != nil {
		return err
	}

	userID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return err
	}

	db, err := dbFromRepository(u.libraries)
	if err != nil {
		return err
	}

	result := db.WithContext(ctx).
		Model(&libraryRecord{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"name":       name,
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	_ = u.writeAudit(ctx, cmd.Actor, "library.update", true, map[string]any{
		"library_id": id,
		"name":       name,
	})
	return nil
}

func (u *LibraryUseCase) Delete(ctx context.Context, cmd DeleteLibraryCommand) error {
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

	db, err := dbFromRepository(u.libraries)
	if err != nil {
		return err
	}

	result := db.WithContext(ctx).
		Model(&libraryRecord{}).
		Where("id = ? AND user_id = ?", cmd.ID, userID).
		Update("deleted_at", time.Now().UTC())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	_ = u.writeAudit(ctx, cmd.Actor, "library.delete", true, map[string]any{
		"library_id": cmd.ID,
	})
	return nil
}

func (u *LibraryUseCase) HasPermission(ctx context.Context, principal actor.Actor, libraryID uint64) (bool, error) {
	if libraryID == 0 {
		return false, fmt.Errorf("%w: library id is required", ErrInvalidArgument)
	}

	db, err := dbFromRepository(u.libraries)
	if err != nil {
		return false, err
	}

	var lib libraryRecord
	if err := db.WithContext(ctx).First(&lib, "id = ?", libraryID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
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
			"name": cmd.Name,
		},
	})
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
