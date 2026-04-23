package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/tui"
)

func TestSidebarNav(t *testing.T) {
	projects := []*domain.Project{
		{ID: "KA", Name: "myapp"},
		{ID: "BE", Name: "backend"},
	}
	counts := map[string]map[domain.Status]int{
		"KA": {domain.StatusBacklog: 2, domain.StatusTodo: 1},
		"BE": {},
	}
	m := tui.NewSidebar(projects, counts, 20, 30)

	// Initial selection is first project
	assert.Equal(t, "KA", m.SelectedProjectID())

	// Down moves cursor
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	sb := m2.(tui.Sidebar)
	assert.Equal(t, "BE", sb.SelectedProjectID())

	// Up wraps back
	m3, _ := sb.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, "KA", m3.(tui.Sidebar).SelectedProjectID())
}
