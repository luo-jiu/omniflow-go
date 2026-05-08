package router

import (
	"omniflow-go/internal/transport/http/handler"

	"github.com/gin-gonic/gin"
)

// registerMigrationRoutes 挂载存储迁移的 6 个端点。
//
// 任务相关动作集中在 /migration/tasks 下；
// 节点 provider 分布查询挂在 libraries 路径下，与 library 域语义一致。
func registerMigrationRoutes(api *gin.RouterGroup, h *handler.MigrationHandler) {
	tasks := api.Group("/migration/tasks")
	tasks.POST("", h.EnqueueTask)
	tasks.GET("", h.ListTasks)
	tasks.GET("/:id", h.GetTask)
	tasks.POST("/:id/cancel", h.CancelTask)
	tasks.GET("/:id/items", h.ListTaskItems)

	api.GET("/libraries/:libraryId/storage-distribution", h.StorageDistribution)
}
