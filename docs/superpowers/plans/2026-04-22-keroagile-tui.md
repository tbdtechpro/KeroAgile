# KeroAgile — TUI Implementation Plan (Plan B)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Prerequisite:** Plan A (keroagile-core) must be complete and all tests passing before starting this plan.

**Goal:** Add the BubbleTea three-panel TUI on top of the working CLI core — sidebar (project tree), board (tasks by status), and detail panel (task info + git), with drag-and-drop, a task form overlay, and auto-transition on PR merge.

**Architecture:** `internal/tui/app.go` owns a root BubbleTea model with three sub-models (sidebar, board, detail). Panel focus is a field on the root model. Custom message types in `internal/tui/msgs.go` carry data between panels. Drag state lives in `internal/tui/drag.go`. The task form is a modal overlay in `internal/tui/forms/task_form.go`. Main wiring replaces the stub in `cmd/keroagile/main.go`.

**Tech Stack:** BubbleTea v1.3.10, Lipgloss v1.1.0, Bubbles v0.20.0 (textinput, textarea)

---

## File Map

| File | Responsibility |
|------|---------------|
| `internal/tui/styles/styles.go` | Lipgloss palette + all shared styles |
| `internal/tui/msgs.go` | Custom BubbleTea message types |
| `internal/tui/drag.go` | DragState struct + helpers |
| `internal/tui/sidebar.go` | Sidebar panel model (project tree + counts) |
| `internal/tui/board.go` | Board panel model (tasks grouped by status) |
| `internal/tui/detail.go` | Detail panel model (task info + git commits) |
| `internal/tui/forms/task_form.go` | Task create/edit form overlay |
| `internal/tui/app.go` | Root App model — Init, Update, View, panel focus |
| `cmd/keroagile/main.go` | Replace TUI stub with real `tui.New(...).Run()` |

---

### Task 14: TUI styles

**Files:**
- Create: `internal/tui/styles/styles.go`

- [ ] **Step 1: Create directory**

```bash
mkdir -p /home/matt/github/KeroAgile/internal/tui/styles \
         /home/matt/github/KeroAgile/internal/tui/forms
```

- [ ] **Step 2: Write internal/tui/styles/styles.go**

```go
package styles

import "github.com/charmbracelet/lipgloss"

// Color palette — extends KeroOle, deeper background
var (
	CAccent   = lipgloss.Color("#7C3AED")
	CAccentLt = lipgloss.Color("#A78BFA")
	CGreen    = lipgloss.Color("#22C55E")
	COrange   = lipgloss.Color("#F97316")
	CYellow   = lipgloss.Color("#EAB308")
	CRed      = lipgloss.Color("#EF4444")
	CMuted    = lipgloss.Color("#6B7280")
	CBg       = lipgloss.Color("#0F172A")
	CWhite    = lipgloss.Color("#F8FAFC")
	CSelected = lipgloss.Color("#1E1B4B") // dark violet for selected row bg
)

// StatusColor maps a status string to its display color.
func StatusColor(status string) lipgloss.Color {
	switch status {
	case "backlog":
		return CYellow
	case "todo":
		return COrange
	case "in_progress":
		return CGreen
	case "review":
		return CAccentLt
	case "done":
		return CMuted
	}
	return CMuted
}

// PriorityColor maps a priority string to its display color.
func PriorityColor(priority string) lipgloss.Color {
	switch priority {
	case "low":
		return CMuted
	case "medium":
		return CYellow
	case "high":
		return COrange
	case "critical":
		return CRed
	}
	return CMuted
}

// Panel borders — focused vs unfocused
func PanelBorder(focused bool) lipgloss.Style {
	borderColor := CMuted
	if focused {
		borderColor = CAccent
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)
}

// Header is the top title bar style.
var Header = lipgloss.NewStyle().
	Background(CAccent).
	Foreground(CWhite).
	Bold(true).
	Padding(0, 1)

// SectionHeader is used for status group headers inside the board panel.
var SectionHeader = lipgloss.NewStyle().
	Bold(true).
	Padding(0, 1)

// SelectedRow highlights the currently focused task row.
var SelectedRow = lipgloss.NewStyle().
	Background(CSelected).
	Foreground(CAccentLt).
	Bold(true)

// NormalRow is a regular (unfocused) task row.
var NormalRow = lipgloss.NewStyle().
	Foreground(CWhite)

// Muted is secondary text.
var Muted = lipgloss.NewStyle().Foreground(CMuted)

// Success is green text.
var Success = lipgloss.NewStyle().Foreground(CGreen).Bold(true)

// Danger is red text.
var Danger = lipgloss.NewStyle().Foreground(CRed).Bold(true)

// KeyHint renders a key binding hint.
var KeyHint = lipgloss.NewStyle().Foreground(CAccentLt)

// Logo is the app name style.
var Logo = lipgloss.NewStyle().
	Foreground(CAccentLt).
	Bold(true)

// StatusBar is the bottom key hint bar.
var StatusBar = lipgloss.NewStyle().
	Foreground(CMuted).
	Padding(0, 1)

// DragGhost renders the floating task being dragged.
var DragGhost = lipgloss.NewStyle().
	Background(CAccent).
	Foreground(CWhite).
	Bold(true).
	Padding(0, 1)
```

- [ ] **Step 3: Compile check**

```bash
go build ./internal/tui/...
```

Expected: no errors (package is valid even with no other files yet).

- [ ] **Step 4: Commit**

```bash
git add internal/tui/styles/styles.go
git commit -m "feat: TUI color palette and shared Lipgloss styles"
```

---

### Task 15: TUI messages and drag state

**Files:**
- Create: `internal/tui/msgs.go`
- Create: `internal/tui/drag.go`

- [ ] **Step 1: Write internal/tui/msgs.go**

