//go:build wireinject
// +build wireinject

package bootstrap

import (
	"omniflow-go/internal/app"
	"omniflow-go/internal/audit"
	"omniflow-go/internal/authz"
	"omniflow-go/internal/config"
	"omniflow-go/internal/repository"
	"omniflow-go/internal/server"
	httpHandler "omniflow-go/internal/transport/http/handler"
	httpRouter "omniflow-go/internal/transport/http/router"
	"omniflow-go/internal/usecase"

	"github.com/google/wire"
)

func InitializeApplication(configPath string) (*app.App, func(), error) {
	wire.Build(
		config.Load,
		NewLogger,
		NewDatabase,
		NewRedis,
		authz.NewAllowAll,
		audit.NewLogSink,
		repository.NewObjectStorage,
		wire.Bind(new(authz.Authorizer), new(*authz.AllowAll)),
		wire.Bind(new(audit.Sink), new(*audit.LogSink)),
		repository.NewSessionRepository,
		repository.NewTransactor,
		repository.NewUserRepository,
		repository.NewLibraryRepository,
		repository.NewNodeRepository,
		repository.NewTagRepository,
		repository.NewBrowserBookmarkRepository,
		repository.NewBrowserFileMappingRepository,
		repository.NewUploadSessionRepository,
		repository.NewMigrationRepository,
		repository.NewResourceMonitorRepository,
		repository.NewResourceMonitorRedisProbeRepository,
		usecase.NewHealthUseCase,
		usecase.NewAuthUseCase,
		usecase.NewUserUseCase,
		usecase.NewLibraryUseCase,
		usecase.NewNodeUseCase,
		usecase.NewDirectoryUseCase,
		usecase.NewUploadSessionUseCaseWithJanitor,
		usecase.NewMigrationUseCase,
		usecase.NewResourceMonitorUseCase,
		usecase.NewMigrationWorkerPool,
		usecase.NewFileUseCase,
		usecase.NewTagUseCase,
		usecase.NewBrowserBookmarkUseCase,
		usecase.NewBrowserFileMappingUseCase,
		httpHandler.NewHealthHandler,
		httpHandler.NewAuthHandler,
		httpHandler.NewUserHandler,
		httpHandler.NewLibraryHandler,
		httpHandler.NewNodeHandler,
		httpHandler.NewDirectoryHandler,
		httpHandler.NewUploadHandler,
		httpHandler.NewFileHandler,
		httpHandler.NewTagHandler,
		httpHandler.NewBrowserBookmarkHandler,
		httpHandler.NewBrowserFileMappingHandler,
		httpHandler.NewMigrationHandler,
		httpHandler.NewResourceMonitorHandler,
		httpRouter.New,
		server.NewHTTPServer,
		app.New,
	)

	return nil, nil, nil
}
