package repository

import domainnode "omniflow-go/internal/domain/node"

func (m nodesEntity) toDomainNode() domainnode.Node {
	ext := ""
	if m.Ext != nil {
		ext = *m.Ext
	}

	nodeType := domainnode.TypeDirectory
	if m.NodeType == nodeTypeFile {
		nodeType = domainnode.TypeFile
	}

	return domainnode.Node{
		ID:          m.ID,
		Name:        m.Name,
		Type:        nodeType,
		ParentID:    parentIDValue(m.ParentID),
		LibraryID:   m.LibraryID,
		Ext:         ext,
		BuiltInType: m.BuiltIn,
		ArchiveMode: boolToArchiveMode(m.Archive),
	}
}

func boolToArchiveMode(enabled bool) int {
	if enabled {
		return 1
	}
	return 0
}

func parentIDValue(parentID *uint64) uint64 {
	if parentID == nil {
		return 0
	}
	return *parentID
}
