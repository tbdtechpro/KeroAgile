package tui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// Reported: the edit form held a copy of the ticket for as long as it was
// open and then wrote every column back, so saving a typo fix reverted
// whatever an agent had changed in the meantime.
func TestEditingATitleDoesNotRevertWhatOthersChangedMeanwhile(t *testing.T) {
	h := newAppHarness(t, seedTickets("first ticket"))
	h.key("e") // the form opens with the ticket as it is now

	// While the form is open, an agent moves the ticket on and links its work.
	_, err := h.svc.MoveTask("AA-001", domain.StatusInProgress)
	require.NoError(t, err)
	require.NoError(t, h.svc.LinkBranch("AA-001", "fix/thing"))
	_, err = h.svc.PatchTask("AA-001", domain.TaskPatch{Priority: domain.To(domain.PriorityCritical), Labels: domain.To([]string{"agent"})})
	require.NoError(t, err)

	h.typeText("!") // the person fixes the title
	h.key("enter")  // and saves

	got := h.ticket("AA-001")
	assert.Equal(t, "first ticket!", got.Title, "the edit is saved")
	assert.Equal(t, domain.StatusInProgress, got.Status, "the move is not reverted")
	assert.Equal(t, "fix/thing", got.Branch, "the branch link is not reverted")
	assert.Equal(t, domain.PriorityCritical, got.Priority, "the priority is not reverted")
	assert.Equal(t, []string{"agent"}, got.Labels, "the labels are not reverted")
}

// The description box holds at most 99 lines. Saving any edit used to write
// back what the box held, cutting a longer description short.
func TestEditingATitleLeavesALongDescriptionIntact(t *testing.T) {
	long := strings.TrimSuffix(strings.Repeat("a line of the description\n", 150), "\n")
	h := newAppHarness(t, func(svc *domain.Service) {
		_ = svc.CreateProject("AA", "Alpha", "")
		_, _ = svc.CreateTask("first ticket", long, "AA", domain.TaskCreateOpts{})
	})
	h.key("e")

	h.typeText("!")
	h.key("enter")

	got := h.ticket("AA-001")
	require.Equal(t, "first ticket!", got.Title)
	assert.Equal(t, 150, strings.Count(got.Description, "\n")+1, "every line of the description is still there")
}

// A ticket in a sprint could not be edited at all unless the sprint list had
// been opened first: the form rejected its own unchanged sprint value.
func TestEditingATicketInASprintWorksBeforeTheSprintListIsLoaded(t *testing.T) {
	var sprintID int64
	h := newAppHarness(t, func(svc *domain.Service) {
		_ = svc.CreateProject("AA", "Alpha", "")
		sprint, _ := svc.CreateSprint("Sprint 1", "AA", nil, nil)
		sprintID = sprint.ID
		_, _ = svc.CreateTask("first ticket", "", "AA", domain.TaskCreateOpts{SprintID: &sprintID})
	})
	h.key("e")

	h.typeText("!")
	h.key("enter")

	got := h.ticket("AA-001")
	assert.Equal(t, "first ticket!", got.Title, "the edit is saved")
	require.NotNil(t, got.SprintID, "the ticket stays in its sprint")
	assert.Equal(t, sprintID, *got.SprintID)
}

func TestEditingAnotherFieldSavesThatField(t *testing.T) {
	h := newAppHarness(t, seedTickets("first ticket"))
	h.key("e")

	h.keys("tab", 5) // title, description, assignee, priority, points, status
	h.key("right")   // backlog -> todo
	h.key("enter")

	got := h.ticket("AA-001")
	assert.Equal(t, domain.StatusTodo, got.Status)
	assert.Equal(t, "first ticket", got.Title)
}

func TestEmptyingPointsInTheFormClearsThem(t *testing.T) {
	five := 5
	h := newAppHarness(t, func(svc *domain.Service) {
		_ = svc.CreateProject("AA", "Alpha", "")
		_, _ = svc.CreateTask("first ticket", "", "AA", domain.TaskCreateOpts{Points: &five})
	})
	h.key("e")

	h.keys("tab", 4) // to the points field
	h.key("backspace")
	h.key("enter")

	assert.Nil(t, h.ticket("AA-001").Points)
}

// Saving without touching anything must not write at all, so it cannot move
// the ticket's update time and make another writer's conditional save fail.
func TestSavingAnUntouchedFormChangesNothing(t *testing.T) {
	h := newAppHarness(t, seedTickets("first ticket"))
	before := h.ticket("AA-001").UpdatedAt
	h.key("e")

	h.key("enter")

	assert.True(t, h.ticket("AA-001").UpdatedAt.Equal(before))
}
