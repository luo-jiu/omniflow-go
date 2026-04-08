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
)

const (
	defaultTagColor = "#4F8CFF"
	fileTabType     = "FILE_TAB"
)

var (
	errTagRepositoryNotConfigured = errors.New("tag repository is not configured")
	tagTypes                      = map[string]struct{}{
		"ASMR":     {},
		"FILE_TAB": {},
		"COMIC":    {},
		"GENERAL":  {},
	}
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
}

type TagUseCase struct {
	tags       *repository.TagRepository
	searchType string
}

func NewTagUseCase(tags *repository.TagRepository) *TagUseCase {
	return &TagUseCase{
		tags:       tags,
		searchType: "PostgreSQL",
	}
}

func (u *TagUseCase) SearchType() string {
	if u == nil || u.searchType == "" {
		return "PostgreSQL"
	}
	return u.searchType
}

func (u *TagUseCase) List(ctx context.Context, query ListTagsQuery) ([]domaintag.Tag, error) {
	if u.tags == nil {
		return []domaintag.Tag{}, nil
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
	if u.tags == nil {
		return domaintag.Tag{}, errTagRepositoryNotConfigured
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

	exists, err := u.tags.ExistsName(ctx, ownerUserID, *tagType, name, 0)
	if err != nil {
		return domaintag.Tag{}, err
	}
	if exists {
		return domaintag.Tag{}, ErrConflict
	}

	targetExists, err := u.tags.ExistsTargetKey(ctx, ownerUserID, *tagType, targetKey, 0)
	if err != nil {
		return domaintag.Tag{}, err
	}
	if targetExists {
		return domaintag.Tag{}, ErrConflict
	}

	tag, err := u.tags.Create(ctx, repository.CreateTagInput{
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
			return domaintag.Tag{}, ErrConflict
		}
		return domaintag.Tag{}, err
	}
	return tag, nil
}

func (u *TagUseCase) Update(ctx context.Context, tagID uint64, cmd UpdateTagCommand) (domaintag.Tag, error) {
	if u.tags == nil {
		return domaintag.Tag{}, errTagRepositoryNotConfigured
	}
	if tagID == 0 {
		return domaintag.Tag{}, fmt.Errorf("%w: tagId is required", ErrInvalidArgument)
	}

	ownerUserID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return domaintag.Tag{}, err
	}
	if _, err := u.tags.FindOwnerByID(ctx, tagID, ownerUserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domaintag.Tag{}, ErrNotFound
		}
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

	exists, err := u.tags.ExistsName(ctx, ownerUserID, *tagType, name, tagID)
	if err != nil {
		return domaintag.Tag{}, err
	}
	if exists {
		return domaintag.Tag{}, ErrConflict
	}

	targetExists, err := u.tags.ExistsTargetKey(ctx, ownerUserID, *tagType, targetKey, tagID)
	if err != nil {
		return domaintag.Tag{}, err
	}
	if targetExists {
		return domaintag.Tag{}, ErrConflict
	}

	updated, err := u.tags.UpdateOwnerByID(ctx, tagID, ownerUserID, repository.UpdateTagInput{
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
			return domaintag.Tag{}, ErrNotFound
		}
		if errors.Is(err, repository.ErrConflict) {
			return domaintag.Tag{}, ErrConflict
		}
		return domaintag.Tag{}, err
	}
	return updated, nil
}

func (u *TagUseCase) Delete(ctx context.Context, principal actor.Actor, tagID uint64) error {
	if u.tags == nil {
		return errTagRepositoryNotConfigured
	}
	if tagID == 0 {
		return fmt.Errorf("%w: tagId is required", ErrInvalidArgument)
	}

	ownerUserID, err := actorIDToUint64(principal)
	if err != nil {
		return err
	}

	ok, err := u.tags.SoftDeleteOwnerByID(ctx, tagID, ownerUserID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
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
