package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func TestGetActiveSprint_NoneExists(t *testing.T) {
	st := testStore(t)
	require.NoError(t, st.CreateProject(&domain.Project{ID: "KA", Name: "test"}))

	_, err := st.GetActiveSprint("KA")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetActiveSprint_PlanningNotReturned(t *testing.T) {
	st := testStore(t)
	require.NoError(t, st.CreateProject(&domain.Project{ID: "KA", Name: "test"}))
	_, err := st.CreateSprint(&domain.Sprint{ProjectID: "KA", Name: "Sprint 1", Status: domain.SprintPlanning})
	require.NoError(t, err)

	_, err = st.GetActiveSprint("KA")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetActiveSprint_ActiveReturned(t *testing.T) {
	st := testStore(t)
	require.NoError(t, st.CreateProject(&domain.Project{ID: "KA", Name: "test"}))
	sp, err := st.CreateSprint(&domain.Sprint{ProjectID: "KA", Name: "Sprint 1", Status: domain.SprintPlanning})
	require.NoError(t, err)

	sp.Status = domain.SprintActive
	require.NoError(t, st.UpdateSprint(sp))

	got, err := st.GetActiveSprint("KA")
	require.NoError(t, err)
	assert.Equal(t, sp.ID, got.ID)
	assert.Equal(t, domain.SprintActive, got.Status)
}

func TestListSprintsWithCounts_Empty(t *testing.T) {
	st := testStore(t)
	require.NoError(t, st.CreateProject(&domain.Project{ID: "KA", Name: "test"}))

	summaries, err := st.ListSprintsWithCounts("KA")
	require.NoError(t, err)
	assert.Empty(t, summaries)
}

func TestListSprintsWithCounts_WithTasks(t *testing.T) {
	st := testStore(t)
	require.NoError(t, st.CreateProject(&domain.Project{ID: "KA", Name: "test"}))
	sp, err := st.CreateSprint(&domain.Sprint{ProjectID: "KA", Name: "Sprint 1", Status: domain.SprintActive})
	require.NoError(t, err)

	// Create two tasks in this sprint, one without
	require.NoError(t, st.CreateTask(&domain.Task{
		ID: "KA-001", ProjectID: "KA", Title: "t1",
		Status: domain.StatusBacklog, Priority: domain.PriorityMedium, SprintID: &sp.ID,
	}))
	require.NoError(t, st.CreateTask(&domain.Task{
		ID: "KA-002", ProjectID: "KA", Title: "t2",
		Status: domain.StatusTodo, Priority: domain.PriorityMedium, SprintID: &sp.ID,
	}))
	require.NoError(t, st.CreateTask(&domain.Task{
		ID: "KA-003", ProjectID: "KA", Title: "t3",
		Status: domain.StatusBacklog, Priority: domain.PriorityMedium,
	}))

	summaries, err := st.ListSprintsWithCounts("KA")
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, sp.ID, summaries[0].Sprint.ID)
	assert.Equal(t, 2, summaries[0].TaskCount)
}
