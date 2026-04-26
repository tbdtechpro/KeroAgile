package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// mockStore is a minimal in-memory Store for service tests.
type mockStore struct {
	projects map[string]*domain.Project
	tasks    map[string]*domain.Task
	users    map[string]*domain.User
	sprints  map[int64]*domain.Sprint
	seqs     map[string]int
	deps     map[string][]string // blocked_id -> []blocker_id
	blocking map[string][]string // blocker_id -> []blocked_id
}

func newMock() *mockStore {
	return &mockStore{
		projects: make(map[string]*domain.Project),
		tasks:    make(map[string]*domain.Task),
		users:    make(map[string]*domain.User),
		sprints:  make(map[int64]*domain.Sprint),
		seqs:     make(map[string]int),
		deps:     make(map[string][]string),
		blocking: make(map[string][]string),
	}
}

func (m *mockStore) CreateProject(p *domain.Project) error {
	m.projects[p.ID] = p
	return nil
}
func (m *mockStore) ListProjects() ([]*domain.Project, error) {
	var out []*domain.Project
	for _, p := range m.projects {
		out = append(out, p)
	}
	return out, nil
}
func (m *mockStore) GetProject(id string) (*domain.Project, error) {
	p, ok := m.projects[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return p, nil
}
func (m *mockStore) UpdateProject(p *domain.Project) error {
	m.projects[p.ID] = p
	return nil
}
func (m *mockStore) CreateTask(t *domain.Task) error {
	m.tasks[t.ID] = t
	return nil
}
func (m *mockStore) ListTasks(projectID string, f domain.TaskFilters) ([]*domain.Task, error) {
	var out []*domain.Task
	for _, t := range m.tasks {
		if t.ProjectID != projectID {
			continue
		}
		if f.Status != nil && t.Status != *f.Status {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}
func (m *mockStore) GetTask(id string) (*domain.Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}
func (m *mockStore) UpdateTask(t *domain.Task) error {
	m.tasks[t.ID] = t
	return nil
}
func (m *mockStore) DeleteTask(id string) error {
	delete(m.tasks, id)
	return nil
}
func (m *mockStore) GetTaskDeps(taskID string) (blockers, blocking []string, err error) {
	return m.deps[taskID], m.blocking[taskID], nil
}
func (m *mockStore) AddDep(blockerID, blockedID string) error {
	m.deps[blockedID] = append(m.deps[blockedID], blockerID)
	m.blocking[blockerID] = append(m.blocking[blockerID], blockedID)
	return nil
}
func (m *mockStore) RemoveDep(blockerID, blockedID string) error { return nil }
func (m *mockStore) SearchTasks(q string, limit int) ([]*domain.TaskSummary, error) {
	return m.SearchTasksWithHint(q, limit, "")
}
func (m *mockStore) SearchTasksWithHint(q string, limit int, hintProjectID string) ([]*domain.TaskSummary, error) {
	var out []*domain.TaskSummary
	for _, t := range m.tasks {
		if strings.Contains(t.ID, q) || strings.Contains(t.Title, q) {
			out = append(out, &domain.TaskSummary{
				ID:        t.ID,
				Title:     t.Title,
				ProjectID: t.ProjectID,
				Status:    t.Status,
			})
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
func (m *mockStore) NextTaskSeq(projectID string) (int, error) {
	m.seqs[projectID]++
	return m.seqs[projectID], nil
}
func (m *mockStore) CreateSprint(s *domain.Sprint) (*domain.Sprint, error) {
	s.ID = int64(len(m.sprints) + 1)
	m.sprints[s.ID] = s
	return s, nil
}
func (m *mockStore) ListSprints(projectID string) ([]*domain.Sprint, error) { return nil, nil }
func (m *mockStore) GetSprint(id int64) (*domain.Sprint, error) {
	s, ok := m.sprints[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s, nil
}
func (m *mockStore) UpdateSprint(s *domain.Sprint) error {
	m.sprints[s.ID] = s
	return nil
}
func (m *mockStore) GetActiveSprint(projectID string) (*domain.Sprint, error) {
	for _, s := range m.sprints {
		if s.ProjectID == projectID && s.Status == domain.SprintActive {
			return s, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (m *mockStore) ListSprintsWithCounts(projectID string) ([]domain.SprintSummary, error) {
	var out []domain.SprintSummary
	for _, sp := range m.sprints {
		if sp.ProjectID != projectID {
			continue
		}
		var count int
		for _, t := range m.tasks {
			if t.SprintID != nil && *t.SprintID == sp.ID {
				count++
			}
		}
		out = append(out, domain.SprintSummary{Sprint: sp, TaskCount: count})
	}
	return out, nil
}
func (m *mockStore) CreateUser(u *domain.User) error {
	m.users[u.ID] = u
	return nil
}
func (m *mockStore) ListUsers() ([]*domain.User, error) { return nil, nil }
func (m *mockStore) GetUser(id string) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}
func (m *mockStore) SetUserPassword(id, hash string) error { return nil }

func TestServiceCreateTask(t *testing.T) {
	svc := domain.NewService(newMock())
	require.NoError(t, svc.CreateProject("KA", "myapp", ""))

	task, err := svc.CreateTask("Fix login", "", "KA", domain.TaskCreateOpts{
		Priority:   domain.PriorityHigh,
		AssigneeID: "claude",
	})
	require.NoError(t, err)
	assert.Equal(t, "KA-001", task.ID)
	assert.Equal(t, domain.StatusBacklog, task.Status)
	assert.Equal(t, domain.PriorityHigh, task.Priority)
}

func TestServiceMoveTask(t *testing.T) {
	svc := domain.NewService(newMock())
	require.NoError(t, svc.CreateProject("KA", "myapp", ""))
	task, err := svc.CreateTask("Fix login", "", "KA", domain.TaskCreateOpts{})
	require.NoError(t, err)

	moved, err := svc.MoveTask(task.ID, domain.StatusTodo)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusTodo, moved.Status)
}

func TestServiceSecondTaskSeq(t *testing.T) {
	svc := domain.NewService(newMock())
	require.NoError(t, svc.CreateProject("KA", "myapp", ""))
	_, _ = svc.CreateTask("First", "", "KA", domain.TaskCreateOpts{})
	second, err := svc.CreateTask("Second", "", "KA", domain.TaskCreateOpts{})
	require.NoError(t, err)
	assert.Equal(t, "KA-002", second.ID)
}

func TestAssignTaskToSprint(t *testing.T) {
	svc := domain.NewService(newMock())
	require.NoError(t, svc.CreateProject("KA", "myapp", ""))
	task, err := svc.CreateTask("Do work", "", "KA", domain.TaskCreateOpts{})
	require.NoError(t, err)
	require.Nil(t, task.SprintID)

	// Assign to sprint 1
	sprintID := int64(1)
	updated, err := svc.AssignTaskToSprint(task.ID, &sprintID)
	require.NoError(t, err)
	require.NotNil(t, updated.SprintID)
	assert.Equal(t, int64(1), *updated.SprintID)

	// Clear assignment
	cleared, err := svc.AssignTaskToSprint(task.ID, nil)
	require.NoError(t, err)
	assert.Nil(t, cleared.SprintID)
}

func TestAssignTaskToSprint_NotFound(t *testing.T) {
	svc := domain.NewService(newMock())
	_, err := svc.AssignTaskToSprint("KA-999", nil)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSearchTasksPassThrough(t *testing.T) {
	svc := domain.NewService(newMock())
	require.NoError(t, svc.CreateProject("KA", "KeroAgile", ""))
	require.NoError(t, svc.CreateProject("KCP", "KeroCareer", ""))
	_, err := svc.CreateTask("Add JWT auth", "", "KA", domain.TaskCreateOpts{})
	require.NoError(t, err)
	_, err = svc.CreateTask("Career DB migration", "", "KCP", domain.TaskCreateOpts{})
	require.NoError(t, err)

	results, err := svc.SearchTasks("JWT", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "KA", results[0].ProjectID)
}

func TestCrossProjectDep(t *testing.T) {
	svc := domain.NewService(newMock())
	require.NoError(t, svc.CreateProject("KA", "KeroAgile", ""))
	require.NoError(t, svc.CreateProject("KCP", "KeroCareer", ""))
	ka, err := svc.CreateTask("KA task", "", "KA", domain.TaskCreateOpts{})
	require.NoError(t, err)
	kcp, err := svc.CreateTask("KCP task", "", "KCP", domain.TaskCreateOpts{})
	require.NoError(t, err)

	require.NoError(t, svc.AddDep(kcp.ID, ka.ID)) // kcp.ID blocks ka.ID

	t2, err := svc.GetTask(ka.ID)
	require.NoError(t, err)
	assert.Contains(t, t2.Blockers, kcp.ID)
}
