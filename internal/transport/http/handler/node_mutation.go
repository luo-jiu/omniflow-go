package handler

import (
	"strings"

	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

// UpdateNode 更新节点元数据标记。
func (h *NodeHandler) UpdateNode(ctx *gin.Context) {
	var uri nodeURI
	if !BindURI(ctx, &uri) {
		return
	}

	var req updateNodeRequest
	if !BindJSON(ctx, &req) {
		return
	}

	if h.nodeUseCase == nil {
		SuccessNoData(ctx)
		return
	}

	var builtInPtr *string
	if req.BuiltInType != nil {
		trimmed := strings.TrimSpace(*req.BuiltInType)
		builtInPtr = &trimmed
	}

	if err := h.nodeUseCase.Update(ctx.Request.Context(), uri.NodeID, usecase.UpdateNodeCommand{
		Actor:       actorFromContext(ctx),
		BuiltInType: builtInPtr,
		ArchiveMode: req.ArchiveMode,
		ViewMeta:    req.ViewMeta,
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// Rename 在同级目录下重命名节点。
func (h *NodeHandler) Rename(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var uri nodeURI
	if !BindURI(ctx, &uri) {
		return
	}

	var req renameNodeRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if !RequireNonEmpty(ctx, strings.TrimSpace(req.Name), "name") {
		return
	}

	if h.nodeUseCase == nil {
		SuccessNoData(ctx)
		return
	}

	if err := h.nodeUseCase.Rename(ctx.Request.Context(), uri.NodeID, usecase.RenameNodeCommand{
		Actor:  actorFromContext(ctx),
		Name:   req.Name,
		Ext:    req.Ext,
		DryRun: dryRun,
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// ReorderNode 与 Java 保持一致：当前为兼容保留接口（空实现）。
func (h *NodeHandler) ReorderNode(ctx *gin.Context) {
	SuccessNoData(ctx)
}

// MoveNode 按路径中的节点 ID 执行移动。
func (h *NodeHandler) MoveNode(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var uri nodeURI
	if !BindURI(ctx, &uri) {
		return
	}

	var req moveNodeRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if req.NodeID == 0 {
		req.NodeID = uri.NodeID
	}

	if h.nodeUseCase == nil {
		SuccessNoData(ctx)
		return
	}

	if err := h.nodeUseCase.Move(ctx.Request.Context(), usecase.MoveNodeCommand{
		Actor:        actorFromContext(ctx),
		LibraryID:    req.LibraryID,
		NodeID:       req.NodeID,
		NewParentID:  req.NewParentID,
		BeforeNodeID: req.BeforeNodeID,
		Name:         req.Name,
		DryRun:       dryRun,
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// SortComicChildrenByName 对 COMIC 目录的直接子节点按名称重排。
func (h *NodeHandler) SortComicChildrenByName(ctx *gin.Context) {
	var uri nodeURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.nodeUseCase == nil {
		SuccessNoData(ctx)
		return
	}

	if err := h.nodeUseCase.SortComicChildrenByName(ctx.Request.Context(), usecase.SortComicChildrenCommand{
		Actor:  actorFromContext(ctx),
		NodeID: uri.NodeID,
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// BatchSetArchiveChildrenBuiltInType 将归档目录的第一代子目录批量设置为父目录内置类型。
func (h *NodeHandler) BatchSetArchiveChildrenBuiltInType(ctx *gin.Context) {
	var uri nodeURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.nodeUseCase == nil {
		SuccessNoData(ctx)
		return
	}

	result, err := h.nodeUseCase.BatchSetArchiveChildrenBuiltInType(
		ctx.Request.Context(),
		usecase.BatchSetArchiveChildrenBuiltInTypeCommand{
			Actor:  actorFromContext(ctx),
			NodeID: uri.NodeID,
		},
	)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, result)
}

// DeleteNodeAndChildren 删除指定祖先节点及其子树。
func (h *NodeHandler) DeleteNodeAndChildren(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)

	var uri deleteNodeURI
	if !BindURI(ctx, &uri) {
		return
	}

	if h.nodeUseCase == nil {
		Success(ctx, true)
		return
	}

	ok, err := h.nodeUseCase.DeleteNodeAndChildren(ctx.Request.Context(), usecase.DeleteNodeTreeCommand{
		Actor:     actorFromContext(ctx),
		LibraryID: uri.LibraryID,
		NodeID:    uri.NodeID,
		DryRun:    dryRun,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, ok)
}
