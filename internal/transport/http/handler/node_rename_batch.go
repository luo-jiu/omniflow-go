package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/usecase"
)

type renameBatchRequest struct {
	LibraryID   uint64 `json:"libraryId"`
	OperationID string `json:"operationId"`
	Items       []struct {
		NodeID   uint64 `json:"nodeId"`
		Name     string `json:"name"`
		Expected *struct {
			Name      string    `json:"name"`
			Ext       *string   `json:"ext"`
			ParentID  uint64    `json:"parentId"`
			UpdatedAt time.Time `json:"updatedAt"`
		} `json:"expected"`
	} `json:"items"`
}

// RenameNodesConditional 对完整旧值快照执行有界原子改名。
func (h *NodeHandler) RenameNodesConditional(ctx *gin.Context) {
	dryRun, ok := QueryBool(ctx, false, "dryRun", "dry_run")
	if !ok {
		return
	}
	MarkDryRunHeader(ctx, dryRun)
	var req renameBatchRequest
	decoder := json.NewDecoder(http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 128<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		BadRequest(ctx, "invalid rename batch JSON: "+err.Error())
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		BadRequest(ctx, "request must contain exactly one JSON object")
		return
	}
	if req.LibraryID == 0 || req.OperationID == "" || len(req.Items) == 0 || len(req.Items) > 50 {
		BadRequest(ctx, "libraryId, operationId and 1 to 50 items are required")
		return
	}
	items := make([]domainnode.RenameBatchItem, 0, len(req.Items))
	for _, item := range req.Items {
		if item.Expected == nil || item.Expected.Ext == nil {
			BadRequest(ctx, "expected snapshot including ext is required")
			return
		}
		items = append(items, domainnode.RenameBatchItem{NodeID: item.NodeID, Name: item.Name,
			Expected: domainnode.RenameExpected{Name: item.Expected.Name, Ext: *item.Expected.Ext,
				ParentID: item.Expected.ParentID, UpdatedAt: item.Expected.UpdatedAt}})
	}
	if h.nodeUseCase == nil {
		InternalError(ctx, "node service not configured")
		return
	}
	result, err := h.nodeUseCase.RenameBatch(ctx.Request.Context(), usecase.RenameBatchCommand{
		Actor: actorFromContext(ctx), LibraryID: req.LibraryID, OperationID: req.OperationID, Items: items, DryRun: dryRun,
	})
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	SuccessWithDryRun(ctx, dryRun, result)
}

// RenameNodesStatus 查询自己的操作回执；not_found 不保证没有在途请求。
func (h *NodeHandler) RenameNodesStatus(ctx *gin.Context) {
	var req struct {
		LibraryID   uint64 `form:"libraryId" binding:"required"`
		OperationID string `form:"operationId" binding:"required"`
	}
	if !BindQuery(ctx, &req) {
		return
	}
	if h.nodeUseCase == nil {
		InternalError(ctx, "node service not configured")
		return
	}
	result, err := h.nodeUseCase.RenameBatchStatus(ctx.Request.Context(), actorFromContext(ctx), req.LibraryID, req.OperationID)
	if err != nil {
		HandleUseCaseError(ctx, err)
		return
	}
	Success(ctx, result)
}
