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
		Actor: actorFromContext(ctx),
		Name:  strings.TrimSpace(req.Name),
		Ext:   req.Ext,
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// ReorderNode 根据请求参数执行节点重排或移动。
func (h *NodeHandler) ReorderNode(ctx *gin.Context) {
	var req moveNodeRequest
	if !BindJSON(ctx, &req) {
		return
	}
	if req.NodeID == 0 {
		BadRequest(ctx, "nodeId is required")
		return
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
		Name:         strings.TrimSpace(req.Name),
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// MoveNode 按路径中的节点 ID 执行移动。
func (h *NodeHandler) MoveNode(ctx *gin.Context) {
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
		Name:         strings.TrimSpace(req.Name),
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// DeleteNodeAndChildren 删除指定祖先节点及其子树。
func (h *NodeHandler) DeleteNodeAndChildren(ctx *gin.Context) {
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
		NodeID:    uri.AncestorID,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, ok)
}
