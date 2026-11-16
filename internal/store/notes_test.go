package store_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func TestNotes_OldestFirst_RemovedWithTicket(t *testing.T) {
	s := testStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "KA", Name: "k"}))
	now := time.Now().UTC()
	for _, id := range []string{"KA-001", "KA-002"} {
		require.NoError(t, s.CreateTask(&domain.Task{ID: id, ProjectID: "KA", Title: id,
			Status: domain.StatusBacklog, Priority: domain.PriorityMedium, CreatedAt: now, UpdatedAt: now}))
	}
	first := &domain.Note{TaskID: "KA-001", AuthorID: "matt", Body: "first", CreatedAt: now}
	require.NoError(t, s.AddNote(first))
	assert.NotZero(t, first.ID)
	require.NoError(t, s.AddNote(&domain.Note{TaskID: "KA-001", Body: "second", CreatedAt: now}))
	require.NoError(t, s.AddNote(&domain.Note{TaskID: "KA-002", Body: "other", CreatedAt: now}))

	notes, err := s.ListNotes("KA-001")
	require.NoError(t, err)
	require.Len(t, notes, 2)
	assert.Equal(t, "first", notes[0].Body)
	assert.Equal(t, "matt", notes[0].AuthorID)
	assert.Equal(t, "second", notes[1].Body)
	assert.Equal(t, now.Truncate(time.Microsecond), notes[0].CreatedAt.Truncate(time.Microsecond))

	require.NoError(t, s.DeleteTask("KA-001"))
	notes, err = s.ListNotes("KA-001")
	require.NoError(t, err)
	assert.Empty(t, notes)
	notes, err = s.ListNotes("KA-002")
	require.NoError(t, err)
	assert.Len(t, notes, 1)
}
