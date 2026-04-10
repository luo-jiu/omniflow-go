package usecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"omniflow-go/internal/actor"
	domaintag "omniflow-go/internal/domain/tag"
	"omniflow-go/internal/repository"

	"github.com/samber/lo"
)

const (
	defaultTagColor = "#4F8CFF"
	fileTabType     = "FILE_TAB"
)

var (
	errTagRepositoryNotConfigured = errors.New("tag repository is not configured")
	tagTypes                      = lo.SliceToMap([]string{"ASMR", "FILE_TAB", "COMIC", "GENERAL"}, func(item string) (string, struct{}) {
		return item, struct{}{}
	})
	hexColorPattern  = regexp.MustCompile(`^#([0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	targetKeyPattern = regexp.MustCompile(`^[A-Z0-9_-]{1,64}$`)
)

type ListTagsQuery struct {
	Actor actor.Actor
	Type  string
}

type CreateTagCommand struct {
	Actor       actor.Actor
	Name        string
	Type        string
	TargetKey   string
	Color       string
	TextColor   string
	SortOrder   *int
	Enabled     *int
	Description string
	DryRun      bool
}

type UpdateTagCommand struct {
	Actor       actor.Actor
	Name        string
	Type        string
	TargetKey   string
	Color       string
	TextColor   string
	SortOrder   *int
	Enabled     *int
	Description string
	DryRun      bool
}

type DeleteTagCommand struct {
	Actor  actor.Actor
	TagID  uint64
	DryRun bool
}

type TagUseCase struct {
	tags       *repository.TagRepository
	tx         repository.Transactor
	searchType string
}

func NewTagUseCase(tags *repository.TagRepository, tx repository.Transactor) *TagUseCase {
	return &TagUseCase{
		tags:       tags,
		tx:         tx,
		searchType: "MySQL",
	}
}

func (u *TagUseCase) SearchType() string {
	if u == nil || u.searchType == "" {
		return "MySQL"
	}
	return u.searchType
}

func (u *TagUseCase) List(ctx context.Context, query ListTagsQuery) ([]domaintag.Tag, error) {
	if err := u.ensureTagsConfigured(); err != nil {
		return nil, err
	}

	ownerUserID, err := actorIDToUint64(query.Actor)
	if err != nil {
		return nil, err
	}

	normalizedType, err := normalizeTagType(query.Type, false)
	if err != nil {
		return nil, err
	}

	return u.tags.ListByOwnerAndType(ctx, ownerUserID, normalizedType)
}

func (u *TagUseCase) Create(ctx context.Context, cmd CreateTagCommand) (domaintag.Tag, error) {
	if err := u.ensureTagsConfigured(); err != nil {
		return domaintag.Tag{}, err
	}

	ownerUserID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return domaintag.Tag{}, err
	}

	name, err := normalizeTagName(cmd.Name)
	if err != nil {
		return domaintag.Tag{}, err
	}
	tagType, err := normalizeTagType(cmd.Type, true)
	if err != nil {
		return domaintag.Tag{}, err
	}
	targetKey, err := normalizeTargetKey(cmd.TargetKey, tagType)
	if err != nil {
		return domaintag.Tag{}, err
	}
	color, err := normalizeTagColor(cmd.Color, true)
	if err != nil {
		return domaintag.Tag{}, err
	}
	textColor, err := normalizeOptionalTagColor(cmd.TextColor)
	if err != nil {
		return domaintag.Tag{}, err
	}
	sortOrder := normalizeSortOrder(cmd.SortOrder)
	enabled, err := normalizeEnabled(cmd.Enabled)
	if err != nil {
		return domaintag.Tag{}, err
	}
	description, err := normalizeDescription(cmd.Description)
	if err != nil {
		return domaintag.Tag{}, err
	}

	var created domaintag.Tag
	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		if err := u.lockTagUniqScopes(txCtx, ownerUserID, *tagType, name, targetKey); err != nil {
			return err
		}

		exists, err := u.tags.ExistsName(txCtx, ownerUserID, *tagType, name, 0)
		if err != nil {
			return err
		}
		if exists {
			return ErrConflict
		}

		targetExists, err := u.tags.ExistsTargetKey(txCtx, ownerUserID, *tagType, targetKey, 0)
		if err != nil {
			return err
		}
		if targetExists {
			return ErrConflict
		}

		tag, err := u.tags.Create(txCtx, repository.CreateTagInput{
			Name:        name,
			Type:        *tagType,
			TargetKey:   targetKey,
			OwnerUserID: ownerUserID,
			Color:       color,
			TextColor:   textColor,
			SortOrder:   sortOrder,
			Enabled:     enabled,
			Description: description,
		})
		if err != nil {
			if errors.Is(err, repository.ErrConflict) {
				return ErrConflict
			}
			return err
		}
		created = tag
		return nil
	}); err != nil {
		return domaintag.Tag{}, err
	}

	return created, nil
}

func (u *TagUseCase) Update(ctx context.Context, tagID uint64, cmd UpdateTagCommand) (domaintag.Tag, error) {
	if err := u.ensureTagsConfigured(); err != nil {
		return domaintag.Tag{}, err
	}
	if tagID == 0 {
		return domaintag.Tag{}, fmt.Errorf("%w: tagId is required", ErrInvalidArgument)
	}

	ownerUserID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return domaintag.Tag{}, err
	}
	name, err := normalizeTagName(cmd.Name)
	if err != nil {
		return domaintag.Tag{}, err
	}
	tagType, err := normalizeTagType(cmd.Type, true)
	if err != nil {
		return domaintag.Tag{}, err
	}
	targetKey, err := normalizeTargetKey(cmd.TargetKey, tagType)
	if err != nil {
		return domaintag.Tag{}, err
	}
	color, err := normalizeTagColor(cmd.Color, true)
	if err != nil {
		return domaintag.Tag{}, err
	}
	textColor, err := normalizeOptionalTagColor(cmd.TextColor)
	if err != nil {
		return domaintag.Tag{}, err
	}
	sortOrder := normalizeSortOrder(cmd.SortOrder)
	enabled, err := normalizeEnabled(cmd.Enabled)
	if err != nil {
		return domaintag.Tag{}, err
	}
	description, err := normalizeDescription(cmd.Description)
	if err != nil {
		return domaintag.Tag{}, err
	}

	var updated domaintag.Tag
	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		if _, err := u.tags.FindOwnerByID(txCtx, tagID, ownerUserID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}

		if err := u.lockTagUniqScopes(txCtx, ownerUserID, *tagType, name, targetKey); err != nil {
			return err
		}

		exists, err := u.tags.ExistsName(txCtx, ownerUserID, *tagType, name, tagID)
		if err != nil {
			return err
		}
		if exists {
			return ErrConflict
		}

		targetExists, err := u.tags.ExistsTargetKey(txCtx, ownerUserID, *tagType, targetKey, tagID)
		if err != nil {
			return err
		}
		if targetExists {
			return ErrConflict
		}

		row, err := u.tags.UpdateOwnerByID(txCtx, tagID, ownerUserID, repository.UpdateTagInput{
			Name:        name,
			Type:        *tagType,
			TargetKey:   targetKey,
			Color:       color,
			TextColor:   textColor,
			SortOrder:   sortOrder,
			Enabled:     enabled,
			Description: description,
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
		return domaintag.Tag{}, err
	}

	return updated, nil
}

func (u *TagUseCase) Delete(ctx context.Context, cmd DeleteTagCommand) error {
	if err := u.ensureTagsConfigured(); err != nil {
		return err
	}
	if cmd.TagID == 0 {
		return fmt.Errorf("%w: tagId is required", ErrInvalidArgument)
	}

	ownerUserID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return err
	}

	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		ok, err := u.tags.SoftDeleteOwnerByID(txCtx, cmd.TagID, ownerUserID)
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
	return nil
}

func normalizeTagName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("%w: tag name is required", ErrInvalidArgument)
	}
	if len(name) > 64 {
		return "", fmt.Errorf("%w: tag name length must be <= 64", ErrInvalidArgument)
	}
	return name, nil
}

func (u *TagUseCase) ensureTagsConfigured() error {
	if u == nil || u.tags == nil {
		return errTagRepositoryNotConfigured
	}
	return nil
}

func normalizeTagType(raw string, required bool) (*string, error) {
	tagType := strings.ToUpper(strings.TrimSpace(raw))
	if tagType == "" {
		if !required {
			return nil, nil
		}
		tagType = "GENERAL"
	}

	if _, ok := tagTypes[tagType]; !ok {
		return nil, fmt.Errorf("%w: unsupported tag type %s", ErrInvalidArgument, tagType)
	}
	return &tagType, nil
}

func normalizeTargetKey(raw string, normalizedType *string) (*string, error) {
	if normalizedType == nil || *normalizedType != fileTabType {
		return nil, nil
	}

	targetKey := strings.ToUpper(strings.TrimSpace(raw))
	if targetKey == "" {
		return nil, fmt.Errorf("%w: FILE_TAB targetKey is required", ErrInvalidArgument)
	}
	if !targetKeyPattern.MatchString(targetKey) {
		return nil, fmt.Errorf("%w: targetKey format is invalid", ErrInvalidArgument)
	}
	return &targetKey, nil
}

func normalizeTagColor(raw string, fallbackDefault bool) (string, error) {
	color := strings.TrimSpace(raw)
	if color == "" {
		if fallbackDefault {
			return defaultTagColor, nil
		}
		return "", nil
	}
	if !hexColorPattern.MatchString(color) {
		return "", fmt.Errorf("%w: color must be HEX (#RRGGBB or #RRGGBBAA)", ErrInvalidArgument)
	}
	return strings.ToUpper(color), nil
}

func normalizeOptionalTagColor(raw string) (*string, error) {
	color, err := normalizeTagColor(raw, false)
	if err != nil {
		return nil, err
	}
	if color == "" {
		return nil, nil
	}
	return &color, nil
}

func normalizeSortOrder(raw *int) int {
	if raw == nil {
		return 0
	}
	return *raw
}

func normalizeEnabled(raw *int) (int, error) {
	if raw == nil {
		return 1, nil
	}
	if *raw != 0 && *raw != 1 {
		return 0, fmt.Errorf("%w: enabled only supports 0 or 1", ErrInvalidArgument)
	}
	return *raw, nil
}

func normalizeDescription(raw string) (*string, error) {
	description := strings.TrimSpace(raw)
	if description == "" {
		return nil, nil
	}
	if len(description) > 255 {
		return nil, fmt.Errorf("%w: description length must be <= 255", ErrInvalidArgument)
	}
	return &description, nil
}

func (u *TagUseCase) withinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if u.tx == nil {
		return fn(ctx)
	}
	return u.tx.WithinTx(ctx, fn)
}

func (u *TagUseCase) withinMutationTx(ctx context.Context, dryRun bool, fn func(ctx context.Context) error) error {
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

func (u *TagUseCase) lockTagUniqScopes(
	ctx context.Context,
	ownerUserID uint64,
	tagType, name string,
	targetKey *string,
) error {
	scopes := []string{
		fmt.Sprintf("tags:name:%d:%s:%s", ownerUserID, tagType, name),
	}
	if targetKey != nil && *targetKey != "" {
		scopes = append(scopes, fmt.Sprintf("tags:target:%d:%s:%s", ownerUserID, tagType, *targetKey))
	}

	uniqueScopes := lo.Uniq(lo.Filter(scopes, func(scope string, _ int) bool {
		return strings.TrimSpace(scope) != ""
	}))
	return u.tags.LockScopes(ctx, uniqueScopes...)
}
