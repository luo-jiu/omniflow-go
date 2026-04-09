package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerNodeRoutes(api *gin.RouterGroup, nodeHandler *handler.NodeHandler) {
	// 节点资源
	api.POST("/nodes", nodeHandler.CreateNode)
	api.POST("/nodes/search", nodeHandler.SearchNodes)
	api.GET("/nodes/library/:libraryId/root", nodeHandler.GetLibraryRootNodeID)
	api.GET("/nodes/:nodeId", nodeHandler.GetNodeDetail)
	api.GET("/nodes/:nodeId/descendants", nodeHandler.GetAllDescendants)
	api.GET("/nodes/:nodeId/children", nodeHandler.GetDirectChildren)
	api.GET("/nodes/:nodeId/ancestors", nodeHandler.GetAncestors)
	api.GET("/nodes/:nodeId/path", nodeHandler.GetFullPath)
	api.PUT("/nodes/:nodeId", nodeHandler.UpdateNode)
	api.PATCH("/nodes/:nodeId/rename", nodeHandler.Rename)
	api.PATCH("/nodes", nodeHandler.ReorderNode)
	api.PATCH("/nodes/:nodeId/move", nodeHandler.MoveNode)
	api.PATCH("/nodes/:nodeId/comic/sort-by-name", nodeHandler.SortComicChildrenByName)
	api.PATCH("/nodes/:nodeId/archive/built-in-type/batch-set", nodeHandler.BatchSetArchiveChildrenBuiltInType)
	api.DELETE("/nodes/:nodeId/library/:libraryId", nodeHandler.DeleteNodeAndChildren)
	api.GET("/nodes/recycle/library/:libraryId", nodeHandler.GetRecycleBinItems)
	api.PATCH("/nodes/:nodeId/library/:libraryId/restore", nodeHandler.RestoreNodeAndChildren)
	api.DELETE("/nodes/:nodeId/library/:libraryId/hard", nodeHandler.HardDeleteNodeAndChildren)
}
