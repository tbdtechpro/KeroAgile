package domain

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	IsAgent     bool   `json:"is_agent"`
}

func (u User) DisplayPrefix() string {
	if u.IsAgent {
		return "🤖 " + u.DisplayName
	}
	return "👤 " + u.DisplayName
}
