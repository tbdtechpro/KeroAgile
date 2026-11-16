package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/config"
)

// runInit runs "KeroAgile init" against a throwaway config directory, feeding
// answers to its prompts.
func runInit(t *testing.T, answers string) {
	t.Helper()
	in := filepath.Join(t.TempDir(), "stdin")
	require.NoError(t, os.WriteFile(in, []byte(answers), 0o600))
	f, err := os.Open(in)
	require.NoError(t, err)
	old := os.Stdin
	os.Stdin = f
	t.Cleanup(func() { os.Stdin = old; f.Close() })

	rootCmd.SetArgs([]string{"init"})
	require.NoError(t, rootCmd.Execute())
}

// Re-running init used to replace the whole file with just the two defaults,
// erasing the signing secret and the sync settings.
func TestInitKeepsTheRestOfTheConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("KEROAGILE_DATA_DIR", filepath.Join(dir, "keroagile"))
	require.NoError(t, config.Save(&config.Config{
		DefaultProject:  "KA",
		DefaultAssignee: "someone",
		APISecret:       "signing-secret",
		RemoteURL:       "http://primary.example",
		Sync:            config.SyncConfig{Mode: "secondary", APIToken: "sync-token"},
	}))

	// display name, user ID, default project (skipped), add Claude? no
	runInit(t, "Matt\nmatt\n\nn\n")

	got, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "matt", got.DefaultAssignee, "the answer given is saved")
	assert.Equal(t, "KA", got.DefaultProject, "skipping the project prompt keeps the existing default")
	assert.Equal(t, "signing-secret", got.APISecret)
	assert.Equal(t, "http://primary.example", got.RemoteURL)
	assert.Equal(t, "secondary", got.Sync.Mode)
	assert.Equal(t, "sync-token", got.Sync.APIToken)
}

func TestInitSetsTheDefaultProjectWhenGiven(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("KEROAGILE_DATA_DIR", filepath.Join(dir, "keroagile"))
	require.NoError(t, config.Save(&config.Config{DefaultProject: "KA"}))

	runInit(t, "Matt\nmatt\nZZ\nn\n")

	got, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "ZZ", got.DefaultProject)
}
