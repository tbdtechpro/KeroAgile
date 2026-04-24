package tui

import (
	"fmt"

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
	tasks        []*domain.Task
	flatIndex    []int // maps linear cursor to tasks slice index
	cursor       int   // linear cursor across all visible tasks
	scrollOffset int   // content lines scrolled past the top of the visible area
	drag         *DragState
	focused      bool
	width        int
	height       int
	panelTop     int    // terminal row where the panel's first content line appears (set by App)
	sprintHeader string // non-empty when filtering by a specific sprint
}

func NewBoard(tasks []*domain.Task, width, height int) Board {
	b := Board{width: width, height: height}
	return b.SetTasks(tasks)
}

func (b Board) SetTasks(tasks []*domain.Task) Board {
	b.tasks = tasks

	// Build tasksByStatus so flatIndex matches the visual row order (status groups top to bottom).
	// Without this, ↑/↓ would jump by task ID sequence across sections instead of moving visually.
	tasksByStatus := make(map[domain.Status][]*domain.Task)
	for _, t := range tasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}
	taskPos := make(map[string]int, len(tasks))
	for i, t := range tasks {
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

// lineOfCursor returns the 0-based content-line index of the cursor task.
func (b Board) lineOfCursor() int {
	if len(b.flatIndex) == 0 {
		return -1
	}
	tasksByStatus := make(map[domain.Status][]*domain.Task)
	for _, t := range b.tasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}
	flatPos := make(map[string]int)
	for fi, idx := range b.flatIndex {
		flatPos[b.tasks[idx].ID] = fi
	}
	line := 0
	if b.sprintHeader != "" {
		line++
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
					TaskTitle: b.tasks[b.flatIndex[taskIdx]].Title,
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
	for _, t := range b.tasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}

	flatPos := make(map[string]int) // task ID → flat cursor index
	for fi, idx := range b.flatIndex {
		flatPos[b.tasks[idx].ID] = fi
	}

	pt := b.panelTop
	if pt == 0 {
		pt = 2
	}
	// Subtract scrollOffset so that content line N maps to terminal row pt + (N - scrollOffset).
	row := pt - b.scrollOffset
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
	for _, t := range b.tasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}
	n := 0
	if b.sprintHeader != "" {
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
	for _, t := range b.tasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}

	pt := b.panelTop
	if pt == 0 {
		pt = 2
	}
	tops := make(map[domain.Status]int)
	row := pt - b.scrollOffset
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

	tasksByStatus := make(map[domain.Status][]*domain.Task)
	for _, t := range b.tasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}

	flatPos := make(map[string]int)
	for fi, idx := range b.flatIndex {
		flatPos[b.tasks[idx].ID] = fi
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

				idStr := styles.Muted.Render(t.ID)
				titleStr := t.Title
				if r := []rune(titleStr); b.width > 23 && len(r) > b.width-20 {
					titleStr = string(r[:b.width-23]) + "..."
				}

				if b.drag.Active() && b.drag.TaskID == t.ID {
					titleStr = "░ " + titleStr
				}

				row := fmt.Sprintf("  %-28s  %s", titleStr, idStr)
				if isCursor {
					lines = append(lines, styles.SelectedRow.Render("▶"+row))
				} else {
					lines = append(lines, styles.NormalRow.Render(" "+row))
				}
			}
		}
		lines = append(lines, "")
	}

	panelTop := b.panelTop
	if panelTop == 0 {
		panelTop = 2 // default: 1 header row + 1 border row
	}
	// visibleLines is the content area height; border takes 2 rows from the panel allocation.
	visibleLines := b.height - 2
	content := ""
	for i, l := range lines {
		if i < b.scrollOffset {
			continue
		}
		if visibleLines > 0 && i >= b.scrollOffset+visibleLines {
			break
		}
		// Drag ghost: terminal Y maps to content line scrollOffset+(Y-panelTop).
		if b.drag.Active() && i == b.scrollOffset+b.drag.CurrentY-panelTop {
			content += styles.DragGhost.Render(" ⠿ "+b.drag.TaskTitle) + "\n"
			continue // replace the original row with the ghost, not prepend
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
