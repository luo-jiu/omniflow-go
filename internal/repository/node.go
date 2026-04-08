package repository

import "gorm.io/gorm"

type NodeRepository struct {
	db *gorm.DB
}

func NewNodeRepository(db *gorm.DB) *NodeRepository {
	return &NodeRepository{db: db}
}

func (r *NodeRepository) WithTx(tx *gorm.DB) *NodeRepository {
	if tx == nil {
		return r
	}
	return &NodeRepository{db: tx}
}
