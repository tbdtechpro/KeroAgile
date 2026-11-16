package store_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/store"
)

// openFile opens its own handle on the database file at path, the way a
// separate KeroAgile process would.
func openFile(t *testing.T, path string) *store.Store {
	t.Helper()
	db, err := store.Open(path)
	require.NoError(t, err)
	s := store.New(db)
	t.Cleanup(func() { s.Close() })
	return s
}

func newTask(id string) *domain.Task {
	now := time.Now()
	return &domain.Task{
		ID: id, ProjectID: "AA", Title: "ticket " + id,
		Status: domain.StatusBacklog, Priority: domain.PriorityMedium,
		CreatedAt: now, UpdatedAt: now,
	}
}

// About 30 MCP servers, the TUI and the web server share one database file.
// Without a busy timeout a second writer failed at once with
// "database is locked (5) (SQLITE_BUSY)"; it must wait its turn instead.
func TestHandlesSharingOneFileWaitInsteadOfFailing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board.db")
	handles := []*store.Store{openFile(t, path), openFile(t, path), openFile(t, path), openFile(t, path)}
	require.NoError(t, handles[0].CreateProject(&domain.Project{ID: "AA", Name: "Alpha"}))

	const perHandle = 25
	errs := make(chan error, len(handles)*perHandle*3)
	var wg sync.WaitGroup
	for _, s := range handles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perHandle {
				seq, err := s.NextTaskSeq("AA")
				errs <- err
				if err != nil {
					continue
				}
				errs <- s.CreateTask(newTask(fmt.Sprintf("AA-%03d", seq)))
				_, err = s.ListTasks("AA", domain.TaskFilters{})
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	failed := 0
	var first error
	for err := range errs {
		if err != nil {
			failed++
			if first == nil {
				first = err
			}
		}
	}
	assert.Zero(t, failed, "operations that failed; first error: %v", first)

	tasks, err := handles[0].ListTasks("AA", domain.TaskFilters{})
	require.NoError(t, err)
	assert.Len(t, tasks, len(handles)*perHandle, "every write landed exactly once")
}

// With the rollback journal a read in progress holds off every writer until
// it finishes. With the write-ahead log the two do not block each other.
func TestWriterIsNotBlockedByAReadInProgress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board.db")
	writer := openFile(t, path)
	require.NoError(t, writer.CreateProject(&domain.Project{ID: "AA", Name: "Alpha"}))
	require.NoError(t, writer.CreateTask(newTask("AA-001")))

	readerDB, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { readerDB.Close() })
	rows, err := readerDB.Query(`SELECT id FROM tasks`)
	require.NoError(t, err)
	t.Cleanup(func() { rows.Close() })
	require.True(t, rows.Next(), "the read is now in progress")

	done := make(chan error, 1)
	go func() { done <- writer.CreateTask(newTask("AA-002")) }()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Error("the write is still waiting for the open read")
	}
}

// Connection settings must reach every connection the pool opens. The
// foreign-key pragma used to be run once, on whichever connection was first.
func TestForeignKeysAreEnforcedOnEveryConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board.db")
	db, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	db.SetMaxIdleConns(0) // every statement from here on gets a fresh connection
	s := store.New(db)

	task := newTask("ZZ-001")
	task.ProjectID = "ZZ" // no such project

	assert.Error(t, s.CreateTask(task), "a ticket in a project that does not exist")
}

// The path is placed in a SQLite URI, where ?, # and % have meaning.
func TestOpenUsesThePathExactlyEvenWithURICharacters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "odd #1 100% ?.db")
	s := openFile(t, path)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "AA", Name: "Alpha"}))

	_, err := os.Stat(path)
	assert.NoError(t, err, "the database file exists at exactly the given path")
}
