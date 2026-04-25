package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/tui/styles"
)

var statusOrder = []domain.Status{
	domain.StatusBacklog, domain.StatusTodo, domain.StatusInProgress,
	domain.StatusReview, domain.StatusDone,
}

// Board is the middle panel showing all tasks grouped by status.
type Board struct {
	tasks        []*domain.Task // all tasks from last reload
	displayTasks []*domain.Task // filtered subset shown on screen (= tasks when no filter)
	flatIndex    []int          // maps linear cursor to displayTasks slice index
	cursor       int            // linear cursor across all visible tasks
	scrollOffset int            // content lines scrolled past the top of the visible area
	drag         *DragState
	focused      bool
	width        int
	height       int
	panelTop     int    // terminal row where the panel's first content line appears (set by App)
	sprintHeader string // non-empty when filtering by a specific sprint

	filterInput  textinput.Model
	filterActive bool // true while the filter bar is open

	blockerInput  textinput.Model
	blockerActive bool // true while the "block by" input bar is open
}

func NewBoard(tasks []*domain.Task, width, height int) Board {
	fi := textinput.New()
	fi.Placeholder = "title  status:in_progress  priority:high  assignee:claude  label:tui"
	fi.Width = 60

	bi := textinput.New()
	bi.Placeholder = "task ID that blocks this task  (e.g. KA-001)"
	bi.Width = 44
	bi.CharLimit = 20

	b := Board{width: width, height: height, filterInput: fi, blockerInput: bi}
	return b.SetTasks(tasks)
}

func (b Board) SetTasks(tasks []*domain.Task) Board {
	b.tasks = tasks
	return b.rebuildDisplay()
}

// rebuildDisplay filters b.tasks using the current query and rebuilds flatIndex.
func (b Board) rebuildDisplay() Board {
	query := strings.TrimSpace(b.filterInput.Value())
	if !b.filterActive || query == "" {
		b.displayTasks = b.tasks
	} else {
		b.displayTasks = nil
		for _, t := range b.tasks {
			if matchesFilter(t, query) {
				b.displayTasks = append(b.displayTasks, t)
			}
		}
	}

	tasksByStatus := make(map[domain.Status][]*domain.Task)
	for _, t := range b.displayTasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}
	taskPos := make(map[string]int, len(b.displayTasks))
	for i, t := range b.displayTasks {
		taskPos[t.ID] = i
	}
	b.flatIndex = nil
	for _, st := range statusOrder {
		for _, t := range tasksByStatus[st] {
			b.flatIndex = append(b.flatIndex, taskPos[t.ID])
		}
	}
	if b.cursor >= len(b.flatIndex) {
		b.cursor = max(0, len(b.flatIndex)-1)
	}
	return b.clampScroll()
}

// matchesFilter returns true if the task satisfies all space-separated terms in query.
// Each term is either a plain title substring or a "field:value" prefix filter.
func matchesFilter(t *domain.Task, query string) bool {
	for _, part := range strings.Fields(strings.ToLower(query)) {
		if !matchesPart(t, part) {
			return false
		}
	}
	return true
}

func matchesPart(t *domain.Task, part string) bool {
	if val, ok := strings.CutPrefix(part, "status:"); ok {
		return strings.HasPrefix(string(t.Status), val)
	}
	if val, ok := strings.CutPrefix(part, "s:"); ok {
		return strings.HasPrefix(string(t.Status), val)
	}
	if val, ok := strings.CutPrefix(part, "priority:"); ok {
		return strings.HasPrefix(string(t.Priority), val)
	}
	if val, ok := strings.CutPrefix(part, "p:"); ok {
		return strings.HasPrefix(string(t.Priority), val)
	}
	if val, ok := strings.CutPrefix(part, "assignee:"); ok {
		if t.AssigneeID == nil {
			return val == "none"
		}
		return strings.HasPrefix(*t.AssigneeID, val)
	}
	if val, ok := strings.CutPrefix(part, "a:"); ok {
		if t.AssigneeID == nil {
			return val == "none"
		}
		return strings.HasPrefix(*t.AssigneeID, val)
	}
	if val, ok := strings.CutPrefix(part, "label:"); ok {
		for _, lbl := range t.Labels {
			if strings.Contains(strings.ToLower(lbl), val) {
				return true
			}
		}
		return false
	}
	if val, ok := strings.CutPrefix(part, "l:"); ok {
		for _, lbl := range t.Labels {
			if strings.Contains(strings.ToLower(lbl), val) {
				return true
			}
		}
		return false
	}
	// Default: title substring match.
	return strings.Contains(strings.ToLower(t.Title), part)
}

