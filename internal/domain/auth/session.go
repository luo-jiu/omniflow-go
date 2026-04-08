package auth

import (
	"context"

	domainuser "omniflow-go/internal/domain/user"
)

// SessionStore 定义登录会话的领域端口，usecase 只依赖该接口。
type SessionStore interface {
	// FindFirstToken 返回用户名下任意一个可用 token；无数据时返回空字符串。
	FindFirstToken(ctx context.Context, username string) (string, error)
	// Save 持久化会话 token 以及对应的用户快照。
	Save(ctx context.Context, username, token string, user domainuser.User) error
	// Exists 判断会话 token 是否仍有效。
	Exists(ctx context.Context, username, token string) (bool, error)
	// GetUser 读取会话中的用户快照；不存在时返回仓储层 not found 错误。
	GetUser(ctx context.Context, username, token string) (domainuser.User, error)
	// Delete 删除指定 token；不存在时返回仓储层 not found 错误。
	Delete(ctx context.Context, username, token string) error
}
