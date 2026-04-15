package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerBrowserBookmarkRoutes(api *gin.RouterGroup, browserBookmarkHandler *handler.BrowserBookmarkHandler) {
	api.GET("/browser-bookmarks/tree", browserBookmarkHandler.Tree)
	api.GET("/browser-bookmarks/match", browserBookmarkHandler.Match)
	api.POST("/browser-bookmarks/import", browserBookmarkHandler.Import)
	api.POST("/browser-bookmarks", browserBookmarkHandler.Create)
	api.PUT("/browser-bookmarks/:bookmarkId", browserBookmarkHandler.Update)
	api.PATCH("/browser-bookmarks/:bookmarkId/move", browserBookmarkHandler.Move)
	api.DELETE("/browser-bookmarks/:bookmarkId", browserBookmarkHandler.Delete)
}
