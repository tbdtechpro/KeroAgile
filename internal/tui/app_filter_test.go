package tui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// Reported: Tab to the board and press / or b, and the app exits with a
// nil-pointer panic. Both keys are advertised in the hint bar.
func TestSlashOpensTheFilterBar(t *testing.T) {
	h := newAppHarness(t, seedTickets("alpha one", "beta two"))
	h.key("tab")

	h.key("/")

	assert.Contains(t, h.view(), "enter·lock", "the filter bar is open")
}

func TestBOpensTheBlockByBar(t *testing.T) {
	h := newAppHarness(t, seedTickets("alpha one", "beta two"))
	h.key("tab")

	h.key("b")

	assert.Contains(t, h.view(), "block-by", "the block-by bar is open")
}

func TestFilterNarrowsTheBoardAsYouType(t *testing.T) {
	h := newAppHarness(t, seedTickets("alpha one", "beta two"))
	h.key("tab")
	h.key("/")

	h.typeText("beta")

	assert.Contains(t, h.view(), "beta two")
	assert.NotContains(t, h.view(), "alpha one")
}

// With the filter bar open every key is text. Hidden behind the crash: d
// deleted the selected ticket, n opened New Task, q quit, m moved a ticket.
func TestTypingInTheFilterDoesNotTriggerHotkeys(t *testing.T) {
	h := newAppHarness(t, seedTickets("first ticket", "second ticket"))
	h.key("tab")
	h.key("/")

	h.typeText("dnqemsr")

	assert.False(t, h.quit, "q must not quit")
	assert.NotContains(t, h.view(), "New Task", "n must not open a form")
	assert.NotContains(t, h.view(), "Delete AA-", "d must not start a delete")
	assert.Contains(t, h.view(), "dnqemsr", "every key went into the filter")
	task, err := h.svc.GetTask("AA-001")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusBacklog, task.Status, "m must not move a ticket")
}

func TestTypingInTheBlockByBarDoesNotTriggerHotkeys(t *testing.T) {
	h := newAppHarness(t, seedTickets("first ticket", "second ticket"))
	h.key("tab")
	h.key("b")

	h.typeText("dqn")

	assert.False(t, h.quit, "q must not quit")
	assert.NotContains(t, h.view(), "New Task", "n must not open a form")
	assert.NotContains(t, h.view(), "Delete AA-", "d must not start a delete")
	assert.Contains(t, h.view(), "dqn", "every key went into the block-by input")
}

func TestEnterInTheBlockByBarAddsTheBlocker(t *testing.T) {
	h := newAppHarness(t, seedTickets("first ticket", "second ticket"))
	h.key("tab") // the board cursor starts on AA-001
	h.key("b")

	h.typeText("aa-002")
	h.key("enter")

	task, err := h.svc.GetTask("AA-001")
	require.NoError(t, err)
	assert.Equal(t, []string{"AA-002"}, task.Blockers, "AA-001 is now blocked by AA-002")
	assert.NotContains(t, h.view(), "block-by", "the bar closes")
}

func TestEscClosesTheOpenFilterBarAndClearsIt(t *testing.T) {
	h := newAppHarness(t, seedTickets("alpha one", "beta two"))
	h.key("tab")
	h.key("/")
	h.typeText("beta")

	h.key("esc")

	assert.Contains(t, h.view(), "alpha one", "every ticket is back")
	assert.NotContains(t, h.view(), "enter·lock", "the bar is closed")
}

// Enter locks the filter: the bar closes and the filter stays applied. It used
// to be dropped by the next reload while its indicator stayed on screen.
func TestLockedFilterSurvivesAReload(t *testing.T) {
	h := newAppHarness(t, seedTickets("alpha one", "beta two"))
	h.key("tab")
	h.key("/")
	h.typeText("beta")
	h.key("enter")

	h.key("r") // hotkeys work again once the bar is closed; r reloads

	assert.Contains(t, h.view(), "beta two")
	assert.NotContains(t, h.view(), "alpha one", "the locked filter still applies")
}

// The locked-filter indicator says "esc to clear".
func TestEscClearsALockedFilter(t *testing.T) {
	h := newAppHarness(t, seedTickets("alpha one", "beta two"))
	h.key("tab")
	h.key("/")
	h.typeText("beta")
	h.key("enter")

	h.key("esc")

	assert.Contains(t, h.view(), "alpha one", "every ticket is back")
	assert.NotContains(t, h.view(), "esc to clear", "the indicator is gone")
}
