package tui_test

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/tui"
)

// Screen rows of the sidebar in the full app: row 0 is the app header, row 1
// the panel's top border, row 2 the "PROJECTS" heading, row 3 the first
// project. Sidebar width 18 is what the app uses.

// sidebarWith builds a sidebar holding n projects: P000 "proj-000", P001 …
func sidebarWith(n, height int) tui.Sidebar {
	projects := make([]*domain.Project, n)
	counts := make(map[string]map[domain.Status]int, n)
	for i := range projects {
		id := fmt.Sprintf("P%03d", i)
		projects[i] = &domain.Project{ID: id, Name: fmt.Sprintf("proj-%03d", i)}
		counts[id] = map[domain.Status]int{domain.StatusBacklog: i}
	}
	return tui.NewSidebar(projects, counts, 18, height)
}

// sprintSummaries builds n sprints named s000, s001, …
func sprintSummaries(n int) []domain.SprintSummary {
	out := make([]domain.SprintSummary, n)
	for i := range out {
		out[i] = domain.SprintSummary{
			Sprint:    &domain.Sprint{ID: int64(i + 1), Name: fmt.Sprintf("s%03d", i)},
			TaskCount: i,
		}
	}
	return out
}

func press(s tui.Sidebar, key tea.KeyType, times int) tui.Sidebar {
	for range times {
		m, _ := s.Update(tea.KeyMsg{Type: key})
		s = m.(tui.Sidebar)
	}
	return s
}

func mouse(s tui.Sidebar, button tea.MouseButton, y int) (tui.Sidebar, tea.Cmd) {
	m, cmd := s.Update(tea.MouseMsg{X: 4, Y: y, Action: tea.MouseActionPress, Button: button})
	return m.(tui.Sidebar), cmd
}

// The reported bug: with more projects than rows the panel grew past its
// height, the app became taller than the terminal, and bubbletea dropped the
// top of the screen.
func TestSidebarProjectListIsExactlyPanelHeight(t *testing.T) {
	for _, n := range []int{0, 1, 5, 41, 200} {
		for _, h := range []int{4, 8, 12, 15, 20, 47} {
			got := lipgloss.Height(sidebarWith(n, h).View())
			assert.Equal(t, h, got, "%d projects in a panel %d rows high", n, h)
		}
	}
}

func TestSidebarSprintListIsExactlyPanelHeight(t *testing.T) {
	for _, n := range []int{0, 3, 60} {
		for _, h := range []int{4, 8, 12, 20, 47} {
			s := sidebarWith(1, h).SetSprints(sprintSummaries(n), true)
			assert.Equal(t, h, lipgloss.Height(s.View()), "%d sprints in a panel %d rows high", n, h)
		}
	}
}

func TestSidebarScrollsDownToKeepSelectedProjectVisible(t *testing.T) {
	s := press(sidebarWith(41, 20), tea.KeyDown, 40)
	require.Equal(t, "P040", s.SelectedProjectID())

	out := s.View()
	assert.Contains(t, out, "▶ proj-040", "the selected project is drawn")
	assert.NotContains(t, out, "proj-000", "the first project has scrolled off")
	assert.Contains(t, out, "PROJECTS", "the heading stays while the list scrolls")
	assert.Contains(t, out, "BOARD", "the board summary stays while the list scrolls")
}

func TestSidebarScrollsBackUpToKeepSelectedProjectVisible(t *testing.T) {
	s := press(sidebarWith(41, 20), tea.KeyDown, 40)
	s = press(s, tea.KeyUp, 40)
	require.Equal(t, "P000", s.SelectedProjectID())

	out := s.View()
	assert.Contains(t, out, "▶ proj-000")
	assert.NotContains(t, out, "proj-040")
}

// A panel 20 rows high has 18 lines inside its border. The heading, the line
// under the list and the 8-line board summary take 10, leaving 8 project rows
// for 41 projects.
func TestSidebarShowsHowManyProjectsAreHidden(t *testing.T) {
	s := sidebarWith(41, 20)

	atTop := s.View()
	assert.Contains(t, atTop, "↓ 33 more")
	assert.NotContains(t, atTop, "↑")

	inMiddle := press(s, tea.KeyDown, 20).View() // rows 13..20 shown
	assert.Contains(t, inMiddle, "↑ 13")
	assert.Contains(t, inMiddle, "↓ 20 more")

	atBottom := press(s, tea.KeyDown, 40).View() // rows 33..40 shown
	assert.Contains(t, atBottom, "↑ 33")
	assert.NotContains(t, atBottom, "↓")
}

func TestSidebarShowsNoScrollMarkersWhenEverythingFits(t *testing.T) {
	out := sidebarWith(5, 30).View()
	assert.NotContains(t, out, "↑")
	assert.NotContains(t, out, "↓")
}

