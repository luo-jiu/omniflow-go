package userpreference

import (
	"encoding/json"
	"time"
)

// Preference 表示一个用户命名空间下可跨设备同步的偏好。
type Preference struct {
	UserID        uint64          `json:"-"`
	Namespace     string          `json:"namespace"`
	Preferences   json.RawMessage `json:"preferences"`
	SchemaVersion int32           `json:"schemaVersion"`
	Revision      int64           `json:"revision"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}
