package router

import (
	"omniflow-go/internal/transport/http/handler"
	"omniflow-go/internal/transport/http/middleware"
)

func buildAuthOptions(authHandler *handler.AuthHandler) middleware.AuthOptions {
	// 白名单采用“精确路径匹配”，仅放登录、健康检查和公开上传能力。
	return middleware.AuthOptions{
		IgnorePaths: []string{
			"/healthz",
			"/api/v1/health",
			"/api/v1/auth/login",
			"/api/v1/auth/status",
			"/api/v1/user",
			"/api/v1/user/exists",
			"/api/v1/files/upload",
			"/api/v1/files/link",
			"/api/v1/directory/upload",
		},
		Authenticator: authHandler,
	}
}
