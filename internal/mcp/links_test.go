package mcp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/mcp"
)

func TestAddLink_Tool(t *testing.T) {
	svc := testSvc(t)
	require.NoError(t, svc.CreateProject("TST", "Test", "https://github.com/o/r.git"))
	_, err := svc.CreateTask("a ticket", "", "TST", domain.TaskCreateOpts{})
	require.NoError(t, err)

	reply, err := mcp.CallTool(svc, "add_link", map[string]any{"task_id": "tst-001", "kind": "pr", "ref": "12"})
	require.NoError(t, err)
	assert.Contains(t, reply, `"url": "https://github.com/o/r/pull/12"`)
	assert.Contains(t, reply, `"created": true`)

	full, err := mcp.CallTool(svc, "get_task", map[string]any{"task_id": "TST-001"})
	require.NoError(t, err)
	assert.Contains(t, full, "/pull/12")

	_, err = mcp.CallTool(svc, "add_link", map[string]any{"task_id": "TST-001", "kind": "commit", "ref": "nothex!"})
	require.ErrorIs(t, err, domain.ErrInvalid)
}
