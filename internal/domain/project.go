package domain

import "time"

type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	RepoPath   string `json:"repo_path"`
	SprintMode bool   `json:"sprint_mode"`
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
