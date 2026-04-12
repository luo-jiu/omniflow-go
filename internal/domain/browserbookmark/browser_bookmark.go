package browserbookmark

import "time"

const (
	KindURL    = "url"
	KindFolder = "folder"
)

type BrowserBookmark struct {
	ID          uint64            `json:"id"`
	OwnerUserID uint64            `json:"ownerUserId"`
	ParentID    *uint64           `json:"parentId"`
	Kind        string            `json:"kind"`
	Title       string            `json:"title"`
	URL         *string           `json:"url"`
	URLMatchKey *string           `json:"urlMatchKey"`
	IconURL     *string           `json:"iconUrl"`
	SortOrder   int               `json:"sortOrder"`
	CreatedAt   *time.Time        `json:"createdAt"`
	UpdatedAt   *time.Time        `json:"updatedAt"`
	Children    []BrowserBookmark `json:"children,omitempty"`
}

type MatchResult struct {
	Matched  bool             `json:"matched"`
	Bookmark *BrowserBookmark `json:"bookmark"`
}
