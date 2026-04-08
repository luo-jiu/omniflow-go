package node

type Type string

const (
	TypeDirectory Type = "dir"
	TypeFile      Type = "file"
)

type Node struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Type        Type   `json:"type"`
	ParentID    uint64 `json:"parentId"`
	LibraryID   uint64 `json:"libraryId"`
	Ext         string `json:"ext,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
	FileSize    int64  `json:"fileSize,omitempty"`
	StorageKey  string `json:"storageKey,omitempty"`
	BuiltInType string `json:"builtInType,omitempty"`
	ArchiveMode int    `json:"archiveMode,omitempty"`
	ViewMeta    string `json:"viewMeta,omitempty"`
}
