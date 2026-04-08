package user

type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
	StatusPending  Status = "pending"
)

type User struct {
	ID       uint64
	Username string
	Phone    string
	Email    string
	Status   Status
}
