package authz

import (
	"context"
	"errors"

	"omniflow-go/internal/actor"
)

var ErrPermissionDenied = errors.New("permission denied")

type Action string

const (
	ActionRead   Action = "read"
	ActionWrite  Action = "write"
	ActionDelete Action = "delete"
	ActionUpload Action = "upload"
	ActionManage Action = "manage"
)

type Resource struct {
	Kind string
	ID   string
}

type Authorizer interface {
	Authorize(ctx context.Context, actor actor.Actor, resource Resource, action Action) error
}

type AllowAll struct{}

func NewAllowAll() *AllowAll {
	return &AllowAll{}
}

func (a *AllowAll) Authorize(_ context.Context, _ actor.Actor, _ Resource, _ Action) error {
	return nil
}
