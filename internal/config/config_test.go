package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/config"
)

// The config holds the secret that signs web sessions and the sync token.
func TestSaveWritesAFileOnlyItsOwnerCanRead(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	require.NoError(t, config.Save(&config.Config{APISecret: "signing-secret"}))

	info, err := os.Stat(config.DefaultPath())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestSaveTightensAnExistingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	require.NoError(t, config.Save(&config.Config{}))
	require.NoError(t, os.Chmod(config.DefaultPath(), 0o664))

	require.NoError(t, config.Save(&config.Config{APISecret: "signing-secret"}))

	info, err := os.Stat(config.DefaultPath())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
