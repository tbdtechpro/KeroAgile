package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/git"
	"github.com/tbdtechpro/KeroAgile/internal/tui/styles"
)

// Detail is the right panel showing full task info and git context.
type Detail struct {
	task     *domain.Task
	commits  []git.Commit
	prStatus *git.PRStatus
	users    map[string]*domain.User
	focused  bool
	width    int
	height   int
}

func NewDetail(width, height int) Detail {
	return Detail{width: width, height: height}
}

func (d Detail) SetTask(t *domain.Task) Detail {
	d.task = t
	return d
}

func (d Detail) SetCommits(commits []git.Commit) Detail {
	d.commits = commits
	return d
}

func (d Detail) SetPRStatus(pr *git.PRStatus) Detail {
	d.prStatus = pr
	return d
}

func (d Detail) SetUsers(users []*domain.User) Detail {
	d.users = make(map[string]*domain.User)
	for _, u := range users {
		d.users[u.ID] = u
	}
	return d
}

func (d Detail) SetFocused(f bool) Detail {
	d.focused = f
	return d
}

func (d Detail) SetSize(w, h int) Detail {
	d.width = w
	d.height = h
	return d
}

func (d Detail) Init() tea.Cmd { return nil }

func (d Detail) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case gitRefreshedMsg:
		d.commits = msg.commits
	case prStatusMsg:
		d.prStatus = msg.prStatus
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			d.focused = true
		}
	case tea.KeyMsg:
		if d.focused && msg.String() == "right" {
			if d.task != nil && len(d.task.Blockers) > 0 {
				target := d.task.Blockers[0]
				return d, func() tea.Msg { return jumpToTaskMsg{taskID: target} }
			}
		}
	}
	return d, nil
}

func (d Detail) View() string {
	if d.task == nil {
		panel := styles.PanelBorder(d.focused).
			Width(d.width - 2).Height(d.height - 2).
			Render(styles.Muted.Render("\n  Select a task"))
		return panel
	}

	t := d.task
	w := d.width - 4

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().Foreground(styles.StatusColor(string(t.Status))).Bold(true)
	sb.WriteString(titleStyle.Render(truncate(t.Title, w)) + "\n")

	priorityStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Priority.Color())).Bold(true)
	sb.WriteString(fmt.Sprintf("%s  ·  %s  ·  %s\n",
		styles.Muted.Render(t.ID),
		priorityStyle.Render(t.Priority.Label()),
		lipgloss.NewStyle().Foreground(styles.StatusColor(string(t.Status))).Render("● "+t.Status.Label()),
	))

	if t.AssigneeID != nil {
		u := d.users[*t.AssigneeID]
		assigneeStr := *t.AssigneeID
		if u != nil {
			assigneeStr = u.DisplayPrefix()
		}
		pts := ""
		if t.Points != nil {
			pts = fmt.Sprintf("  ·  %s pts", styles.Muted.Render(fmt.Sprintf("%d", *t.Points)))
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(styles.CAccentLt).Render(assigneeStr) + pts + "\n")
	}
	sb.WriteString("\n")

	if t.Branch != "" {
		sb.WriteString(detailField("Branch", lipgloss.NewStyle().Foreground(styles.CGreen).Render(t.Branch)) + "\n")
	}
	if d.prStatus != nil {
		prColor := styles.CYellow
		if d.prStatus.State == "MERGED" {
			prColor = styles.CGreen
		} else if d.prStatus.State == "CLOSED" {
			prColor = styles.CMuted
		}
		prStr := lipgloss.NewStyle().Foreground(prColor).Render(
			fmt.Sprintf("#%d  ·  %d comments", d.prStatus.Number, d.prStatus.Comments),
		)
		sb.WriteString(detailField("PR", prStr) + "\n")
	} else if t.PRNumber != nil {
		sb.WriteString(detailField("PR", styles.Muted.Render(fmt.Sprintf("#%d", *t.PRNumber))) + "\n")
	}

	if t.Description != "" {
		sb.WriteString("\n")
		desc := truncate(t.Description, w)
		for _, line := range strings.Split(desc, "\n") {
			sb.WriteString(styles.NormalRow.Render(line) + "\n")
		}
	}

	if len(t.Blockers) > 0 {
		sb.WriteString("\n" + styles.Muted.Render("Blockers") + "\n")
		for _, b := range t.Blockers {
			sb.WriteString(styles.Danger.Render("⚠ ") + styles.NormalRow.Render(b) + "\n")
		}
	}

	if len(d.commits) > 0 {
		sb.WriteString("\n" + styles.Muted.Render("Recent commits") + "\n")
		for _, c := range d.commits {
			hash := styles.Muted.Render(c.Hash)
			subject := truncate(c.Subject, w-20)
			when := styles.Muted.Render(c.When)
			sb.WriteString(fmt.Sprintf("%s  %-*s  %s\n", hash, w-20, subject, when))
		}
	}

	if len(t.Labels) > 0 {
		sb.WriteString("\n")
		chips := make([]string, len(t.Labels))
		for i, l := range t.Labels {
			chips[i] = lipgloss.NewStyle().
				Foreground(styles.CAccentLt).
				Border(lipgloss.NormalBorder()).
				BorderForeground(styles.CAccent).
				Padding(0, 1).
				Render(l)
		}
		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, chips...) + "\n")
	}

	panel := styles.PanelBorder(d.focused).
		Width(d.width - 2).Height(d.height - 2).
		Render(sb.String())
	return panel
}

func detailField(label, value string) string {
	return fmt.Sprintf("%-10s%s", styles.Muted.Render(label), value)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if n <= 0 || len(r) <= n {
		return s
	}
	return string(r[:n-3]) + "..."
}
