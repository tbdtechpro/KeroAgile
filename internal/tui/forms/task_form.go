package forms

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/tui/styles"
)

type formField int

const (
	fieldTitle formField = iota
	fieldDesc
	fieldAssignee
	fieldPriority
	fieldPoints
	fieldStatus
	fieldLabels
	fieldSprint // inserted between labels and blocks
	fieldBlocks
	fieldBlockedBy
	fieldCount
)

// TaskForm is a modal overlay for creating or editing a task.
type TaskForm struct {
	task      *domain.Task
	projectID string
	users     []*domain.User

	titleInput    textinput.Model
	descInput     textarea.Model
	assigneeIdx   int // 0 = none, 1..len(users) = users[idx-1]
	priorityInput textinput.Model
	pointsInput   textinput.Model
	statusInput   textinput.Model
	labelsInput   textinput.Model
	sprints       []*domain.Sprint
	sprintInput   textinput.Model
	blocksInput   textinput.Model
	blockedByIn   textinput.Model

	focus  formField
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
	SprintID    *int64
	Blocks      []string
	BlockedBy   []string
	IsNew       bool
	TaskID      string
}

// CancelledMsg is emitted when the form is dismissed.
type CancelledMsg struct{}

func New(projectID string, users []*domain.User, defaultID string, task *domain.Task, width, height int, sprints []*domain.Sprint) TaskForm {
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

	// Set assigneeIdx from defaultID for new tasks.
	if defaultID != "" {
		for i, u := range users {
			if u.ID == defaultID {
				f.assigneeIdx = i + 1
				break
			}
		}
	}

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

	f.sprints = sprints
	f.sprintInput = textinput.New()
	f.sprintInput.Placeholder = "sprint ID or name"
	f.sprintInput.Width = 20

	f.blocksInput = textinput.New()
	f.blocksInput.Placeholder = "KA-001, KA-002"
	f.blocksInput.Width = 20

	f.blockedByIn = textinput.New()
	f.blockedByIn.Placeholder = "KA-003"
	f.blockedByIn.Width = 20

	if task != nil {
		f.assigneeIdx = 0 // reset; edit form never inherits defaultID pre-selection
		f.titleInput.SetValue(task.Title)
		f.descInput.SetValue(task.Description)
		if task.AssigneeID != nil {
			for i, u := range users {
				if u.ID == *task.AssigneeID {
					f.assigneeIdx = i + 1
					break
				}
			}
		}
		f.priorityInput.SetValue(string(task.Priority))
		if task.Points != nil {
			f.pointsInput.SetValue(fmt.Sprintf("%d", *task.Points))
		}
		f.statusInput.SetValue(string(task.Status))
		f.labelsInput.SetValue(strings.Join(task.Labels, ", "))
		if task.SprintID != nil {
			for _, sp := range sprints {
				if sp.ID == *task.SprintID {
					f.sprintInput.SetValue(sp.Name)
					break
				}
			}
			if f.sprintInput.Value() == "" {
				f.sprintInput.SetValue(fmt.Sprintf("%d", *task.SprintID))
			}
		}
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
				break
			}
			if errStr := f.validate(); errStr != "" {
				f.err = errStr
				return f, nil
			}
			return f, func() tea.Msg { return f.buildSavedMsg() }
		case "tab":
			f = f.nextField()
			return f, nil
		case "shift+tab":
			f = f.prevField()
			return f, nil
		case "left":
			if f.focus == fieldAssignee {
				f.assigneeIdx = (f.assigneeIdx - 1 + len(f.users) + 1) % (len(f.users) + 1)
				return f, nil
			}
		case "right":
			if f.focus == fieldAssignee {
				f.assigneeIdx = (f.assigneeIdx + 1) % (len(f.users) + 1)
				return f, nil
			}
		}
	}

	var cmd tea.Cmd
	switch f.focus {
	case fieldTitle:
		f.titleInput, cmd = f.titleInput.Update(msg)
	case fieldDesc:
		f.descInput, cmd = f.descInput.Update(msg)
	case fieldPriority:
		f.priorityInput, cmd = f.priorityInput.Update(msg)
	case fieldPoints:
		f.pointsInput, cmd = f.pointsInput.Update(msg)
	case fieldStatus:
		f.statusInput, cmd = f.statusInput.Update(msg)
	case fieldLabels:
		f.labelsInput, cmd = f.labelsInput.Update(msg)
	case fieldSprint:
		f.sprintInput, cmd = f.sprintInput.Update(msg)
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
	f.priorityInput.Blur()
	f.pointsInput.Blur()
	f.statusInput.Blur()
	f.labelsInput.Blur()
	f.sprintInput.Blur()
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
		// cycling only — no textinput to focus
	case fieldPriority:
		f.priorityInput.Focus()
	case fieldPoints:
		f.pointsInput.Focus()
	case fieldStatus:
		f.statusInput.Focus()
	case fieldLabels:
		f.labelsInput.Focus()
	case fieldSprint:
		f.sprintInput.Focus()
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
	switch domain.Priority(strings.TrimSpace(f.priorityInput.Value())) {
	case domain.PriorityLow, domain.PriorityMedium, domain.PriorityHigh, domain.PriorityCritical:
	default:
		return "invalid priority — use: low  medium  high  critical"
	}
	switch domain.Status(strings.TrimSpace(f.statusInput.Value())) {
	case domain.StatusBacklog, domain.StatusTodo, domain.StatusInProgress, domain.StatusReview, domain.StatusDone:
	default:
		return "invalid status — use: backlog  todo  in_progress  review  done"
	}
	if raw := strings.TrimSpace(f.sprintInput.Value()); raw != "" {
		if err := f.resolveSprintID(raw); err != nil {
			return err.Error()
		}
	}
	return ""
}