// lineOfCursor returns the 0-based content-line index of the cursor task.
func (b Board) lineOfCursor() int {
	if len(b.flatIndex) == 0 {
		return -1
	}
	tasksByStatus := make(map[domain.Status][]*domain.Task)
	for _, t := range b.displayTasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}
	flatPos := make(map[string]int)
	for fi, idx := range b.flatIndex {
		flatPos[b.displayTasks[idx].ID] = fi
	}
	line := 0
	if b.sprintHeader != "" {
		line++
	}
	if b.filterActive || strings.TrimSpace(b.filterInput.Value()) != "" {
		line++ // filter bar or locked indicator
	}
	if b.blockerActive {
		line++ // blocker input bar
	}
	for _, st := range statusOrder {
		tsks := tasksByStatus[st]
		line++ // section header
		line++ // divider
		if st == domain.StatusDone && len(tsks) > 3 {
			for _, t := range tsks {
				if fi, ok := flatPos[t.ID]; ok && fi == b.cursor {
					return line // point to the collapsed row
				}
			}
			line++
		} else {
			for _, t := range tsks {
				if fi, ok := flatPos[t.ID]; ok && fi == b.cursor {
					return line
				}
				line++
			}
		}
		line++ // blank separator
	}
	return -1
}

// clampScroll adjusts scrollOffset so the cursor task is within the visible window.
func (b Board) clampScroll() Board {
	visible := b.height - 2
	if visible <= 0 {
		return b
	}
	cl := b.lineOfCursor()
	if cl < 0 {
		b.scrollOffset = 0
		return b
	}
	if cl < b.scrollOffset {
		b.scrollOffset = cl
	} else if cl >= b.scrollOffset+visible {
		b.scrollOffset = cl - visible + 1
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

func (b Board) SetPanelTop(row int) Board {
	b.panelTop = row
	return b
}

// SetSprintHeader sets the dim header line shown above the board when a sprint filter is active.
func (b Board) SetSprintHeader(h string) Board {
	b.sprintHeader = h
	return b
}

func (b Board) SelectedTaskID() string {
	if len(b.flatIndex) == 0 {
		return ""
	}
	return b.displayTasks[b.flatIndex[b.cursor]].ID
}

// SetCursorToTask moves the cursor to the task with the given ID (if visible in displayTasks).
func (b Board) SetCursorToTask(id string) Board {
	for fi, idx := range b.flatIndex {
		if b.displayTasks[idx].ID == id {
			b.cursor = fi
			return b.clampScroll()
		}
	}
	return b
}

func (b Board) CountsByStatus() map[domain.Status]int {
	out := make(map[domain.Status]int)
	for _, t := range b.tasks {
		out[t.Status]++
	}
	return out
}

func (b Board) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// When the blocker input is open, feed all keys into it.
	if b.blockerActive {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				b.blockerActive = false
				b.blockerInput.SetValue("")
				b.blockerInput.Blur()
				return b, nil
			case "enter":
				val := strings.TrimSpace(strings.ToUpper(b.blockerInput.Value()))
				b.blockerActive = false
				b.blockerInput.SetValue("")
				b.blockerInput.Blur()
				if val != "" && len(b.flatIndex) > 0 {
					blockedID := b.SelectedTaskID()
					return b, func() tea.Msg { return addBlockerMsg{blockerID: val, blockedID: blockedID} }
				}
				return b, nil
			default:
				var cmd tea.Cmd
				b.blockerInput, cmd = b.blockerInput.Update(msg)
				return b, cmd
			}
		default:
			var cmd tea.Cmd
			b.blockerInput, cmd = b.blockerInput.Update(msg)
			return b, cmd
		}
	}

	// When the filter bar is open, most keys feed the text input.
	if b.filterActive {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				b.filterInput.SetValue("")
				b.filterActive = false
				b.filterInput.Blur()
				b = b.rebuildDisplay()
				return b, nil
			case "enter":
				// Accept the current filter and close the bar (filter stays applied).
				b.filterActive = false
				b.filterInput.Blur()
				return b, nil
			default:
				var cmd tea.Cmd
				b.filterInput, cmd = b.filterInput.Update(msg)
				b = b.rebuildDisplay()
				b = b.clampScroll()
				return b, cmd
			}
		default:
			var cmd tea.Cmd
			b.filterInput, cmd = b.filterInput.Update(msg)
			return b, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "/":
			b.filterActive = true
			b.filterInput.Focus()
			return b, textinput.Blink
		case "b":
			if len(b.flatIndex) > 0 {
				b.blockerActive = true
				b.blockerInput.Focus()
				return b, textinput.Blink
			}
		case "up", "k":
			if b.cursor > 0 {
				b.cursor--
				b = b.clampScroll()
				id := b.SelectedTaskID()
				return b, func() tea.Msg { return taskSelectedMsg{id} }
			}
		case "down", "j":
			if b.cursor < len(b.flatIndex)-1 {
				b.cursor++
				b = b.clampScroll()
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
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if b.scrollOffset > 0 {
				b.scrollOffset--
			}
			return b, nil
		case tea.MouseButtonWheelDown:
			visible := b.height - 2
			totalLines := b.totalContentLines()
			if b.scrollOffset < totalLines-visible {
				b.scrollOffset++
			}
			return b, nil
		case tea.MouseButtonLeft:
			taskIdx := b.taskAtY(msg.Y)
			if taskIdx >= 0 {
				b.cursor = taskIdx
				id := b.SelectedTaskID()
				b.drag = &DragState{
					TaskID:    id,
					TaskTitle: b.displayTasks[b.flatIndex[taskIdx]].Title,
					StartY:    msg.Y,
					CurrentY:  msg.Y,
				}
				return b, func() tea.Msg { return taskSelectedMsg{id} }
			}
		}
	case tea.MouseActionMotion:
		if b.drag.Active() {
			b.drag.CurrentY = msg.Y
			b.drag.TargetStatus = resolveTargetStatus(msg.Y, b.computeSectionTops())
		}
	case tea.MouseActionRelease:
		if b.drag.Active() {
			target := b.drag.TargetStatus
			taskID := b.drag.TaskID
			b.drag = nil
			if target != "" {
				return b, func() tea.Msg {
					return taskMovedMsg{taskID: taskID, status: target}
				}
			}
		}
	}
	return b, nil
}

