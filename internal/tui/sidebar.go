package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/tui/styles"
)

type sidebarMode int

const (
	modeProjList   sidebarMode = iota
	modeSprintList             // drilled into a project's sprint list
)

// Sidebar is the left project tree panel.
type Sidebar struct {
	projects []*domain.Project
	counts   map[string]map[domain.Status]int
	cursor   int
	focused  bool
	width    int
	height   int

	mode         sidebarMode
	sprints      []domain.SprintSummary
	sprintCursor int // 0 = "All tasks", 1..N = sprint index in sprints slice
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

// InSprintListMode reports whether the sidebar is showing the sprint list.
func (s Sidebar) InSprintListMode() bool { return s.mode == modeSprintList }

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

// SetSprints loads sprint summaries and optionally switches to sprint list mode.
func (s Sidebar) SetSprints(summaries []domain.SprintSummary, enterMode bool) Sidebar {
	s.sprints = summaries
	if enterMode {
		s.mode = modeSprintList
		s.sprintCursor = 0
	}
	return s
}

// ExitSprintMode returns to project list mode.
func (s Sidebar) ExitSprintMode() Sidebar {
	s.mode = modeProjList
	return s
}

func (s Sidebar) Init() tea.Cmd { return nil }

func (s Sidebar) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sprintsLoadedMsg:
		s = s.SetSprints(msg.summaries, msg.enterMode)
		return s, nil

	case tea.KeyMsg:
		if s.mode == modeSprintList {
			return s.updateSprintList(msg)
		}
		return s.updateProjList(msg)

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if s.mode == modeProjList {
				idx := msg.Y - 2
				if idx >= 0 && idx < len(s.projects) {
					s.cursor = idx
					return s, func() tea.Msg { return projectSelectedMsg{s.SelectedProjectID()} }
				}
			}
		}
	}
	return s, nil
}

func (s Sidebar) updateProjList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case "enter":
		pid := s.SelectedProjectID()
		if pid != "" {
			return s, func() tea.Msg {
				return reloadTasksMsg{projectID: pid}
			}
		}
	}
	return s, nil
}

func (s Sidebar) updateSprintList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// sprintCursor: 0 = All tasks, 1..N = sprints[sprintCursor-1]
	maxCursor := len(s.sprints)
	switch msg.String() {
	case "up", "k":
		if s.sprintCursor > 0 {
			s.sprintCursor--
		}
	case "down", "j":
		if s.sprintCursor < maxCursor {
			s.sprintCursor++
		}
	case "enter":
		pid := s.SelectedProjectID()
		var sprintID *int64
		if s.sprintCursor > 0 {
			id := s.sprints[s.sprintCursor-1].Sprint.ID
			sprintID = &id
		}
		s.mode = modeProjList
		return s, func() tea.Msg { return sprintSelectedMsg{projectID: pid, sprintID: sprintID} }
	case "esc":
		s.mode = modeProjList
	case "n":
		return s, func() tea.Msg { return openSprintFormMsg{} }
	}
	return s, nil
}

func (s Sidebar) View() string {
	if s.mode == modeSprintList {
		return s.viewSprintList()
	}
	return s.viewProjList()
}

func (s Sidebar) viewProjList() string {
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
		inner += "\n" + styles.Muted.Render("↵ open sprints")
	}

	panel := styles.PanelBorder(s.focused).
		Width(s.width - 2).
		Height(s.height - 2).
		Render(inner)
	return panel
}

func (s Sidebar) viewSprintList() string {
	projName := ""
	if len(s.projects) > 0 {
		projName = s.projects[s.cursor].Name
	}
	inner := styles.Muted.Render("◂ PROJECTS") + "\n"
	inner += lipgloss.NewStyle().Foreground(styles.CAccent).Bold(true).Render(projName) + "\n\n"
	inner += styles.Muted.Render("SPRINTS") + "\n"

	// "All tasks" row — cursor position 0
	allRow := "  All tasks"
	if s.sprintCursor == 0 {
		allRow = styles.SelectedRow.Render("▶ All tasks")
	} else {
		allRow = styles.NormalRow.Render(allRow)
	}
	inner += allRow + "\n"

	for i, sum := range s.sprints {
		icon, iconStyle := sprintIcon(sum.Sprint.Status)
		nameStr := fmt.Sprintf("%s %s  %d", icon, sum.Sprint.Name, sum.TaskCount)
		var row string
		if s.sprintCursor == i+1 {
			row = styles.SelectedRow.Render("▶ " + iconStyle.Render(nameStr))
		} else {
			row = styles.NormalRow.Render("  " + iconStyle.Render(nameStr))
		}
		inner += row + "\n"
	}

	inner += "\n" + styles.Muted.Render("esc back · ↵ select · n new")

	panel := styles.PanelBorder(s.focused).
		Width(s.width - 2).
		Height(s.height - 2).
		Render(inner)
	return panel
}

func sprintIcon(status domain.SprintStatus) (string, lipgloss.Style) {
	switch status {
	case domain.SprintActive:
		return "●", lipgloss.NewStyle().Foreground(styles.CGreen)
	case domain.SprintCompleted:
		return "✓", styles.Muted
	default: // planning
		return "○", styles.Muted
	}
}
