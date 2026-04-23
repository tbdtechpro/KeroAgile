package styles

import "github.com/charmbracelet/lipgloss"

// Color palette — extends KeroOle, deeper background
var (
	CAccent   = lipgloss.Color("#7C3AED")
	CAccentLt = lipgloss.Color("#A78BFA")
	CGreen    = lipgloss.Color("#22C55E")
	COrange   = lipgloss.Color("#F97316")
	CYellow   = lipgloss.Color("#EAB308")
	CRed      = lipgloss.Color("#EF4444")
	CMuted    = lipgloss.Color("#6B7280")
	CBg       = lipgloss.Color("#0F172A")
	CWhite    = lipgloss.Color("#F8FAFC")
	CSelected = lipgloss.Color("#1E1B4B") // dark violet for selected row bg
)

// StatusColor maps a status string to its display color.
func StatusColor(status string) lipgloss.Color {
	switch status {
	case "backlog":
		return CYellow
	case "todo":
		return COrange
	case "in_progress":
		return CGreen
	case "review":
		return CAccentLt
	case "done":
		return CMuted
	}
	return CMuted
}

// PriorityColor maps a priority string to its display color.
func PriorityColor(priority string) lipgloss.Color {
	switch priority {
	case "low":
		return CMuted
	case "medium":
		return CYellow
	case "high":
		return COrange
	case "critical":
		return CRed
	}
	return CMuted
}

// PanelBorder returns a focused or unfocused panel border style.
func PanelBorder(focused bool) lipgloss.Style {
	borderColor := CMuted
	if focused {
		borderColor = CAccent
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)
}

// Header is the top title bar style.
var Header = lipgloss.NewStyle().
	Background(CAccent).
	Foreground(CWhite).
	Bold(true).
	Padding(0, 1)

// SectionHeader is used for status group headers inside the board panel.
var SectionHeader = lipgloss.NewStyle().
	Bold(true).
	Padding(0, 1)

// SelectedRow highlights the currently focused task row.
var SelectedRow = lipgloss.NewStyle().
	Background(CSelected).
	Foreground(CAccentLt).
	Bold(true)

// NormalRow is a regular (unfocused) task row.
var NormalRow = lipgloss.NewStyle().
	Foreground(CWhite)

// Muted is secondary text.
var Muted = lipgloss.NewStyle().Foreground(CMuted)

// Success is green text.
var Success = lipgloss.NewStyle().Foreground(CGreen).Bold(true)

// Danger is red text.
var Danger = lipgloss.NewStyle().Foreground(CRed).Bold(true)

// KeyHint renders a key binding hint.
var KeyHint = lipgloss.NewStyle().Foreground(CAccentLt)

// Logo is the app name style.
var Logo = lipgloss.NewStyle().
	Foreground(CAccentLt).
	Bold(true)

// StatusBar is the bottom key hint bar.
var StatusBar = lipgloss.NewStyle().
	Foreground(CMuted).
	Padding(0, 1)

// DragGhost renders the floating task being dragged.
var DragGhost = lipgloss.NewStyle().
	Background(CAccent).
	Foreground(CWhite).
	Bold(true).
	Padding(0, 1)
