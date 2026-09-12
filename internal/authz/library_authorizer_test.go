package authz

import (
	"context"
	"errors"
	"testing"

	"omniflow-go/internal/actor"
)

type ownerLookupStub struct {
	ownerID uint64
	err     error
}

func (s ownerLookupStub) LibraryOwnerID(context.Context, uint64) (uint64, error) {
	return s.ownerID, s.err
}

func TestLibraryAuthorizerAllowsOwnerAndScopedSystem(t *testing.T) {
	authorizer := NewLibraryAuthorizer(ownerLookupStub{ownerID: 42})

	if err := authorizer.Authorize(context.Background(), actor.Actor{ID: "42", Kind: actor.KindUser}, Resource{Kind: "library", ID: "7"}, ActionRead); err != nil {
		t.Fatalf("owner should be allowed: %v", err)
	}
	if err := authorizer.Authorize(context.Background(), actor.Actor{Kind: actor.KindSystem, Scopes: []string{"library:*"}}, Resource{Kind: "library", ID: "7"}, ActionWrite); err != nil {
		t.Fatalf("scoped system actor should be allowed: %v", err)
	}
}

func TestLibraryAuthorizerDeniesWrongOwnerAndLookupFailure(t *testing.T) {
	authorizer := NewLibraryAuthorizer(ownerLookupStub{ownerID: 42})
	for _, principal := range []actor.Actor{
		{ID: "41", Kind: actor.KindUser},
		{ID: "not-a-number", Kind: actor.KindUser},
		{Kind: actor.KindAnonymous},
	} {
		if err := authorizer.Authorize(context.Background(), principal, Resource{Kind: "library", ID: "7"}, ActionRead); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("principal %+v should be denied, got %v", principal, err)
		}
	}

	authorizer = NewLibraryAuthorizer(ownerLookupStub{err: errors.New("database unavailable")})
	if err := authorizer.Authorize(context.Background(), actor.Actor{ID: "42", Kind: actor.KindUser}, Resource{Kind: "library", ID: "7"}, ActionRead); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("lookup failure should deny, got %v", err)
	}
}
