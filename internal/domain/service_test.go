package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"keroagile/internal/domain"
)

// mockStore is a minimal in-memory Store for service tests.
type mockStore struct {
	projects map[string]*domain.Project
	tasks    map[string]*domain.Task
	users    map[string]*domain.User
	sprints  map[int64]*domain.Sprint
	seqs     map[string]int
	deps     map[string][]string // blocked_id -> []blocker_id
}

func newMock() *mockStore {
	return &mockStore{
		projects: make(map[string]*domain.Project),
		tasks:    make(map[string]*domain.Task),
		users:    make(map[string]*domain.User),
		sprints:  make(map[int64]*domain.Sprint),
		seqs:     make(map[string]int),
		deps:     make(map[string][]string),
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
	return m.deps[taskID], nil, nil
}
func (m *mockStore) AddDep(blockerID, blockedID string) error {
	m.deps[blockedID] = append(m.deps[blockedID], blockerID)
	return nil
}
func (m *mockStore) RemoveDep(blockerID, blockedID string) error { return nil }
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
