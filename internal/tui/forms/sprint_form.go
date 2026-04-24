package forms

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tbdtechpro/KeroAgile/internal/tui/styles"
)

type sprintField int

const (
	sfName sprintField = iota
	sfStart
	sfEnd
	sfCount
)

// SprintForm is a modal for creating a new sprint.
type SprintForm struct {
	nameInput  textinput.Model
	startInput textinput.Model
	endInput   textinput.Model

	focus  sprintField
	width  int
	height int
	err    string
}

// SprintSavedMsg is emitted when the sprint form is submitted.
type SprintSavedMsg struct {
	Name  string
	Start *time.Time
	End   *time.Time
}

// SprintCancelledMsg is emitted when the sprint form is dismissed.
type SprintCancelledMsg struct{}

// NewSprintForm creates a ready-to-use sprint creation modal.
func NewSprintForm(width, height int) SprintForm {
	f := SprintForm{width: width, height: height}

	f.nameInput = textinput.New()
	f.nameInput.Placeholder = "Sprint name"
	f.nameInput.Width = 24
	f.nameInput.Focus()

	f.startInput = textinput.New()
	f.startInput.Placeholder = "YYYY-MM-DD"
	f.startInput.Width = 12

	f.endInput = textinput.New()
	f.endInput.Placeholder = "YYYY-MM-DD"
	f.endInput.Width = 12

	return f
}

func (f SprintForm) Init() tea.Cmd { return textinput.Blink }

func (f SprintForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return f, func() tea.Msg { return SprintCancelledMsg{} }
		case "enter":
			if errStr := f.validate(); errStr != "" {
				f.err = errStr
				return f, nil
			}
			saved, err := f.buildSavedMsg()
			if err != nil {
				f.err = err.Error()
				return f, nil
			}
			return f, func() tea.Msg { return saved }
		case "tab":
			f = f.nextField()
			return f, nil
		case "shift+tab":
			f = f.prevField()
			return f, nil
		}
	}

	var cmd tea.Cmd
	switch f.focus {
	case sfName:
		f.nameInput, cmd = f.nameInput.Update(msg)
	case sfStart:
		f.startInput, cmd = f.startInput.Update(msg)
	case sfEnd:
		f.endInput, cmd = f.endInput.Update(msg)
	}
	return f, cmd
}

func (f SprintForm) nextField() SprintForm {
	f.blurAll()
	f.focus = (f.focus + 1) % sfCount
	f.focusCurrent()
	return f
}

func (f SprintForm) prevField() SprintForm {
	f.blurAll()
	if f.focus == 0 {
		f.focus = sfCount - 1
	} else {
		f.focus--
	}
	f.focusCurrent()
	return f
}

func (f *SprintForm) blurAll() {
	f.nameInput.Blur()
	f.startInput.Blur()
	f.endInput.Blur()
}

func (f *SprintForm) focusCurrent() {
	switch f.focus {
	case sfName:
		f.nameInput.Focus()
	case sfStart:
		f.startInput.Focus()
	case sfEnd:
		f.endInput.Focus()
	}
}

func (f SprintForm) validate() string {
	if strings.TrimSpace(f.nameInput.Value()) == "" {
		return "name is required"
	}
	if v := strings.TrimSpace(f.startInput.Value()); v != "" {
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return "start must be YYYY-MM-DD"
		}
	}
	if v := strings.TrimSpace(f.endInput.Value()); v != "" {
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return "end must be YYYY-MM-DD"
		}
	}
	return ""
}

func (f SprintForm) buildSavedMsg() (SprintSavedMsg, error) {
	msg := SprintSavedMsg{Name: strings.TrimSpace(f.nameInput.Value())}
	if v := strings.TrimSpace(f.startInput.Value()); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return msg, err
		}
		msg.Start = &t
	}
	if v := strings.TrimSpace(f.endInput.Value()); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return msg, err
		}
		msg.End = &t
	}
	return msg, nil
}

func (f SprintForm) View() string {
	label := func(text string, active bool) string {
		if active {
			return lipgloss.NewStyle().Foreground(styles.CAccentLt).Bold(true).Render(text)
		}
		return styles.Muted.Render(text)
	}

	errLine := ""
	if f.err != "" {
		errLine = "\n" + styles.Danger.Render("✗ "+f.err)
	}

	body := fmt.Sprintf("%s\n%s\n\n%s  %s\n%s  %s\n%s\n[tab]next  [shift+tab]prev  [enter]save  [esc]cancel%s",
		label("Name", f.focus == sfName), f.nameInput.View(),
		label("Start", f.focus == sfStart), label("End", f.focus == sfEnd),
		f.startInput.View(), f.endInput.View(),
		styles.Muted.Render("────────────────────────────────────"),
		errLine,
	)

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.CAccent).
		Padding(1, 2).
		Width(44).
		Render(styles.Logo.Render("⬡ New Sprint") + "\n" + body)

	return lipgloss.Place(f.width, f.height,
		lipgloss.Center, lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceForeground(styles.CMuted),
	)
}
