package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"omniflow-go/internal/actor"
	"omniflow-go/internal/audit"
	domainuserpreference "omniflow-go/internal/domain/userpreference"
	"omniflow-go/internal/repository"
)

const maxUserPreferenceDataBytes = 64 * 1024

var (
	errUserPreferenceRepositoryNotConfigured = errors.New("user preference repository is not configured")
	userPreferenceNamespacePattern           = regexp.MustCompile(`^[a-z][a-z0-9._:-]{0,63}$`)
	errUserPreferenceRevisionConflict        = newClientMessageError(
		ErrConflict,
		"偏好已被其他设备更新，请重新加载后再保存",
	)
)

type ListUserPreferencesQuery struct {
	Actor actor.Actor
}

type GetUserPreferenceQuery struct {
	Actor     actor.Actor
	Namespace string
}

type UpsertUserPreferenceCommand struct {
	Actor            actor.Actor
	Namespace        string
	Preferences      json.RawMessage
	SchemaVersion    int32
	ExpectedRevision int64
	DryRun           bool
}

type userPreferenceRepository interface {
	ListByUser(ctx context.Context, userID uint64) ([]domainuserpreference.Preference, error)
	FindByUserAndNamespace(
		ctx context.Context,
		userID uint64,
		namespace string,
	) (domainuserpreference.Preference, error)
	Create(
		ctx context.Context,
		input repository.CreateUserPreferenceInput,
	) (domainuserpreference.Preference, error)
	UpdateByRevision(
		ctx context.Context,
		input repository.UpdateUserPreferenceInput,
	) (domainuserpreference.Preference, error)
}

type UserPreferenceUseCase struct {
	preferences userPreferenceRepository
	tx          repository.Transactor
	auditLog    audit.Sink
}

func NewUserPreferenceUseCase(
	preferences userPreferenceRepository,
	tx repository.Transactor,
	auditLog audit.Sink,
) *UserPreferenceUseCase {
	return &UserPreferenceUseCase{
		preferences: preferences,
		tx:          tx,
		auditLog:    auditLog,
	}
}

func (u *UserPreferenceUseCase) List(
	ctx context.Context,
	query ListUserPreferencesQuery,
) ([]domainuserpreference.Preference, error) {
	if err := u.ensureConfigured(); err != nil {
		return nil, err
	}
	userID, err := actorIDToUint64(query.Actor)
	if err != nil {
		return nil, err
	}
	return u.preferences.ListByUser(ctx, userID)
}

func (u *UserPreferenceUseCase) Get(
	ctx context.Context,
	query GetUserPreferenceQuery,
) (domainuserpreference.Preference, error) {
	if err := u.ensureConfigured(); err != nil {
		return domainuserpreference.Preference{}, err
	}
	userID, err := actorIDToUint64(query.Actor)
	if err != nil {
		return domainuserpreference.Preference{}, err
	}
	namespace, err := normalizeUserPreferenceNamespace(query.Namespace)
	if err != nil {
		return domainuserpreference.Preference{}, err
	}

	preference, err := u.preferences.FindByUserAndNamespace(ctx, userID, namespace)
	if errors.Is(err, repository.ErrNotFound) {
		return domainuserpreference.Preference{}, ErrNotFound
	}
	return preference, err
}

