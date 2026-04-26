package forms

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/tui/styles"
)

// BlockerPickedMsg is emitted when the user selects a task from the picker.
type BlockerPickedMsg struct{ ID string }

// BlockerPickerCancelledMsg is emitted when the user presses Esc.
type BlockerPickerCancelledMsg struct{}

// OpenBlockerPickerMsg is emitted by TaskForm to ask App to open the picker.
// Field is "blocks" or "blockedBy".
type OpenBlockerPickerMsg struct{ Field string }

type pickerSearchTickMsg struct{ version int }
type pickerResultsMsg struct {
	results []*domain.TaskSummary
	version int
}

// BlockerPicker is a modal overlay for fuzzy-searching tasks across all projects.
type BlockerPicker struct {
	svc      *domain.Service
	search   textinput.Model
	results  []*domain.TaskSummary
	cursor   int
	debounce int
	width    int
	height   int
}

// NewBlockerPicker creates a ready-to-use picker overlay.
func NewBlockerPicker(svc *domain.Service, width, height int) BlockerPicker {
	ti := textinput.New()
	ti.Placeholder = "Search tasks…"
	ti.Width = width - 16
	if ti.Width < 20 {
		ti.Width = 20
	}
	ti.Focus()
	return BlockerPicker{svc: svc, search: ti, width: width, height: height}
}

func (f BlockerPicker) Init() tea.Cmd { return textinput.Blink }

func (f BlockerPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return f, func() tea.Msg { return BlockerPickerCancelledMsg{} }
		case "enter":
			if len(f.results) > 0 && f.cursor < len(f.results) {
				id := f.results[f.cursor].ID
				return f, func() tea.Msg { return BlockerPickedMsg{ID: id} }
			}
			return f, nil
		case "up":
			if f.cursor > 0 {
				f.cursor--
			}
			return f, nil
		case "down":
			if f.cursor < len(f.results)-1 {
				f.cursor++
			}
			return f, nil
		}
		// All other keys: update text input and schedule debounced search.
		var tiCmd tea.Cmd
		f.search, tiCmd = f.search.Update(msg)
		f.debounce++
		v := f.debounce
		return f, tea.Batch(tiCmd, tea.Tick(300*time.Millisecond, func(_ time.Time) tea.Msg {
			return pickerSearchTickMsg{version: v}
		}))

	case pickerSearchTickMsg:
		if msg.version != f.debounce {
			return f, nil
		}
		q := f.search.Value()
		v := f.debounce
		return f, func() tea.Msg {
			results, _ := f.svc.SearchTasksWithHint(q, 10, "")
			return pickerResultsMsg{results: results, version: v}
		}

	case pickerResultsMsg:
		if msg.version != f.debounce {
			return f, nil
		}
		f.results = msg.results
		if f.cursor >= len(f.results) {
			f.cursor = 0
		}
		return f, nil
	}
	return f, nil
}

func (f BlockerPicker) View() string {
	w := f.width - 8
	if w < 24 {
		w = 24
	}

	titleStyle := lipgloss.NewStyle().Foreground(styles.CAccentLt).Bold(true)
	title := titleStyle.Render("Add blocker — type to search")

	searchLine := f.search.View()

	var rows []string
	for i, ts := range f.results {
		label := fmt.Sprintf("%s · %s", ts.ID, ts.Title)
		if ts.ProjectID != "" {
			label = fmt.Sprintf("[%s] %s", ts.ProjectID, label)
		}
		maxLen := w - 4
		if len(label) > maxLen {
			label = label[:maxLen-1] + "…"
		}
		style := lipgloss.NewStyle().Width(w - 4)
		if i == f.cursor {
			style = style.
				Background(styles.CAccent).
				Foreground(lipgloss.Color("#ffffff"))
		}
		rows = append(rows, style.Render(label))
	}
	if len(rows) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(styles.CMuted).Render("No results"))
	}

	hint := lipgloss.NewStyle().Foreground(styles.CMuted).
		Render("↑↓ navigate · Enter select · Esc cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		searchLine,
		"",
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		"",
		hint,
	)

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.CAccent).
		Padding(1, 2).
		Width(w).
		Render(content)

	return lipgloss.Place(f.width, f.height, lipgloss.Center, lipgloss.Center, modal)
}
