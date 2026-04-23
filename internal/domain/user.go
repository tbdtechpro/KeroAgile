package domain

type User struct {
	ID          string
	DisplayName string
	IsAgent     bool
}

func (u *User) DisplayPrefix() string {
	if u.IsAgent {
		return "🤖 " + u.DisplayName
	}
	return "👤 " + u.DisplayName
}
