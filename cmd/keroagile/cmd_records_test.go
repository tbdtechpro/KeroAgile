package main

import (
	"encoding/json"
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
