package node

import "time"

// RenameExpected 记录批准时的完整名称及节点版本快照。
type RenameExpected struct {
	Name      string    `json:"name"`
	Ext       string    `json:"ext"`
	ParentID  uint64    `json:"parentId"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RenameBatchItem 仅修改文件主体名，扩展名保持旧值。
type RenameBatchItem struct {
	NodeID   uint64         `json:"nodeId"`
	Expected RenameExpected `json:"expected"`
	Name     string         `json:"name"`
}

// RenameState 是不包含存储地址的名称与版本投影。
type RenameState struct {
	Name      string    `json:"name"`
	Ext       string    `json:"ext"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RenameItemResult 给出单项的旧名称、结果及真实数据库版本。
type RenameItemResult struct {
	NodeID   uint64      `json:"nodeId"`
	ParentID uint64      `json:"parentId"`
	Previous RenameState `json:"previous"`
	Current  RenameState `json:"current"`
	Status   string      `json:"status"`
}

// RenameBatchResult 是原子改名结果；validated 不表示已经提交。
type RenameBatchResult struct {
	OperationID       string             `json:"operationId"`
	LibraryID         uint64             `json:"libraryId"`
	Atomic            bool               `json:"atomic"`
	State             string             `json:"state"`
	Replayed          bool               `json:"replayed"`
	Items             []RenameItemResult `json:"items"`
	AffectedParentIDs []uint64           `json:"affectedParentIds"`
}

// RenameBatchStatus 的 not_found 仅表示当前没有已提交回执，不排除在途请求。
type RenameBatchStatus struct {
	OperationID string             `json:"operationId"`
	LibraryID   uint64             `json:"libraryId"`
	State       string             `json:"state"`
	Result      *RenameBatchResult `json:"result,omitempty"`
}

// MutationReceipt 与节点修改同事务保存，作为查询与安全重放的依据。
type MutationReceipt struct {
	ActorID, OperationID, Kind, RequestHash, Result string
	LibraryID                                       uint64
}
