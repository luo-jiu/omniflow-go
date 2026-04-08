package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

func registerNodeRoutes(api *gin.RouterGroup, nodeHandler *handler.NodeHandler) {
	// 节点资源
	api.POST("/nodes", nodeHandler.CreateNode)
	api.GET("/nodes/:nodeId/descendants", nodeHandler.GetAllDescendants)
	api.GET("/nodes/:nodeId/children", nodeHandler.GetDirectChildren)
	api.GET("/nodes/:nodeId/ancestors", nodeHandler.GetAncestors)
	api.GET("/nodes/:nodeId/path", nodeHandler.GetFullPath)
	api.PUT("/nodes/:nodeId", nodeHandler.UpdateNode)
	api.PATCH("/nodes/:nodeId/rename", nodeHandler.Rename)
	api.PATCH("/nodes", nodeHandler.ReorderNode)
	api.PATCH("/nodes/:nodeId/move", nodeHandler.MoveNode)
	api.DELETE("/nodes/:ancestorId/library/:libraryId", nodeHandler.DeleteNodeAndChildren)
}