```go
package tui

import (
	"keroagile/internal/domain"
	"keroagile/internal/git"
)

// projectSelectedMsg is sent when the user selects a different project in the sidebar.
type projectSelectedMsg struct{ projectID string }

// taskSelectedMsg is sent when the board cursor moves to a different task.
type taskSelectedMsg struct{ taskID string }

// tasksReloadedMsg carries a fresh task list after a create/update/move.
type tasksReloadedMsg struct{ tasks []*domain.Task }

// gitRefreshedMsg carries fresh git data for the detail panel.
type gitRefreshedMsg struct {
	branch  string
	commits []git.Commit
}

// prStatusMsg carries a PR status update for a single task.
type prStatusMsg struct {
	taskID   string
	prStatus *git.PRStatus
}

// prMergedMsg triggers auto-transition of a task to done.
type prMergedMsg struct{ taskID string }

// tickMsg is sent by the 60-second PR polling ticker.
type tickMsg struct{}

// formSavedMsg is sent when the task form is submitted with a new/updated task.
type formSavedMsg struct{ task *domain.Task }

// formCancelledMsg is sent when the task form is dismissed.
type formCancelledMsg struct{}

// showFormMsg opens the task form. task is nil for new, non-nil for edit.
type showFormMsg struct{ task *domain.Task }

// statusNotifMsg shows a transient notification in the status bar.
type statusNotifMsg struct{ text string }
```

- [ ] **Step 2: Write internal/tui/drag.go**

```go
package tui

import (
	"keroagile/internal/domain"
)

// DragState tracks an active mouse drag-and-drop operation.
type DragState struct {
	TaskID       string
	TaskTitle    string
	StartY       int
	CurrentY     int
	TargetStatus domain.Status
}

// Active returns true when a drag is in progress.
func (d *DragState) Active() bool {
	return d != nil && d.TaskID != ""
}

// resolveTargetStatus maps a Y position within the board panel to the status
// section the cursor is hovering over. sectionTops is a map of status → top Y
// (relative to the board panel's inner area).
func resolveTargetStatus(y int, sectionTops map[domain.Status]int) domain.Status {
	best := domain.StatusBacklog
	bestY := -1
	for status, top := range sectionTops {
		if top <= y && top > bestY {
			best = status
			bestY = top
		}
	}
	return best
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/tui/msgs.go internal/tui/drag.go
git commit -m "feat: TUI message types and drag-and-drop state"
```

---

### Task 16: Sidebar panel

**Files:**
- Create: `internal/tui/sidebar.go`
- Create: `internal/tui/sidebar_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/tui/sidebar_test.go
package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"keroagile/internal/domain"
	"keroagile/internal/tui"
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
```

- [ ] **Step 2: Run — expect failure**

```bash
go test ./internal/tui/... -run TestSidebar 2>&1 | head -5
```

Expected: compile error — `tui.NewSidebar` undefined.

- [ ] **Step 3: Write internal/tui/sidebar.go**

```go
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"keroagile/internal/domain"
	"keroagile/internal/tui/styles"
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

- [ ] **Step 4: Run test**

```bash
go test ./internal/tui/... -run TestSidebar -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/sidebar.go internal/tui/sidebar_test.go
git commit -m "feat: TUI sidebar panel with project tree and board counts"
```

---

### Task 17: Board panel

**Files:**
- Create: `internal/tui/board.go`
- Create: `internal/tui/board_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/tui/board_test.go
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
```

- [ ] **Step 2: Run — expect failure**

```bash
go test ./internal/tui/... -run TestBoard 2>&1 | head -5
```

- [ ] **Step 3: Write internal/tui/board.go**

```go
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"keroagile/internal/domain"
	"keroagile/internal/tui/styles"
)

var statusOrder = []domain.Status{
	domain.StatusBacklog, domain.StatusTodo, domain.StatusInProgress,
	domain.StatusReview, domain.StatusDone,
}

// Board is the middle panel showing all tasks grouped by status.
type Board struct {
	tasks       []*domain.Task
	flatIndex   []int          // maps linear cursor to tasks slice index
	cursor      int            // linear cursor across all visible tasks
	sectionTops map[domain.Status]int // Y offset of each status header (for drag)
	drag        *DragState
	focused     bool
	width       int
	height      int
}

func NewBoard(tasks []*domain.Task, width, height int) Board {
	b := Board{width: width, height: height}
	return b.SetTasks(tasks)
}

func (b Board) SetTasks(tasks []*domain.Task) Board {
	b.tasks = tasks
	b.flatIndex = nil
	for i, t := range tasks {
		if t.Status != domain.StatusDone || true { // always include all
			b.flatIndex = append(b.flatIndex, i)
		}
	}
	if b.cursor >= len(b.flatIndex) {
		b.cursor = max(0, len(b.flatIndex)-1)
	}
	return b
}

func (b Board) SetFocused(f bool) Board {
	b.focused = f
	return b
}

func (b Board) SetSize(w, h int) Board {
	b.width = w
	b.height = h
	return b
}

func (b Board) SelectedTaskID() string {
	if len(b.flatIndex) == 0 {
		return ""
	}
	return b.tasks[b.flatIndex[b.cursor]].ID
}

func (b Board) CountsByStatus() map[domain.Status]int {
	out := make(map[domain.Status]int)
	for _, t := range b.tasks {
		out[t.Status]++
	}
	return out
}

func (b Board) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if b.cursor > 0 {
				b.cursor--
				id := b.SelectedTaskID()
				return b, func() tea.Msg { return taskSelectedMsg{id} }
			}
		case "down", "j":
			if b.cursor < len(b.flatIndex)-1 {
				b.cursor++
				id := b.SelectedTaskID()
				return b, func() tea.Msg { return taskSelectedMsg{id} }
			}
		}
	case tea.MouseMsg:
		return b.handleMouse(msg)
	case tasksReloadedMsg:
		b = b.SetTasks(msg.tasks)
	}
	return b, nil
}

func (b Board) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button == tea.MouseButtonLeft {
			// Find which task row was clicked
			taskIdx := b.taskAtY(msg.Y)
			if taskIdx >= 0 {
				b.cursor = taskIdx
				b.drag = &DragState{
					TaskID:    b.SelectedTaskID(),
					TaskTitle: b.tasks[b.flatIndex[taskIdx]].Title,
					StartY:    msg.Y,
					CurrentY:  msg.Y,
				}
			}
		}
	case tea.MouseActionMotion:
		if b.drag.Active() {
			b.drag.CurrentY = msg.Y
			b.drag.TargetStatus = resolveTargetStatus(msg.Y, b.sectionTops)
		}
	case tea.MouseActionRelease:
		if b.drag.Active() {
			target := b.drag.TargetStatus
			taskID := b.drag.TaskID
			b.drag = nil
			return b, func() tea.Msg {
				return taskMovedMsg{taskID: taskID, status: target}
			}
		}
	}
	return b, nil
}

