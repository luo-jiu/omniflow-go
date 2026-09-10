package node

// TagState 是正式关系表的直接标签快照，不包含存储凭据或视图配置。
type TagState struct {
	NodeID            uint64   `json:"nodeId"`
	LibraryID         uint64   `json:"libraryId"`
	ParentID          uint64   `json:"parentId"`
	TagIDs            []uint64 `json:"tagIds"`
	AffectedParentIDs []uint64 `json:"affectedParentIds"`
	DryRun            bool     `json:"dryRun"`
}
