package mcp_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/mcp"
)

// richTicket returns a service whose ticket AA-001 has every optional field
// filled in, and the ID of a sprint in project AA.
func richTicket(t *testing.T) (*domain.Service, int64) {
	t.Helper()
	svc := twoProjects(t)
	sprint, err := svc.CreateSprint("Sprint 1", "AA", nil, nil)
	require.NoError(t, err)
	points := 5
	add(t, svc, "AA", "original title", "original description", domain.TaskCreateOpts{
		AssigneeID: "claude", Points: &points, Labels: []string{"tui"}, SprintID: &sprint.ID,
	})
	require.NoError(t, svc.LinkBranch("AA-001", "fix/thing"))
	return svc, sprint.ID
}

func ticket(t *testing.T, svc *domain.Service, id string) *domain.Task {
	t.Helper()
	task, err := svc.GetTask(id)
	require.NoError(t, err)
	return task
}

func TestUpdateTaskChangesOnlyWhatIsPassed(t *testing.T) {
	svc, sprintID := richTicket(t)

	_, err := mcp.CallTool(svc, "update_task", map[string]any{"task_id": "AA-001", "title": "renamed"})

	require.NoError(t, err)
	got := ticket(t, svc, "AA-001")
	assert.Equal(t, "renamed", got.Title)
	assert.Equal(t, "original description", got.Description)
	assert.Equal(t, "claude", *got.AssigneeID)
	assert.Equal(t, 5, *got.Points)
	assert.Equal(t, []string{"tui"}, got.Labels)
	assert.Equal(t, sprintID, *got.SprintID)
	assert.Equal(t, "fix/thing", got.Branch)
}

// Agents sometimes pass "" for fields they do not mean to touch. That must
// never wipe a description or fail on an empty title.
func TestUpdateTaskIgnoresEmptyStrings(t *testing.T) {
	svc, _ := richTicket(t)

	_, err := mcp.CallTool(svc, "update_task", map[string]any{
		"task_id": "AA-001", "title": "", "description": "", "priority": "", "status": "",
	})

	require.NoError(t, err)
	got := ticket(t, svc, "AA-001")
	assert.Equal(t, "original title", got.Title)
	assert.Equal(t, "original description", got.Description)
}

// Points, description and sprint could be set through this tool but never
// emptied.
func TestUpdateTaskClearsTheFieldsListedInClear(t *testing.T) {
	svc, _ := richTicket(t)

	_, err := mcp.CallTool(svc, "update_task", map[string]any{
		"task_id": "AA-001",
		"clear":   []any{"description", "points", "labels", "assignee_id", "sprint_id"},
	})

	require.NoError(t, err)
	got := ticket(t, svc, "AA-001")
	assert.Empty(t, got.Description)
	assert.Nil(t, got.Points)
	assert.Empty(t, got.Labels)
	assert.Nil(t, got.AssigneeID)
	assert.Nil(t, got.SprintID)
	assert.Equal(t, "original title", got.Title)
}

func TestUpdateTaskRejectsAClearItCannotHonour(t *testing.T) {
	svc, _ := richTicket(t)

	_, unknown := mcp.CallTool(svc, "update_task", map[string]any{"task_id": "AA-001", "clear": []any{"title"}})
	_, both := mcp.CallTool(svc, "update_task", map[string]any{"task_id": "AA-001", "points": float64(3), "clear": []any{"points"}})

	require.Error(t, unknown)
	assert.Contains(t, unknown.Error(), "description, points, labels, assignee_id, sprint_id", "the error lists what can be cleared")
	require.Error(t, both, "a field cannot be both set and cleared")
	assert.Equal(t, 5, *ticket(t, svc, "AA-001").Points, "nothing was changed")
}

func TestUpdateTaskStillClearsTheAssigneeWithAnEmptyString(t *testing.T) {
	svc, _ := richTicket(t)

	_, err := mcp.CallTool(svc, "update_task", map[string]any{"task_id": "AA-001", "assignee_id": ""})

	require.NoError(t, err)
	assert.Nil(t, ticket(t, svc, "AA-001").AssigneeID)
}

// The planning command passes sprint_id to update_task, which used to ignore
// it, so planned sprints stayed empty.
func TestUpdateTaskAssignsASprint(t *testing.T) {
	svc, sprintID := richTicket(t)
	add(t, svc, "AA", "second", "", domain.TaskCreateOpts{})

	_, err := mcp.CallTool(svc, "update_task", map[string]any{"task_id": "AA-002", "sprint_id": float64(sprintID)})

	require.NoError(t, err)
	require.NotNil(t, ticket(t, svc, "AA-002").SprintID)
	assert.Equal(t, sprintID, *ticket(t, svc, "AA-002").SprintID)
}

// An agent can pass the updated_at it got from get_task. If the ticket has
// changed since, the update is refused rather than applied on top.
func TestUpdateTaskWithAStaleExpectedUpdatedAtIsRefused(t *testing.T) {
	svc, _ := richTicket(t)
	out, err := mcp.CallTool(svc, "get_task", map[string]any{"task_id": "AA-001"})
	require.NoError(t, err)
	var read struct {
		UpdatedAt string `json:"updated_at"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &read))
	_, err = svc.MoveTask("AA-001", domain.StatusTodo) // someone else changes it
	require.NoError(t, err)

	_, err = mcp.CallTool(svc, "update_task", map[string]any{
		"task_id": "AA-001", "title": "mine", "expected_updated_at": read.UpdatedAt,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "changed")
	assert.Equal(t, "original title", ticket(t, svc, "AA-001").Title)
}

func TestUpdateTaskWithACurrentExpectedUpdatedAtIsApplied(t *testing.T) {
	svc, _ := richTicket(t)
	out, err := mcp.CallTool(svc, "get_task", map[string]any{"task_id": "AA-001"})
	require.NoError(t, err)
	var read struct {
		UpdatedAt string `json:"updated_at"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &read))

	_, err = mcp.CallTool(svc, "update_task", map[string]any{
		"task_id": "AA-001", "title": "mine", "expected_updated_at": read.UpdatedAt,
	})

	require.NoError(t, err)
	assert.Equal(t, "mine", ticket(t, svc, "AA-001").Title)
}