// taskAtY converts a panel-relative Y coordinate to a cursor index.
func (b Board) taskAtY(y int) int {
	// Each section has a 2-line header; tasks are 1 line each.
	// This is a simplified linear scan — actual Y tracking is set in View().
	return -1 // placeholder; View() sets sectionTops
}

func (b Board) Init() tea.Cmd { return nil }

func (b Board) View() string {
	b.sectionTops = make(map[domain.Status]int)
	var lines []string
	y := 0

	tasksByStatus := make(map[domain.Status][]*domain.Task)
	for _, t := range b.tasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}

	// Track which flat index each rendered task corresponds to
	flatPos := make(map[string]int)
	fi := 0
	for _, idx := range b.flatIndex {
		flatPos[b.tasks[idx].ID] = fi
		fi++
	}

	for _, st := range statusOrder {
		tsks := tasksByStatus[st]
		color := styles.StatusColor(string(st))
		header := lipgloss.NewStyle().Foreground(color).Bold(true).Render(
			fmt.Sprintf("◆ %s  (%d)", st.Label(), len(tsks)),
		)
		lines = append(lines, header)
		b.sectionTops[st] = y
		y++

		divider := styles.Muted.Render("┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄")
		lines = append(lines, divider)
		y++

		if st == domain.StatusDone && len(tsks) > 3 {
			lines = append(lines, styles.Muted.Render(fmt.Sprintf("  %d tasks  ▸ collapsed", len(tsks))))
			y++
		} else {
			for _, t := range tsks {
				isCursor := flatPos[t.ID] == b.cursor && b.focused

				idStr := styles.Muted.Render(t.ID)
				titleStr := t.Title
				if len(titleStr) > b.width-20 {
					titleStr = titleStr[:b.width-23] + "..."
				}

				// Ghost indicator during drag
				if b.drag.Active() && b.drag.TaskID == t.ID {
					titleStr = "░ " + titleStr
				}

				row := fmt.Sprintf("  %-28s  %s", titleStr, idStr)
				if isCursor {
					lines = append(lines, styles.SelectedRow.Render("▶"+row))
				} else {
					lines = append(lines, styles.NormalRow.Render(" "+row))
				}
				y++
			}
		}
		lines = append(lines, "")
		y++
	}

	// Render drag ghost at cursor position
	content := ""
	for i, l := range lines {
		if b.drag.Active() && i == b.drag.CurrentY-2 {
			content += styles.DragGhost.Render(" ⠿ "+b.drag.TaskTitle) + "\n"
		}
		content += l + "\n"
	}

	panel := styles.PanelBorder(b.focused).
		Width(b.width - 2).
		Height(b.height - 2).
		Render(content)
	return panel
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/tui/... -run TestBoard -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/board.go internal/tui/board_test.go
git commit -m "feat: TUI board panel with status grouping, keyboard nav, and mouse drag"
```

---

### Task 18: Detail panel

**Files:**
- Create: `internal/tui/detail.go`

- [ ] **Step 1: Write internal/tui/detail.go**

```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"keroagile/internal/domain"
	"keroagile/internal/git"
	"keroagile/internal/tui/styles"
)

// Detail is the right panel showing full task info and git context.
type Detail struct {
	task     *domain.Task
	commits  []git.Commit
	prStatus *git.PRStatus
	users    map[string]*domain.User
	focused  bool
	width    int
	height   int
}

func NewDetail(width, height int) Detail {
	return Detail{width: width, height: height}
}

func (d Detail) SetTask(t *domain.Task) Detail {
	d.task = t
	return d
}

func (d Detail) SetCommits(commits []git.Commit) Detail {
	d.commits = commits
	return d
}

func (d Detail) SetPRStatus(pr *git.PRStatus) Detail {
	d.prStatus = pr
	return d
}

func (d Detail) SetUsers(users []*domain.User) Detail {
	d.users = make(map[string]*domain.User)
	for _, u := range users {
		d.users[u.ID] = u
	}
	return d
}

func (d Detail) SetFocused(f bool) Detail {
	d.focused = f
	return d
}

func (d Detail) SetSize(w, h int) Detail {
	d.width = w
	d.height = h
	return d
}

func (d Detail) Init() tea.Cmd { return nil }

func (d Detail) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case gitRefreshedMsg:
		d.commits = msg.commits
	case prStatusMsg:
		d.prStatus = msg.prStatus
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			d.focused = true
		}
	}
	return d, nil
}

