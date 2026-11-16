package mcp_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/mcp"
)

func TestAddNote_Tool(t *testing.T) {
	svc := testSvc(t)
	require.NoError(t, svc.CreateProject("TST", "Test", ""))
	_, err := svc.CreateUser("claude", "Claude", true)
	require.NoError(t, err)
	_, err = svc.CreateTask("a ticket", "", "TST", domain.TaskCreateOpts{})
	require.NoError(t, err)
	t.Setenv("KEROAGILE_USER", "")

	reply, err := mcp.CallTool(svc, "add_note", map[string]any{"task_id": "tst-001", "body": "the cause was X"})
	require.NoError(t, err)
	var n map[string]any
	require.NoError(t, json.Unmarshal([]byte(reply), &n))
	assert.NotZero(t, n["id"])
	assert.Equal(t, "claude", n["author_id"], "MCP notes are signed by the agent user when there is one")

	full, err := mcp.CallTool(svc, "get_task", map[string]any{"task_id": "TST-001"})
	require.NoError(t, err)
	assert.Contains(t, full, "the cause was X")

	list, err := mcp.CallTool(svc, "list_tasks", map[string]any{"project_id": "TST"})
	require.NoError(t, err)
	assert.NotContains(t, list, "the cause was X")

	_, err = mcp.CallTool(svc, "add_note", map[string]any{"task_id": "TST-001", "body": "  "})
	require.ErrorIs(t, err, domain.ErrInvalid)
}

func TestAddNote_AuthorFromEnvironment(t *testing.T) {
	svc := testSvc(t)
	require.NoError(t, svc.CreateProject("TST", "Test", ""))
	_, err := svc.CreateUser("matt", "Matt", false)
	require.NoError(t, err)
	_, err = svc.CreateTask("a ticket", "", "TST", domain.TaskCreateOpts{})
	require.NoError(t, err)
	t.Setenv("KEROAGILE_USER", "matt")

	reply, err := mcp.CallTool(svc, "add_note", map[string]any{"task_id": "TST-001", "body": "x"})
	require.NoError(t, err)
	assert.Contains(t, reply, `"author_id": "matt"`)
}
