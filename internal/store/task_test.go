package store_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"keroagile/internal/domain"
)

func seedProject(t *testing.T, s interface {
	CreateProject(*domain.Project) error
}) {
	t.Helper()
	require.NoError(t, s.CreateProject(&domain.Project{ID: "KA", Name: "test"}))
}

func TestTaskCRUD(t *testing.T) {
	s := testStore(t)
	seedProject(t, s)

	pts := 3
	task := &domain.Task{
		ID:        "KA-001",
		ProjectID: "KA",
		Title:     "Fix login",
		Status:    domain.StatusBacklog,
		Priority:  domain.PriorityHigh,
		Points:    &pts,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, s.CreateTask(task))

	got, err := s.GetTask("KA-001")
	require.NoError(t, err)
	assert.Equal(t, "Fix login", got.Title)
	assert.Equal(t, domain.PriorityHigh, got.Priority)
	require.NotNil(t, got.Points)
	assert.Equal(t, 3, *got.Points)

	got.Status = domain.StatusTodo
	got.UpdatedAt = time.Now().UTC()
	require.NoError(t, s.UpdateTask(got))

	list, err := s.ListTasks("KA", domain.TaskFilters{})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, domain.StatusTodo, list[0].Status)

	require.NoError(t, s.DeleteTask("KA-001"))
	_, err = s.GetTask("KA-001")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTaskListFilter(t *testing.T) {
	s := testStore(t)
	seedProject(t, s)
	require.NoError(t, s.CreateUser(&domain.User{ID: "claude", DisplayName: "Claude"}))

	now := time.Now().UTC()
	assignee := "claude"
	for _, task := range []*domain.Task{
		{ID: "KA-001", ProjectID: "KA", Title: "A", Status: domain.StatusBacklog, Priority: domain.PriorityMedium, CreatedAt: now, UpdatedAt: now},
		{ID: "KA-002", ProjectID: "KA", Title: "B", Status: domain.StatusTodo, Priority: domain.PriorityMedium, AssigneeID: &assignee, CreatedAt: now, UpdatedAt: now},
	} {
		require.NoError(t, s.CreateTask(task))
	}

	status := domain.StatusTodo
	list, err := s.ListTasks("KA", domain.TaskFilters{Status: &status})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "KA-002", list[0].ID)
}

func TestTaskDeps(t *testing.T) {
	s := testStore(t)
	seedProject(t, s)

	now := time.Now().UTC()
	for _, task := range []*domain.Task{
		{ID: "KA-001", ProjectID: "KA", Title: "A", Status: domain.StatusBacklog, Priority: domain.PriorityMedium, CreatedAt: now, UpdatedAt: now},
		{ID: "KA-002", ProjectID: "KA", Title: "B", Status: domain.StatusBacklog, Priority: domain.PriorityMedium, CreatedAt: now, UpdatedAt: now},
	} {
		require.NoError(t, s.CreateTask(task))
	}

	require.NoError(t, s.AddDep("KA-001", "KA-002")) // KA-001 blocks KA-002
	blockers, blocking, err := s.GetTaskDeps("KA-001")
	require.NoError(t, err)
	assert.Empty(t, blockers)
	assert.Equal(t, []string{"KA-002"}, blocking)

	blockers2, _, err := s.GetTaskDeps("KA-002")
	require.NoError(t, err)
	assert.Equal(t, []string{"KA-001"}, blockers2)
}

func TestNextTaskSeq(t *testing.T) {
	s := testStore(t)
	seedProject(t, s)

	seq1, err := s.NextTaskSeq("KA")
	require.NoError(t, err)
	assert.Equal(t, 1, seq1)

	seq2, err := s.NextTaskSeq("KA")
	require.NoError(t, err)
	assert.Equal(t, 2, seq2)
}
