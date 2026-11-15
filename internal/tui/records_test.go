package tui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/tui"
)

func ticketWithNotes(n int) *domain.Task {
	t := &domain.Task{ID: "AA-001", ProjectID: "AA", Title: "a ticket", Status: domain.StatusTodo, Priority: domain.PriorityMedium}
	for i := 1; i <= n; i++ {
		t.Notes = append(t.Notes, &domain.Note{TaskID: "AA-001", AuthorID: "matt", Body: fmt.Sprintf("note number %d", i),
			CreatedAt: time.Date(2026, 11, 15, 10, i, 0, 0, time.UTC)})
	}
	return t
}

func TestDetail_ShowsNotes(t *testing.T) {
	d := tui.NewDetail(70, 30).SetTask(ticketWithNotes(2))
	view := d.View()
	assert.Contains(t, view, "Notes")
	assert.Contains(t, view, "note number 1")
	assert.Contains(t, view, "note number 2")
	assert.Contains(t, view, "matt")
}

// A panel taller than its height pushes the top of the screen off (see
// CLAUDE.md). A ticket with many notes must still fit.
func TestDetail_ClipsToHeight(t *testing.T) {
	d := tui.NewDetail(70, 20).SetTask(ticketWithNotes(30))
	view := d.View()
	assert.Equal(t, 20, lipgloss.Height(view))
	assert.Contains(t, view, "more")
}

func TestApp_AddNoteKey(t *testing.T) {
	h := newAppHarness(t, func(svc *domain.Service) {
		seedTickets("first ticket")(svc)
		_, _ = svc.CreateUser("matt", "Matt", false)
	})
	h.key("a")
	assert.Contains(t, h.view(), "Note on AA-001")
	h.typeText("found the cause")
	h.key("enter")

	got := h.ticket("AA-001")
	require.Len(t, got.Notes, 1)
	assert.Equal(t, "found the cause", got.Notes[0].Body)
	assert.Equal(t, "matt", got.Notes[0].AuthorID)
	assert.True(t, strings.Contains(h.view(), "found the cause"), "the detail panel shows the new note")
}

func TestApp_AddNoteEscCancels(t *testing.T) {
	h := newAppHarness(t, seedTickets("first ticket"))
	h.key("a")
	h.typeText("never mind")
	h.key("esc")
	assert.Empty(t, h.ticket("AA-001").Notes)
	assert.NotContains(t, h.view(), "Note on AA-001")
}

// The TUI's user comes from the config and may not be on the board; the note
// is kept, unsigned, rather than lost.
func TestApp_AddNoteWithUnknownUserIsUnsigned(t *testing.T) {
	h := newAppHarness(t, seedTickets("first ticket"))
	h.key("a")
	h.typeText("kept anyway")
	h.key("enter")
	got := h.ticket("AA-001")
	require.Len(t, got.Notes, 1)
	assert.Equal(t, "", got.Notes[0].AuthorID)
}
