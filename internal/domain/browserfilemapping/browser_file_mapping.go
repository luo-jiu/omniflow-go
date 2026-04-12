package browserfilemapping

import "time"

type BrowserFileMapping struct {
	ID          uint64     `json:"id"`
	FileExt     string     `json:"fileExt"`
	SiteURL     string     `json:"siteUrl"`
	OwnerUserID uint64     `json:"ownerUserId"`
	CreatedAt   *time.Time `json:"createdAt"`
	UpdatedAt   *time.Time `json:"updatedAt"`
}
