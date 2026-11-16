package tui

import (
	"fmt"
	"slices"
	"strings"

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

const (
	// sidebarPanelTop is the screen row of the panel's first content line:
	// one app header row plus the panel's top border.
	sidebarPanelTop = 2
	// minListRows is the fewest list rows worth showing. When the footer under
	// a list would leave fewer, the footer is dropped and the list takes its
	// lines.
	minListRows = 3
)

// Sidebar is the left project tree panel.
type Sidebar struct {
	projects   []*domain.Project
	counts     map[string]map[domain.Status]int
	cursor     int
	projScroll int // index of the first project row shown
	focused    bool
	width      int
	height     int

	mode         sidebarMode
	sprints      []domain.SprintSummary
	sprintCursor int // 0 = "All tasks", 1..N = sprint index in sprints slice
	sprintScroll int // index of the first sprint-list row shown
}

// sidebarList is the content of one sidebar mode: heading lines, every row of
// its scrolling list, and a footer drawn under the list when there is room.
type sidebarList struct {
	head   []string
	rows   []string
	foot   []string
	offset int // index of the first row shown
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
	return s.clampScroll()
}

func (s Sidebar) SetProjects(projects []*domain.Project, counts map[string]map[domain.Status]int) Sidebar {
	s.projects = projects
	s.counts = counts
	if s.cursor >= len(projects) {
		s.cursor = max(0, len(projects)-1)
	}
	return s.clampScroll()
}

// SetSprints loads sprint summaries and optionally switches to sprint list mode.
func (s Sidebar) SetSprints(summaries []domain.SprintSummary, enterMode bool) Sidebar {
	s.sprints = summaries
	if enterMode {
		s.mode = modeSprintList
		s.sprintCursor = 0
	}
	return s.clampScroll()
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
		if msg.Action != tea.MouseActionPress {
			return s, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			return s.scrollBy(-1), nil
		case tea.MouseButtonWheelDown:
			return s.scrollBy(1), nil
		case tea.MouseButtonLeft:
			if idx, ok := s.projectAtY(msg.Y); ok {
				s.cursor = idx
				return s, func() tea.Msg { return projectSelectedMsg{s.SelectedProjectID()} }
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
			s = s.clampScroll()
			return s, func() tea.Msg { return projectSelectedMsg{s.SelectedProjectID()} }
		}
	case "down", "j":
		if s.cursor < len(s.projects)-1 {
			s.cursor++
			s = s.clampScroll()
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
	return s.clampScroll(), nil
}

// innerHeight is the number of content lines inside the panel border.
func (s Sidebar) innerHeight() int { return max(s.height-2, 0) }

// window reports which rows of l are on screen (the index of the first, and
// how many) and whether the footer fits under them. One line under the list is
// always kept: it separates the footer and carries the "more below" marker.
//
// Drawing, scrolling and click hit-testing all go through here so they cannot
// disagree about what is visible.
func (s Sidebar) window(l sidebarList) (first, count int, showFoot bool) {
	n := len(l.rows)
	room := s.innerHeight() - len(l.head) - 1
	count, showFoot = min(n, room-len(l.foot)), true
	if count < min(n, minListRows) {
		count, showFoot = max(min(n, room), 0), false
	}
	first = min(max(l.offset, 0), n-count)
	return first, count, showFoot
}

// clampScroll scrolls each list just far enough to keep its cursor on screen.
func (s Sidebar) clampScroll() Sidebar {
	s.projScroll = s.scrollTo(s.projList(), s.cursor)
	s.sprintScroll = s.scrollTo(s.sprintList(), s.sprintCursor)
	return s
}

// scrollTo returns the offset for l that shows row cursor, moving the window
// no further than it has to.
func (s Sidebar) scrollTo(l sidebarList, cursor int) int {
	first, count, _ := s.window(l)
	switch {
	case cursor < first:
		return cursor
	case cursor >= first+count:
		return cursor - count + 1
	}
	return first
}

// scrollBy moves the current list's window by delta rows and leaves its
// cursor where it is, as the mouse wheel does on the board.
func (s Sidebar) scrollBy(delta int) Sidebar {
	if s.mode == modeSprintList {
		l := s.sprintList()
		l.offset += delta
		s.sprintScroll, _, _ = s.window(l)
		return s
	}
	l := s.projList()
	l.offset += delta
	s.projScroll, _, _ = s.window(l)
	return s
}

// projectAtY maps a screen row to the index of the project drawn on it.
func (s Sidebar) projectAtY(y int) (int, bool) {
	if s.mode != modeProjList {
		return 0, false
	}
	l := s.projList()
	first, count, _ := s.window(l)
	row := y - sidebarPanelTop - len(l.head)
	if row < 0 || row >= count {
		return 0, false
	}
	return first + row, true
}

func (s Sidebar) View() string {
	if s.mode == modeSprintList {
		return s.render(s.sprintList())
	}
	return s.render(s.projList())
}

// render draws l inside the panel border: its heading, the window of rows that
// fits, and the footer when there is room.
func (s Sidebar) render(l sidebarList) string {
	first, count, showFoot := s.window(l)

	lines := slices.Clone(l.head)
	if first > 0 {
		lines[len(lines)-1] += styles.Muted.Render(fmt.Sprintf(" ↑ %d", first))
	}
	lines = append(lines, l.rows[first:first+count]...)
	more := ""
	if below := len(l.rows) - first - count; below > 0 {
		more = styles.Muted.Render(fmt.Sprintf("  ↓ %d more", below))
	}
	lines = append(lines, more)
	if showFoot {
		lines = append(lines, l.foot...)
	}

	// lipgloss treats Height as a minimum, and bubbletea drops lines from the
	// top of a view taller than the terminal. Wrap to the panel width and cut
	// to the panel height here, so nothing drawn in the sidebar can push the
	// app header off screen.
	w, h := s.width-2, s.innerHeight()
	body := strings.Split(lipgloss.NewStyle().Width(w).Render(strings.Join(lines, "\n")), "\n")
	body = body[:min(len(body), h)]

	return styles.PanelBorder(s.focused).
		Width(w).
		Height(h).
		Render(strings.Join(body, "\n"))
}

// projList is the project list with the selected project's board summary
// under it.
func (s Sidebar) projList() sidebarList {
	// Available width for the name itself: panel inner width minus the
	// 2-column "▶ " (selected) or "  " (unselected) prefix, minus 2 for
	// the panel border. We use the same budget for both row styles so the
	// truncation column lines up between rows.
	nameWidth := s.width - 4
	if nameWidth < 1 {
		nameWidth = 1
	}
	rows := make([]string, len(s.projects))
	for i, p := range s.projects {
		name := styles.TruncateRune(p.Name, nameWidth)
		if i == s.cursor {
			rows[i] = styles.SelectedRow.Render(fmt.Sprintf("▶ %s", name))
		} else {
			rows[i] = styles.NormalRow.Render(fmt.Sprintf("  %s", name))
		}
	}

	var foot []string
	if len(s.projects) > 0 {
		foot = append(foot, styles.Muted.Render("BOARD"))
		pid := s.SelectedProjectID()
		for _, st := range []domain.Status{
			domain.StatusBacklog, domain.StatusTodo, domain.StatusInProgress,
			domain.StatusReview, domain.StatusDone,
		} {
			n := s.counts[pid][st]
			color := styles.StatusColor(string(st))
			label := lipgloss.NewStyle().Foreground(color).Render(st.Label())
			foot = append(foot, fmt.Sprintf("  %-12s %d", label, n))
		}
		foot = append(foot, "", styles.Muted.Render("↵ open sprints"))
	}

	return sidebarList{
		head:   []string{styles.Muted.Render("PROJECTS")},
		rows:   rows,
		foot:   foot,
		offset: s.projScroll,
	}
}

// sprintList is the selected project's sprint list: "All tasks" then one row
// per sprint.
func (s Sidebar) sprintList() sidebarList {
	inner := s.width - 2

	projName := ""
	if len(s.projects) > 0 {
		projName = styles.TruncateRune(s.projects[s.cursor].Name, inner)
	}

	// "All tasks" row — cursor position 0
	rows := make([]string, 0, len(s.sprints)+1)
	if s.sprintCursor == 0 {
		rows = append(rows, styles.SelectedRow.Render("▶ All tasks"))
	} else {
		rows = append(rows, styles.NormalRow.Render("  All tasks"))
	}

	// Available width for the sprint name itself. Budget breakdown:
	//   2 cols panel border + 2 cols cursor/gutter prefix + 1 col icon
	//   + 1 space + 2 spaces + up to 3 cols for task count = 11 cols overhead.
	// Use s.width - 11 with a minimum of 1.
	sprintNameWidth := s.width - 11
	if sprintNameWidth < 1 {
		sprintNameWidth = 1
	}
	for i, sum := range s.sprints {
		icon, iconStyle := sprintIcon(sum.Sprint.Status)
		name := styles.TruncateRune(sum.Sprint.Name, sprintNameWidth)
		nameStr := fmt.Sprintf("%s %s  %d", icon, name, sum.TaskCount)
		if s.sprintCursor == i+1 {
			rows = append(rows, styles.SelectedRow.Render("▶ "+iconStyle.Render(nameStr)))
		} else {
			rows = append(rows, styles.NormalRow.Render("  "+iconStyle.Render(nameStr)))
		}
	}

	return sidebarList{
		head: []string{
			styles.Muted.Render("◂ PROJECTS"),
			lipgloss.NewStyle().Foreground(styles.CAccent).Bold(true).Render(projName),
			"",
			styles.Muted.Render("SPRINTS"),
		},
		rows: rows,
		// The hint is wider than the panel. Wrap it here so the layout knows
		// how many lines it really takes.
		foot:   strings.Split(styles.Muted.Width(inner).Render("esc back · ↵ select · n new"), "\n"),
		offset: s.sprintScroll,
	}
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
