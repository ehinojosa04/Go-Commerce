package Models

type Role string

const (
	Admin    Role = "admin"
	Consumer Role = "consumer"
)

type User struct {
	ID   int
	Name string
	Role Role
}

func NewUser(id int, name string, role Role) *User {
	return &User{ID: id, Name: name, Role: role}
}

func (u *User) IsAdmin() bool {
	return u.Role == Admin
}

func (u *User) IsConsumer() bool {
	return u.Role == Consumer
}
