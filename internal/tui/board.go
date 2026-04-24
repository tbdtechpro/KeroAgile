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
	tasks     []*domain.Task
	flatIndex []int // maps linear cursor to tasks slice index
	cursor    int   // linear cursor across all visible tasks
	drag      *DragState
	focused   bool
	width     int
	height    int
	panelTop  int // terminal row where the panel's first content line appears (set by App)
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
	row := pt // align with absolute terminal Y (panelTop = rows before first content line)
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
	row := pt
	for _, st := range statusOrder {
		tsks := tasksByStatus[st]
		row++           // header line
		tops[st] = row  // record at the header line itself (not one row before)
		row++           // divider
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
	content := ""
	for i, l := range lines {
		if b.drag.Active() && i == b.drag.CurrentY-panelTop {
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
