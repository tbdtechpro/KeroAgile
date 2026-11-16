package mcp_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/mcp"
)

// listing is the shape list_tasks replies with.
type listing struct {
	ProjectID string           `json:"project_id"`
	Total     int              `json:"total"`
	Returned  int              `json:"returned"`
	Truncated bool             `json:"truncated"`
	Hint      string           `json:"hint"`
	Tasks     []map[string]any `json:"tasks"`
}

func listTasks(t *testing.T, svc *domain.Service, args map[string]any) (string, listing) {
	t.Helper()
	out, err := mcp.CallTool(svc, "list_tasks", args)
	require.NoError(t, err)
	var l listing
	require.NoError(t, json.Unmarshal([]byte(out), &l), out)
	return out, l
}

// boardOf returns a service over project TST holding n tickets, each with a
// description of bodyLen characters that starts with "BODY-OF-".
func boardOf(t *testing.T, n, bodyLen int) *domain.Service {
	t.Helper()
	svc := testSvc(t)
	require.NoError(t, svc.CreateProject("TST", "Test", ""))
	for i := 1; i <= n; i++ {
		body := fmt.Sprintf("BODY-OF-%03d ", i) + strings.Repeat("x", bodyLen)
		_, err := svc.CreateTask(fmt.Sprintf("ticket number %03d", i), body, "TST", domain.TaskCreateOpts{})
		require.NoError(t, err)
	}
	return svc
}

func row(t *testing.T, l listing, id string) map[string]any {
	t.Helper()
	for _, r := range l.Tasks {
		if r["id"] == id {
			return r
		}
	}
	t.Fatalf("no row for %s", id)
	return nil
}

// Reported: list_tasks for a 98-ticket project returned 308,567 characters and
// the client refused it. Every ticket's description was included.
func TestListTasksLeavesOutDescriptions(t *testing.T) {
	svc := boardOf(t, 3, 100)

	out, l := listTasks(t, svc, map[string]any{"project_id": "TST"})

	assert.NotContains(t, out, "BODY-OF-", "descriptions are not part of a listing")
	assert.Equal(t, 3, l.Total)
	assert.Equal(t, 3, l.Returned)
	assert.Equal(t, "ticket number 002", row(t, l, "TST-002")["title"])
	assert.Equal(t, "backlog", row(t, l, "TST-002")["status"])
}

// The real board's largest project has 149 tickets averaging 1,341 characters
// of description. A listing of it must fit comfortably in one tool result.
func TestListTasksStaysSmallForALargeProject(t *testing.T) {
	svc := boardOf(t, 150, 2000)

	out, l := listTasks(t, svc, map[string]any{"project_id": "TST"})

	assert.Equal(t, 150, l.Returned, "nothing is cut")
	assert.Less(t, len(out), 40_000, "about 10,000 tokens at most")
	assert.NotContains(t, out, "\n", "no pretty-printing")
}

// The standup, plan and update commands read these fields from a listing.
func TestListTasksKeepsWhatTheCommandsRead(t *testing.T) {
	svc := boardOf(t, 2, 10)
	_, err := svc.CreateUser("claude", "Claude", true)
	require.NoError(t, err)
	_, err = mcp.CallTool(svc, "update_task", map[string]any{
		"task_id": "TST-001", "assignee_id": "claude", "points": float64(5),
		"labels": []any{"tui", "safety"}, "priority": "high",
	})
	require.NoError(t, err)
	require.NoError(t, svc.LinkBranch("TST-001", "fix/thing"))
	require.NoError(t, svc.LinkPR("TST-001", 42))
	require.NoError(t, svc.AddDep("TST-002", "TST-001")) // 002 blocks 001

	_, l := listTasks(t, svc, map[string]any{"project_id": "TST"})

	one := row(t, l, "TST-001")
	assert.Equal(t, "claude", one["assignee_id"])
	assert.Equal(t, float64(5), one["points"])
	assert.Equal(t, "high", one["priority"])
	assert.Equal(t, []any{"tui", "safety"}, one["labels"])
	assert.Equal(t, "fix/thing", one["branch"])
	assert.Equal(t, float64(42), one["pr_number"])
	assert.Equal(t, []any{"TST-002"}, one["blockers"])
	assert.Equal(t, []any{"TST-001"}, row(t, l, "TST-002")["blocking"])
}

func TestListTasksFullDetailIncludesDescriptions(t *testing.T) {
	svc := boardOf(t, 2, 10)

	out, l := listTasks(t, svc, map[string]any{"project_id": "TST", "detail": "full"})

	assert.Contains(t, out, "BODY-OF-001")
	assert.Equal(t, 2, l.Returned)
}

func TestListTasksRejectsAnUnknownDetail(t *testing.T) {
	svc := boardOf(t, 1, 10)

	_, err := mcp.CallTool(svc, "list_tasks", map[string]any{"project_id": "TST", "detail": "everything"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "summary")
	assert.Contains(t, err.Error(), "full")
}

func TestListTasksSaysWhenItCutsTheList(t *testing.T) {
	svc := boardOf(t, 5, 10)

	_, l := listTasks(t, svc, map[string]any{"project_id": "TST", "limit": float64(2)})

	assert.Equal(t, 5, l.Total, "how many tickets matched")
	assert.Equal(t, 2, l.Returned, "how many are in this reply")
	assert.True(t, l.Truncated)
	assert.Contains(t, l.Hint, "3 more")
}

func TestListTasksDoesNotClaimTruncationWhenEverythingFits(t *testing.T) {
	svc := boardOf(t, 5, 10)

	out, l := listTasks(t, svc, map[string]any{"project_id": "TST"})

	assert.False(t, l.Truncated)
	assert.NotContains(t, out, "hint")
}

func TestListTasksStillFiltersByStatus(t *testing.T) {
	svc := boardOf(t, 3, 10)
	_, err := svc.MoveTask("TST-002", domain.StatusTodo)
	require.NoError(t, err)

	_, l := listTasks(t, svc, map[string]any{"project_id": "TST", "status": "todo"})

	require.Equal(t, 1, l.Returned)
	assert.Equal(t, "TST-002", l.Tasks[0]["id"])
}

func TestGetSprintListsSummaries(t *testing.T) {
	svc := boardOf(t, 2, 10)
	sprint, err := svc.CreateSprint("Sprint 1", "TST", nil, nil)
	require.NoError(t, err)
	_, err = svc.AssignTaskToSprint("TST-001", &sprint.ID)
	require.NoError(t, err)

	out, err := mcp.CallTool(svc, "get_sprint", map[string]any{"sprint_id": float64(sprint.ID)})

	require.NoError(t, err)
	assert.Contains(t, out, "TST-001")
	assert.Contains(t, out, "Sprint 1")
	assert.NotContains(t, out, "BODY-OF-", "descriptions are not part of a listing")
}
