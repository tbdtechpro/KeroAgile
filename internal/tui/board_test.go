package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"keroagile/internal/domain"
	"keroagile/internal/tui"
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
