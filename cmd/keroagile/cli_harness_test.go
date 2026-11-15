package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

// isolate points the CLI at an empty configuration and database of its own.
func isolate(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("KEROAGILE_DATA_DIR", filepath.Join(root, "keroagile"))
	t.Setenv("HOME", root)
	return root
}

// runCLI runs the CLI with args and returns what it printed. Output is not a
// terminal, so commands print JSON.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return runCLIIn(t, "", args...)
}

// runCLIIn is runCLI with stdin.
func runCLIIn(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	resetFlags(rootCmd)
	rootCmd.SetIn(strings.NewReader(stdin))
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	rootCmd.SetArgs(args)
	runErr := rootCmd.Execute()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if st != nil {
		st.Close()
		st = nil
	}
	return buf.String(), runErr
}

// resetFlags puts every flag back to its default: cobra keeps values from one
// Execute to the next.
func resetFlags(c *cobra.Command) {
	reset := func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	}
	c.Flags().VisitAll(reset)
	c.PersistentFlags().VisitAll(reset)
	for _, sub := range c.Commands() {
		resetFlags(sub)
	}
}
