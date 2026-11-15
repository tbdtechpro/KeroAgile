package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// boardWith returns a service over project KA holding one ticket per title:
// KA-001, KA-002, … plus the users matt and claude.
func boardWith(t *testing.T, titles ...string) (*domain.Service, *mockStore) {
	t.Helper()
	m := newMock()
	svc := domain.NewService(m)
	require.NoError(t, svc.CreateProject("KA", "KeroAgile", ""))
	_, err := svc.CreateUser("matt", "Matt", false)
	require.NoError(t, err)
	_, err = svc.CreateUser("claude", "Claude", true)
	require.NoError(t, err)
	for _, title := range titles {
		_, err := svc.CreateTask(title, "", "KA", domain.TaskCreateOpts{})
		require.NoError(t, err)
	}
	return svc, m
}

// Found on the live board: a ticket with status "in_review", which no board
// shows. Every front end cast whatever string it was given.
func TestMoveTaskRejectsAnUnknownStatus(t *testing.T) {
	for _, status := range []domain.Status{"in_review", "doing", "Done", "in-progress", ""} {
		svc, m := boardWith(t, "a ticket")

		_, err := svc.MoveTask("KA-001", status)

		require.ErrorIs(t, err, domain.ErrInvalid, "status %q", status)
		assert.Contains(t, err.Error(), "backlog, todo, in_progress, review or done", "the error lists what is accepted")
		assert.Equal(t, domain.StatusBacklog, m.tasks["KA-001"].Status, "the ticket is unchanged")
	}
}

func TestCreateTaskRejectsAnUnknownStatusOrPriority(t *testing.T) {
	svc, m := boardWith(t)

	_, err := svc.CreateTask("a ticket", "", "KA", domain.TaskCreateOpts{Status: "doing"})
	require.ErrorIs(t, err, domain.ErrInvalid)

	_, err = svc.CreateTask("a ticket", "", "KA", domain.TaskCreateOpts{Priority: "urgent"})
	require.ErrorIs(t, err, domain.ErrInvalid)
	assert.Contains(t, err.Error(), "low, medium, high or critical")

	assert.Empty(t, m.tasks, "nothing was stored")
}

func TestCreateTaskNeedsATitle(t *testing.T) {
	svc, _ := boardWith(t)

	_, err := svc.CreateTask("   ", "", "KA", domain.TaskCreateOpts{})

	require.ErrorIs(t, err, domain.ErrInvalid)
}

// These used to surface as a raw "FOREIGN KEY constraint failed".
func TestCreateTaskNamesWhatIsMissing(t *testing.T) {
	svc, _ := boardWith(t)

	_, err := svc.CreateTask("a ticket", "", "NOPE", domain.TaskCreateOpts{})
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Contains(t, err.Error(), `project "NOPE"`)

	_, err = svc.CreateTask("a ticket", "", "KA", domain.TaskCreateOpts{AssigneeID: "nobody"})
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Contains(t, err.Error(), `assignee "nobody"`)

	missing := int64(99)
	_, err = svc.CreateTask("a ticket", "", "KA", domain.TaskCreateOpts{SprintID: &missing})
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Contains(t, err.Error(), "sprint 99")
}

func TestProjectIDIsCaseInsensitive(t *testing.T) {
	svc, _ := boardWith(t, "first")

	task, err := svc.CreateTask("second", "", "ka", domain.TaskCreateOpts{})
	require.NoError(t, err)
	assert.Equal(t, "KA-002", task.ID)

	tasks, err := svc.ListTasks("ka", domain.TaskFilters{})
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
}

// An empty list for a mistyped project reads as "nothing to do".
func TestListTasksInAnUnknownProjectIsNotFound(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")

	_, err := svc.ListTasks("NOPE", domain.TaskFilters{})

	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Contains(t, err.Error(), `project "NOPE"`)
}

func TestARejectedCreateDoesNotUseUpATicketNumber(t *testing.T) {
	svc, _ := boardWith(t)

	_, err := svc.CreateTask("bad", "", "KA", domain.TaskCreateOpts{Status: "doing"})
	require.Error(t, err)
	task, err := svc.CreateTask("good", "", "KA", domain.TaskCreateOpts{})
	require.NoError(t, err)

	assert.Equal(t, "KA-001", task.ID)
}

func TestATicketCannotBlockItself(t *testing.T) {
	svc, m := boardWith(t, "a ticket")

	err := svc.AddDep("KA-001", "KA-001")

	require.ErrorIs(t, err, domain.ErrInvalid)
	assert.Empty(t, m.deps, "no dependency was stored")
}

