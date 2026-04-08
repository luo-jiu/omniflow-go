package tag

import "time"

type Tag struct {
	ID          uint64     `json:"id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	TargetKey   *string    `json:"targetKey"`
	OwnerUserID *uint64    `json:"ownerUserId"`
	Color       string     `json:"color"`
	TextColor   *string    `json:"textColor"`
	SortOrder   int        `json:"sortOrder"`
	Enabled     int        `json:"enabled"`
	Description *string    `json:"description"`
	CreatedAt   *time.Time `json:"createdAt"`
	UpdatedAt   *time.Time `json:"updatedAt"`
}
