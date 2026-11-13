package tui_test

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/store"
	"github.com/tbdtechpro/KeroAgile/internal/tui"
)

// appHarness drives the whole TUI the way bubbletea does: it feeds a message
// to Update, runs the commands that come back, and feeds their messages in
// turn. The app runs over a real service and an in-memory database, so tests
// assert on what the user would see and on what ends up stored.
type appHarness struct {
	t    *testing.T
	app  tea.Model
	svc  *domain.Service
	quit bool // the app asked to exit
}

// newAppHarness starts the app in a 120x40 terminal over a board prepared by
// seed.
func newAppHarness(t *testing.T, seed func(svc *domain.Service)) *appHarness {
	t.Helper()
	db, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	svc := domain.NewService(store.New(db))
	seed(svc)

	h := &appHarness{t: t, app: tui.New(svc, "matt"), svc: svc}
	h.run(h.app.Init())
	h.send(tea.WindowSizeMsg{Width: 120, Height: 40})
	return h
}

func (h *appHarness) send(msg tea.Msg) {
	var cmd tea.Cmd
	h.app, cmd = h.app.Update(msg)
	h.run(cmd)
}

// run executes cmd and feeds what it produces back into the app. A command
// that does not return promptly is a timer (cursor blink, PR polling) and is
// left behind.
func (h *appHarness) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	out := make(chan tea.Msg, 1)
	go func() { out <- cmd() }()
	select {
	case msg := <-out:
		switch msg := msg.(type) {
		case nil:
		case tea.BatchMsg:
			for _, c := range msg {
				h.run(c)
			}
		case tea.QuitMsg:
			h.quit = true
		case spinner.TickMsg:
			// Feeding this back re-arms the spinner forever.
		default:
			h.send(msg)
		}
	case <-time.After(50 * time.Millisecond):
	}
}

// key presses one key: "tab", "enter", "esc", "up", "down", or a character.
func (h *appHarness) key(name string) {
	named := map[string]tea.KeyType{
		"tab": tea.KeyTab, "enter": tea.KeyEnter, "esc": tea.KeyEsc,
		"up": tea.KeyUp, "down": tea.KeyDown,
	}
	if k, ok := named[name]; ok {
		h.send(tea.KeyMsg{Type: k})
		return
	}
	h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)})
}

// typeText presses each character of text in turn.
func (h *appHarness) typeText(text string) {
	for _, r := range text {
		h.key(string(r))
	}
}

func (h *appHarness) view() string { return h.app.View() }

// exists reports whether the ticket is still stored.
func (h *appHarness) exists(id string) bool {
	_, err := h.svc.GetTask(id)
	return err == nil
}

// seedTickets returns a seed that creates project AA holding one ticket per
// title: AA-001, AA-002, …
func seedTickets(titles ...string) func(*domain.Service) {
	return func(svc *domain.Service) {
		_ = svc.CreateProject("AA", "Alpha", "")
		for _, title := range titles {
			_, _ = svc.CreateTask(title, "", "AA", domain.TaskCreateOpts{})
		}
	}
}
