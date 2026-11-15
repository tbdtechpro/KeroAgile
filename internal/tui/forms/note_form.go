package forms

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tbdtechpro/KeroAgile/internal/tui/styles"
)

// NoteForm is a modal for adding a note to a ticket. Enter saves; ctrl+j
// starts a new line; esc cancels.
type NoteForm struct {
	taskID string
	input  textarea.Model
	width  int
	height int
	err    string
}

// NoteSavedMsg is emitted when the note form is submitted with some text.
type NoteSavedMsg struct {
	TaskID string
	Body   string
}

// NoteCancelledMsg is emitted when the note form is dismissed.
type NoteCancelledMsg struct{}

// NewNoteForm opens a note form for taskID.
func NewNoteForm(taskID string, width, height int) NoteForm {
	in := textarea.New()
	in.Placeholder = "What was found, decided or tried"
	in.ShowLineNumbers = false
	in.SetWidth(min(60, max(20, width-12)))
	in.SetHeight(5)
	in.CharLimit = 20000
	in.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("ctrl+j"))
	in.Focus()
	return NoteForm{taskID: taskID, input: in, width: width, height: height}
}

func (f NoteForm) Init() tea.Cmd { return textarea.Blink }

func (f NoteForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			return f, func() tea.Msg { return NoteCancelledMsg{} }
		case "enter":
			body := strings.TrimSpace(f.input.Value())
			if body == "" {
				f.err = "the note is empty"
				return f, nil
			}
			id := f.taskID
			return f, func() tea.Msg { return NoteSavedMsg{TaskID: id, Body: body} }
		}
	}
	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)
	f.err = ""
	return f, cmd
}

func (f NoteForm) View() string {
	errLine := ""
	if f.err != "" {
		errLine = "\n" + styles.Danger.Render("✗ "+f.err)
	}
	body := f.input.View() + "\n" + styles.Muted.Render("[enter]save  [ctrl+j]new line  [esc]cancel") + errLine
	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.CAccent).
		Padding(1, 2).
		Render(styles.Logo.Render("⬡ Note on "+f.taskID) + "\n\n" + body)
	return lipgloss.Place(f.width, f.height, lipgloss.Center, lipgloss.Center, modal,
		lipgloss.WithWhitespaceForeground(styles.CMuted))
}
