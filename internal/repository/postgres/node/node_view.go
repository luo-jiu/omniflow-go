package repository

import domainnode "omniflow-go/internal/domain/node"

type descendantRow struct {
	ID    uint64 `gorm:"column:id"`
	Depth int    `gorm:"column:depth"`
}

// nodeWithSort 用于组装树查询结果时暂存排序与深度信息。
type nodeWithSort struct {
	Node      domainnode.Node
	Depth     int
	SortOrder int
}
