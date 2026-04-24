package mcp_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/mcp"
	"github.com/tbdtechpro/KeroAgile/internal/store"
)

func testSvc(t *testing.T) *domain.Service {
	t.Helper()
	db, err := store.Open(":memory:")
	require.NoError(t, err)
	s := store.New(db)
	t.Cleanup(func() { s.Close() })
	return domain.NewService(s)
}

func TestListProjects(t *testing.T) {
	svc := testSvc(t)
	require.NoError(t, svc.CreateProject("TST", "Test Project", ""))
	result, err := mcp.CallTool(svc, "list_projects", nil)
	require.NoError(t, err)
	require.Contains(t, result, "TST")
}

func TestCreateAndGetTask(t *testing.T) {
	svc := testSvc(t)
	require.NoError(t, svc.CreateProject("TST", "Test", ""))

	result, err := mcp.CallTool(svc, "create_task", map[string]any{
		"title":      "Test task",
		"project_id": "TST",
		"priority":   "high",
	})
	require.NoError(t, err)
	require.Contains(t, result, "TST-001")

	result, err = mcp.CallTool(svc, "get_task", map[string]any{"task_id": "TST-001"})
	require.NoError(t, err)
	require.Contains(t, result, "Test task")
}

func TestMoveTask(t *testing.T) {
	svc := testSvc(t)
	require.NoError(t, svc.CreateProject("TST", "Test", ""))
	_, err := svc.CreateTask("task", "", "TST", domain.TaskCreateOpts{})
	require.NoError(t, err)

	result, err := mcp.CallTool(svc, "move_task", map[string]any{
		"task_id": "TST-001",
		"status":  "in_progress",
	})
	require.NoError(t, err)
	require.Contains(t, result, "in_progress")
}

func TestAddRemoveBlocker(t *testing.T) {
	svc := testSvc(t)
	require.NoError(t, svc.CreateProject("TST", "Test", ""))
	_, err := svc.CreateTask("task A", "", "TST", domain.TaskCreateOpts{})
	require.NoError(t, err)
	_, err = svc.CreateTask("task B", "", "TST", domain.TaskCreateOpts{})
	require.NoError(t, err)

	result, err := mcp.CallTool(svc, "add_blocker", map[string]any{
		"task_id":    "TST-001",
		"blocked_by": "TST-002",
	})
	require.NoError(t, err)
	require.Contains(t, result, `"added": true`)

	result, err = mcp.CallTool(svc, "remove_blocker", map[string]any{
		"task_id":    "TST-001",
		"blocked_by": "TST-002",
	})
	require.NoError(t, err)
	require.Contains(t, result, `"removed": true`)
}

func TestToolValidationErrors(t *testing.T) {
	svc := testSvc(t)

	_, err := mcp.CallTool(svc, "get_task", map[string]any{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "task_id")

	_, err = mcp.CallTool(svc, "create_task", map[string]any{"project_id": "X"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "title")

	_, err = mcp.CallTool(svc, "add_blocker", map[string]any{"task_id": "X"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "blocked_by")
}

func TestUpdateTaskMutation(t *testing.T) {
	svc := testSvc(t)
	require.NoError(t, svc.CreateProject("TST", "Test", ""))
	_, err := svc.CreateTask("original title", "", "TST", domain.TaskCreateOpts{})
	require.NoError(t, err)

	result, err := mcp.CallTool(svc, "update_task", map[string]any{
		"task_id": "TST-001",
		"title":   "updated title",
	})
	require.NoError(t, err)
	require.Contains(t, result, "updated title")
	require.NotContains(t, result, "original title")
}

func TestToolsListCount(t *testing.T) {
	svc := testSvc(t)
	req := mcp.Request{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/list", Params: json.RawMessage(`{}`)}
	resp := mcp.Dispatch(svc, req)
	require.Nil(t, resp.Error)
	tools, ok := resp.Result.(map[string]any)["tools"]
	require.True(t, ok)
	require.Len(t, tools, 13)
}

func TestRPCDispatch(t *testing.T) {
	svc := testSvc(t)

	// initialize
	req := mcp.Request{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "initialize", Params: json.RawMessage(`{}`)}
	resp := mcp.Dispatch(svc, req)
	require.Nil(t, resp.Error)
	require.NotNil(t, resp.Result)

	// tools/list
	req = mcp.Request{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "tools/list", Params: json.RawMessage(`{}`)}
	resp = mcp.Dispatch(svc, req)
	require.Nil(t, resp.Error)

	// notifications/initialized — no response (nil)
	notif := mcp.Request{JSONRPC: "2.0", Method: "notifications/initialized", Params: json.RawMessage(`{}`)}
	require.Nil(t, mcp.Dispatch(svc, notif))

	// unknown method → -32601
	req = mcp.Request{JSONRPC: "2.0", ID: json.RawMessage(`3`), Method: "no/such/method", Params: json.RawMessage(`{}`)}
	resp = mcp.Dispatch(svc, req)
	require.NotNil(t, resp.Error)
	require.Equal(t, -32601, resp.Error.Code)
}
