package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"omniflow-go/internal/actor"
	domaintag "omniflow-go/internal/domain/tag"
	"omniflow-go/internal/repository"

	"github.com/samber/lo"
)

const (
	defaultTagColor     = "#4F8CFF"
	defaultTagScope     = "resource"
	defaultTagDimension = "custom"
	fileTabType         = "FILE_TAB"
	fileTabScope        = "ui"
)

var (
	errTagRepositoryNotConfigured = errors.New("tag repository is not configured")
	tagScopes                     = lo.SliceToMap([]string{"resource", "ui"}, func(item string) (string, struct{}) {
		return item, struct{}{}
	})
	tagDimensions = lo.SliceToMap([]string{
		"genre",
		"creator",
		"character",
		"series",
		"source",
		"language",
		"region",
		"technical",
		"status",
		"custom",
	}, func(item string) (string, struct{}) {
		return item, struct{}{}
	})
	hexColorPattern     = regexp.MustCompile(`^#([0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	tagTypePattern      = regexp.MustCompile(`^[A-Z0-9_-]{1,64}$`)
	resourceKindPattern = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
	targetKindPattern   = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
	targetKeyPattern    = regexp.MustCompile(`^[A-Z0-9_-]{1,64}$`)
)

type ListTagsQuery struct {
	Actor        actor.Actor
	Type         string
	Scope        string
	Dimension    string
	ResourceKind string
}

type CreateTagCommand struct {
	Actor        actor.Actor
	Name         string
	Type         string
	Scope        string
	Dimension    string
	ResourceKind string
	TargetKinds  []string
	TargetKey    string
	Color        string
	TextColor    string
	SortOrder    *int
	Enabled      *int
	Description  string
	DryRun       bool
}

type UpdateTagCommand struct {
	Actor        actor.Actor
	Name         string
	Type         string
	Scope        string
	Dimension    string
	ResourceKind string
	TargetKinds  *[]string
	TargetKey    string
	Color        string
	TextColor    string
	SortOrder    *int
	Enabled      *int
	Description  string
	DryRun       bool
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
		slog.Debug("tag.search_type.read", "value", "MySQL", "fallback", true)
		return "MySQL"
	}
	slog.Debug("tag.search_type.read", "value", u.searchType, "fallback", false)
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
	normalizedScope, err := normalizeOptionalTagScope(query.Scope)
	if err != nil {
		return nil, err
	}
	normalizedDimension, err := normalizeOptionalTagDimension(query.Dimension)
	if err != nil {
		return nil, err
	}
	normalizedResourceKind, err := normalizeOptionalResourceKind(query.ResourceKind)
	if err != nil {
		return nil, err
	}

	rows, err := u.tags.ListByOwnerAndFilter(ctx, ownerUserID, repository.ListTagsFilter{
		Type:         normalizedType,
		Scope:        normalizedScope,
		Dimension:    normalizedDimension,
		ResourceKind: normalizedResourceKind,
	})
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "tag.list.completed",
		"owner_user_id", ownerUserID,
		"type_filter", query.Type,
		"scope_filter", query.Scope,
		"dimension_filter", query.Dimension,
		"resource_kind_filter", query.ResourceKind,
		"result_count", len(rows),
	)
	return rows, nil
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
	scope, dimension, resourceKind, err := normalizeTagClassification(cmd.Scope, cmd.Dimension, cmd.ResourceKind, *tagType)
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
	targetKinds, err := normalizeTagTargetKinds(cmd.TargetKinds, *tagType)
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
		if err := u.ensureTargetKinds(txCtx, targetKinds); err != nil {
			return err
		}
		if err := u.lockTagUniqScopes(txCtx, ownerUserID, *tagType, scope, dimension, resourceKind, name, targetKey); err != nil {
			return err
		}

		exists, err := u.tags.ExistsName(txCtx, ownerUserID, repository.TagNameScope{
			Type:         *tagType,
			Scope:        scope,
			Dimension:    dimension,
			ResourceKind: resourceKind,
			Name:         name,
		}, 0)
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
			Name:         name,
			Type:         *tagType,
			Scope:        scope,
			Dimension:    dimension,
			ResourceKind: resourceKind,
			TargetKey:    targetKey,
			OwnerUserID:  ownerUserID,
			Color:        color,
			TextColor:    textColor,
			SortOrder:    sortOrder,
			Enabled:      enabled,
			Description:  description,
			TargetKinds:  targetKinds,
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

	slog.InfoContext(ctx, "tag.created",
		"tag_id", created.ID,
		"owner_user_id", ownerUserID,
		"type", created.Type,
		"dry_run", cmd.DryRun,
	)
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
	scope, dimension, resourceKind, err := normalizeTagClassification(cmd.Scope, cmd.Dimension, cmd.ResourceKind, *tagType)
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
	var targetKinds *[]string
	if cmd.TargetKinds != nil {
		normalizedTargetKinds, err := normalizeTagTargetKinds(*cmd.TargetKinds, *tagType)
		if err != nil {
			return domaintag.Tag{}, err
		}
		targetKinds = &normalizedTargetKinds
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

		if targetKinds != nil {
			if err := u.ensureTargetKinds(txCtx, *targetKinds); err != nil {
				return err
			}
		}
		if err := u.lockTagUniqScopes(txCtx, ownerUserID, *tagType, scope, dimension, resourceKind, name, targetKey); err != nil {
			return err
		}

		exists, err := u.tags.ExistsName(txCtx, ownerUserID, repository.TagNameScope{
			Type:         *tagType,
			Scope:        scope,
			Dimension:    dimension,
			ResourceKind: resourceKind,
			Name:         name,
		}, tagID)
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
			Name:         name,
			Type:         *tagType,
			Scope:        scope,
			Dimension:    dimension,
			ResourceKind: resourceKind,
			TargetKey:    targetKey,
			Color:        color,
			TextColor:    textColor,
			SortOrder:    sortOrder,
			Enabled:      enabled,
			Description:  description,
			TargetKinds:  targetKinds,
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

	slog.InfoContext(ctx, "tag.updated",
		"tag_id", updated.ID,
		"owner_user_id", ownerUserID,
		"type", updated.Type,
		"dry_run", cmd.DryRun,
	)
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
	slog.InfoContext(ctx, "tag.deleted",
		"tag_id", cmd.TagID,
		"owner_user_id", ownerUserID,
		"dry_run", cmd.DryRun,
	)
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

func (u *TagUseCase) ensureTargetKinds(ctx context.Context, targetKinds []string) error {
	if len(targetKinds) == 0 {
		return nil
	}
	if err := u.tags.EnsureTargetKinds(ctx, targetKinds); err != nil {
		if errors.Is(err, repository.ErrInvalidState) {
			return fmt.Errorf("%w: unsupported targetKinds", ErrInvalidArgument)
		}
		return err
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

	if !tagTypePattern.MatchString(tagType) {
		return nil, fmt.Errorf("%w: tag type format is invalid", ErrInvalidArgument)
	}
	return &tagType, nil
}

func normalizeTagClassification(rawScope, rawDimension, rawResourceKind, tagType string) (string, string, *string, error) {
	scope := strings.ToLower(strings.TrimSpace(rawScope))
	if scope == "" {
		scope = defaultTagScope
	}
	if tagType == fileTabType {
		scope = fileTabScope
	}
	if _, ok := tagScopes[scope]; !ok {
		return "", "", nil, fmt.Errorf("%w: unsupported tag scope %s", ErrInvalidArgument, scope)
	}

	dimension := strings.ToLower(strings.TrimSpace(rawDimension))
	if dimension == "" {
		dimension = defaultTagDimension
	}
	if _, ok := tagDimensions[dimension]; !ok {
		return "", "", nil, fmt.Errorf("%w: unsupported tag dimension %s", ErrInvalidArgument, dimension)
	}

	resourceKind, err := normalizeOptionalResourceKind(rawResourceKind)
	if err != nil {
		return "", "", nil, err
	}
	if resourceKind == nil {
		resourceKind = inferResourceKindFromType(tagType)
	}
	if tagType == fileTabType {
		resourceKind = nil
	}
	return scope, dimension, resourceKind, nil
}

func normalizeOptionalTagScope(raw string) (*string, error) {
	scope := strings.ToLower(strings.TrimSpace(raw))
	if scope == "" {
		return nil, nil
	}
	if _, ok := tagScopes[scope]; !ok {
		return nil, fmt.Errorf("%w: unsupported tag scope %s", ErrInvalidArgument, scope)
	}
	return &scope, nil
}

func normalizeOptionalTagDimension(raw string) (*string, error) {
	dimension := strings.ToLower(strings.TrimSpace(raw))
	if dimension == "" {
		return nil, nil
	}
	if _, ok := tagDimensions[dimension]; !ok {
		return nil, fmt.Errorf("%w: unsupported tag dimension %s", ErrInvalidArgument, dimension)
	}
	return &dimension, nil
}

func normalizeOptionalResourceKind(raw string) (*string, error) {
	resourceKind := strings.ToLower(strings.TrimSpace(raw))
	if resourceKind == "" {
		return nil, nil
	}
	if !resourceKindPattern.MatchString(resourceKind) {
		return nil, fmt.Errorf("%w: resourceKind format is invalid", ErrInvalidArgument)
	}
	return &resourceKind, nil
}

func inferResourceKindFromType(tagType string) *string {
	switch strings.ToUpper(strings.TrimSpace(tagType)) {
	case "ASMR":
		return lo.ToPtr("asmr")
	case "COMIC":
		return lo.ToPtr("comic")
	case "VIDEO":
		return lo.ToPtr("video")
	case "AUDIO":
		return lo.ToPtr("audio")
	case "FILE":
		return lo.ToPtr("file")
	case "FOLDER":
		return lo.ToPtr("folder")
	case "GENERAL":
		return lo.ToPtr("general")
	default:
		return nil
	}
}

func normalizeTagTargetKinds(raw []string, tagType string) ([]string, error) {
	normalizedType := strings.ToUpper(strings.TrimSpace(tagType))
	if normalizedType == fileTabType {
		return []string{}, nil
	}

	values := normalizeStringSet(raw)
	if len(values) == 0 {
		values = defaultTargetKindsForTagType(normalizedType)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("%w: targetKinds is required", ErrInvalidArgument)
	}
	if len(values) > 16 {
		return nil, fmt.Errorf("%w: targetKinds length must be <= 16", ErrInvalidArgument)
	}
	for _, value := range values {
		if !targetKindPattern.MatchString(value) {
			return nil, fmt.Errorf("%w: targetKind format is invalid", ErrInvalidArgument)
		}
	}
	return values, nil
}

func normalizeStringSet(raw []string) []string {
	if len(raw) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(raw))
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func defaultTargetKindsForTagType(tagType string) []string {
	switch strings.ToUpper(strings.TrimSpace(tagType)) {
	case "ASMR":
		return []string{"asmr_work", "archive_root", "folder"}
	case "COMIC":
		return []string{"comic_work", "archive_root", "folder"}
	case "AUDIO":
		return []string{"audio_track", "audio_album", "folder"}
	case "VIDEO":
		return []string{"video_file", "video_collection", "folder"}
	case "FILE":
		return []string{"file"}
	case "FOLDER":
		return []string{"folder"}
	case "GENERAL":
		return []string{"file", "folder", "archive_root"}
	default:
		return []string{"file", "folder"}
	}
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
	tagType, scope, dimension string,
	resourceKind *string,
	name string,
	targetKey *string,
) error {
	resourceKindValue := ""
	if resourceKind != nil {
		resourceKindValue = *resourceKind
	}
	scopes := []string{
		fmt.Sprintf(
			"tags:name:%d:%s:%s:%s:%s:%s",
			ownerUserID,
			scope,
			dimension,
			resourceKindValue,
			tagType,
			name,
		),
	}
	if targetKey != nil && *targetKey != "" {
		scopes = append(scopes, fmt.Sprintf("tags:target:%d:%s:%s", ownerUserID, tagType, *targetKey))
	}

	uniqueScopes := lo.Uniq(lo.Filter(scopes, func(scope string, _ int) bool {
		return strings.TrimSpace(scope) != ""
	}))
	return u.tags.LockScopes(ctx, uniqueScopes...)
}
