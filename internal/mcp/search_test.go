package mcp_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/mcp"
)

// twoProjects returns a service over projects AA and BB with the users matt
// and claude.
func twoProjects(t *testing.T) *domain.Service {
	t.Helper()
	svc := testSvc(t)
	require.NoError(t, svc.CreateProject("AA", "Alpha", ""))
	require.NoError(t, svc.CreateProject("BB", "Beta", ""))
	_, err := svc.CreateUser("matt", "Matt", false)
	require.NoError(t, err)
	_, err = svc.CreateUser("claude", "Claude", true)
	require.NoError(t, err)
	return svc
}

func add(t *testing.T, svc *domain.Service, project, title, body string, opts domain.TaskCreateOpts) string {
	t.Helper()
	task, err := svc.CreateTask(title, body, project, opts)
	require.NoError(t, err)
	return task.ID
}

func call(t *testing.T, svc *domain.Service, tool string, args map[string]any) (string, listing) {
	t.Helper()
	out, err := mcp.CallTool(svc, tool, args)
	require.NoError(t, err)
	var l listing
	require.NoError(t, json.Unmarshal([]byte(out), &l), out)
	return out, l
}

func rowIDs(l listing) []string {
	out := make([]string, len(l.Tasks))
	for i, r := range l.Tasks {
		out[i] = r["id"].(string)
	}
	return out
}

// The service could already search, but no MCP tool exposed it.
func TestSearchTasksFindsTextInEveryProject(t *testing.T) {
	svc := twoProjects(t)
	add(t, svc, "AA", "Fix the login form", "BODY-ONE", domain.TaskCreateOpts{})
	add(t, svc, "BB", "Flicker", "BODY-TWO the login page flickers", domain.TaskCreateOpts{})
	add(t, svc, "BB", "Unrelated", "BODY-THREE", domain.TaskCreateOpts{})

	out, l := call(t, svc, "search_tasks", map[string]any{"query": "login"})

	assert.Equal(t, []string{"AA-001", "BB-001"}, rowIDs(l), "title match first, then the description match")
	assert.Equal(t, 2, l.Total)
	assert.NotContains(t, out, "BODY-", "results are summaries")
	assert.NotContains(t, out, "\n", "compact JSON")
}

func TestSearchTasksCanBeNarrowedToOneProject(t *testing.T) {
	svc := twoProjects(t)
	add(t, svc, "AA", "shared word", "", domain.TaskCreateOpts{})
	add(t, svc, "BB", "shared word", "", domain.TaskCreateOpts{})

	_, l := call(t, svc, "search_tasks", map[string]any{"query": "shared", "project_id": "bb"})

	assert.Equal(t, []string{"BB-001"}, rowIDs(l))
}

// The cross-project view: no text, just who and what state.
func TestSearchTasksWithoutTextListsWorkInFlightEverywhere(t *testing.T) {
	svc := twoProjects(t)
	add(t, svc, "AA", "mine, started", "", domain.TaskCreateOpts{AssigneeID: "claude", Status: domain.StatusInProgress})
	add(t, svc, "AA", "mine, finished", "", domain.TaskCreateOpts{AssigneeID: "claude", Status: domain.StatusDone})
	add(t, svc, "BB", "mine, started", "", domain.TaskCreateOpts{AssigneeID: "claude", Status: domain.StatusInProgress})
	add(t, svc, "BB", "someone else's", "", domain.TaskCreateOpts{AssigneeID: "matt", Status: domain.StatusInProgress})

	_, l := call(t, svc, "search_tasks", map[string]any{"assignee_id": "claude", "status": "in_progress"})

	assert.Equal(t, []string{"AA-001", "BB-001"}, rowIDs(l))
}

func TestSearchTasksOpenMeansEverythingNotDone(t *testing.T) {
	svc := twoProjects(t)
	add(t, svc, "AA", "backlog", "", domain.TaskCreateOpts{AssigneeID: "claude"})
	add(t, svc, "AA", "review", "", domain.TaskCreateOpts{AssigneeID: "claude", Status: domain.StatusReview})
	add(t, svc, "AA", "done", "", domain.TaskCreateOpts{AssigneeID: "claude", Status: domain.StatusDone})

	_, l := call(t, svc, "search_tasks", map[string]any{"assignee_id": "claude", "status": "open"})

	assert.Equal(t, []string{"AA-001", "AA-002"}, rowIDs(l))
}

