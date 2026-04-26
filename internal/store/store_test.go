package store_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s := store.New(db)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestProjectSyncColumns(t *testing.T) {
	s := testStore(t)
	err := s.CreateProject(&domain.Project{ID: "SY", Name: "Sync", RepoPath: ""})
	require.NoError(t, err)
	p, err := s.GetProject("SY")
	require.NoError(t, err)
	assert.Equal(t, "", p.SyncOrigin)
	assert.Equal(t, int64(0), p.SyncCursor)
	assert.Equal(t, "", p.SyncStatus)

	// Non-zero round-trip
	p.SyncOrigin = "https://primary.example.com"
	p.SyncCursor = 42
	p.SyncStatus = "active"
	require.NoError(t, s.UpdateProject(p))
	p2, err := s.GetProject("SY")
	require.NoError(t, err)
	assert.Equal(t, "https://primary.example.com", p2.SyncOrigin)
	assert.Equal(t, int64(42), p2.SyncCursor)
	assert.Equal(t, "active", p2.SyncStatus)
}

func TestSearchTasksFuzzy(t *testing.T) {
	s := testStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "KA", Name: "KeroAgile"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "KCP", Name: "KeroCareer"}))

	seq1, _ := s.NextTaskSeq("KA")
	kaID := fmt.Sprintf("KA-%03d", seq1)
	require.NoError(t, s.CreateTask(&domain.Task{
		ID: kaID, ProjectID: "KA", Title: "Add JWT auth",
		Status: domain.StatusBacklog, Priority: domain.PriorityMedium,
	}))

	seq2, _ := s.NextTaskSeq("KCP")
	kcpID := fmt.Sprintf("KCP-%03d", seq2)
	require.NoError(t, s.CreateTask(&domain.Task{
		ID: kcpID, ProjectID: "KCP", Title: "Career DB migration",
		Status: domain.StatusTodo, Priority: domain.PriorityMedium,
	}))

	// Title substring
	results, err := s.SearchTasks("JWT", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, kaID, results[0].ID)
	assert.Equal(t, "Add JWT auth", results[0].Title)
	assert.Equal(t, "KA", results[0].ProjectID)
	assert.Equal(t, domain.StatusBacklog, results[0].Status)

	// ID prefix
	results, err = s.SearchTasks("KCP", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, kcpID, results[0].ID)

	// Cross-project: both tasks match "a"
	results, err = s.SearchTasks("a", 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)

	// limit respected
	results, err = s.SearchTasks("a", 1)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSearchTasksWithHintOrdering(t *testing.T) {
	s := testStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "KA", Name: "KeroAgile"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "KCP", Name: "KeroCareer"}))

	seq1, _ := s.NextTaskSeq("KA")
	kaID := fmt.Sprintf("KA-%03d", seq1)
	require.NoError(t, s.CreateTask(&domain.Task{
		ID: kaID, ProjectID: "KA", Title: "auth feature",
		Status: domain.StatusBacklog, Priority: domain.PriorityMedium,
	}))

	seq2, _ := s.NextTaskSeq("KCP")
	kcpID := fmt.Sprintf("KCP-%03d", seq2)
	require.NoError(t, s.CreateTask(&domain.Task{
		ID: kcpID, ProjectID: "KCP", Title: "auth middleware",
		Status: domain.StatusBacklog, Priority: domain.PriorityMedium,
	}))

	// With hint=KCP, KCP task should come first
	results, err := s.SearchTasksWithHint("auth", 10, "KCP")
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, kcpID, results[0].ID, "hinted project should sort first")
}

func TestUserSyncOriginColumn(t *testing.T) {
	s := testStore(t)
	err := s.CreateUser(&domain.User{
		ID:          "u1",
		DisplayName: "Test User",
		SyncOrigin:  "https://primary.example.com",
	})
	require.NoError(t, err)
	u, err := s.GetUser("u1")
	require.NoError(t, err)
	assert.Equal(t, "https://primary.example.com", u.SyncOrigin)
}
