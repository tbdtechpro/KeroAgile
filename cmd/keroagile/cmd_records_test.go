package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskNote(t *testing.T) {
	isolate(t)
	_, err := runCLI(t, "project", "add", "AA", "Alpha")
	require.NoError(t, err)
	_, err = runCLI(t, "task", "add", "a ticket", "--project", "AA")
	require.NoError(t, err)

	out, err := runCLI(t, "task", "note", "aa-001", "found", "the", "cause")
	require.NoError(t, err)
	var n map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &n))
	assert.Equal(t, "found the cause", n["body"])

	out, err = runCLI(t, "task", "get", "AA-001")
	require.NoError(t, err)
	assert.Contains(t, out, "found the cause")

	_, err = runCLI(t, "task", "note", "AA-001", " ")
	assert.Equal(t, 2, exitCode(err))
}

func TestArtifactsCommands(t *testing.T) {
	root := isolate(t)
	_, err := runCLI(t, "project", "add", "AA", "Alpha")
	require.NoError(t, err)
	_, err = runCLI(t, "task", "add", "a ticket", "--project", "AA")
	require.NoError(t, err)
	path := filepath.Join(root, "run.log")
	require.NoError(t, os.WriteFile(path, []byte("it worked"), 0o644))

	out, err := runCLI(t, "artifacts", "attach", "aa-001", path, "--note", "passing run")
	require.NoError(t, err)
	var a map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &a))
	assert.Equal(t, "log", a["kind"])
	id := fmt.Sprint(a["id"])

	out, err = runCLI(t, "artifacts", "list", "--task", "AA-001")
	require.NoError(t, err)
	assert.Contains(t, out, "run.log")

	out, err = runCLI(t, "artifacts", "fetch", id)
	require.NoError(t, err)
	var f map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &f))
	held := f["path"].(string)
	assert.True(t, strings.HasPrefix(held, filepath.Join(root, "keroagile", "artifacts")), "held beside the database: %s", held)

	copyTo := filepath.Join(root, "out", "copy.log")
	_, err = runCLI(t, "artifacts", "fetch", id, "--to", copyTo)
	require.NoError(t, err)
	got, err := os.ReadFile(copyTo)
	require.NoError(t, err)
	assert.Equal(t, "it worked", string(got))

	out, err = runCLI(t, "artifacts", "status")
	require.NoError(t, err)
	assert.Contains(t, out, `"local_bytes": 9`)

	_, err = runCLI(t, "artifacts", "discard", id)
	require.NoError(t, err)
	out, err = runCLI(t, "artifacts", "list", "--task", "AA-001")
	require.NoError(t, err)
	assert.NotContains(t, out, "run.log")
}

func TestTaskLink(t *testing.T) {
	isolate(t)
	_, err := runCLI(t, "project", "add", "AA", "Alpha", "--repo", "github.com/o/r")
	require.NoError(t, err)
	_, err = runCLI(t, "task", "add", "a ticket", "--project", "AA")
	require.NoError(t, err)

	out, err := runCLI(t, "task", "link", "AA-001", "--commit", "abcdef12")
	require.NoError(t, err)
	assert.Contains(t, out, "https://github.com/o/r/commit/abcdef12")
	_, err = runCLI(t, "task", "link", "AA-001", "--pr", "5")
	require.NoError(t, err)

	out, err = runCLI(t, "task", "get", "AA-001")
	require.NoError(t, err)
	assert.Contains(t, out, `"links"`)
	assert.Contains(t, out, "/pull/5")

	_, err = runCLI(t, "task", "link", "AA-001", "--commit", "zzz")
	assert.Equal(t, 2, exitCode(err))
	_, err = runCLI(t, "task", "link", "AA-001")
	assert.Equal(t, 2, exitCode(err), "say what to link")
}

func TestArtifactsDiscardMany(t *testing.T) {
	root := isolate(t)
	_, err := runCLI(t, "project", "add", "AA", "Alpha")
	require.NoError(t, err)
	_, err = runCLI(t, "task", "add", "a ticket", "--project", "AA")
	require.NoError(t, err)
	var ids []string
	for _, name := range []string{"a.log", "b.log"} {
		p := filepath.Join(root, name)
		require.NoError(t, os.WriteFile(p, []byte(name), 0o644))
		out, err := runCLI(t, "artifacts", "attach", "AA-001", p)
		require.NoError(t, err)
		var a map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &a))
		ids = append(ids, fmt.Sprint(a["id"]))
	}
	_, err = runCLI(t, append([]string{"artifacts", "discard"}, ids...)...)
	require.NoError(t, err)
	out, err := runCLI(t, "artifacts", "list", "--task", "AA-001")
	require.NoError(t, err)
	assert.NotContains(t, out, ".log")
}
