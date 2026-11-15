package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/config"
)

func TestParseSize(t *testing.T) {
	cases := map[string]int64{
		"1GiB": 1 << 30, "256KiB": 256 << 10, "100MiB": 100 << 20, "2GiB": 2 << 30,
		"1TiB": 1 << 40, "1GB": 1_000_000_000, "500 MB": 500_000_000, "42": 42, "42B": 42, "1.5GiB": 3 << 29,
	}
	for in, want := range cases {
		got, err := config.ParseSize(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, bad := range []string{"", "lots", "-1GiB", "1XB"} {
		_, err := config.ParseSize(bad)
		assert.Error(t, err, bad)
	}
}

func TestParseDays(t *testing.T) {
	d, err := config.ParseDays("14d")
	require.NoError(t, err)
	assert.Equal(t, 14*24*time.Hour, d)
	d, err = config.ParseDays("36h")
	require.NoError(t, err)
	assert.Equal(t, 36*time.Hour, d)
	_, err = config.ParseDays("soon")
	assert.Error(t, err)
}

func TestArtifactsDefaults(t *testing.T) {
	a, err := config.ArtifactsConfig{}.Resolved()
	require.NoError(t, err)
	assert.Equal(t, int64(1<<30), a.LocalMax)
	assert.Equal(t, int64(2<<30), a.MinFree)
	assert.Equal(t, int64(256<<10), a.KeepLocalUnder)
	assert.Equal(t, int64(2<<30), a.MaxFile)
	assert.Equal(t, int64(100<<20), a.SweepMaxFile)
	assert.Equal(t, 14*24*time.Hour, a.KeepUnreferenced)
	assert.Equal(t, "local", a.DefaultPolicy)
	assert.Equal(t, "none", a.Remote)
}

func TestArtifactsConfigIsRead(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(config.DefaultPath()[:len(config.DefaultPath())-len("/config.toml")], 0o755))
	require.NoError(t, os.WriteFile(config.DefaultPath(), []byte(`
default_project = "KA"
[artifacts]
local_max = "512MiB"
remote = "s3"
[artifacts.s3]
endpoint = "http://127.0.0.1:3900"
bucket = "keroagile"
path_style = true
`), 0o600))
	cfg, err := config.Load()
	require.NoError(t, err)
	a, err := cfg.Artifacts.Resolved()
	require.NoError(t, err)
	assert.Equal(t, int64(512<<20), a.LocalMax)
	assert.Equal(t, "s3", a.Remote)
	assert.Equal(t, "keroagile", cfg.Artifacts.S3.Bucket)
	assert.True(t, cfg.Artifacts.S3.PathStyle)

	_, err = config.ArtifactsConfig{Remote: "ftp"}.Resolved()
	assert.Error(t, err)
	_, err = config.ArtifactsConfig{DefaultPolicy: "upload"}.Resolved()
	assert.Error(t, err)
}

// init saves the config; an unused [artifacts] section must not appear.
func TestSaveLeavesOutAnEmptyArtifactsSection(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	require.NoError(t, config.Save(&config.Config{DefaultProject: "KA"}))
	b, err := os.ReadFile(config.DefaultPath())
	require.NoError(t, err)
	assert.NotContains(t, string(b), "artifacts")
}
