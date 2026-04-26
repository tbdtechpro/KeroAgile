package domain

// TaskFilters narrows ListTasks results.
type TaskFilters struct {
	Status     *Status
	AssigneeID *string
	SprintID   *int64
}

// Store is the persistence contract. internal/store provides the SQLite implementation.
type Store interface {
	// Projects
	CreateProject(p *Project) error
	ListProjects() ([]*Project, error)
	GetProject(id string) (*Project, error)
	UpdateProject(p *Project) error

	// Tasks
	CreateTask(t *Task) error
	ListTasks(projectID string, f TaskFilters) ([]*Task, error)
	GetTask(id string) (*Task, error)
	TaskByBranch(branch string) (*Task, error)
	UpdateTask(t *Task) error
	DeleteTask(id string) error
	GetTaskDeps(taskID string) (blockers, blocking []string, err error)
	AddDep(blockerID, blockedID string) error
	RemoveDep(blockerID, blockedID string) error
	SearchTasks(q string, limit int) ([]*TaskSummary, error)
	SearchTasksWithHint(q string, limit int, hintProjectID string) ([]*TaskSummary, error)
	NextTaskSeq(projectID string) (int, error)

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
