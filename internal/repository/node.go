package repository

import (
	nodepg "omniflow-go/internal/repository/postgres/impl/node"

	"gorm.io/gorm"
)

type NodeRepository = nodepg.NodeRepository
type NodePathItem = nodepg.NodePathItem
type DeleteNodeTreeResult = nodepg.DeleteNodeTreeResult
type CreateNodeInput = nodepg.CreateNodeInput
type MoveNodeInput = nodepg.MoveNodeInput
type SearchNodesInput = nodepg.SearchNodesInput

func NewNodeRepository(db *gorm.DB) *NodeRepository {
	return nodepg.NewNodeRepository(db)
}
