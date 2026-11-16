package store_test

import (
	"path/filepath"

	"github.com/tbdtechpro/KeroAgile/internal/store"
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

// Old rows held a directory or a remote in repo_path. Opening the database
// sorts each into repo_dir or repo_remote, once, and leaves repo_path alone.
func TestProjectRepoBackfill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "k.db")
	db, err := store.Open(path)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO projects(id, name, repo_path) VALUES
		('D','dir','/work/d'), ('R','remote','https://github.com/o/r.git'), ('N','none','')`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	db, err = store.Open(path)
	require.NoError(t, err)
	defer db.Close()
	s := store.New(db)

	d, err := s.GetProject("D")
	require.NoError(t, err)
	assert.Equal(t, "/work/d", d.RepoDir)
	assert.Equal(t, "", d.RepoRemote)
	assert.Equal(t, "/work/d", d.RepoPath)

	r, err := s.GetProject("R")
	require.NoError(t, err)
	assert.Equal(t, "github.com/o/r", r.RepoRemote)
	assert.Equal(t, "", r.RepoDir)

	// A value set since is not overwritten by a later open.
	require.NoError(t, s.PatchProject("R", domain.ProjectPatch{RepoRemote: domain.To("")}))
	require.NoError(t, db.Close())
	db2, err := store.Open(path)
	require.NoError(t, err)
	defer db2.Close()
	r, err = store.New(db2).GetProject("R")
	require.NoError(t, err)
	assert.Equal(t, "github.com/o/r", r.RepoRemote, "both empty again, so it is filled again")
}

func TestPatchProject_OnlyNamedFields(t *testing.T) {
	s := testStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "KA", Name: "k", RepoPath: "/x", RepoDir: "/x", SprintMode: true}))

	require.NoError(t, s.PatchProject("KA", domain.ProjectPatch{ArtifactPolicy: domain.To(domain.PolicyRemote)}))

	p, err := s.GetProject("KA")
	require.NoError(t, err)
	assert.Equal(t, domain.PolicyRemote, p.ArtifactPolicy)
	assert.Equal(t, "/x", p.RepoDir)
	assert.True(t, p.SprintMode)

	require.ErrorIs(t, s.PatchProject("NOPE", domain.ProjectPatch{SprintMode: domain.To(false)}), domain.ErrNotFound)
}