func (d Detail) View() string {
	if d.task == nil {
		panel := styles.PanelBorder(d.focused).
			Width(d.width - 2).Height(d.height - 2).
			Render(styles.Muted.Render("\n  Select a task"))
		return panel
	}

	t := d.task
	w := d.width - 4

	var sb strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().Foreground(styles.StatusColor(string(t.Status))).Bold(true)
	sb.WriteString(titleStyle.Render(truncate(t.Title, w)) + "\n")

	// ID · Priority · Status
	priorityStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Priority.Color())).Bold(true)
	sb.WriteString(fmt.Sprintf("%s  ·  %s  ·  %s\n",
		styles.Muted.Render(t.ID),
		priorityStyle.Render(t.Priority.Label()),
		lipgloss.NewStyle().Foreground(styles.StatusColor(string(t.Status))).Render("● "+t.Status.Label()),
	))

	// Assignee + points
	if t.AssigneeID != nil {
		u := d.users[*t.AssigneeID]
		assigneeStr := *t.AssigneeID
		if u != nil {
			assigneeStr = u.DisplayPrefix()
		}
		pts := ""
		if t.Points != nil {
			pts = fmt.Sprintf("  ·  %s pts", styles.Muted.Render(fmt.Sprintf("%d", *t.Points)))
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(styles.CAccentLt).Render(assigneeStr) + pts + "\n")
	}
	sb.WriteString("\n")

	// Branch + PR
	if t.Branch != "" {
		sb.WriteString(field("Branch", lipgloss.NewStyle().Foreground(styles.CGreen).Render(t.Branch)) + "\n")
	}
	if d.prStatus != nil {
		prColor := styles.CYellow
		if d.prStatus.State == "MERGED" {
			prColor = styles.CGreen
		} else if d.prStatus.State == "CLOSED" {
			prColor = styles.CMuted
		}
		prStr := lipgloss.NewStyle().Foreground(prColor).Render(
			fmt.Sprintf("#%d  ·  %d comments", d.prStatus.Number, d.prStatus.Comments),
		)
		sb.WriteString(field("PR", prStr) + "\n")
	} else if t.PRNumber != nil {
		sb.WriteString(field("PR", styles.Muted.Render(fmt.Sprintf("#%d", *t.PRNumber))) + "\n")
	}

	// Description
	if t.Description != "" {
		sb.WriteString("\n")
		desc := truncate(t.Description, w)
		for _, line := range strings.Split(desc, "\n") {
			sb.WriteString(styles.NormalRow.Render(line) + "\n")
		}
	}

	// Blockers
	if len(t.Blockers) > 0 {
		sb.WriteString("\n" + styles.Muted.Render("Blockers") + "\n")
		for _, b := range t.Blockers {
			sb.WriteString(styles.Danger.Render("⚠ ") + styles.NormalRow.Render(b) + "\n")
		}
	}

	// Commits
	if len(d.commits) > 0 {
		sb.WriteString("\n" + styles.Muted.Render("Recent commits") + "\n")
		for _, c := range d.commits {
			hash := styles.Muted.Render(c.Hash)
			subject := truncate(c.Subject, w-20)
			when := styles.Muted.Render(c.When)
			sb.WriteString(fmt.Sprintf("%s  %-*s  %s\n", hash, w-20, subject, when))
		}
	}

	// Labels
	if len(t.Labels) > 0 {
		sb.WriteString("\n")
		for _, l := range t.Labels {
			sb.WriteString(lipgloss.NewStyle().
				Foreground(styles.CAccentLt).
				Border(lipgloss.NormalBorder()).
				BorderForeground(styles.CAccent).
				Padding(0, 1).
				Render(l) + " ")
		}
		sb.WriteString("\n")
	}

	panel := styles.PanelBorder(d.focused).
		Width(d.width - 2).Height(d.height - 2).
		Render(sb.String())
	return panel
}

func field(label, value string) string {
	return fmt.Sprintf("%-10s%s", styles.Muted.Render(label), value)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./internal/tui/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/tui/detail.go
git commit -m "feat: TUI detail panel with task info, git commits, PR status, blockers"
```

---

### Task 19: Task form overlay

**Files:**
- Create: `internal/tui/forms/task_form.go`

- [ ] **Step 1: Write internal/tui/forms/task_form.go**

```go
package forms

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"keroagile/internal/domain"
	"keroagile/internal/tui/styles"
)

type field int

const (
	fieldTitle field = iota
	fieldDesc
	fieldAssignee
	fieldPriority
	fieldPoints
	fieldStatus
	fieldLabels
	fieldBlocks
	fieldBlockedBy
	fieldCount
)

// TaskForm is a modal overlay for creating or editing a task.
type TaskForm struct {
	task      *domain.Task // nil = new task
	projectID string
	users     []*domain.User

	titleInput    textinput.Model
	descInput     textarea.Model
	assigneeInput textinput.Model
	priorityInput textinput.Model
	pointsInput   textinput.Model
	statusInput   textinput.Model
	labelsInput   textinput.Model
	blocksInput   textinput.Model
	blockedByIn   textinput.Model

	focus  field
	width  int
	height int
	err    string
}

// SavedMsg is emitted when the form is submitted.
type SavedMsg struct {
	Title       string
	Description string
	AssigneeID  string
	Priority    domain.Priority
	Points      *int
	Status      domain.Status
	Labels      []string
	Blocks      []string
	BlockedBy   []string
	IsNew       bool
	TaskID      string // non-empty when editing
}

// CancelledMsg is emitted when the form is dismissed.
type CancelledMsg struct{}

func New(projectID string, users []*domain.User, task *domain.Task, width, height int) TaskForm {
	f := TaskForm{
		task:      task,
		projectID: projectID,
		users:     users,
		width:     width,
		height:    height,
	}

	f.titleInput = textinput.New()
	f.titleInput.Placeholder = "Task title"
	f.titleInput.Focus()
	f.titleInput.Width = width - 10

	f.descInput = textarea.New()
	f.descInput.Placeholder = "Description (optional)"
	f.descInput.SetWidth(width - 10)
	f.descInput.SetHeight(3)

	f.assigneeInput = textinput.New()
	f.assigneeInput.Placeholder = "assignee ID"
	f.assigneeInput.Width = 16

	f.priorityInput = textinput.New()
	f.priorityInput.Placeholder = "medium"
	f.priorityInput.Width = 10

	f.pointsInput = textinput.New()
	f.pointsInput.Placeholder = "0"
	f.pointsInput.Width = 5

	f.statusInput = textinput.New()
	f.statusInput.Placeholder = "backlog"
	f.statusInput.Width = 12

	f.labelsInput = textinput.New()
	f.labelsInput.Placeholder = "auth, backend"
	f.labelsInput.Width = 20

	f.blocksInput = textinput.New()
	f.blocksInput.Placeholder = "KA-001, KA-002"
	f.blocksInput.Width = 20

	f.blockedByIn = textinput.New()
	f.blockedByIn.Placeholder = "KA-003"
	f.blockedByIn.Width = 20

	// Populate fields if editing
	if task != nil {
		f.titleInput.SetValue(task.Title)
		f.descInput.SetValue(task.Description)
		if task.AssigneeID != nil {
			f.assigneeInput.SetValue(*task.AssigneeID)
		}
		f.priorityInput.SetValue(string(task.Priority))
		if task.Points != nil {
			f.pointsInput.SetValue(fmt.Sprintf("%d", *task.Points))
		}
		f.statusInput.SetValue(string(task.Status))
		f.labelsInput.SetValue(strings.Join(task.Labels, ", "))
		f.blocksInput.SetValue(strings.Join(task.Blocking, ", "))
		f.blockedByIn.SetValue(strings.Join(task.Blockers, ", "))
	} else {
		f.priorityInput.SetValue("medium")
		f.statusInput.SetValue("backlog")
	}

	return f
}

