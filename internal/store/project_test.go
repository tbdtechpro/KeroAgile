package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func TestProjectCRUD(t *testing.T) {
	s := testStore(t)

	p := &domain.Project{ID: "KA", Name: "myapp", RepoPath: "/tmp/myapp"}
	require.NoError(t, s.CreateProject(p))

	got, err := s.GetProject("KA")
	require.NoError(t, err)
	assert.Equal(t, "myapp", got.Name)
	assert.Equal(t, "/tmp/myapp", got.RepoPath)

	got.SprintMode = true
	require.NoError(t, s.UpdateProject(got))

	list, err := s.ListProjects()
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.True(t, list[0].SprintMode)
}

func TestGetProjectNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.GetProject("NOPE")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
