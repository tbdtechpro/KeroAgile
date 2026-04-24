package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("not found")

type TaskCreateOpts struct {
	Priority   Priority
	Status     Status
	AssigneeID string
	Points     *int
	Labels     []string
	SprintID   *int64
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) CreateProject(id, name, repoPath string) error {
	return s.store.CreateProject(&Project{
		ID:       strings.ToUpper(id),
		Name:     name,
		RepoPath: repoPath,
	})
}

func (s *Service) ListProjects() ([]*Project, error) {
	return s.store.ListProjects()
}

func (s *Service) GetProject(id string) (*Project, error) {
	return s.store.GetProject(id)
}

func (s *Service) SetSprintMode(projectID string, enabled bool) error {
	p, err := s.store.GetProject(projectID)
	if err != nil {
		return err
	}
	p.SprintMode = enabled
	return s.store.UpdateProject(p)
}

func (s *Service) CreateTask(title, description, projectID string, opts TaskCreateOpts) (*Task, error) {
	seq, err := s.store.NextTaskSeq(projectID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	t := &Task{
		ID:          fmt.Sprintf("%s-%03d", projectID, seq),
		ProjectID:   projectID,
		Title:       title,
		Description: description,
		Status:      StatusBacklog,
		Priority:    PriorityMedium,
		SprintID:    opts.SprintID,
		Labels:      opts.Labels,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if opts.Priority != "" {
		t.Priority = opts.Priority
	}
	if opts.Status != "" {
		t.Status = opts.Status
	}
	if opts.AssigneeID != "" {
		t.AssigneeID = &opts.AssigneeID
	}
	if opts.Points != nil {
		t.Points = opts.Points
	}
	return t, s.store.CreateTask(t)
}

func (s *Service) ListTasks(projectID string, f TaskFilters) ([]*Task, error) {
	tasks, err := s.store.ListTasks(projectID, f)
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		blockers, blocking, err := s.store.GetTaskDeps(t.ID)
		if err != nil {
			return nil, err
		}
		t.Blockers = blockers
		t.Blocking = blocking
	}
	return tasks, nil
}

func (s *Service) GetTask(id string) (*Task, error) {
	t, err := s.store.GetTask(id)
	if err != nil {
		return nil, err
	}
	blockers, blocking, err := s.store.GetTaskDeps(id)
	if err != nil {
		return nil, err
	}
	t.Blockers = blockers
	t.Blocking = blocking
	return t, nil
}

func (s *Service) UpdateTask(t *Task) (*Task, error) {
	t.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateTask(t); err != nil {
		return nil, err
	}
	blockers, blocking, err := s.store.GetTaskDeps(t.ID)
	if err != nil {
		return nil, err
	}
	t.Blockers = blockers
	t.Blocking = blocking
	return t, nil
}

func (s *Service) MoveTask(id string, status Status) (*Task, error) {
	t, err := s.store.GetTask(id)
	if err != nil {
		return nil, err
	}
	t.Status = status
	t.UpdatedAt = time.Now().UTC()
	return t, s.store.UpdateTask(t)
}

func (s *Service) DeleteTask(id string) error {
	return s.store.DeleteTask(id)
}

func (s *Service) LinkBranch(taskID, branch string) error {
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return err
	}
	t.Branch = branch
	t.UpdatedAt = time.Now().UTC()
	return s.store.UpdateTask(t)
}

func (s *Service) LinkPR(taskID string, prNumber int) error {
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return err
	}
	t.PRNumber = &prNumber
	t.UpdatedAt = time.Now().UTC()
	return s.store.UpdateTask(t)
}

func (s *Service) MarkPRMerged(taskID string) error {
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return err
	}
	t.PRMerged = true
	t.Status = StatusDone
	t.UpdatedAt = time.Now().UTC()
	return s.store.UpdateTask(t)
}

func (s *Service) CreateSprint(name, projectID string, start, end *time.Time) (*Sprint, error) {
	return s.store.CreateSprint(&Sprint{
		ProjectID: projectID,
		Name:      name,
		StartDate: start,
		EndDate:   end,
		Status:    SprintPlanning,
	})
}

func (s *Service) ListSprints(projectID string) ([]*Sprint, error) {
	return s.store.ListSprints(projectID)
}

func (s *Service) GetSprint(id int64) (*Sprint, error) {
	return s.store.GetSprint(id)
}

func (s *Service) ActivateSprint(id int64) error {
	sp, err := s.store.GetSprint(id)
	if err != nil {
		return err
	}
	sp.Status = SprintActive
	return s.store.UpdateSprint(sp)
}

func (s *Service) CreateUser(id, displayName string, isAgent bool) (*User, error) {
	u := &User{ID: id, DisplayName: displayName, IsAgent: isAgent}
	return u, s.store.CreateUser(u)
}

func (s *Service) ListUsers() ([]*User, error) {
	return s.store.ListUsers()
}

func (s *Service) GetUser(id string) (*User, error) {
	return s.store.GetUser(id)
}

func (s *Service) AddDep(blockerID, blockedID string) error {
	return s.store.AddDep(blockerID, blockedID)
}

func (s *Service) RemoveDep(blockerID, blockedID string) error {
	return s.store.RemoveDep(blockerID, blockedID)
}

// AssignTaskToSprint sets or clears the sprint assignment for a task.
// sprintID == nil clears the assignment.
func (s *Service) AssignTaskToSprint(taskID string, sprintID *int64) (*Task, error) {
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	t.SprintID = sprintID
	if err := s.store.UpdateTask(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) GetActiveSprint(projectID string) (*Sprint, error) {
	return s.store.GetActiveSprint(projectID)
}

func (s *Service) ListSprintsWithCounts(projectID string) ([]SprintSummary, error) {
	return s.store.ListSprintsWithCounts(projectID)
}
