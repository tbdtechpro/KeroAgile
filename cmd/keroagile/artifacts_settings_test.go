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

func TestNewRemote_Folder(t *testing.T) {
	dir := t.TempDir()
	cfg = &config.Config{Artifacts: config.ArtifactsConfig{Remote: "folder", Folder: config.FolderConfig{Path: dir}}}
	a, err := cfg.Artifacts.Resolved()
	require.NoError(t, err)
	r, err := newRemote(a)
	require.NoError(t, err)
	assert.Contains(t, r.Name(), dir)
	cfg.Artifacts.Folder.Path = ""
	_, err = newRemote(a)
	require.Error(t, err)
}

// Review I5: a remote that cannot be set up (no credentials yet) must not
// switch off keeping files here; status says what is wrong.
func TestNewKeeper_RemoteFailureKeepsLocalCapture(t *testing.T) {
	root := isolate(t)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "keroagile"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "keroagile", "config.toml"), []byte(
		"[artifacts]\nremote = \"s3\"\n[artifacts.s3]\nendpoint = \"http://127.0.0.1:1\"\nbucket = \"b\"\nprofile = \"missing\"\n"), 0o600))
	_, err := runCLI(t, "project", "add", "AA", "Alpha")
	require.NoError(t, err)
	_, err = runCLI(t, "task", "add", "a ticket", "--project", "AA")
	require.NoError(t, err)
	p := filepath.Join(root, "x.log")
	require.NoError(t, os.WriteFile(p, []byte("kept anyway"), 0o644))

	_, err = runCLI(t, "artifacts", "attach", "AA-001", p)
	require.NoError(t, err, "files are still kept here")
	out, err := runCLI(t, "artifacts", "status")
	require.NoError(t, err)
	assert.Contains(t, out, `"remote_error"`)
	assert.Contains(t, out, "missing")
}
