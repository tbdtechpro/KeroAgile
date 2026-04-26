package syncsrv_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/store"
	"github.com/tbdtechpro/KeroAgile/internal/syncsrv"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(":memory:")
	require.NoError(t, err)
	s := store.New(db)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestWriteAndReadChangeLog(t *testing.T) {
	s := testStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "T1", Name: "Test"}))

	payload, _ := json.Marshal(map[string]string{"id": "T1-001", "title": "hello"})
	cursor, err := s.WriteChangeLog("T1", syncsrv.EventTaskCreated, payload, "")
	require.NoError(t, err)
	assert.Greater(t, cursor, int64(0))

	events, err := s.ReadChanges("T1", 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, syncsrv.EventTaskCreated, events[0].EventType)
	assert.Equal(t, cursor, events[0].Cursor)
	assert.Equal(t, "T1", events[0].ProjectID)
}

func TestAddAndGetSecondary(t *testing.T) {
	s := testStore(t)

	token, err := s.AddSecondary("sec-1", "My Laptop")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Token must be >= 32 hex chars
	assert.GreaterOrEqual(t, len(token), 64)

	// Look up by hash
	hash := syncsrv.SHA256Hex(token)
	sec, err := s.GetSecondaryByTokenHash(hash)
	require.NoError(t, err)
	require.NotNil(t, sec)
	assert.Equal(t, "sec-1", sec.ID)
	assert.Equal(t, "My Laptop", sec.DisplayName)
}

func TestGrantAndRevokeProject(t *testing.T) {
	s := testStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "P1", Name: "Project 1"}))
	_, err := s.AddSecondary("sec-1", "Laptop")
	require.NoError(t, err)

	require.NoError(t, s.GrantProject("sec-1", "P1"))

	granted, err := s.IsGranted("sec-1", "P1")
	require.NoError(t, err)
	assert.True(t, granted)

	projects, err := s.ListGrantedProjects("sec-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"P1"}, projects)

	require.NoError(t, s.RevokeGrant("sec-1", "P1"))

	granted, err = s.IsGranted("sec-1", "P1")
	require.NoError(t, err)
	assert.False(t, granted)
}

func TestSecondaryStoreInterface(t *testing.T) {
	s := testStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "P1", Name: "Project 1"}))

	require.NoError(t, s.SetProjectSyncCursor("P1", 42))
	require.NoError(t, s.SetProjectSyncStatus("P1", "active"))

	p, err := s.GetProject("P1")
	require.NoError(t, err)
	assert.Equal(t, int64(42), p.SyncCursor)
	assert.Equal(t, "active", p.SyncStatus)
}

func TestReadChangesFiltersCorrectly(t *testing.T) {
	s := testStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "P1", Name: "P1"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "P2", Name: "P2"}))

	payload := []byte(`{}`)
	c1, _ := s.WriteChangeLog("P1", syncsrv.EventTaskCreated, payload, "")
	_, _ = s.WriteChangeLog("P2", syncsrv.EventTaskCreated, payload, "")
	c3, _ := s.WriteChangeLog("P1", syncsrv.EventTaskUpdated, payload, "origin-sec")

	// Only P1 events
	events, err := s.ReadChanges("P1", 0)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, c1, events[0].Cursor)
	assert.Equal(t, c3, events[1].Cursor)
	assert.Equal(t, "origin-sec", events[1].Origin)

	// Since cursor — only events after c1
	events2, err := s.ReadChanges("P1", c1)
	require.NoError(t, err)
	require.Len(t, events2, 1)
	assert.Equal(t, syncsrv.EventTaskUpdated, events2[0].EventType)
}
