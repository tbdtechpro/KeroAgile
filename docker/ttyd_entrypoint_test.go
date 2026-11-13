package docker_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ttydArgs runs the entrypoint with a stand-in ttyd on PATH and returns the
// arguments ttyd would have been started with.
func ttydArgs(t *testing.T, env ...string) []string {
	t.Helper()
	bin := t.TempDir()
	standIn := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "ttyd"), []byte(standIn), 0o755))

	cmd := exec.Command("sh", "ttyd-entrypoint.sh")
	cmd.Env = append([]string{"PATH=" + bin + ":" + os.Getenv("PATH")}, env...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.Split(strings.TrimSpace(string(out)), "\n")
}

// Anyone who can reach the port gets a writable board, so the defaults matter.
func TestTtydChecksOriginAndNeedsNoLoginByDefault(t *testing.T) {
	args := ttydArgs(t)

	assert.Contains(t, args, "--check-origin", "a page on another site must not be able to drive the terminal")
	assert.NotContains(t, args, "--credential")
	assert.Contains(t, args, "--writable")
	assert.Equal(t, "KeroAgile", args[len(args)-1], "ttyd runs the TUI")
}

func TestTtydRequiresALoginWhenACredentialIsSet(t *testing.T) {
	args := ttydArgs(t, "TTYD_CREDENTIAL=kero:pass word")

	i := indexOf(args, "--credential")
	require.GreaterOrEqual(t, i, 0, "the credential flag is passed")
	assert.Equal(t, "kero:pass word", args[i+1], "the credential arrives as one argument, spaces and all")
}

func TestTtydOriginCheckCanBeTurnedOff(t *testing.T) {
	args := ttydArgs(t, "TTYD_CHECK_ORIGIN=false")

	assert.NotContains(t, args, "--check-origin")
}

func TestTtydLimitsClientsWhenAsked(t *testing.T) {
	args := ttydArgs(t, "TTYD_MAX_CLIENTS=2")

	i := indexOf(args, "--max-clients")
	require.GreaterOrEqual(t, i, 0)
	assert.Equal(t, "2", args[i+1])
}

func indexOf(args []string, want string) int {
	for i, a := range args {
		if a == want {
			return i
		}
	}
	return -1
}
