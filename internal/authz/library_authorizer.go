package authz

import (
	"context"
	"strconv"
	"strings"

	"omniflow-go/internal/actor"
)

// LibraryOwnerLookup 查询资料库归属用户。
// 实现位于 repository 层，授权器只依赖这个最小端口。
type LibraryOwnerLookup interface {
	LibraryOwnerID(ctx context.Context, libraryID uint64) (uint64, error)
}

// LibraryAuthorizer 按资料库 owner 和 actor scope 执行 fail-closed 授权。
type LibraryAuthorizer struct {
	libraries LibraryOwnerLookup
}

// NewLibraryAuthorizer 创建资料库授权器。
func NewLibraryAuthorizer(libraries LibraryOwnerLookup) *LibraryAuthorizer {
	return &LibraryAuthorizer{libraries: libraries}
}

// Authorize 检查 actor 是否可以访问指定资料库。
func (a *LibraryAuthorizer) Authorize(ctx context.Context, principal actor.Actor, resource Resource, action Action) error {
	if a == nil || a.libraries == nil || resource.Kind != "library" {
		return ErrPermissionDenied
	}

	libraryID, err := strconv.ParseUint(strings.TrimSpace(resource.ID), 10, 64)
	if err != nil || libraryID == 0 {
		return ErrPermissionDenied
	}

	if hasLibraryScope(principal, action) {
		return nil
	}
	if principal.Kind != actor.KindUser && principal.Kind != actor.KindAgent {
		return ErrPermissionDenied
	}
	userID, err := strconv.ParseUint(strings.TrimSpace(principal.ID), 10, 64)
	if err != nil || userID == 0 {
		return ErrPermissionDenied
	}

	ownerID, err := a.libraries.LibraryOwnerID(ctx, libraryID)
	if err != nil || ownerID != userID {
		return ErrPermissionDenied
	}
	return nil
}

func hasLibraryScope(principal actor.Actor, action Action) bool {
	wanted := "library:" + string(action)
	for _, scope := range principal.Scopes {
		scope = strings.TrimSpace(scope)
		if scope == "library:*" || scope == wanted {
			return true
		}
	}
	return false
}
