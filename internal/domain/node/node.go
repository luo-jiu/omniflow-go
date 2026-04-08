package node

type Type string

const (
	TypeDirectory Type = "dir"
	TypeFile      Type = "file"
)

type Node struct {
	ID         uint64
	Name       string
	Type       Type
	ParentID   uint64
	LibraryID  uint64
	Ext        string
	MIMEType   string
	FileSize   int64
	StorageKey string
}
