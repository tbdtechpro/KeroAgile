package tui_test

import (
	"strings"
	"testing"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/tui"
)

func TestSidebarProjectNameTruncation(t *testing.T) {
	projects := []*domain.Project{
		{ID: "CR", Name: "Career Research"}, // long enough to overflow narrow panel
		{ID: "KA", Name: "KeroAgile"},
	}
	counts := map[string]map[domain.Status]int{
		"CR": {domain.StatusBacklog: 6},
		"KA": {domain.StatusBacklog: 0},
	}
	// Narrow panel — 14 columns wide, content area ~10.
	s := tui.NewSidebar(projects, counts, 14, 20)
	out := s.View()

	// Each line in the project list section should be a single row — the
	// long name must be truncated with "…" rather than wrap onto two lines.
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis in output for overflowing project name; got:\n%s", out)
	}
	// "Research" should NOT appear on its own row (which would indicate wrap).
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Research" {
			t.Fatalf("project name wrapped to second line; full output:\n%s", out)
		}
	}
}

func TestSidebarShortProjectNameUnchanged(t *testing.T) {
	projects := []*domain.Project{
		{ID: "KA", Name: "KeroAgile"},
	}
	counts := map[string]map[domain.Status]int{"KA": {}}
	s := tui.NewSidebar(projects, counts, 30, 20) // plenty of width
	out := s.View()
	if !strings.Contains(out, "KeroAgile") {
		t.Fatalf("expected unmodified project name; got:\n%s", out)
	}
	if strings.Contains(out, "KeroAgil…") || strings.Contains(out, "Kero…") {
		t.Fatalf("short name was unexpectedly truncated; got:\n%s", out)
	}
}

func TestSidebarSprintNameTruncation(t *testing.T) {
	projects := []*domain.Project{{ID: "KA", Name: "KeroAgile"}}
	counts := map[string]map[domain.Status]int{"KA": {}}
	s := tui.NewSidebar(projects, counts, 14, 20) // narrow

	summaries := []domain.SprintSummary{
		{
			Sprint:    &domain.Sprint{ID: 1, Name: "Long Sprint Name That Overflows", Status: domain.SprintActive},
			TaskCount: 3,
		},
	}
	s = s.SetSprints(summaries, true) // enter sprint-list mode
	out := s.View()

	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis in sprint-list output; got:\n%s", out)
	}
	// Original full name must not appear (would mean no truncation).
	if strings.Contains(out, "Long Sprint Name That Overflows") {
		t.Fatalf("sprint name was not truncated; got:\n%s", out)
	}
}
