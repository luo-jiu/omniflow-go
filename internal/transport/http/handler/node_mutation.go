package handler

import (
	"strings"

	"omniflow-go/internal/usecase"

	"github.com/gin-gonic/gin"
)

// UpdateNode updates node metadata flags.
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
		LibraryID:   req.LibraryID,
		BuiltInType: builtInPtr,
		ArchiveMode: req.ArchiveMode,
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// Rename updates node display name under the same parent.
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
		Actor:     actorFromContext(ctx),
		LibraryID: req.LibraryID,
		Name:      strings.TrimSpace(req.Name),
	}); err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessNoData(ctx)
}

// ReorderNode reorders/moves node based on request payload.
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

// MoveNode handles move with explicit node id in path.
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

// DeleteNodeAndChildren removes a node subtree by ancestor id.
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
