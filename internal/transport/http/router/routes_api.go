package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerAPIRoutes(
	api *gin.RouterGroup,
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
) {
	registerHealthRoutes(api, healthHandler)
	registerAuthRoutes(api, authHandler)
	registerUserRoutes(api, userHandler)
	registerLibraryRoutes(api, libraryHandler)
	registerNodeRoutes(api, nodeHandler)
	registerDirectoryRoutes(api, directoryHandler)
	registerMultipartUploadRoutes(api, multipartUploadHandler)
	registerFileRoutes(api, fileHandler)
	registerTagRoutes(api, tagHandler)
	registerBrowserBookmarkRoutes(api, browserBookmarkHandler)
	registerBrowserFileMappingRoutes(api, browserFileMappingHandler)
}
