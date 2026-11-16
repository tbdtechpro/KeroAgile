package tui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The reported behavior: pressing d right after launch deleted the first
// ticket of the first project, with focus on the sidebar and no prompt.
func TestDeleteKeyAsksBeforeDeleting(t *testing.T) {
	h := newAppHarness(t, seedTickets("first ticket", "second ticket"))

	h.key("d")

	assert.True(t, h.exists("AA-001"), "d alone must not delete anything")
	assert.Contains(t, h.view(), "Delete AA-001", "the prompt names the ticket")
	assert.Contains(t, h.view(), "first ticket", "the prompt shows the title")
}

func TestDeleteHappensOnlyAfterY(t *testing.T) {
	h := newAppHarness(t, seedTickets("first ticket", "second ticket"))

	h.key("d")
	h.key("y")

	assert.False(t, h.exists("AA-001"), "the confirmed ticket is deleted")
	assert.True(t, h.exists("AA-002"), "no other ticket is touched")
	assert.NotContains(t, h.view(), "Delete AA-001", "the prompt is gone")
}

// Any key other than y cancels, and that key does nothing else: it must not
// open a form, quit, or start another delete.
func TestDeleteCancelledByAnyOtherKey(t *testing.T) {
	for _, k := range []string{"n", "q", "d", "e", "esc", "enter"} {
		h := newAppHarness(t, seedTickets("first ticket", "second ticket"))

		h.key("d")
		h.key(k)

		assert.True(t, h.exists("AA-001"), "d then %q", k)
		assert.False(t, h.quit, "d then %q must not quit", k)
		assert.NotContains(t, h.view(), "New Task", "d then %q must not open a form", k)
		assert.NotContains(t, h.view(), "Edit AA-001", "d then %q must not open a form", k)
		assert.NotContains(t, h.view(), "Delete AA-001", "d then %q clears the prompt", k)
	}
}

func TestDeleteKeyWithNoTicketDoesNothing(t *testing.T) {
	h := newAppHarness(t, seedTickets())

	h.key("d")

	assert.NotContains(t, h.view(), "Delete", "there is nothing to confirm")
}