// taskAtY maps an absolute terminal Y to a flat cursor index.
// Returns -1 if Y doesn't land on a task row.
func (b Board) taskAtY(y int) int {
	tasksByStatus := make(map[domain.Status][]*domain.Task)
	for _, t := range b.displayTasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}
	flatPos := make(map[string]int)
	for fi, idx := range b.flatIndex {
		flatPos[b.displayTasks[idx].ID] = fi
	}

	pt := b.panelTop
	if pt == 0 {
		pt = 2
	}
	row := pt - b.scrollOffset
	if b.sprintHeader != "" {
		row++
	}
	if b.filterActive || strings.TrimSpace(b.filterInput.Value()) != "" {
		row++
	}
	if b.blockerActive {
		row++
	}
	for _, st := range statusOrder {
		tsks := tasksByStatus[st]
		row++ // header line
		row++ // divider line
		if st == domain.StatusDone && len(tsks) > 3 {
			if y == row {
				return -1 // collapsed line, not a selectable task
			}
			row++
		} else {
			for _, t := range tsks {
				if y == row {
					if fi, ok := flatPos[t.ID]; ok {
						return fi
					}
				}
				row++
			}
		}
		row++ // blank line between sections
	}
	return -1
}

// totalContentLines returns the total number of content lines the board would render.
func (b Board) totalContentLines() int {
	tasksByStatus := make(map[domain.Status][]*domain.Task)
	for _, t := range b.displayTasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}
	n := 0
	if b.sprintHeader != "" {
		n++
	}
	if b.filterActive || strings.TrimSpace(b.filterInput.Value()) != "" {
		n++
	}
	if b.blockerActive {
		n++
	}
	for _, st := range statusOrder {
		tsks := tasksByStatus[st]
		n++ // header
		n++ // divider
		if st == domain.StatusDone && len(tsks) > 3 {
			n++ // collapsed
		} else {
			n += len(tsks)
		}
		n++ // blank
	}
	return n
}

// computeSectionTops returns a map of status → terminal Y of that section's header row.
// Used by resolveTargetStatus to snap drag targets to sections.
func (b Board) computeSectionTops() map[domain.Status]int {
	tasksByStatus := make(map[domain.Status][]*domain.Task)
	for _, t := range b.displayTasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}

	pt := b.panelTop
	if pt == 0 {
		pt = 2
	}
	tops := make(map[domain.Status]int)
	row := pt - b.scrollOffset
	if b.sprintHeader != "" {
		row++
	}
	if b.filterActive || strings.TrimSpace(b.filterInput.Value()) != "" {
		row++
	}
	if b.blockerActive {
		row++
	}
	for _, st := range statusOrder {
		tsks := tasksByStatus[st]
		row++          // header line
		tops[st] = row // record at the header line itself (not one row before)
		row++          // divider
		if st == domain.StatusDone && len(tsks) > 3 {
			row++ // collapsed line
		} else {
			row += len(tsks)
		}
		row++ // blank
	}
	return tops
}

