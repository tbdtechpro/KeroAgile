package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/config"
)

func TestNewRemote_S3FromProfile(t *testing.T) {
	creds := filepath.Join(t.TempDir(), "credentials")
	require.NoError(t, os.WriteFile(creds, []byte("[garage]\naws_access_key_id = GK1\naws_secret_access_key = s3cr3t\n"), 0o600))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", creds)

	cfg = &config.Config{Artifacts: config.ArtifactsConfig{Remote: "s3", S3: config.S3Config{
		Endpoint: "http://127.0.0.1:3900", Region: "garage", Bucket: "keroagile", PathStyle: true, Profile: "garage"}}}
	a, err := cfg.Artifacts.Resolved()
	require.NoError(t, err)
	r, err := newRemote(a)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Contains(t, r.Name(), "keroagile")
	assert.NotContains(t, r.Name(), "s3cr3t")

	cfg.Artifacts.S3.Endpoint = ""
	_, err = newRemote(a)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint")
}

func TestArtifactsSyncWithoutRemote(t *testing.T) {
	isolate(t)
	_, err := runCLI(t, "artifacts", "sync")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no remote store")
}
