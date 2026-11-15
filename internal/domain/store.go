package domain

import "time"

// TaskFilters narrows ListTasks results.
type TaskFilters struct {
	Status     *Status
	AssigneeID *string
	SprintID   *int64
}

// TaskQuery finds tickets across projects. Every field is optional; an empty
// query matches every ticket.
type TaskQuery struct {
	Text       string   // matches the ticket ID, title or description, in any letter case
	ProjectID  string   // one project; empty means every project
	Statuses   []Status // any of these; empty means any status
	AssigneeID string
	Label      string // a whole label, in any letter case
	SprintID   *int64
	Limit      int // most tickets to return; zero or less means no limit
}

// Store is the persistence contract. internal/store provides the SQLite implementation.
type Store interface {
	// Projects
	CreateProject(p *Project) error
	ListProjects() ([]*Project, error)
	GetProject(id string) (*Project, error)
	UpdateProject(p *Project) error
	// PatchProject changes only the settings p names.
	PatchProject(id string, p ProjectPatch) error

	// Tasks
	CreateTask(t *Task) error
	ListTasks(projectID string, f TaskFilters) ([]*Task, error)
	GetTask(id string) (*Task, error)
	TaskByBranch(branch string) (*Task, error)
	UpdateTask(t *Task) error
	// PatchTask changes only the fields p names, in a single statement, and
	// records now as the ticket's update time. It returns ErrNotFound for a
	// missing ticket and ErrConflict when p.IfUpdatedAt no longer matches.
	PatchTask(id string, p TaskPatch, now time.Time) error
	DeleteTask(id string) error
	GetTaskDeps(taskID string) (blockers, blocking []string, err error)
	AddDep(blockerID, blockedID string) error
	RemoveDep(blockerID, blockedID string) error
	SearchTasks(q string, limit int) ([]*TaskSummary, error)
	SearchTasksWithHint(q string, limit int, hintProjectID string) ([]*TaskSummary, error)
	NextTaskSeq(projectID string) (int, error)
	// FindTasks returns the tickets matching q, at most q.Limit of them, and
	// how many match in all.
	FindTasks(q TaskQuery) (tasks []*Task, total int, err error)
	// TaskCounts returns the number of tickets per project and status.
	TaskCounts() (map[string]map[Status]int, error)

	// Sprints
	CreateSprint(s *Sprint) (*Sprint, error)
	ListSprints(projectID string) ([]*Sprint, error)
	GetSprint(id int64) (*Sprint, error)
	UpdateSprint(s *Sprint) error
	GetActiveSprint(projectID string) (*Sprint, error)
	ListSprintsWithCounts(projectID string) ([]SprintSummary, error)

	// Users
	CreateUser(u *User) error
	ListUsers() ([]*User, error)
	GetUser(id string) (*User, error)
	SetUserPassword(id, hash string) error
}
