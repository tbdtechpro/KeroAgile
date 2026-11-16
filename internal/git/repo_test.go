package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/git"
)

func TestAutoLinkTaskID(t *testing.T) {
	cases := []struct {
		branch string
		want   string
	}{
		{"feature/KA-001-api-layer", "KA-001"},
		{"feature/KA-012", "KA-012"},
		{"main", ""},
		{"fix/some-bug", ""},
		{"PROJ-42-thing", "PROJ-42"},
	}
	for _, tc := range cases {
		t.Run(tc.branch, func(t *testing.T) {
			assert.Equal(t, tc.want, git.AutoLinkTaskID(tc.branch))
		})
	}
}

// newRepo makes a git repository with one commit in a temporary directory.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "first (KA-001)")
	return dir
}

func TestCommitLog(t *testing.T) {
	dir := newRepo(t)
	commits, err := git.New(dir).CommitLog("main", 5)
	require.NoError(t, err)
	require.Len(t, commits, 1)
	assert.Equal(t, "first (KA-001)", commits[0].Subject)
}

func TestCommitLogRefusesOptionLikeBranch(t *testing.T) {
	dir := newRepo(t)
	out := filepath.Join(t.TempDir(), "written")
	_, err := git.New(dir).CommitLog("--output="+out, 5)
	require.Error(t, err)
	_, statErr := os.Stat(out)
	assert.True(t, os.IsNotExist(statErr), "git did not take the branch as an option")
}

func TestRemoteURL(t *testing.T) {
	dir := newRepo(t)
	_, err := git.RemoteURL(dir)
	require.Error(t, err, "no origin yet")

	cmd := exec.Command("git", "remote", "add", "origin", "git@github.com:o/r.git")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())
	url, err := git.RemoteURL(dir)
	require.NoError(t, err)
	assert.Equal(t, "git@github.com:o/r.git", url)
}
