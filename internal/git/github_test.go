package git_test

import (
	"fmt"
	"os"
	"path/filepath"
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

			got, err := git.PRForBranch("owner/repo", "feature/test")
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
