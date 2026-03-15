package Models

type Role string

const (
	Admin    Role = "admin"
	Consumer Role = "consumer"
)

type User struct {
	UserID   string
	Password string
	Role     Role
}

func NewUser(name string, password string, role Role) *User {
	return &User{UserID: name, Password: password, Role: role}
}

func (u *User) IsAdmin() bool {
	return u.Role == Admin
}

func (u *User) IsConsumer() bool {
	return u.Role == Consumer
}
