package syncsrv

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type Mode string

const (
	ModeStandalone Mode = "standalone"
	ModePrimary    Mode = "primary"
	ModeSecondary  Mode = "secondary"
)

type SyncState string

const (
	StateOnline       SyncState = "online"
	StateReconnecting SyncState = "reconnecting"
	StateOffline      SyncState = "offline"
)

const (
	EventTaskCreated    = "task.created"
	EventTaskUpdated    = "task.updated"
	EventTaskDeleted    = "task.deleted"
	EventSprintCreated  = "sprint.created"
	EventSprintUpdated  = "sprint.updated"
	EventProjectUpdated = "project.updated"
	EventUserMirrored   = "user.mirrored"
)

// ChangeEvent is one row from the primary's change_log, delivered via SSE.
type ChangeEvent struct {
	Cursor    int64           `json:"cursor"`
	ProjectID string          `json:"project_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	Origin    string          `json:"origin,omitempty"`
}

// Secondary is a registered secondary install on the primary.
type Secondary struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	TokenHash   string  `json:"-"`
	LastSeenAt  *string `json:"last_seen_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

// SHA256Hex returns the hex-encoded SHA-256 of s. Exported so auth middleware can use it.
func SHA256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