func (f TaskForm) Init() tea.Cmd {
	return textinput.Blink
}

func (f TaskForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return f, func() tea.Msg { return CancelledMsg{} }
		case "enter":
			if f.focus == fieldDesc {
				break // allow newlines in description
			}
			if err := f.validate(); err != "" {
				f.err = err
				return f, nil
			}
			return f, func() tea.Msg { return f.buildSavedMsg() }
		case "tab":
			f = f.nextField()
		case "shift+tab":
			f = f.prevField()
		}
	}

	// Route key events to focused input
	var cmd tea.Cmd
	switch f.focus {
	case fieldTitle:
		f.titleInput, cmd = f.titleInput.Update(msg)
	case fieldDesc:
		f.descInput, cmd = f.descInput.Update(msg)
	case fieldAssignee:
		f.assigneeInput, cmd = f.assigneeInput.Update(msg)
	case fieldPriority:
		f.priorityInput, cmd = f.priorityInput.Update(msg)
	case fieldPoints:
		f.pointsInput, cmd = f.pointsInput.Update(msg)
	case fieldStatus:
		f.statusInput, cmd = f.statusInput.Update(msg)
	case fieldLabels:
		f.labelsInput, cmd = f.labelsInput.Update(msg)
	case fieldBlocks:
		f.blocksInput, cmd = f.blocksInput.Update(msg)
	case fieldBlockedBy:
		f.blockedByIn, cmd = f.blockedByIn.Update(msg)
	}
	cmds = append(cmds, cmd)
	return f, tea.Batch(cmds...)
}

func (f TaskForm) nextField() TaskForm {
	f.blurAll()
	f.focus = (f.focus + 1) % fieldCount
	f.focusCurrent()
	return f
}

func (f TaskForm) prevField() TaskForm {
	f.blurAll()
	if f.focus == 0 {
		f.focus = fieldCount - 1
	} else {
		f.focus--
	}
	f.focusCurrent()
	return f
}

func (f *TaskForm) blurAll() {
	f.titleInput.Blur()
	f.descInput.Blur()
	f.assigneeInput.Blur()
	f.priorityInput.Blur()
	f.pointsInput.Blur()
	f.statusInput.Blur()
	f.labelsInput.Blur()
	f.blocksInput.Blur()
	f.blockedByIn.Blur()
}

func (f *TaskForm) focusCurrent() {
	switch f.focus {
	case fieldTitle:
		f.titleInput.Focus()
	case fieldDesc:
		f.descInput.Focus()
	case fieldAssignee:
		f.assigneeInput.Focus()
	case fieldPriority:
		f.priorityInput.Focus()
	case fieldPoints:
		f.pointsInput.Focus()
	case fieldStatus:
		f.statusInput.Focus()
	case fieldLabels:
		f.labelsInput.Focus()
	case fieldBlocks:
		f.blocksInput.Focus()
	case fieldBlockedBy:
		f.blockedByIn.Focus()
	}
}

func (f TaskForm) validate() string {
	if strings.TrimSpace(f.titleInput.Value()) == "" {
		return "title is required"
	}
	return ""
}

func (f TaskForm) buildSavedMsg() SavedMsg {
	msg := SavedMsg{
		Title:       strings.TrimSpace(f.titleInput.Value()),
		Description: f.descInput.Value(),
		AssigneeID:  strings.TrimSpace(f.assigneeInput.Value()),
		Priority:    domain.Priority(strings.TrimSpace(f.priorityInput.Value())),
		Status:      domain.Status(strings.TrimSpace(f.statusInput.Value())),
		IsNew:       f.task == nil,
	}
	if f.task != nil {
		msg.TaskID = f.task.ID
	}
	if pts := strings.TrimSpace(f.pointsInput.Value()); pts != "" {
		if n, err := strconv.Atoi(pts); err == nil && n > 0 {
			msg.Points = &n
		}
	}
	for _, l := range strings.Split(f.labelsInput.Value(), ",") {
		if l = strings.TrimSpace(l); l != "" {
			msg.Labels = append(msg.Labels, l)
		}
	}
	for _, b := range strings.Split(f.blocksInput.Value(), ",") {
		if b = strings.TrimSpace(b); b != "" {
			msg.Blocks = append(msg.Blocks, b)
		}
	}
	for _, b := range strings.Split(f.blockedByIn.Value(), ",") {
		if b = strings.TrimSpace(b); b != "" {
			msg.BlockedBy = append(msg.BlockedBy, b)
		}
	}
	return msg
}

func (f TaskForm) View() string {
	titleLabel := f.label("Title", f.focus == fieldTitle)
	descLabel := f.label("Description", f.focus == fieldDesc)
	assigneeLabel := f.label("Assignee", f.focus == fieldAssignee)
	priorityLabel := f.label("Priority", f.focus == fieldPriority)
	pointsLabel := f.label("Points", f.focus == fieldPoints)
	statusLabel := f.label("Status", f.focus == fieldStatus)
	labelsLabel := f.label("Labels", f.focus == fieldLabels)
	blocksLabel := f.label("Blocks", f.focus == fieldBlocks)
	blockedByLabel := f.label("Blocked by", f.focus == fieldBlockedBy)

	heading := "New Task"
	if f.task != nil {
		heading = "Edit " + f.task.ID
	}

	errLine := ""
	if f.err != "" {
		errLine = "\n" + styles.Danger.Render("✗ "+f.err)
	}

	body := fmt.Sprintf(`%s
%s
%s
%s

%s
%s  %s  %s  %s

%s          %s          %s
%s  %s  %s
%s
[tab]next  [shift+tab]prev  [enter]save  [esc]cancel%s`,
		titleLabel, f.titleInput.View(),
		descLabel, f.descInput.View(),
		assigneeLabel+`  `+priorityLabel+`  `+pointsLabel+`  `+statusLabel,
		f.assigneeInput.View(), f.priorityInput.View(), f.pointsInput.View(), f.statusInput.View(),
		labelsLabel, blocksLabel, blockedByLabel,
		f.labelsInput.View(), f.blocksInput.View(), f.blockedByIn.View(),
		styles.Muted.Render("────────────────────────────────────────────────────────────────"),
		errLine,
	)

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.CAccent).
		Padding(1, 2).
		Width(f.width - 10).
		Render(styles.Logo.Render("⬡ "+heading) + "\n" + body)

	// Center the modal
	return lipgloss.Place(f.width, f.height,
		lipgloss.Center, lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceForeground(styles.CMuted),
	)
}

