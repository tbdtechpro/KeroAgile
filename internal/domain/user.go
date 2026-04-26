package domain

type User struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	IsAgent      bool   `json:"is_agent"`
	PasswordHash string `json:"-"` // bcrypt hash; never serialised
	SyncOrigin   string `json:"sync_origin,omitempty"`
}

func (u User) DisplayPrefix() string {
	if u.IsAgent {
		return "🤖 " + u.DisplayName
	}
	return "👤 " + u.DisplayName
}
