package git_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/git"
)

func fakeGH(t *testing.T, stdout, stderr string, exitCode int) {
	t.Helper()
	dir := t.TempDir()

	script := "#!/bin/sh\n"
	if stdout != "" {
		script += fmt.Sprintf("printf '%%s' '%s'\n", stdout)
	}
	if stderr != "" {
		script += fmt.Sprintf("printf '%%s' '%s' >&2\n", stderr)
	}
	if exitCode != 0 {
		script += fmt.Sprintf("exit %d\n", exitCode)
	}

	ghPath := filepath.Join(dir, "gh")
	require.NoError(t, os.WriteFile(ghPath, []byte(script), 0755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

func TestPRForBranch(t *testing.T) {
	cases := []struct {
		name     string
		stdout   string
		stderr   string
		exitCode int
		want     *git.PRStatus
		wantErr  bool
	}{
		{
			name:   "open PR",
			stdout: `{"state":"OPEN","number":42}`,
			want:   &git.PRStatus{State: "OPEN", Number: 42},
		},
		{
			name:   "merged PR",
			stdout: `{"state":"MERGED","number":7}`,
			want:   &git.PRStatus{State: "MERGED", Number: 7},
		},
		{
			name:     "no PR found",
			exitCode: 1,
			stderr:   "no pull requests found for branch",
			want:     nil,
			wantErr:  false,
		},
		{
			name:     "gh auth error",
			exitCode: 1,
			stderr:   "authentication required",
			wantErr:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeGH(t, tc.stdout, tc.stderr, tc.exitCode)

			got, err := git.Repo{Remote: "owner/repo"}.PRForBranch("feature/test")
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// recordingGH installs a gh that answers with stdout and writes its working
// directory and arguments, one per line, to the returned file.
func recordingGH(t *testing.T, stdout string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := "#!/bin/sh\n" +
		fmt.Sprintf("pwd > '%s'\nfor a in \"$@\"; do printf '%%s\\n' \"$a\" >> '%s'; done\n", log, log) +
		fmt.Sprintf("printf '%%s' '%s'\n", stdout)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return log
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

func TestGH_UsesRemoteWhenKnown(t *testing.T) {
	log := recordingGH(t, `{"state":"OPEN","number":1}`)
	_, err := git.Repo{Dir: t.TempDir(), Remote: "github.com/o/r"}.PRForBranch("feat/x")
	require.NoError(t, err)
	args := readLines(t, log)[1:]
	assert.Contains(t, strings.Join(args, " "), "-R github.com/o/r")
}

func TestGH_RunsInDirWithoutRemote(t *testing.T) {
	log := recordingGH(t, `{"state":"OPEN","number":1}`)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	_, err = git.Repo{Dir: dir}.PRForBranch("feat/x")
	require.NoError(t, err)
	lines := readLines(t, log)
	assert.Equal(t, dir, lines[0])
	assert.NotContains(t, lines[1:], "-R")
}

func TestPRForBranch_BranchAfterEndOfOptions(t *testing.T) {
	log := recordingGH(t, `{"state":"OPEN","number":1}`)
	_, err := git.Repo{Remote: "o/r"}.PRForBranch("feat/x")
	require.NoError(t, err)
	args := readLines(t, log)[1:]
	require.GreaterOrEqual(t, len(args), 2)
	assert.Equal(t, []string{"--", "feat/x"}, args[len(args)-2:])
}

func TestPRForBranch_RefusesOptionLikeBranch(t *testing.T) {
	log := recordingGH(t, `{}`)
	_, err := git.Repo{Remote: "o/r"}.PRForBranch("--repo=evil/x")
	require.Error(t, err)
	_, statErr := os.Stat(log)
	assert.True(t, os.IsNotExist(statErr), "gh was not run")
}

func TestPRView_NoRepoIsAnError(t *testing.T) {
	_, err := git.Repo{}.PRView(1)
	require.Error(t, err)
}