func (u *UserPreferenceUseCase) Upsert(
	ctx context.Context,
	cmd UpsertUserPreferenceCommand,
) (domainuserpreference.Preference, error) {
	if err := u.ensureConfigured(); err != nil {
		return domainuserpreference.Preference{}, err
	}
	userID, err := actorIDToUint64(cmd.Actor)
	if err != nil {
		return domainuserpreference.Preference{}, err
	}
	namespace, err := normalizeUserPreferenceNamespace(cmd.Namespace)
	if err != nil {
		return domainuserpreference.Preference{}, err
	}
	if cmd.SchemaVersion <= 0 {
		return domainuserpreference.Preference{}, fmt.Errorf(
			"%w: schemaVersion must be greater than zero",
			ErrInvalidArgument,
		)
	}
	if cmd.ExpectedRevision < 0 {
		return domainuserpreference.Preference{}, fmt.Errorf(
			"%w: expectedRevision must not be negative",
			ErrInvalidArgument,
		)
	}
	data, err := normalizeUserPreferenceData(cmd.Preferences)
	if err != nil {
		return domainuserpreference.Preference{}, err
	}

	now := time.Now().UTC()
	var saved domainuserpreference.Preference
	if err := u.withinMutationTx(ctx, cmd.DryRun, func(txCtx context.Context) error {
		if cmd.ExpectedRevision == 0 {
			created, createErr := u.preferences.Create(txCtx, repository.CreateUserPreferenceInput{
				UserID:        userID,
				Namespace:     namespace,
				Data:          string(data),
				SchemaVersion: cmd.SchemaVersion,
				Now:           now,
			})
			if errors.Is(createErr, repository.ErrConflict) {
				return errUserPreferenceRevisionConflict
			}
			if createErr != nil {
				return createErr
			}
			saved = created
			return nil
		}

		updated, updateErr := u.preferences.UpdateByRevision(txCtx, repository.UpdateUserPreferenceInput{
			UserID:           userID,
			Namespace:        namespace,
			Data:             string(data),
			SchemaVersion:    cmd.SchemaVersion,
			ExpectedRevision: cmd.ExpectedRevision,
			Now:              now,
		})
		if errors.Is(updateErr, repository.ErrConflict) {
			return errUserPreferenceRevisionConflict
		}
		if updateErr != nil {
			return updateErr
		}
		saved = updated
		return nil
	}); err != nil {
		return domainuserpreference.Preference{}, err
	}

	_ = u.writeAudit(ctx, cmd.Actor, "user.preference.upsert", true, map[string]any{
		"user_id":        userID,
		"namespace":      namespace,
		"schema_version": cmd.SchemaVersion,
		"revision":       saved.Revision,
		"mode":           resolveMutationMode(cmd.DryRun),
		"dry_run":        cmd.DryRun,
	})
	slog.InfoContext(ctx, "user.preference.updated",
		"user_id", userID,
		"namespace", namespace,
		"schema_version", cmd.SchemaVersion,
		"revision", saved.Revision,
		"dry_run", cmd.DryRun,
	)
	return saved, nil
}

func (u *UserPreferenceUseCase) ensureConfigured() error {
	if u == nil || u.preferences == nil {
		return errUserPreferenceRepositoryNotConfigured
	}
	return nil
}

func (u *UserPreferenceUseCase) withinMutationTx(
	ctx context.Context,
	dryRun bool,
	fn func(ctx context.Context) error,
) error {
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

func (u *UserPreferenceUseCase) writeAudit(
	ctx context.Context,
	principal actor.Actor,
	action string,
	success bool,
	metadata map[string]any,
) error {
	if u.auditLog == nil {
		return nil
	}
	return u.auditLog.Write(ctx, audit.Event{
		Actor:      principal,
		Action:     action,
		Resource:   "user_preference",
		Success:    success,
		OccurredAt: time.Now().UTC(),
		Metadata:   metadata,
	})
}

func normalizeUserPreferenceNamespace(raw string) (string, error) {
	namespace := strings.TrimSpace(raw)
	if !userPreferenceNamespacePattern.MatchString(namespace) {
		return "", fmt.Errorf("%w: invalid preference namespace", ErrInvalidArgument)
	}
	return namespace, nil
}

func normalizeUserPreferenceData(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || len(trimmed) > maxUserPreferenceDataBytes || !json.Valid(trimmed) {
		return nil, fmt.Errorf("%w: preferences must be a valid JSON object up to 64 KiB", ErrInvalidArgument)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil || object == nil {
		return nil, fmt.Errorf("%w: preferences must be a JSON object", ErrInvalidArgument)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return nil, fmt.Errorf("%w: preferences must be valid JSON", ErrInvalidArgument)
	}
	return json.RawMessage(compact.Bytes()), nil
}
