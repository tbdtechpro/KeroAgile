package domain

import "time"

type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	RepoPath   string `json:"repo_path"` // as first given; RepoDir and RepoRemote are what is used
	SprintMode bool   `json:"sprint_mode"`
	SyncOrigin string `json:"sync_origin,omitempty"`
	SyncCursor int64  `json:"sync_cursor,omitempty"`
	SyncStatus string `json:"sync_status,omitempty"`

	RepoDir        string         `json:"repo_dir,omitempty"`        // local checkout directory
	RepoRemote     string         `json:"repo_remote,omitempty"`     // "host/owner/repo"
	ArtifactPolicy ArtifactPolicy `json:"artifact_policy,omitempty"` // empty: follow the configured default
}

// ArtifactPolicy says what happens to files kept for a project's tickets.
type ArtifactPolicy string

const (
	PolicyDefault ArtifactPolicy = ""       // follow the configured default
	PolicyRemote  ArtifactPolicy = "remote" // keep, and upload to the remote store
	PolicyLocal   ArtifactPolicy = "local"  // keep on this machine only
	PolicyOff     ArtifactPolicy = "off"    // do not capture
)

// Valid reports whether p is a known policy, including the default.
func (p ArtifactPolicy) Valid() bool {
	switch p {
	case PolicyDefault, PolicyRemote, PolicyLocal, PolicyOff:
		return true
	}
	return false
}

// ProjectPatch changes some of a project's settings and leaves the rest as
// they are in the store.
type ProjectPatch struct {
	RepoDir        Change[string]
	RepoRemote     Change[string]
	ArtifactPolicy Change[ArtifactPolicy]
	SprintMode     Change[bool]
}

type SprintStatus string

const (
	SprintPlanning  SprintStatus = "planning"
	SprintActive    SprintStatus = "active"
	SprintCompleted SprintStatus = "completed"
)

type Sprint struct {
	ID        int64        `json:"id"`
	ProjectID string       `json:"project_id"`
	Name      string       `json:"name"`
	StartDate *time.Time   `json:"start_date"`
	EndDate   *time.Time   `json:"end_date"`
	Status    SprintStatus `json:"status"`
}

// SprintSummary pairs a sprint with its task count for display in the sidebar.
type SprintSummary struct {
	Sprint    *Sprint `json:"sprint"`
	TaskCount int     `json:"task_count"`
}
