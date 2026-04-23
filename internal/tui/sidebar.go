package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/tui/styles"
)

// Sidebar is the left project tree panel.
type Sidebar struct {
	projects []*domain.Project
	counts   map[string]map[domain.Status]int
	cursor   int
	focused  bool
	width    int
	height   int
}

func NewSidebar(projects []*domain.Project, counts map[string]map[domain.Status]int, width, height int) Sidebar {
	return Sidebar{projects: projects, counts: counts, width: width, height: height}
}

func (s Sidebar) SelectedProjectID() string {
	if len(s.projects) == 0 {
		return ""
	}
	return s.projects[s.cursor].ID
}

func (s Sidebar) SetFocused(f bool) Sidebar {
	s.focused = f
	return s
}

func (s Sidebar) SetSize(w, h int) Sidebar {
	s.width = w
	s.height = h
	return s
}

func (s Sidebar) SetProjects(projects []*domain.Project, counts map[string]map[domain.Status]int) Sidebar {
	s.projects = projects
	s.counts = counts
	if s.cursor >= len(projects) {
		s.cursor = max(0, len(projects)-1)
	}
	return s
}

func (s Sidebar) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
				return s, func() tea.Msg { return projectSelectedMsg{s.SelectedProjectID()} }
			}
		case "down", "j":
			if s.cursor < len(s.projects)-1 {
				s.cursor++
				return s, func() tea.Msg { return projectSelectedMsg{s.SelectedProjectID()} }
			}
		}
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// Map click Y to project index (each project = 1 line, after 2-line header)
			idx := msg.Y - 2
			if idx >= 0 && idx < len(s.projects) {
				s.cursor = idx
				return s, func() tea.Msg { return projectSelectedMsg{s.SelectedProjectID()} }
			}
		}
	}
	return s, nil
}

func (s Sidebar) Init() tea.Cmd { return nil }

func (s Sidebar) View() string {
	inner := styles.Muted.Render("PROJECTS") + "\n"

	for i, p := range s.projects {
		row := fmt.Sprintf("  %s", p.Name)
		if i == s.cursor {
			row = styles.SelectedRow.Render(fmt.Sprintf("▶ %s", p.Name))
		} else {
			row = styles.NormalRow.Render(row)
		}
		inner += row + "\n"
	}

	if len(s.projects) > 0 {
		inner += "\n" + styles.Muted.Render("BOARD") + "\n"
		pid := s.SelectedProjectID()
		for _, st := range []domain.Status{
			domain.StatusBacklog, domain.StatusTodo, domain.StatusInProgress,
			domain.StatusReview, domain.StatusDone,
		} {
			n := s.counts[pid][st]
			color := styles.StatusColor(string(st))
			label := lipgloss.NewStyle().Foreground(color).Render(st.Label())
			inner += fmt.Sprintf("  %-12s %d\n", label, n)
		}
	}

	panel := styles.PanelBorder(s.focused).
		Width(s.width - 2).
		Height(s.height - 2).
		Render(inner)
	return panel
}
