package store_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/store"
)

func get(t *testing.T, s *store.Store, id string) *domain.Task {
	t.Helper()
	task, err := s.GetTask(id)
	require.NoError(t, err)
	return task
}

func patch(t *testing.T, s *store.Store, id string, p domain.TaskPatch) {
	t.Helper()
	require.NoError(t, s.PatchTask(id, p, time.Now()))
}

func TestPatchTaskChangesOnlyTheFieldsItNames(t *testing.T) {
	s := board(t)
	claude := "claude"
	put(t, s, "AA-001", "original title", func(k *domain.Task) {
		k.Description = "original description"
		k.AssigneeID = &claude
		k.Labels = []string{"tui"}
		k.Branch = "fix/thing"
	})

	patch(t, s, "AA-001", domain.TaskPatch{Title: domain.To("new title")})

	got := get(t, s, "AA-001")
	assert.Equal(t, "new title", got.Title)
	assert.Equal(t, "original description", got.Description)
	assert.Equal(t, "claude", *got.AssigneeID)
	assert.Equal(t, []string{"tui"}, got.Labels)
	assert.Equal(t, "fix/thing", got.Branch)
	assert.Equal(t, domain.StatusBacklog, got.Status)
}

// The reported failure: two writers each read the ticket, then each saved the
// whole row, so the second save reverted the first one's change.
func TestTwoWritersChangingDifferentFieldsBothKeepTheirChange(t *testing.T) {
	s := board(t)
	put(t, s, "AA-001", "a ticket", nil)

	// An agent moves the ticket and links its branch...
	patch(t, s, "AA-001", domain.TaskPatch{Status: domain.To(domain.StatusInProgress), Branch: domain.To("fix/thing")})
	// ...then a person who opened the edit form earlier saves a new title.
	patch(t, s, "AA-001", domain.TaskPatch{Title: domain.To("a better title")})

	got := get(t, s, "AA-001")
	assert.Equal(t, "a better title", got.Title)
	assert.Equal(t, domain.StatusInProgress, got.Status, "the agent's move survives")
	assert.Equal(t, "fix/thing", got.Branch, "the agent's branch survives")
}

func TestPatchTaskSetsEveryKindOfField(t *testing.T) {
	s := board(t)
	sprint, err := s.CreateSprint(&domain.Sprint{ProjectID: "AA", Name: "Sprint 1", Status: domain.SprintPlanning})
	require.NoError(t, err)
	put(t, s, "AA-001", "a ticket", nil)
	matt, five, pr := "matt", 5, 42

	patch(t, s, "AA-001", domain.TaskPatch{
		Description: domain.To("described"),
		Status:      domain.To(domain.StatusReview),
		Priority:    domain.To(domain.PriorityCritical),
		AssigneeID:  domain.To(&matt),
		Points:      domain.To(&five),
		Labels:      domain.To([]string{"a", "b"}),
		SprintID:    domain.To(&sprint.ID),
		PRNumber:    domain.To(&pr),
		PRMerged:    domain.To(true),
	})

	got := get(t, s, "AA-001")
	assert.Equal(t, "described", got.Description)
	assert.Equal(t, domain.StatusReview, got.Status)
	assert.Equal(t, domain.PriorityCritical, got.Priority)
	assert.Equal(t, "matt", *got.AssigneeID)
	assert.Equal(t, 5, *got.Points)
	assert.Equal(t, []string{"a", "b"}, got.Labels)
	assert.Equal(t, sprint.ID, *got.SprintID)
	assert.Equal(t, 42, *got.PRNumber)
	assert.True(t, got.PRMerged)
}

