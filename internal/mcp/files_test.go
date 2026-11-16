package mcp_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/mcp"
	"github.com/tbdtechpro/KeroAgile/internal/store"
)

func filesEnv(t *testing.T) (mcp.Env, string) {
	t.Helper()
	db, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	st := store.New(db)
	svc := domain.NewService(st)
	require.NoError(t, svc.CreateProject("TST", "Test", ""))
	for _, title := range []string{"one", "two"} {
		_, err := svc.CreateTask(title, "", "TST", domain.TaskCreateOpts{})
		require.NoError(t, err)
	}
	k := artifacts.New(svc, st, artifacts.Settings{
		Dir: t.TempDir(), LocalMax: 1 << 30, MaxFile: 1 << 30, KeepLocalUnder: 256 << 10,
		KeepUnreferenced: time.Hour, DefaultPolicy: domain.PolicyLocal,
		DiskFree: func(string) (uint64, error) { return 1 << 40, nil },
	})
	return mcp.Env{Svc: svc, Files: k}, t.TempDir()
}

func callEnv(t *testing.T, env mcp.Env, name string, args map[string]any) map[string]any {
	t.Helper()
	out, err := mcp.CallToolEnv(env, name, args)
	require.NoError(t, err, name)
	var v map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &v), out)
	return v
}

func TestAttachListFetchFileDiscard_Tools(t *testing.T) {
	env, work := filesEnv(t)
	path := filepath.Join(work, "run.log")
	require.NoError(t, os.WriteFile(path, []byte("it worked"), 0o644))

	a := callEnv(t, env, "attach_file", map[string]any{"task_id": "tst-001", "path": path, "note": "the passing run"})
	assert.Equal(t, "log", a["kind"])
	id := a["id"]

	full, err := mcp.CallToolEnv(env, "get_task", map[string]any{"task_id": "TST-001"})
	require.NoError(t, err)
	assert.Contains(t, full, `"name": "run.log"`)
	assert.Contains(t, full, `"where": "here"`)
	assert.NotContains(t, full, "source_path", "get_task lists files compactly")

	list := callEnv(t, env, "list_artifacts", map[string]any{"task_id": "TST-001"})
	assert.EqualValues(t, 1, list["total"])

	f := callEnv(t, env, "fetch_artifact", map[string]any{"artifact_id": id})
	got, err := os.ReadFile(f["path"].(string))
	require.NoError(t, err)
	assert.Equal(t, "it worked", string(got))

	moved := callEnv(t, env, "file_artifact", map[string]any{"artifact_id": id, "task_id": "TST-002"})
	assert.Equal(t, "TST-002", moved["task_id"])

	callEnv(t, env, "discard_artifact", map[string]any{"artifact_id": id})
	list = callEnv(t, env, "list_artifacts", map[string]any{"task_id": "TST-002"})
	assert.EqualValues(t, 0, list["total"])
}

func TestListArtifacts_Unfiled(t *testing.T) {
	env, _ := filesEnv(t)
	list := callEnv(t, env, "list_artifacts", map[string]any{"project_id": "tst", "unfiled": true})
	assert.EqualValues(t, 0, list["total"])
	_, err := mcp.CallToolEnv(env, "list_artifacts", map[string]any{})
	require.Error(t, err)
}

func TestFileTools_WithoutKeeperSayWhy(t *testing.T) {
	env, work := filesEnv(t)
	env.Files = nil
	path := filepath.Join(work, "x.log")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))
	_, err := mcp.CallToolEnv(env, "attach_file", map[string]any{"task_id": "TST-001", "path": path})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

// Review I8: a long unfiled list must fit in a tool result, and be
// discardable in bulk.
func TestListArtifacts_LimitAndBulkDiscard(t *testing.T) {
	env, work := filesEnv(t)
	var ids []any
	for i := 0; i < 5; i++ {
		p := filepath.Join(work, fmt.Sprintf("f%d.log", i))
		require.NoError(t, os.WriteFile(p, []byte(fmt.Sprint(i)), 0o644))
		a := callEnv(t, env, "attach_file", map[string]any{"task_id": "TST-001", "path": p})
		ids = append(ids, a["id"])
	}
	list := callEnv(t, env, "list_artifacts", map[string]any{"task_id": "TST-001", "limit": float64(2)})
	assert.EqualValues(t, 5, list["total"])
	assert.EqualValues(t, 2, list["returned"])
	assert.Equal(t, true, list["truncated"])
	assert.NotEmpty(t, list["hint"])

	callEnv(t, env, "discard_artifact", map[string]any{"artifact_ids": ids[:4]})
	list = callEnv(t, env, "list_artifacts", map[string]any{"task_id": "TST-001"})
	assert.EqualValues(t, 1, list["total"])
}
