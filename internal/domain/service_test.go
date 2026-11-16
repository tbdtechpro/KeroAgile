package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// mockStore is a minimal in-memory Store for service tests.
type mockStore struct {
	// Store is nil: a method the mock does not define panics if a test
	// reaches it, so new record methods need no stubs here.
	domain.Store

	projects map[string]*domain.Project
	tasks    map[string]*domain.Task
	users    map[string]*domain.User
	sprints  map[int64]*domain.Sprint
	seqs     map[string]int
	deps     map[string][]string // blocked_id -> []blocker_id
	blocking map[string][]string // blocker_id -> []blocked_id

	notes   []*domain.Note
	links   []*domain.Link
	linkSeq int64
	noteSeq int64

	lastQuery domain.TaskQuery   // the query FindTasks was last called with
	patches   []domain.TaskPatch // every patch PatchTask was called with
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
func (m *mockStore) PatchProject(id string, p domain.ProjectPatch) error {
	pr, ok := m.projects[id]
	if !ok {
		return domain.ErrNotFound
	}
	if p.RepoDir.Set {
		pr.RepoDir = p.RepoDir.Value
	}
	if p.RepoRemote.Set {
		pr.RepoRemote = p.RepoRemote.Value
	}
	if p.ArtifactPolicy.Set {
		pr.ArtifactPolicy = p.ArtifactPolicy.Value
	}
	if p.SprintMode.Set {
		pr.SprintMode = p.SprintMode.Value
	}
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
func (m *mockStore) TaskByBranch(branch string) (*domain.Task, error) {
	for _, t := range m.tasks {
		if t.Branch == branch {
			return t, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (m *mockStore) UpdateTask(t *domain.Task) error {
	m.tasks[t.ID] = t
	return nil
}
func (m *mockStore) PatchTask(id string, p domain.TaskPatch, now time.Time) error {
	m.patches = append(m.patches, p)
	t, ok := m.tasks[id]
	if !ok {
		return domain.ErrNotFound
	}
	if p.IfUpdatedAt != nil && !p.IfUpdatedAt.Equal(t.UpdatedAt) {
		return domain.ErrConflict
	}
	changed := *t
	if p.Title.Set {
		changed.Title = p.Title.Value
	}
	if p.Description.Set {
		changed.Description = p.Description.Value
	}
	if p.Status.Set {
		changed.Status = p.Status.Value
	}
	if p.Priority.Set {
		changed.Priority = p.Priority.Value
	}
	if p.AssigneeID.Set {
		changed.AssigneeID = p.AssigneeID.Value
	}
	if p.Points.Set {
		changed.Points = p.Points.Value
	}
	if p.Labels.Set {
		changed.Labels = p.Labels.Value
	}
	if p.SprintID.Set {
		changed.SprintID = p.SprintID.Value
	}
	if p.Branch.Set {
		changed.Branch = p.Branch.Value
	}
	if p.PRNumber.Set {
		changed.PRNumber = p.PRNumber.Value
	}
	if p.PRMerged.Set {
		changed.PRMerged = p.PRMerged.Value
	}
	changed.UpdatedAt = now
	m.tasks[id] = &changed
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
func (m *mockStore) FindTasks(q domain.TaskQuery) ([]*domain.Task, int, error) {
	m.lastQuery = q
	var out []*domain.Task
	for _, t := range m.tasks {
		if q.ProjectID == "" || t.ProjectID == q.ProjectID {
			out = append(out, t)
		}
	}
	return out, len(out), nil
}
func (m *mockStore) TaskCounts() (map[string]map[domain.Status]int, error) { return nil, nil }
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
	_, err := svc.CreateUser("claude", "Claude", true)
	require.NoError(t, err)

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

	// Assign to a sprint of the same project
	sprint, err := svc.CreateSprint("Sprint 1", "KA", nil, nil)
	require.NoError(t, err)
	updated, err := svc.AssignTaskToSprint(task.ID, &sprint.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.SprintID)
	assert.Equal(t, sprint.ID, *updated.SprintID)

	// Clear assignment
	cleared, err := svc.AssignTaskToSprint(task.ID, nil)
	require.NoError(t, err)
	assert.Nil(t, cleared.SprintID)
}

func TestServiceTaskByBranch(t *testing.T) {
	svc := domain.NewService(newMock())
	require.NoError(t, svc.CreateProject("KA", "KeroAgile", ""))

	task, err := svc.CreateTask("Fix login", "", "KA", domain.TaskCreateOpts{})
	require.NoError(t, err)
	require.NoError(t, svc.LinkBranch(task.ID, "feature/fix-login"))

	got, err := svc.TaskByBranch("feature/fix-login")
	require.NoError(t, err)
	assert.Equal(t, task.ID, got.ID)

	_, err = svc.TaskByBranch("feature/no-such")
	assert.ErrorIs(t, err, domain.ErrNotFound)
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

func (m *mockStore) AddNote(n *domain.Note) error {
	m.noteSeq++
	n.ID = m.noteSeq
	m.notes = append(m.notes, n)
	return nil
}

func (m *mockStore) ListNotes(taskID string) ([]*domain.Note, error) {
	var out []*domain.Note
	for _, n := range m.notes {
		if n.TaskID == taskID {
			out = append(out, n)
		}
	}
	return out, nil
}

func (m *mockStore) ListArtifacts(q domain.ArtifactQuery) ([]*domain.Artifact, error) { return nil, nil }

func (m *mockStore) UpsertLink(l *domain.Link) (bool, error) {
	for _, have := range m.links {
		if have.TaskID == l.TaskID && have.Kind == l.Kind && have.Repo == l.Repo && have.Ref == l.Ref {
			if l.Title != "" {
				have.Title = l.Title
			}
			if l.URL != "" {
				have.URL = l.URL
			}
			if l.State != "" {
				have.State = l.State
			}
			*l = *have
			return false, nil
		}
	}
	m.linkSeq++
	l.ID = m.linkSeq
	cp := *l
	m.links = append(m.links, &cp)
	return true, nil
}

func (m *mockStore) ListLinks(taskID string) ([]*domain.Link, error) {
	var out []*domain.Link
	for _, l := range m.links {
		if l.TaskID == taskID {
			out = append(out, l)
		}
	}
	return out, nil
}
