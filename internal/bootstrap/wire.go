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
	"omniflow-go/internal/storage"
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
		storage.NewMinIOStore,
		wire.Bind(new(storage.ObjectStorage), new(*storage.MinIOStore)),
		wire.Bind(new(authz.Authorizer), new(*authz.AllowAll)),
		wire.Bind(new(audit.Sink), new(*audit.LogSink)),
		repository.NewUserRepository,
		repository.NewLibraryRepository,
		repository.NewNodeRepository,
		usecase.NewHealthUseCase,
		usecase.NewAuthUseCase,
		usecase.NewUserUseCase,
		usecase.NewLibraryUseCase,
		usecase.NewNodeUseCase,
		usecase.NewDirectoryUseCase,
		usecase.NewFileUseCase,
		usecase.NewTagUseCase,
		httpHandler.NewHealthHandler,
		httpHandler.NewAuthHandler,
		httpHandler.NewUserHandler,
		httpHandler.NewLibraryHandler,
		httpHandler.NewNodeHandler,
		httpHandler.NewDirectoryHandler,
		httpHandler.NewFileHandler,
		httpHandler.NewTagHandler,
		httpRouter.New,
		server.NewHTTPServer,
		app.New,
	)

	return nil, nil, nil
}
