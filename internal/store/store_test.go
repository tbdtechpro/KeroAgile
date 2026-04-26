package store_test

import (
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