func TestSearchTasksByLabelAcrossProjects(t *testing.T) {
	svc := twoProjects(t)
	add(t, svc, "AA", "a", "", domain.TaskCreateOpts{Labels: []string{"safety", "tui"}})
	add(t, svc, "BB", "b", "", domain.TaskCreateOpts{Labels: []string{"safety"}})
	add(t, svc, "BB", "c", "", domain.TaskCreateOpts{Labels: []string{"safety-net"}})

	_, l := call(t, svc, "search_tasks", map[string]any{"label": "safety"})

	assert.Equal(t, []string{"AA-001", "BB-001"}, rowIDs(l))
}

// With nothing to match, the answer would be every ticket on the board.
func TestSearchTasksNeedsSomethingToSearchFor(t *testing.T) {
	svc := twoProjects(t)

	_, err := mcp.CallTool(svc, "search_tasks", map[string]any{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "query")
}

func TestSearchTasksSaysWhenItCutsTheList(t *testing.T) {
	svc := twoProjects(t)
	for range 5 {
		add(t, svc, "AA", "same words", "", domain.TaskCreateOpts{})
	}

	_, l := call(t, svc, "search_tasks", map[string]any{"query": "same", "limit": float64(2)})

	assert.Equal(t, 5, l.Total)
	assert.Equal(t, 2, l.Returned)
	assert.True(t, l.Truncated)
	assert.Contains(t, l.Hint, "3 more")
}

// A mistyped status used to return an empty list, which reads as "nothing in
// that state".
func TestListingToolsRejectAnUnknownStatus(t *testing.T) {
	svc := twoProjects(t)

	for _, tool := range []string{"list_tasks", "search_tasks"} {
		_, err := mcp.CallTool(svc, tool, map[string]any{"project_id": "AA", "status": "doing"})

		require.Error(t, err, tool)
		assert.Contains(t, err.Error(), "open", "%s: the error lists what is accepted", tool)
		assert.Contains(t, err.Error(), "in_progress", tool)
	}
}

func TestListTasksOpenLeavesOutDone(t *testing.T) {
	svc := twoProjects(t)
	add(t, svc, "AA", "todo", "", domain.TaskCreateOpts{Status: domain.StatusTodo})
	add(t, svc, "AA", "done", "", domain.TaskCreateOpts{Status: domain.StatusDone})

	_, l := call(t, svc, "list_tasks", map[string]any{"project_id": "AA", "status": "open"})

	assert.Equal(t, []string{"AA-001"}, rowIDs(l))
	assert.Equal(t, 1, l.Total)
}

// The server tells agents to group work with labels; nothing could list by one.
func TestListTasksFiltersByLabel(t *testing.T) {
	svc := twoProjects(t)
	add(t, svc, "AA", "a", "", domain.TaskCreateOpts{Labels: []string{"tui"}})
	add(t, svc, "AA", "b", "", domain.TaskCreateOpts{Labels: []string{"web"}})

	_, l := call(t, svc, "list_tasks", map[string]any{"project_id": "AA", "label": "tui"})

	assert.Equal(t, []string{"AA-001"}, rowIDs(l))
}

func TestListTasksFiltersBySprint(t *testing.T) {
	svc := twoProjects(t)
	sprint, err := svc.CreateSprint("Sprint 1", "AA", nil, nil)
	require.NoError(t, err)
	add(t, svc, "AA", "in the sprint", "", domain.TaskCreateOpts{SprintID: &sprint.ID})
	add(t, svc, "AA", "not in it", "", domain.TaskCreateOpts{})

	_, l := call(t, svc, "list_tasks", map[string]any{"project_id": "AA", "sprint_id": float64(sprint.ID)})

	assert.Equal(t, []string{"AA-001"}, rowIDs(l))
}

func TestListProjectsIsCompactAndCountsTickets(t *testing.T) {
	svc := twoProjects(t)
	add(t, svc, "AA", "open one", "", domain.TaskCreateOpts{})
	add(t, svc, "AA", "open two", "", domain.TaskCreateOpts{Status: domain.StatusReview})
	add(t, svc, "AA", "finished", "", domain.TaskCreateOpts{Status: domain.StatusDone})

	out, err := mcp.CallTool(svc, "list_projects", nil)
	require.NoError(t, err)

	var reply struct {
		Total    int `json:"total"`
		Projects []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Open  int    `json:"open"`
			Total int    `json:"total"`
		} `json:"projects"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &reply), out)
	assert.NotContains(t, out, "\n", "compact JSON")
	require.Len(t, reply.Projects, 2)
	assert.Equal(t, 2, reply.Total)
	assert.Equal(t, "AA", reply.Projects[0].ID)
	assert.Equal(t, 2, reply.Projects[0].Open, "tickets not done")
	assert.Equal(t, 3, reply.Projects[0].Total)
	assert.Equal(t, 0, reply.Projects[1].Total, "a project with no tickets")
}
