package router

import (
	"log/slog"

	"omniflow-go/internal/config"
	"omniflow-go/internal/transport/http/handler"
	"omniflow-go/internal/transport/http/middleware"

	"github.com/gin-gonic/gin"
)

func New(
	cfg *config.Config,
	logger *slog.Logger,
	healthHandler *handler.HealthHandler,
	authHandler *handler.AuthHandler,
	userHandler *handler.UserHandler,
	libraryHandler *handler.LibraryHandler,
	nodeHandler *handler.NodeHandler,
	directoryHandler *handler.DirectoryHandler,
	fileHandler *handler.FileHandler,
	tagHandler *handler.TagHandler,
	browserBookmarkHandler *handler.BrowserBookmarkHandler,
	browserFileMappingHandler *handler.BrowserFileMappingHandler,
	multipartUploadHandler *handler.MultipartUploadHandler,
) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(middleware.RequestID())
	engine.Use(middleware.Auth(buildAuthOptions(authHandler)))
	engine.Use(middleware.Logger(logger))

	engine.GET("/healthz", healthHandler.Check)

	v1 := engine.Group("/api/v1")
	registerAPIRoutes(
		v1,
		healthHandler,
		authHandler,
		userHandler,
		libraryHandler,
		nodeHandler,
		directoryHandler,
		fileHandler,
		tagHandler,
		browserBookmarkHandler,
		browserFileMappingHandler,
		multipartUploadHandler,
	)

	return engine
}
