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
			b.drag.TargetStatus = resolveTargetStatus(msg.Y, b.computeSectionTops())
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

// taskAtY converts a panel-relative Y (1-based, inside border) to a flat cursor index.
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

	row := 1 // start after top border
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

// computeSectionTops returns a map of status → Y position of that section's header,
// using the same layout logic as View().
func (b Board) computeSectionTops() map[domain.Status]int {
	tasksByStatus := make(map[domain.Status][]*domain.Task)
	for _, t := range b.tasks {
		tasksByStatus[t.Status] = append(tasksByStatus[t.Status], t)
	}

	tops := make(map[domain.Status]int)
	row := 1 // start after top border
	for _, st := range statusOrder {
		tops[st] = row
		tsks := tasksByStatus[st]
		row++ // header
		row++ // divider
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
			}
		}
		lines = append(lines, "")
	}

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
