package router

import (
	"omniflow-go/internal/transport/http/handler"
	"omniflow-go/internal/transport/http/middleware"
)

func buildAuthOptions(authHandler *handler.AuthHandler) middleware.AuthOptions {
	// 白名单采用“精确 method+path 匹配”，仅放登录、健康检查和公开上传能力。
	return middleware.AuthOptions{
		IgnorePaths: []string{
			"GET /healthz",
			"GET /api/v1/health",
			"POST /api/v1/auth/login",
			"POST /api/v1/user",
			"GET /api/v1/user/exists",
			"POST /api/v1/files/upload",
			"GET /api/v1/files/link",
			"POST /api/v1/directory/upload",
		},
		Authenticator: authHandler,
	}
}
