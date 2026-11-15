package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectSetAndWhich(t *testing.T) {
	root := isolate(t)
	work := filepath.Join(root, "work", "alpha")
	require.NoError(t, os.MkdirAll(filepath.Join(work, "sub"), 0o755))

	_, err := runCLI(t, "project", "add", "AA", "Alpha")
	require.NoError(t, err)
	out, err := runCLI(t, "project", "set", "aa", "--dir", work, "--artifact-policy", "local")
	require.NoError(t, err)
	var p map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &p))
	assert.Equal(t, work, p["repo_dir"])
	assert.Equal(t, "local", p["artifact_policy"])

	out, err = runCLI(t, "project", "which", "--dir", filepath.Join(work, "sub"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(out), &p))
	assert.Equal(t, "AA", p["id"])

	_, err = runCLI(t, "project", "which", "--dir", root)
	assert.Equal(t, 1, exitCode(err), "no project there")

	_, err = runCLI(t, "project", "set", "AA", "--artifact-policy", "upload")
	assert.Equal(t, 2, exitCode(err))
}