func TestSidebarClickSelectsTheProjectUnderThePointer(t *testing.T) {
	s, cmd := mouse(sidebarWith(5, 30), tea.MouseButtonLeft, 5) // third project row
	assert.Equal(t, "P002", s.SelectedProjectID())
	assert.NotNil(t, cmd, "selecting a project asks the app to load its tasks")
}

func TestSidebarClickOnHeadingSelectsNothing(t *testing.T) {
	s := press(sidebarWith(5, 30), tea.KeyDown, 2)
	s, cmd := mouse(s, tea.MouseButtonLeft, 2)
	assert.Equal(t, "P002", s.SelectedProjectID())
	assert.Nil(t, cmd)
}

func TestSidebarClickBelowTheListSelectsNothing(t *testing.T) {
	// 8 project rows occupy screen rows 3..10; row 12 is the board summary.
	s, cmd := mouse(sidebarWith(41, 20), tea.MouseButtonLeft, 12)
	assert.Equal(t, "P000", s.SelectedProjectID())
	assert.Nil(t, cmd)
}

func TestSidebarClickAccountsForScroll(t *testing.T) {
	s := press(sidebarWith(41, 20), tea.KeyDown, 40) // rows 33..40 shown
	s, _ = mouse(s, tea.MouseButtonLeft, 3)          // first visible row
	assert.Equal(t, "P033", s.SelectedProjectID())
}

func TestSidebarWheelScrollsListWithoutChangingSelection(t *testing.T) {
	s := sidebarWith(41, 20)
	for range 3 {
		s, _ = mouse(s, tea.MouseButtonWheelDown, 5)
	}

	out := s.View()
	assert.Equal(t, "P000", s.SelectedProjectID())
	assert.Contains(t, out, "↑ 3")
	assert.NotContains(t, out, "proj-002")
	assert.Contains(t, out, "proj-003")
}

func TestSidebarWheelStopsAtTheEndsOfTheList(t *testing.T) {
	s, _ := mouse(sidebarWith(41, 20), tea.MouseButtonWheelUp, 5)
	assert.Contains(t, s.View(), "proj-000", "wheel up at the top stays at the top")

	for range 100 {
		s, _ = mouse(s, tea.MouseButtonWheelDown, 5)
	}
	out := s.View()
	assert.Contains(t, out, "↑ 33")
	assert.Contains(t, out, "proj-040")
	assert.NotContains(t, out, "↓")
}

// A panel 10 rows high has 8 lines inside its border: not enough for the
// 8-line board summary and a usable list, so the list takes the space. The
// heading and the line under the list leave 6 rows.
func TestSidebarDropsBoardSummaryWhenPanelIsTooShort(t *testing.T) {
	out := sidebarWith(41, 10).View()
	assert.NotContains(t, out, "BOARD")
	assert.Contains(t, out, "▶ proj-000")
	assert.Contains(t, out, "↓ 35 more")
}

// The summary comes back as soon as it leaves three project rows. 15 rows
// high is 13 inner lines: heading, 3 projects, the line under the list, and
// the 8-line summary. One row shorter would leave 2, so the list keeps all 10
// lines under the heading and marker.
func TestSidebarKeepsBoardSummaryOnceThreeProjectRowsFit(t *testing.T) {
	justFits := sidebarWith(41, 15).View()
	assert.Contains(t, justFits, "BOARD")
	assert.Contains(t, justFits, "↓ 38 more")

	oneRowShorter := sidebarWith(41, 14).View()
	assert.NotContains(t, oneRowShorter, "BOARD")
	assert.Contains(t, oneRowShorter, "↓ 31 more")
}

func TestSidebarSprintListScrollsToKeepSelectionVisible(t *testing.T) {
	s := sidebarWith(1, 20).SetSprints(sprintSummaries(60), true)
	s = press(s, tea.KeyDown, 60) // row 0 is "All tasks", so row 60 is the last sprint

	out := s.View()
	assert.Contains(t, out, "▶ ○ s059", "the selected sprint is drawn")
	assert.NotContains(t, out, "All tasks", "the first row has scrolled off")
	assert.Contains(t, out, "SPRINTS", "the heading stays while the list scrolls")
}

// The project name above the sprint list used to wrap onto a second line when
// it was wider than the panel, adding a row the layout did not account for.
func TestSidebarSprintHeadingTruncatesLongProjectName(t *testing.T) {
	projects := []*domain.Project{{ID: "LP", Name: "An Extremely Long Project Name"}}
	counts := map[string]map[domain.Status]int{"LP": {}}
	s := tui.NewSidebar(projects, counts, 18, 20).SetSprints(nil, true)

	out := s.View()
	assert.Contains(t, out, "An Extremely Lo…")
	assert.NotContains(t, out, "Name", "the tail of the name is only drawn if it wrapped")
}
