package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hookInput(t *testing.T, cwd, scratch string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"session_id": "sess-9", "cwd": cwd, "scratchpad_dir": scratch,
		"hook_event_name": "Stop", "transcript_path": "/x", "stop_hook_active": false,
	})
	require.NoError(t, err)
	return string(b)
}

func oldFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	old := time.Now().Add(-time.Minute)
	require.NoError(t, os.Chtimes(path, old, old))
}

func TestHookStop_SweepsIntoUnfiled(t *testing.T) {
	root := isolate(t)
	work := filepath.Join(root, "work", "alpha")
	require.NoError(t, os.MkdirAll(work, 0o755))
	_, err := runCLI(t, "project", "add", "AA", "Alpha", "--repo", work)
	require.NoError(t, err)
	scratch := filepath.Join(root, "scratch")
	oldFile(t, filepath.Join(scratch, "driver.py"), "print(1)")
	oldFile(t, filepath.Join(scratch, ".env"), "SECRET=1")

	out, err := runCLIIn(t, hookInput(t, filepath.Join(work, "sub"), scratch), "hook", "stop")
	require.NoError(t, err)
	assert.Empty(t, out, "a Stop hook prints nothing")

	out, err = runCLI(t, "artifacts", "list", "--project", "AA", "--unfiled")
	require.NoError(t, err)
	assert.Contains(t, out, "driver.py")
	assert.NotContains(t, out, ".env")
}

func TestHookStop_NoProjectRecordsSkip(t *testing.T) {
	root := isolate(t)
	scratch := filepath.Join(root, "scratch")
	oldFile(t, filepath.Join(scratch, "x.log"), "x")

	out, err := runCLIIn(t, hookInput(t, filepath.Join(root, "nowhere"), scratch), "hook", "stop")
	require.NoError(t, err)
	assert.Empty(t, out)

	out, err = runCLI(t, "artifacts", "status")
	require.NoError(t, err)
	assert.Contains(t, out, "nowhere")
	assert.Contains(t, out, "no project")
}

func TestHookStop_BadInputExitsZero(t *testing.T) {
	isolate(t)
	for _, in := range []string{"", "not json", `{"cwd": 5}`} {
		out, err := runCLIIn(t, in, "hook", "stop")
		require.NoError(t, err, in)
		assert.Empty(t, out, in)
	}
}

func TestHookStop_ExitsZeroWhateverHappens(t *testing.T) {
	assert.True(t, isHookStop([]string{"hook", "stop"}))
	assert.True(t, isHookStop([]string{"--json", "hook", "stop"}))
	assert.False(t, isHookStop([]string{"hook", "print"}))
	assert.False(t, isHookStop([]string{"task", "list"}))
}

func TestHookPrint_IsValidJSON(t *testing.T) {
	isolate(t)
	out, err := runCLI(t, "hook", "print")
	require.NoError(t, err)
	var v struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &v), out)
	require.Len(t, v.Hooks["Stop"], 1)
	assert.Equal(t, "KeroAgile hook stop", v.Hooks["Stop"][0].Hooks[0].Command)
}

func TestArtifactsSweepCommand(t *testing.T) {
	root := isolate(t)
	_, err := runCLI(t, "project", "add", "AA", "Alpha")
	require.NoError(t, err)
	dir := filepath.Join(root, "evidence")
	oldFile(t, filepath.Join(dir, "a.png"), "png")

	out, err := runCLI(t, "artifacts", "sweep", "--dir", dir, "--project", "aa")
	require.NoError(t, err)
	assert.Contains(t, out, `"kept": 1`)
	out, err = runCLI(t, "artifacts", "list", "--project", "AA", "--unfiled")
	require.NoError(t, err)
	assert.Contains(t, out, "a.png")
}