// Assignee, points and sprint could be set but never emptied.
func TestPatchTaskCanClearFields(t *testing.T) {
	s := board(t)
	sprint, err := s.CreateSprint(&domain.Sprint{ProjectID: "AA", Name: "Sprint 1", Status: domain.SprintPlanning})
	require.NoError(t, err)
	matt, five := "matt", 5
	put(t, s, "AA-001", "a ticket", func(k *domain.Task) {
		k.AssigneeID, k.Points, k.SprintID = &matt, &five, &sprint.ID
		k.Labels = []string{"a"}
		k.Description = "described"
	})

	patch(t, s, "AA-001", domain.TaskPatch{
		AssigneeID:  domain.Clear[string](),
		Points:      domain.Clear[int](),
		SprintID:    domain.Clear[int64](),
		Labels:      domain.To([]string(nil)),
		Description: domain.To(""),
	})

	got := get(t, s, "AA-001")
	assert.Nil(t, got.AssigneeID)
	assert.Nil(t, got.Points)
	assert.Nil(t, got.SprintID)
	assert.Empty(t, got.Labels)
	assert.Empty(t, got.Description)
}

func TestPatchTaskWithAStaleReadIsAConflict(t *testing.T) {
	s := board(t)
	put(t, s, "AA-001", "a ticket", nil)
	readAt := get(t, s, "AA-001").UpdatedAt

	patch(t, s, "AA-001", domain.TaskPatch{Status: domain.To(domain.StatusTodo)}) // someone else gets there first

	err := s.PatchTask("AA-001", domain.TaskPatch{Title: domain.To("mine"), IfUpdatedAt: &readAt}, time.Now())

	require.ErrorIs(t, err, domain.ErrConflict)
	assert.Equal(t, "a ticket", get(t, s, "AA-001").Title, "the stale change was not applied")
}

func TestPatchTaskWithAFreshReadGoesThrough(t *testing.T) {
	s := board(t)
	put(t, s, "AA-001", "a ticket", nil)
	patch(t, s, "AA-001", domain.TaskPatch{Status: domain.To(domain.StatusTodo)})
	readAt := get(t, s, "AA-001").UpdatedAt

	err := s.PatchTask("AA-001", domain.TaskPatch{Title: domain.To("mine"), IfUpdatedAt: &readAt}, time.Now())

	require.NoError(t, err)
	assert.Equal(t, "mine", get(t, s, "AA-001").Title)
}

// With whole-second timestamps, a change made in the same second as the read
// would be invisible to the staleness check.
func TestPatchTaskNoticesAChangeMadeInTheSameSecond(t *testing.T) {
	s := board(t)
	put(t, s, "AA-001", "a ticket", nil)
	second := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	require.NoError(t, s.PatchTask("AA-001", domain.TaskPatch{Status: domain.To(domain.StatusTodo)}, second.Add(100*time.Millisecond)))
	readAt := get(t, s, "AA-001").UpdatedAt

	require.NoError(t, s.PatchTask("AA-001", domain.TaskPatch{Status: domain.To(domain.StatusReview)}, second.Add(600*time.Millisecond)))
	err := s.PatchTask("AA-001", domain.TaskPatch{Title: domain.To("mine"), IfUpdatedAt: &readAt}, second.Add(900*time.Millisecond))

	require.ErrorIs(t, err, domain.ErrConflict)
}

// Tickets written before this change carry whole-second timestamps. A caller
// that read one must still be able to make a conditional change to it.
func TestPatchTaskConditionWorksOnATicketWithAWholeSecondTimestamp(t *testing.T) {
	s := board(t)
	put(t, s, "AA-001", "a ticket", func(k *domain.Task) {
		k.UpdatedAt = time.Date(2026, 4, 26, 23, 40, 22, 0, time.UTC)
	})
	readAt := get(t, s, "AA-001").UpdatedAt

	err := s.PatchTask("AA-001", domain.TaskPatch{Title: domain.To("mine"), IfUpdatedAt: &readAt}, time.Now())

	require.NoError(t, err)
}

func TestPatchTaskOnAMissingTicketIsNotFound(t *testing.T) {
	s := board(t)

	err := s.PatchTask("AA-999", domain.TaskPatch{Title: domain.To("x")}, time.Now())

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPatchTaskRecordsWhenItHappened(t *testing.T) {
	s := board(t)
	put(t, s, "AA-001", "a ticket", nil)
	when := time.Date(2026, 11, 15, 10, 0, 0, 123456789, time.UTC)

	require.NoError(t, s.PatchTask("AA-001", domain.TaskPatch{Title: domain.To("x")}, when))

	assert.True(t, get(t, s, "AA-001").UpdatedAt.Equal(when), "to the nanosecond")
}
