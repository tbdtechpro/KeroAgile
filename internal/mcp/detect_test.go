package mcp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/mcp"
)

// A project whose directory holds the working directory is found without a
// project_id, also from a subdirectory and with no git remote at all. Before
// KA-060 only a fragment of the remote URL could be matched.
func TestListTasks_DetectsProjectFromDirectory(t *testing.T) {
	svc := testSvc(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	sub := filepath.Join(root, "proj", "internal")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, svc.CreateProject("OUT", "outer", root))
	require.NoError(t, svc.CreateProject("PRJ", "inner", filepath.Join(root, "proj")))
	t.Chdir(sub)

	reply, err := mcp.CallTool(svc, "list_tasks", map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, reply, `"project_id":"PRJ"`)
}

func TestListTasks_TwoProjectsOnOneDirectoryIsAnError(t *testing.T) {
	svc := testSvc(t)
	root := t.TempDir()
	require.NoError(t, svc.CreateProject("ONE", "one", root))
	require.NoError(t, svc.CreateProject("TWO", "two", root))
	t.Chdir(root)

	_, err := mcp.CallTool(svc, "list_tasks", map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project_id required")
}

func TestListProjects_ShowsDirAndRemote(t *testing.T) {
	svc := testSvc(t)
	require.NoError(t, svc.CreateProject("D", "dir", "/work/d"))
	_, err := mcp.CallTool(svc, "create_project", map[string]any{
		"id": "R", "name": "remote", "repo_dir": "/work/r", "repo_remote": "git@github.com:o/r.git",
	})
	require.NoError(t, err)

	reply, err := mcp.CallTool(svc, "list_projects", nil)
	require.NoError(t, err)
	assert.Contains(t, reply, `"repo_dir":"/work/d"`)
	assert.Contains(t, reply, `"repo_dir":"/work/r"`)
	assert.Contains(t, reply, `"repo_remote":"github.com/o/r"`)
}
