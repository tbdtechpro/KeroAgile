package mcp_test

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/mcp"
)

func instructions(t *testing.T) string {
	t.Helper()
	resp := mcp.Dispatch(testSvc(t), mcp.Request{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "initialize", Params: json.RawMessage(`{}`)})
	require.Nil(t, resp.Error)
	return resp.Result.(map[string]any)["instructions"].(string)
}

func TestInstructions_TellClaudeToAttach(t *testing.T) {
	text := instructions(t)
	assert.Contains(t, text, "attach_file")
	assert.Contains(t, text, "before finishing")
	assert.Contains(t, text, "commit")
	assert.Contains(t, text, "add_note")
}

// The flat-tasks section is mirrored from CLAUDE.md; it must stay intact.
func TestInstructions_KeepTheConceptsSection(t *testing.T) {
	text := instructions(t)
	b, err := os.ReadFile("../../CLAUDE.md")
	require.NoError(t, err)
	section := regexp.MustCompile(`(?s)## Concepts that don't exist\n(.*?)\n## `).FindSubmatch(b)
	require.NotNil(t, section)
	for _, item := range []string{"parent tasks", "sub-tasks", "epics", "themes", `"umbrella" tasks`} {
		assert.Contains(t, string(section[1]), item)
		assert.Contains(t, text, item)
	}
	assert.True(t, strings.HasPrefix(text, "KeroAgile data model"))
}

func TestSetProject_Policy(t *testing.T) {
	svc := testSvc(t)
	require.NoError(t, svc.CreateProject("TST", "Test", ""))

	_, err := mcp.CallTool(svc, "set_project", map[string]any{"project_id": "tst", "artifact_policy": "upload"})
	require.ErrorIs(t, err, domain.ErrInvalid)

	_, err = mcp.CallTool(svc, "set_project", map[string]any{"project_id": "tst", "artifact_policy": "remote", "repo_dir": "/work/tst"})
	require.NoError(t, err)
	reply, err := mcp.CallTool(svc, "list_projects", nil)
	require.NoError(t, err)
	assert.Contains(t, reply, `"artifact_policy":"remote"`)
	assert.Contains(t, reply, `"repo_dir":"/work/tst"`)

	_, err = mcp.CallTool(svc, "set_project", map[string]any{"project_id": "tst", "artifact_policy": "default"})
	require.NoError(t, err)
	p, err := svc.GetProject("TST")
	require.NoError(t, err)
	assert.Equal(t, domain.PolicyDefault, p.ArtifactPolicy)
}

func TestEffectivePolicy(t *testing.T) {
	assert.Equal(t, domain.PolicyLocal, domain.EffectivePolicy(&domain.Project{}, ""))
	assert.Equal(t, domain.PolicyRemote, domain.EffectivePolicy(&domain.Project{}, domain.PolicyRemote))
	assert.Equal(t, domain.PolicyOff, domain.EffectivePolicy(&domain.Project{ArtifactPolicy: domain.PolicyOff}, domain.PolicyRemote))
}

func TestListProjects_ShowsEffectivePolicy(t *testing.T) {
	env, _ := filesEnv(t)
	reply, err := mcp.CallToolEnv(env, "list_projects", nil)
	require.NoError(t, err)
	assert.Contains(t, reply, `"effective_policy":"local"`)
}
