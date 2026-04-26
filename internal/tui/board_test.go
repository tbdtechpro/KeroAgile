package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/tui"
)

func testTasks() []*domain.Task {
	return []*domain.Task{
		{ID: "KA-001", Title: "First", Status: domain.StatusBacklog, Priority: domain.PriorityMedium},
		{ID: "KA-002", Title: "Second", Status: domain.StatusTodo, Priority: domain.PriorityHigh},
	}
}

func TestBoardNav(t *testing.T) {
	b := tui.NewBoard(testTasks(), 60, 30)
	assert.Equal(t, "KA-001", b.SelectedTaskID())

	b2, _ := b.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, "KA-002", b2.(tui.Board).SelectedTaskID())
}

func TestBoardCountsByStatus(t *testing.T) {
	b := tui.NewBoard(testTasks(), 60, 30)
	counts := b.CountsByStatus()
	assert.Equal(t, 1, counts[domain.StatusBacklog])
	assert.Equal(t, 1, counts[domain.StatusTodo])
	assert.Equal(t, 0, counts[domain.StatusInProgress])
}

// TestBoardNarrowWidthNoOverflow verifies that task rows are not word-wrapped at
// narrow widths. The bug: at split-screen widths the board panel is ~21 cols wide,
// falling below the old b.width>23 truncation guard, so the fixed %-28s format
// produced rows far wider than the panel. Lipgloss word-wrapped them across multiple
// terminal lines, causing the scroll logic to undercount and sections to be clipped.
//
// After the fix: the title is truncated to fit the available column, so the task ID
// and its (truncated) title appear on the same terminal line.
func TestBoardNarrowWidthNoOverflow(t *testing.T) {
	longTitle := "A very long task title that definitely exceeds any narrow panel width"
	tasks := []*domain.Task{
		{ID: "KA-001", Title: longTitle, Status: domain.StatusBacklog, Priority: domain.PriorityHigh},
		{ID: "KA-002", Title: longTitle, Status: domain.StatusBacklog, Priority: domain.PriorityMedium,
			Blockers: []string{"KA-001"}}, // blocked: gets the ⚠ prefix, narrower budget
	}

	for _, width := range []int{21, 22, 25} {
		t.Run(fmt.Sprintf("width=%d", width), func(t *testing.T) {
			b := tui.NewBoard(tasks, width, 30)
			view := b.View()

			// After the fix, each task's ID and its truncated title ("...") appear
			// on the same terminal line — lipgloss is not word-wrapping the row.
			for _, id := range []string{"KA-001", "KA-002"} {
				foundOnSameLine := false
				for _, line := range strings.Split(view, "\n") {
					if strings.Contains(line, id) && strings.Contains(line, "...") {
						foundOnSameLine = true
						break
					}
				}
				assert.Truef(t, foundOnSameLine,
					"width=%d: task %s and its truncated title should be on the same line (row is wrapping)", width, id)
			}
		})
	}
}
