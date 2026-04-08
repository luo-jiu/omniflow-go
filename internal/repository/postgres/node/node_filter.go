package repository

import "gorm.io/gorm"

type nodeParentFilter struct {
	ParentID uint64
}

func applyParentFilter(query *gorm.DB, filter nodeParentFilter) *gorm.DB {
	if filter.ParentID == 0 {
		return query.Where("parent_id IS NULL")
	}
	return query.Where("parent_id = ?", filter.ParentID)
}
