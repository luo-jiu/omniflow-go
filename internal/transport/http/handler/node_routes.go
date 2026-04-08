package handler

import "github.com/gin-gonic/gin"

func (h *NodeHandler) Register(group *gin.RouterGroup) {
	group.POST("", h.CreateNode)
	group.GET("/:nodeId/descendants", h.GetAllDescendants)
	group.GET("/:nodeId/children", h.GetDirectChildren)
	group.GET("/:nodeId/ancestors", h.GetAncestors)
	group.GET("/:nodeId/path", h.GetFullPath)
	group.PUT("/:nodeId", h.UpdateNode)
	group.PATCH("/:nodeId/rename", h.Rename)
	group.PATCH("", h.ReorderNode)
	group.PATCH("/:nodeId/move", h.MoveNode)
	group.DELETE("/:ancestorId/library/:libraryId", h.DeleteNodeAndChildren)
}