func (f TaskForm) buildSavedMsg() SavedMsg {
	msg := SavedMsg{
		Title:       strings.TrimSpace(f.titleInput.Value()),
		Description: f.descInput.Value(),
		AssigneeID:  f.selectedAssigneeID(),
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
	if raw := strings.TrimSpace(f.sprintInput.Value()); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
			msg.SprintID = &n
		} else {
			lower := strings.ToLower(raw)
			for _, sp := range f.sprints {
				if strings.ToLower(sp.Name) == lower {
					msg.SprintID = &sp.ID
					break
				}
			}
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

func (f TaskForm) selectedAssigneeID() string {
	if f.assigneeIdx == 0 || f.assigneeIdx > len(f.users) {
		return ""
	}
	return f.users[f.assigneeIdx-1].ID
}

func (f TaskForm) assigneeView() string {
	name := "—"
	if f.assigneeIdx > 0 && f.assigneeIdx <= len(f.users) {
		u := f.users[f.assigneeIdx-1]
		if u.IsAgent {
			name = "🤖 " + u.DisplayName
		} else {
			name = "👤 " + u.DisplayName
		}
	}
	if f.focus == fieldAssignee {
		return lipgloss.NewStyle().Foreground(styles.CAccentLt).Render("‹ " + name + " ›")
	}
	return styles.Muted.Render(name)
}

func (f TaskForm) View() string {
	titleLabel := f.fieldLabel("Title", f.focus == fieldTitle)
	descLabel := f.fieldLabel("Description", f.focus == fieldDesc)
	assigneeLabel := f.fieldLabel("Assignee", f.focus == fieldAssignee)
	priorityLabel := f.fieldLabel("Priority", f.focus == fieldPriority)
	pointsLabel := f.fieldLabel("Points", f.focus == fieldPoints)
	statusLabel := f.fieldLabel("Status", f.focus == fieldStatus)
	labelsLabel := f.fieldLabel("Labels", f.focus == fieldLabels)
	sprintLabel := f.fieldLabel("Sprint", f.focus == fieldSprint)
	blocksLabel := f.fieldLabel("Blocks", f.focus == fieldBlocks)
	blockedByLabel := f.fieldLabel("Blocked by", f.focus == fieldBlockedBy)

	heading := "New Task"
	if f.task != nil {
		heading = "Edit " + f.task.ID
	}

	errLine := ""
	if f.err != "" {
		errLine = "\n" + styles.Danger.Render("✗ "+f.err)
	}

	body := fmt.Sprintf("%s\n%s\n%s\n%s\n\n%s\n%s  %s  %s  %s\n\n%s          %s          %s\n%s  %s  %s\n\n%s\n%s\n%s\n[tab]next  [shift+tab]prev  [←/→]cycle assignee  [enter]save  [esc]cancel%s",
		titleLabel, f.titleInput.View(),
		descLabel, f.descInput.View(),
		assigneeLabel+"  "+priorityLabel+"  "+pointsLabel+"  "+statusLabel,
		f.assigneeView(), f.priorityInput.View(), f.pointsInput.View(), f.statusInput.View(),
		labelsLabel, blocksLabel, blockedByLabel,
		f.labelsInput.View(), f.blocksInput.View(), f.blockedByIn.View(),
		sprintLabel, f.sprintInput.View(),
		styles.Muted.Render("────────────────────────────────────────────────────────────────"),
		errLine,
	)

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.CAccent).
		Padding(1, 2).
		Width(f.width - 10).
		Render(styles.Logo.Render("⬡ "+heading) + "\n" + body)

	return lipgloss.Place(f.width, f.height,
		lipgloss.Center, lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceForeground(styles.CMuted),
	)
}

func (f TaskForm) fieldLabel(text string, active bool) string {
	if active {
		return lipgloss.NewStyle().Foreground(styles.CAccentLt).Bold(true).Render(text)
	}
	return styles.Muted.Render(text)
}

// resolveSprintID resolves the sprint input string to a sprint ID.
// Returns error when the string matches no sprint or matches multiple by name.
func (f TaskForm) resolveSprintID(raw string) error {
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		for _, sp := range f.sprints {
			if sp.ID == n {
				return nil
			}
		}
		return fmt.Errorf("sprint %d not found", n)
	}
	lower := strings.ToLower(raw)
	var matches int
	for _, sp := range f.sprints {
		if strings.ToLower(sp.Name) == lower {
			matches++
		}
	}
	switch matches {
	case 0:
		return fmt.Errorf("unknown sprint")
	case 1:
		return nil
	default:
		return fmt.Errorf("ambiguous sprint name — use ID")
	}
}
