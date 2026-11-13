package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/store"
)

// board is a store holding projects AA and BB and the users matt and claude.
func board(t *testing.T) *store.Store {
	t.Helper()
	s := testStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "AA", Name: "Alpha"}))
	require.NoError(t, s.CreateProject(&domain.Project{ID: "BB", Name: "Beta"}))
	require.NoError(t, s.CreateUser(&domain.User{ID: "matt", DisplayName: "Matt"}))
	require.NoError(t, s.CreateUser(&domain.User{ID: "claude", DisplayName: "Claude", IsAgent: true}))
	return s
}

// put stores a ticket; change adjusts it first.
func put(t *testing.T, s *store.Store, id, title string, change func(*domain.Task)) {
	t.Helper()
	task := newTask(id)
	task.ProjectID = id[:2]
	task.Title = title
	if change != nil {
		change(task)
	}
	require.NoError(t, s.CreateTask(task))
}

func ids(tasks []*domain.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.ID
	}
	return out
}

func find(t *testing.T, s *store.Store, q domain.TaskQuery) ([]string, int) {
	t.Helper()
	tasks, total, err := s.FindTasks(q)
	require.NoError(t, err)
	return ids(tasks), total
}

func TestFindTasksMatchesIDTitleAndDescription(t *testing.T) {
	s := board(t)
	put(t, s, "AA-001", "Fix the login form", nil)
	put(t, s, "AA-002", "Flicker", func(k *domain.Task) { k.Description = "the LOGIN page flickers on load" })
	put(t, s, "AA-003", "Unrelated", nil)

	byText, _ := find(t, s, domain.TaskQuery{Text: "login"})
	assert.Equal(t, []string{"AA-001", "AA-002"}, byText, "a title match comes before a description match")

	byID, _ := find(t, s, domain.TaskQuery{Text: "aa-003"})
	assert.Equal(t, []string{"AA-003"}, byID, "IDs match in any letter case")
}

func TestFindTasksPutsAnExactIDFirst(t *testing.T) {
	s := board(t)
	put(t, s, "AA-001", "The ticket itself", nil)
	put(t, s, "AA-002", "Follow-up to AA-001", nil)

	got, _ := find(t, s, domain.TaskQuery{Text: "AA-001"})

	assert.Equal(t, []string{"AA-001", "AA-002"}, got)
}

func TestFindTasksSearchesEveryProjectUnlessNarrowed(t *testing.T) {
	s := board(t)
	put(t, s, "AA-001", "shared word", nil)
	put(t, s, "BB-001", "shared word", nil)

	everywhere, _ := find(t, s, domain.TaskQuery{Text: "shared"})
	assert.ElementsMatch(t, []string{"AA-001", "BB-001"}, everywhere)

	oneProject, _ := find(t, s, domain.TaskQuery{Text: "shared", ProjectID: "BB"})
	assert.Equal(t, []string{"BB-001"}, oneProject)
}

// With no text this is the cross-project view: "what is in flight, for whom".
func TestFindTasksByStatusAndAssigneeAcrossProjects(t *testing.T) {
	s := board(t)
	claude, matt := "claude", "matt"
	put(t, s, "AA-001", "a", func(k *domain.Task) { k.Status = domain.StatusInProgress; k.AssigneeID = &claude })
	put(t, s, "AA-002", "b", func(k *domain.Task) { k.Status = domain.StatusDone; k.AssigneeID = &claude })
	put(t, s, "BB-001", "c", func(k *domain.Task) { k.Status = domain.StatusReview; k.AssigneeID = &claude })
	put(t, s, "BB-002", "d", func(k *domain.Task) { k.Status = domain.StatusInProgress; k.AssigneeID = &matt })

	got, total := find(t, s, domain.TaskQuery{
		AssigneeID: "claude",
		Statuses:   []domain.Status{domain.StatusInProgress, domain.StatusReview},
	})

	assert.Equal(t, []string{"AA-001", "BB-001"}, got, "grouped by project")
	assert.Equal(t, 2, total)
}

func TestFindTasksByLabelMatchesWholeLabelsOnly(t *testing.T) {
	s := board(t)
	put(t, s, "AA-001", "a", func(k *domain.Task) { k.Labels = []string{"tui", "safety"} })
	put(t, s, "AA-002", "b", func(k *domain.Task) { k.Labels = []string{"tuist"} })
	put(t, s, "AA-003", "c", func(k *domain.Task) { k.Labels = []string{"web", " tui"} }) // stored as "web, tui"
	put(t, s, "AA-004", "d", nil)

	got, _ := find(t, s, domain.TaskQuery{Label: "TUI"})

	assert.Equal(t, []string{"AA-001", "AA-003"}, got)
}

func TestFindTasksBySprint(t *testing.T) {
	s := board(t)
	sprint, err := s.CreateSprint(&domain.Sprint{ProjectID: "AA", Name: "Sprint 1", Status: domain.SprintPlanning})
	require.NoError(t, err)
	put(t, s, "AA-001", "in the sprint", func(k *domain.Task) { k.SprintID = &sprint.ID })
	put(t, s, "AA-002", "not in it", nil)

	got, _ := find(t, s, domain.TaskQuery{ProjectID: "AA", SprintID: &sprint.ID})

	assert.Equal(t, []string{"AA-001"}, got)
}

// % and _ are wildcards in SQL LIKE; in a search box they are just text.
func TestFindTasksTreatsWildcardCharactersAsText(t *testing.T) {
	s := board(t)
	put(t, s, "AA-001", "100% done", nil)
	put(t, s, "AA-002", "1000 done", nil)
	put(t, s, "AA-003", "snake_case name", nil)
	put(t, s, "AA-004", "snakeXcase name", nil)

	percent, _ := find(t, s, domain.TaskQuery{Text: "100%"})
	underscore, _ := find(t, s, domain.TaskQuery{Text: "snake_case"})

	assert.Equal(t, []string{"AA-001"}, percent)
	assert.Equal(t, []string{"AA-003"}, underscore)
}

func TestFindTasksReturnsUpToTheLimitAndTheFullCount(t *testing.T) {
	s := board(t)
	for _, id := range []string{"AA-001", "AA-002", "AA-003", "AA-004", "AA-005"} {
		put(t, s, id, "same", nil)
	}

	got, total := find(t, s, domain.TaskQuery{ProjectID: "AA", Limit: 2})

	assert.Equal(t, []string{"AA-001", "AA-002"}, got, "oldest first within a project")
	assert.Equal(t, 5, total, "how many match, not how many were returned")
}

func TestTaskCountsGroupsByProjectAndStatus(t *testing.T) {
	s := board(t)
	put(t, s, "AA-001", "a", nil)
	put(t, s, "AA-002", "b", nil)
	put(t, s, "AA-003", "c", func(k *domain.Task) { k.Status = domain.StatusDone })
	put(t, s, "BB-001", "d", func(k *domain.Task) { k.Status = domain.StatusReview })

	counts, err := s.TaskCounts()

	require.NoError(t, err)
	assert.Equal(t, map[string]map[domain.Status]int{
		"AA": {domain.StatusBacklog: 2, domain.StatusDone: 1},
		"BB": {domain.StatusReview: 1},
	}, counts)
}
