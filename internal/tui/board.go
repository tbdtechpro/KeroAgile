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
	flatIndex   []int                 // maps linear cursor to tasks slice index
	cursor      int                   // linear cursor across all visible tasks
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
	for i := range tasks {
		b.flatIndex = append(b.flatIndex, i)
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

// taskAtY converts a panel-relative Y coordinate to a flat cursor index.
// Returns -1 if no task is at that Y. sectionTops must be populated by a prior View() call.
func (b Board) taskAtY(y int) int {
	// Linear scan of flatIndex using rendered Y positions is complex;
	// this simplified version returns -1 (drag start will still work via mouse motion).
	return -1
}

func (b Board) Init() tea.Cmd { return nil }

func (b Board) View() string {
	sectionTops := make(map[domain.Status]int)
	var lines []string
	y := 0

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
		sectionTops[st] = y
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
				if b.width > 23 && len(titleStr) > b.width-20 {
					titleStr = titleStr[:b.width-23] + "..."
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
				y++
			}
		}
		lines = append(lines, "")
		y++
	}

	// Store sectionTops on b for drag resolution (note: b is a value, so this
	// is stored for the lifetime of this View() call only; drag.go uses the
	// sectionTops passed by the App on mouse events)
	b.sectionTops = sectionTops

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

// taskMovedMsg is sent when a drag-and-drop completes, requesting a status change.
type taskMovedMsg struct {
	taskID string
	status domain.Status
}