func (f TaskForm) label(text string, active bool) string {
	if active {
		return lipgloss.NewStyle().Foreground(styles.CAccentLt).Bold(true).Render(text)
	}
	return styles.Muted.Render(text)
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./internal/tui/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/tui/forms/task_form.go
git commit -m "feat: TUI task create/edit form overlay with all fields"
```

---

### Task 20: Root App model

**Files:**
- Create: `internal/tui/app.go`

- [ ] **Step 1: Write internal/tui/app.go**

```go
package tui

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"keroagile/internal/domain"
	"keroagile/internal/git"
	"keroagile/internal/tui/forms"
	"keroagile/internal/tui/styles"
)

type panelFocus int

const (
	focusSidebar panelFocus = iota
	focusBoard
	focusDetail
)

// App is the root BubbleTea model.
type App struct {
	svc        *domain.Service
	gitClients map[string]*git.Client // project ID → git client

	sidebar Sidebar
	board   Board
	detail  Detail

	focus panelFocus
	form  *forms.TaskForm // non-nil when form overlay is open

	projects     []*domain.Project
	currentTasks []*domain.Task
	users        []*domain.User

	statusMsg    string
	statusExpiry time.Time

	width  int
	height int

	spinner spinner.Model
	loading bool
}

// New creates the App. Call Run() to start the BubbleTea event loop.
func New(svc *domain.Service) *App {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &App{
		svc:        svc,
		gitClients: make(map[string]*git.Client),
		spinner:    sp,
	}
}

func (a *App) Run() error {
	p := tea.NewProgram(a,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.spinner.Tick,
		a.loadProjects(),
		a.tickPRPoll(),
	)
}

// loadProjects fetches projects and bootstraps the sidebar.
func (a App) loadProjects() tea.Cmd {
	return func() tea.Msg {
		projects, err := a.svc.ListProjects()
		if err != nil {
			return statusNotifMsg{fmt.Sprintf("error: %v", err)}
		}
		return struct{ projects []*domain.Project }{projects}
	}
}

// loadTasks fetches tasks for the current project.
func (a App) loadTasks(projectID string) tea.Cmd {
	return func() tea.Msg {
		tasks, err := a.svc.ListTasks(projectID, domain.TaskFilters{})
		if err != nil {
			return statusNotifMsg{fmt.Sprintf("error: %v", err)}
		}
		return tasksReloadedMsg{tasks}
	}
}