func (b Board) Init() tea.Cmd { return nil }

func (b Board) View() string {
	var lines []string

	if b.sprintHeader != "" {
		lines = append(lines, styles.Muted.Render(b.sprintHeader))
	}

	if b.filterActive {
		bar := styles.Muted.Render("/") + " " + b.filterInput.View() +
			"  " + styles.Muted.Render("esc·clear  enter·lock")
		lines = append(lines, bar)
	} else if q := strings.TrimSpace(b.filterInput.Value()); q != "" {
		indicator := styles.Muted.Render("/ ") +
			lipgloss.NewStyle().Foreground(styles.CAccentLt).Render(q) +
			styles.Muted.Render("  / to edit  esc to clear")
		lines = append(lines, indicator)
	}

	if b.blockerActive {
		bar := styles.Danger.Render("⚠ block-by") + "  " + b.blockerInput.View() +
			"  " + styles.Muted.Render("esc·cancel  enter·add")
		lines = append(lines, bar)
	}

	tasksByStatus := make(map[domain.Status][]*domain.Task)
	for _, t := range b.displayTasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}
	flatPos := make(map[string]int)
	for fi, idx := range b.flatIndex {
		flatPos[b.displayTasks[idx].ID] = fi
	}

	for _, st := range statusOrder {
		tsks := tasksByStatus[st]
		color := styles.StatusColor(string(st))
		header := lipgloss.NewStyle().Foreground(color).Bold(true).Render(
			fmt.Sprintf("◆ %s  (%d)", st.Label(), len(tsks)),
		)
		lines = append(lines, header)

		divider := styles.Muted.Render("┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄")
		lines = append(lines, divider)

		if st == domain.StatusDone && len(tsks) > 3 {
			lines = append(lines, styles.Muted.Render(fmt.Sprintf("  %d tasks  ▸ collapsed", len(tsks))))
		} else {
			for _, t := range tsks {
				isCursor := flatPos[t.ID] == b.cursor && b.focused
				isBlocked := len(t.Blockers) > 0

				idStr := styles.Muted.Render(t.ID)
				baseTitle := t.Title
				truncLimit := b.width - 20
				if isBlocked {
					truncLimit -= 2
				}
				if r := []rune(baseTitle); b.width > 23 && len(r) > truncLimit-3 {
					baseTitle = string(r[:truncLimit-3]) + "..."
				}
				if b.drag.Active() && b.drag.TaskID == t.ID {
					baseTitle = "░ " + baseTitle
				}

				row := fmt.Sprintf("  %-28s  %s", baseTitle, idStr)
				var rowLine string
				if isCursor {
					if isBlocked {
						rowLine = styles.Danger.Render("⚠") + styles.SelectedRow.Render("▶"+row)
					} else {
						rowLine = styles.SelectedRow.Render("▶" + row)
					}
				} else {
					if isBlocked {
						rowLine = styles.Danger.Render("⚠") + styles.NormalRow.Render(" "+row)
					} else {
						rowLine = styles.NormalRow.Render(" " + row)
					}
				}
				lines = append(lines, rowLine)
			}
		}
		lines = append(lines, "")
	}

	panelTop := b.panelTop
	if panelTop == 0 {
		panelTop = 2 // default: 1 header row + 1 border row
	}
	visibleLines := b.height - 2
	content := ""
	for i, l := range lines {
		if i < b.scrollOffset {
			continue
		}
		if visibleLines > 0 && i >= b.scrollOffset+visibleLines {
			break
		}
		if b.drag.Active() && i == b.scrollOffset+b.drag.CurrentY-panelTop {
			content += styles.DragGhost.Render(" ⠿ "+b.drag.TaskTitle) + "\n"
			continue
		}
		content += l + "\n"
	}

	panel := styles.PanelBorder(b.focused).
		Width(b.width - 2).
		Height(b.height - 2).
		Render(content)
	return panel
}

// taskMovedMsg is sent when a drag-and-drop completes, requesting a status change.
type taskMovedMsg struct {
	taskID string
	status domain.Status
}
