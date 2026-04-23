package domain

import "time"

type Project struct {
	ID         string
	Name       string
	RepoPath   string
	SprintMode bool
}

type SprintStatus string

const (
	SprintPlanning  SprintStatus = "planning"
	SprintActive    SprintStatus = "active"
	SprintCompleted SprintStatus = "completed"
)

type Sprint struct {
	ID        int64
	ProjectID string
	Name      string
	StartDate *time.Time
	EndDate   *time.Time
	Status    SprintStatus
}
