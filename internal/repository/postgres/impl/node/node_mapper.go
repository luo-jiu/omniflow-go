package repository

import (
	domainnode "omniflow-go/internal/domain/node"
	pgmodel "omniflow-go/internal/repository/postgres/model"
)

func toDomainNodeModel(m *pgmodel.Node) domainnode.Node {
	nodeType := domainnode.TypeDirectory
	if m.NodeType == nodeTypeFile {
		nodeType = domainnode.TypeFile
	}

	return domainnode.Node{
		ID:          toDomainUint64(m.ID),
		Name:        m.Name,
		Type:        nodeType,
		ParentID:    parentIDValue(m.ParentID),
		LibraryID:   toDomainUint64(m.LibraryID),
		Ext:         derefString(m.Ext),
		BuiltInType: m.BuiltInType,
		ArchiveMode: boolToArchiveMode(m.ArchiveMode),
		ViewMeta:    m.ViewMeta,
	}
}

func boolToArchiveMode(enabled bool) int {
	if enabled {
		return 1
	}
	return 0
}