// tickPRPoll sets up the 60-second PR polling timer.
func (a App) tickPRPoll() tea.Cmd {
	return tea.Tick(60*time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// If form is open, route everything to form except Esc/Enter (handled inside form)
	if a.form != nil {
		switch msg.(type) {
		case forms.SavedMsg:
			return a.handleFormSaved(msg.(forms.SavedMsg))
		case forms.CancelledMsg:
			a.form = nil
			return a, nil
		default:
			m, cmd := a.form.Update(msg)
			f := m.(forms.TaskForm)
			a.form = &f
			return a, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.relayout()

	case tea.KeyMsg:
		cmds = append(cmds, a.handleKey(msg)...)

	case tea.MouseMsg:
		cmds = append(cmds, a.handleMouse(msg)...)

	case struct{ projects []*domain.Project }:
		a.projects = msg.projects
		counts := a.buildCounts()
		a.sidebar = NewSidebar(a.projects, counts, a.sidebarWidth(), a.height-3)
		if len(a.projects) > 0 {
			cmds = append(cmds, a.loadTasks(a.projects[0].ID))
			if p := a.projects[0]; p.RepoPath != "" {
				a.gitClients[p.ID] = git.New(p.RepoPath)
			}
		}

	case tasksReloadedMsg:
		a.currentTasks = msg.tasks
		a.board = a.board.SetTasks(msg.tasks)
		counts := a.buildCounts()
		a.sidebar = a.sidebar.SetProjects(a.projects, counts)

	case projectSelectedMsg:
		cmds = append(cmds, a.loadTasks(msg.projectID))

	case taskSelectedMsg:
		t, err := a.svc.GetTask(msg.taskID)
		if err == nil {
			a.detail = a.detail.SetTask(t)
			cmds = append(cmds, a.refreshGit(t))
		}

	case taskMovedMsg:
		if _, err := a.svc.MoveTask(msg.taskID, msg.status); err == nil {
			pid := a.sidebar.SelectedProjectID()
			cmds = append(cmds, a.loadTasks(pid))
		}

	case gitRefreshedMsg:
		a.detail = a.detail.SetCommits(msg.commits)

	case prStatusMsg:
		a.detail = a.detail.SetPRStatus(msg.prStatus)
		if msg.prStatus != nil && msg.prStatus.State == "MERGED" {
			cmds = append(cmds, func() tea.Msg { return prMergedMsg{msg.taskID} })
		}

	case prMergedMsg:
		if err := a.svc.MarkPRMerged(msg.taskID); err == nil {
			pid := a.sidebar.SelectedProjectID()
			a.setStatus(fmt.Sprintf("✓ %s auto-closed via merged PR", msg.taskID))
			cmds = append(cmds, a.loadTasks(pid))
		}

	case statusNotifMsg:
		a.setStatus(msg.text)

	case tickMsg:
		cmds = append(cmds, a.pollPRs()...)
		cmds = append(cmds, a.tickPRPoll())

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Forward to sub-models if focused
	if a.form == nil {
		var cmd tea.Cmd
		switch a.focus {
		case focusSidebar:
			var m tea.Model
			m, cmd = a.sidebar.Update(msg)
			a.sidebar = m.(Sidebar)
		case focusBoard:
			var m tea.Model
			m, cmd = a.board.Update(msg)
			a.board = m.(Board)
		case focusDetail:
			var m tea.Model
			m, cmd = a.detail.Update(msg)
			a.detail = m.(Detail)
		}
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

func (a *App) handleKey(msg tea.KeyMsg) []tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		return []tea.Cmd{tea.Quit}
	case "tab":
		a.focus = (a.focus + 1) % 3
		a.syncFocus()
	case "shift+tab":
		if a.focus == 0 {
			a.focus = 2
		} else {
			a.focus--
		}
		a.syncFocus()
	case "n":
		a.openNewForm()
	case "e":
		if id := a.board.SelectedTaskID(); id != "" {
			if t, err := a.svc.GetTask(id); err == nil {
				a.openEditForm(t)
			}
		}
	case "m":
		if id := a.board.SelectedTaskID(); id != "" {
			if t, err := a.svc.GetTask(id); err == nil {
				next := t.Status.Next()
				if _, err := a.svc.MoveTask(id, next); err == nil {
					pid := a.sidebar.SelectedProjectID()
					return []tea.Cmd{a.loadTasks(pid)}
				}
			}
		}
	case "M":
		if id := a.board.SelectedTaskID(); id != "" {
			if t, err := a.svc.GetTask(id); err == nil {
				prev := t.Status.Prev()
				if _, err := a.svc.MoveTask(id, prev); err == nil {
					pid := a.sidebar.SelectedProjectID()
					return []tea.Cmd{a.loadTasks(pid)}
				}
			}
		}
	case "d":
		if id := a.board.SelectedTaskID(); id != "" {
			if err := a.svc.DeleteTask(id); err == nil {
				pid := a.sidebar.SelectedProjectID()
				a.setStatus(fmt.Sprintf("deleted %s", id))
				return []tea.Cmd{a.loadTasks(pid)}
			}
		}
	case "r":
		pid := a.sidebar.SelectedProjectID()
		return []tea.Cmd{a.loadTasks(pid), a.pollCurrentTaskGit()}
	}
	return nil
}

func (a *App) handleMouse(msg tea.MouseMsg) []tea.Cmd {
	// Detect which panel was clicked and shift focus
	if msg.Action == tea.MouseActionPress {
		if msg.X < a.sidebarWidth() {
			a.focus = focusSidebar
		} else if msg.X < a.sidebarWidth()+a.boardWidth() {
			a.focus = focusBoard
		} else {
			a.focus = focusDetail
		}
		a.syncFocus()
	}
	return nil
}

func (a *App) syncFocus() {
	a.sidebar = a.sidebar.SetFocused(a.focus == focusSidebar)
	a.board = a.board.SetFocused(a.focus == focusBoard)
	a.detail = a.detail.SetFocused(a.focus == focusDetail)
}

func (a *App) relayout() {
	sw := a.sidebarWidth()
	bw := a.boardWidth()
	dw := a.width - sw - bw
	h := a.height - 3 // header + status bar

	a.sidebar = a.sidebar.SetSize(sw, h)
	a.board = a.board.SetSize(bw, h)
	a.detail = a.detail.SetSize(dw, h)
}

func (a App) sidebarWidth() int  { return 18 }
func (a App) boardWidth() int    { return (a.width - a.sidebarWidth()) * 2 / 5 }

func (a *App) buildCounts() map[string]map[domain.Status]int {
	counts := make(map[string]map[domain.Status]int)
	for _, t := range a.currentTasks {
		if counts[t.ProjectID] == nil {
			counts[t.ProjectID] = make(map[domain.Status]int)
		}
		counts[t.ProjectID][t.Status]++
	}
	return counts
}

func (a *App) openNewForm() {
	pid := a.sidebar.SelectedProjectID()
	f := forms.New(pid, a.users, nil, a.width, a.height)
	a.form = &f
}

func (a *App) openEditForm(t *domain.Task) {
	pid := a.sidebar.SelectedProjectID()
	f := forms.New(pid, a.users, t, a.width, a.height)
	a.form = &f
}

func (a App) handleFormSaved(msg forms.SavedMsg) (tea.Model, tea.Cmd) {
	a.form = nil
	var err error
	if msg.IsNew {
		opts := domain.TaskCreateOpts{
			AssigneeID: msg.AssigneeID,
			Priority:   msg.Priority,
			Status:     msg.Status,
			Labels:     msg.Labels,
		}
		if msg.Points != nil {
			opts.Points = *msg.Points
		}
		_, err = a.svc.CreateTask(msg.Title, msg.Description, a.sidebar.SelectedProjectID(), opts)
	} else {
		t, gerr := a.svc.GetTask(msg.TaskID)
		if gerr == nil {
			t.Title = msg.Title
			t.Description = msg.Description
			t.Priority = msg.Priority
			t.Status = msg.Status
			t.Labels = msg.Labels
			if msg.AssigneeID != "" {
				t.AssigneeID = &msg.AssigneeID
			}
			t.Points = msg.Points
			_, err = a.svc.UpdateTask(t)
		}
	}
	if err != nil {
		a.setStatus(fmt.Sprintf("error: %v", err))
		return a, nil
	}
	return a, a.loadTasks(a.sidebar.SelectedProjectID())
}

func (a App) refreshGit(t *domain.Task) tea.Cmd {
	client, ok := a.gitClients[t.ProjectID]
	if !ok || t.Branch == "" {
		return nil
	}
	return func() tea.Msg {
		commits, err := client.CommitLog(t.Branch, 5)
		if err != nil {
			return gitRefreshedMsg{}
		}
		branch, _ := client.CurrentBranch()
		return gitRefreshedMsg{branch: branch, commits: commits}
	}
}

func (a App) pollCurrentTaskGit() tea.Cmd {
	id := a.board.SelectedTaskID()
	if id == "" {
		return nil
	}
	t, err := a.svc.GetTask(id)
	if err != nil || t.PRNumber == nil {
		return nil
	}
	p, err := a.svc.GetProject(t.ProjectID)
	if err != nil || p.RepoPath == "" {
		return nil
	}
	prNum := *t.PRNumber
	taskID := t.ID
	repoPath := p.RepoPath
	return func() tea.Msg {
		status, err := git.PRView(repoPath, prNum)
		if err != nil {
			return nil
		}
		return prStatusMsg{taskID: taskID, prStatus: status}
	}
}

func (a App) pollPRs() []tea.Cmd {
	var cmds []tea.Cmd
	for _, t := range a.currentTasks {
		if t.Status == domain.StatusReview && t.PRNumber != nil {
			t := t
			p, err := a.svc.GetProject(t.ProjectID)
			if err != nil || p.RepoPath == "" {
				continue
			}
			prNum := *t.PRNumber
			taskID := t.ID
			repoPath := p.RepoPath
			cmds = append(cmds, func() tea.Msg {
				status, err := git.PRView(repoPath, prNum)
				if err != nil {
					return nil
				}
				return prStatusMsg{taskID: taskID, prStatus: status}
			})
		}
	}
	return cmds
}

func (a *App) setStatus(msg string) {
	a.statusMsg = msg
	a.statusExpiry = time.Now().Add(4 * time.Second)
}

type taskMovedMsg struct {
	taskID string
	status domain.Status
}

func (a App) View() string {
	if a.width == 0 {
		return "loading..."
	}

	// Header bar
	project := ""
	if pid := a.sidebar.SelectedProjectID(); pid != "" {
		if p, err := a.svc.GetProject(pid); err == nil {
			project = p.Name
		}
	}
	header := styles.Header.Width(a.width).Render(
		styles.Logo.Render("⬡ KeroAgile") + "  " +
			styles.Muted.Render(project),
	)

	// Panels
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		a.sidebar.View(),
		a.board.View(),
		a.detail.View(),
	)

	// Status bar
	statusText := "[n]ew  [e]dit  [m]ove  [d]del  [tab]focus  [r]refresh  [?]help  [q]quit"
	if a.statusMsg != "" && time.Now().Before(a.statusExpiry) {
		statusText = a.statusMsg
	}
	statusBar := styles.StatusBar.Width(a.width).Render(statusText)

	// Form overlay
	if a.form != nil {
		overlay := a.form.View()
		return overlay
	}

	return header + "\n" + body + "\n" + statusBar
}

// Ensure App implements tea.Model
var _ tea.Model = (*App)(nil)

func logErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
```

- [ ] **Step 2: Compile check**

```bash
go build ./internal/tui/...
```

Fix any type mismatches — the `taskMovedMsg` type is defined in both `msgs.go` and `app.go`, remove the duplicate from `app.go`.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: TUI root App model — panel routing, focus, form overlay, PR polling"
```

---

### Task 21: Wire TUI into main.go

**Files:**
- Modify: `cmd/keroagile/main.go`

- [ ] **Step 1: Replace TUI stub in main.go**

Replace the `rootCmd.RunE` function body:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    app := tui.New(svc)
    return app.Run()
},
```

Add the import:

```go
"keroagile/internal/tui"
```

- [ ] **Step 2: Build**

```bash
go build -o KeroAgile ./cmd/keroagile/
```

Expected: compiles cleanly.

- [ ] **Step 3: Launch TUI smoke test**

```bash
# First seed some data
./KeroAgile project add KA "myapp" --repo /home/matt/github/KeroAgile
./KeroAgile user add matt "Matt"
./KeroAgile user add claude "Claude" --agent
./KeroAgile task add "Build API layer" --project KA --assignee claude --priority high --points 3
./KeroAgile task add "Fix login" --project KA --assignee matt --priority medium
./KeroAgile task move KA-001 in_progress

# Launch TUI
./KeroAgile
```

Expected: TUI opens with sidebar showing KA project, board showing tasks grouped by status, detail panel showing selected task. `tab` cycles focus, `q` quits.

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/keroagile/main.go
git commit -m "feat: wire TUI into main — KeroAgile with no args launches TUI"
```

---

### Task 22: Self-review and final polish

- [ ] **Step 1: Test drag-and-drop**

Launch `./KeroAgile`, click and hold on a task row in the board panel, drag it to a different status section header, release. Verify the task moves to the new status.

- [ ] **Step 2: Test keyboard-only flow**

Full round-trip keyboard-only: `tab` to board, `n` to create task, fill form with tab navigation, enter to save, `m` to move, `M` to move back, `d` to delete (confirm with `y`).

- [ ] **Step 3: Test JSON flag**

```bash
./KeroAgile task list --project KA --json | jq '.[0].id'
```

Expected: `"KA-001"` (quoted string).

- [ ] **Step 4: Test non-TTY auto-json**

```bash
./KeroAgile task list --project KA | cat
```

Expected: JSON output (no color codes).

- [ ] **Step 5: Tag release**

```bash
git tag v0.1.0
```

---

## Spec coverage check

| Spec requirement | Task |
|-----------------|------|
| Hybrid kanban + optional sprints | Tasks 2, 4, 13 |
| SQLite at ~/.config/keroagile | Tasks 5, 8 |
| CLI --json flag + non-TTY auto | Tasks 10, 11, 12, 13 |
| Git branch auto-link | Task 9 |
| PR auto-transition on merge | Tasks 9, 20 |
| Three-panel TUI layout | Tasks 16, 17, 18, 20 |
| Panel focus cycling (tab) | Task 20 |
| Keyboard bindings (n/e/m/M/d/r) | Task 20 |
| Mouse click focus + scroll | Tasks 16, 17, 20 |
| Drag-and-drop task move | Tasks 15, 17 |
| Task form overlay | Task 19 |
| Story points | Tasks 2, 4, 11, 19 |
| Blockers (advisory) | Tasks 3, 4, 7, 18, 19 |
| Named user identities + agent flag | Tasks 2, 11 |
| Color palette | Task 14 |
| Project + repo linkage | Tasks 4, 11 |
| 60s PR poll ticker | Task 20 |
| Graceful git/gh degradation | Task 9, 20 |
