package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"omniflow-go/internal/actor"
	domainuserpreference "omniflow-go/internal/domain/userpreference"
	"omniflow-go/internal/repository"
)

type fakeUserPreferenceRepository struct {
	row *domainuserpreference.Preference
}

type rollbackUserPreferenceTransactor struct {
	repo *fakeUserPreferenceRepository
}

func (t *rollbackUserPreferenceTransactor) WithinTx(
	ctx context.Context,
	fn func(ctx context.Context) error,
) error {
	var snapshot *domainuserpreference.Preference
	if t.repo.row != nil {
		copy := *t.repo.row
		copy.Preferences = append(json.RawMessage(nil), t.repo.row.Preferences...)
		snapshot = &copy
	}
	err := fn(ctx)
	if err != nil {
		t.repo.row = snapshot
	}
	return err
}

func (f *fakeUserPreferenceRepository) ListByUser(
	_ context.Context,
	userID uint64,
) ([]domainuserpreference.Preference, error) {
	if f.row == nil || f.row.UserID != userID {
		return []domainuserpreference.Preference{}, nil
	}
	return []domainuserpreference.Preference{*f.row}, nil
}

func (f *fakeUserPreferenceRepository) FindByUserAndNamespace(
	_ context.Context,
	userID uint64,
	namespace string,
) (domainuserpreference.Preference, error) {
	if f.row == nil || f.row.UserID != userID || f.row.Namespace != namespace {
		return domainuserpreference.Preference{}, repository.ErrNotFound
	}
	return *f.row, nil
}

func (f *fakeUserPreferenceRepository) Create(
	_ context.Context,
	input repository.CreateUserPreferenceInput,
) (domainuserpreference.Preference, error) {
	if f.row != nil {
		return domainuserpreference.Preference{}, repository.ErrConflict
	}
	f.row = &domainuserpreference.Preference{
		UserID:        input.UserID,
		Namespace:     input.Namespace,
		Preferences:   json.RawMessage(input.Data),
		SchemaVersion: input.SchemaVersion,
		Revision:      1,
		CreatedAt:     input.Now,
		UpdatedAt:     input.Now,
	}
	return *f.row, nil
}

func (f *fakeUserPreferenceRepository) UpdateByRevision(
	_ context.Context,
	input repository.UpdateUserPreferenceInput,
) (domainuserpreference.Preference, error) {
	if f.row == nil ||
		f.row.UserID != input.UserID ||
		f.row.Namespace != input.Namespace ||
		f.row.Revision != input.ExpectedRevision {
		return domainuserpreference.Preference{}, repository.ErrConflict
	}
	f.row.Preferences = json.RawMessage(input.Data)
	f.row.SchemaVersion = input.SchemaVersion
	f.row.Revision++
	f.row.UpdatedAt = input.Now
	return *f.row, nil
}

func TestUserPreferenceUseCaseUpsertCreateAndUpdate(t *testing.T) {
	t.Parallel()

	repo := &fakeUserPreferenceRepository{}
	uc := NewUserPreferenceUseCase(repo, &fakeTransactor{}, nil)
	principal := actor.Actor{ID: "7", Kind: actor.KindUser}

	created, err := uc.Upsert(context.Background(), UpsertUserPreferenceCommand{
		Actor:            principal,
		Namespace:        "tool-workspace",
		Preferences:      json.RawMessage(`{"navWidth":224}`),
		SchemaVersion:    1,
		ExpectedRevision: 0,
	})
	if err != nil {
		t.Fatalf("create preference: %v", err)
	}
	if created.Revision != 1 || string(created.Preferences) != `{"navWidth":224}` {
		t.Fatalf("unexpected created preference: %+v", created)
	}

	updated, err := uc.Upsert(context.Background(), UpsertUserPreferenceCommand{
		Actor:            principal,
		Namespace:        "tool-workspace",
		Preferences:      json.RawMessage(`{"navWidth":240}`),
		SchemaVersion:    1,
		ExpectedRevision: created.Revision,
	})
	if err != nil {
		t.Fatalf("update preference: %v", err)
	}
	if updated.Revision != 2 || string(updated.Preferences) != `{"navWidth":240}` {
		t.Fatalf("unexpected updated preference: %+v", updated)
	}
}

func TestUserPreferenceUseCaseRejectsStaleRevision(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &fakeUserPreferenceRepository{row: &domainuserpreference.Preference{
		UserID:        9,
		Namespace:     "tool-workspace",
		Preferences:   json.RawMessage(`{"navWidth":224}`),
		SchemaVersion: 1,
		Revision:      4,
		CreatedAt:     now,
		UpdatedAt:     now,
	}}
	uc := NewUserPreferenceUseCase(repo, &fakeTransactor{}, nil)

	_, err := uc.Upsert(context.Background(), UpsertUserPreferenceCommand{
		Actor:            actor.Actor{ID: "9", Kind: actor.KindUser},
		Namespace:        "tool-workspace",
		Preferences:      json.RawMessage(`{"navWidth":240}`),
		SchemaVersion:    1,
		ExpectedRevision: 3,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestUserPreferenceUseCaseDryRunReturnsPreviewWithoutPersisting(t *testing.T) {
	t.Parallel()

	repo := &fakeUserPreferenceRepository{}
	uc := NewUserPreferenceUseCase(repo, &rollbackUserPreferenceTransactor{repo: repo}, nil)

	preview, err := uc.Upsert(context.Background(), UpsertUserPreferenceCommand{
		Actor:            actor.Actor{ID: "11", Kind: actor.KindUser},
		Namespace:        "tool-workspace",
		Preferences:      json.RawMessage(`{"navWidth":216}`),
		SchemaVersion:    1,
		ExpectedRevision: 0,
		DryRun:           true,
	})
	if err != nil {
		t.Fatalf("dry-run preference: %v", err)
	}
	if preview.Revision != 1 || string(preview.Preferences) != `{"navWidth":216}` {
		t.Fatalf("unexpected dry-run preview: %+v", preview)
	}
	if repo.row != nil {
		t.Fatalf("dry-run must not persist preference, got %+v", repo.row)
	}
}

func TestNormalizeUserPreferenceData(t *testing.T) {
	t.Parallel()

	valid, err := normalizeUserPreferenceData(json.RawMessage(" { \"enabled\" : true } "))
	if err != nil {
		t.Fatalf("normalize valid data: %v", err)
	}
	if string(valid) != `{"enabled":true}` {
		t.Fatalf("unexpected normalized data: %s", valid)
	}

	for _, raw := range []string{"", "null", "[]", `"text"`, "{"} {
		if _, err := normalizeUserPreferenceData(json.RawMessage(raw)); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("expected invalid argument for %q, got %v", raw, err)
		}
	}

	oversized := json.RawMessage(`{"value":"` + strings.Repeat("x", maxUserPreferenceDataBytes) + `"}`)
	if _, err := normalizeUserPreferenceData(oversized); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected oversized preference to be rejected, got %v", err)
	}
}
