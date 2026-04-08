package library

type Library struct {
	ID      uint64 `json:"id"`
	UserID  uint64 `json:"userId"`
	Name    string `json:"name"`
	Starred bool   `json:"starred"`
}