// The blocker graph is meant to be acyclic: tickets on a cycle can never
// start, because each waits for the other.
func TestABlockerCycleIsRejected(t *testing.T) {
	svc, m := boardWith(t, "first", "second", "third")
	require.NoError(t, svc.AddDep("KA-001", "KA-002")) // 1 blocks 2
	require.NoError(t, svc.AddDep("KA-002", "KA-003")) // 2 blocks 3

	direct := svc.AddDep("KA-002", "KA-001")  // 2 blocks 1: closes a two-ticket loop
	through := svc.AddDep("KA-003", "KA-001") // 3 blocks 1: closes a three-ticket loop

	require.ErrorIs(t, direct, domain.ErrInvalid)
	require.ErrorIs(t, through, domain.ErrInvalid)
	assert.Empty(t, m.deps["KA-001"], "KA-001 gained no blocker")
}

func TestAnAcyclicBlockerIsStillAccepted(t *testing.T) {
	svc, m := boardWith(t, "first", "second", "third")
	require.NoError(t, svc.AddDep("KA-001", "KA-002"))

	require.NoError(t, svc.AddDep("KA-001", "KA-003"), "one ticket may block several")
	require.NoError(t, svc.AddDep("KA-002", "KA-003"), "a ticket may have several blockers")

	assert.ElementsMatch(t, []string{"KA-001", "KA-002"}, m.deps["KA-003"])
}

func TestAddDepNamesTheMissingTicket(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")

	err := svc.AddDep("KA-999", "KA-001")

	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Contains(t, err.Error(), `ticket "KA-999"`)
}

func TestAddDepAcceptsLowerCaseIDs(t *testing.T) {
	svc, m := boardWith(t, "first", "second")

	require.NoError(t, svc.AddDep("ka-001", "ka-002"))

	assert.Equal(t, []string{"KA-001"}, m.deps["KA-002"])
}

// Deleting a ticket that does not exist used to report success.
func TestDeletingAMissingTicketIsNotFound(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")

	err := svc.DeleteTask("KA-999")

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestATicketCannotJoinAnotherProjectsSprint(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")
	require.NoError(t, svc.CreateProject("ZZ", "Other", ""))
	sprint, err := svc.CreateSprint("Sprint 1", "ZZ", nil, nil)
	require.NoError(t, err)

	_, err = svc.AssignTaskToSprint("KA-001", &sprint.ID)

	require.ErrorIs(t, err, domain.ErrInvalid)
}

func TestCreateSprintInAnUnknownProjectIsNotFound(t *testing.T) {
	svc, _ := boardWith(t)

	_, err := svc.CreateSprint("Sprint 1", "NOPE", nil, nil)

	require.ErrorIs(t, err, domain.ErrNotFound)
}

// Every lookup must resolve a project ID the same way. A front end that
// checks a project with one spelling and writes with another would be making
// its decision about a different project than the one it changes.
func TestEveryProjectLookupIgnoresLetterCase(t *testing.T) {
	svc, _ := boardWith(t)

	project, err := svc.GetProject("ka")
	require.NoError(t, err)
	assert.Equal(t, "KA", project.ID)

	require.NoError(t, svc.SetSprintMode("ka", true))
	project, err = svc.GetProject("KA")
	require.NoError(t, err)
	assert.True(t, project.SprintMode)
}

func TestFindTasksChecksItsFilters(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")

	_, _, err := svc.FindTasks(domain.TaskQuery{Statuses: []domain.Status{"doing"}})
	require.ErrorIs(t, err, domain.ErrInvalid)

	_, _, err = svc.FindTasks(domain.TaskQuery{ProjectID: "NOPE"})
	require.ErrorIs(t, err, domain.ErrNotFound)

	_, _, err = svc.FindTasks(domain.TaskQuery{AssigneeID: "nobody"})
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestFindTasksResolvesTheProjectInAnyLetterCase(t *testing.T) {
	svc, m := boardWith(t, "a ticket")

	_, _, err := svc.FindTasks(domain.TaskQuery{ProjectID: "ka", Text: "  ticket  "})

	require.NoError(t, err)
	assert.Equal(t, "KA", m.lastQuery.ProjectID)
	assert.Equal(t, "ticket", m.lastQuery.Text, "surrounding spaces are not part of the search")
}

func TestFindTasksFillsInWhatBlocksEachTicket(t *testing.T) {
	svc, _ := boardWith(t, "first", "second")
	require.NoError(t, svc.AddDep("KA-001", "KA-002"))

	tasks, total, err := svc.FindTasks(domain.TaskQuery{ProjectID: "KA"})

	require.NoError(t, err)
	require.Equal(t, 2, total)
	byID := map[string]*domain.Task{}
	for _, task := range tasks {
		byID[task.ID] = task
	}
	assert.Equal(t, []string{"KA-001"}, byID["KA-002"].Blockers)
	assert.Equal(t, []string{"KA-002"}, byID["KA-001"].Blocking)
}
