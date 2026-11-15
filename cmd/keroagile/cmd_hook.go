package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Claude Code hooks: keep each turn's files, link commits",
}

// stopInput is what Claude Code passes a Stop hook on standard input.
// scratchpad_dir needs Claude Code 2.1.257 or later.
type stopInput struct {
	SessionID     string `json:"session_id"`
	CWD           string `json:"cwd"`
	ScratchpadDir string `json:"scratchpad_dir"`
}

const (
	hookSweepBudget = 20 * time.Second
	hookScanBudget  = 10 * time.Second
)

var hookStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Run from a Claude Code Stop hook after every turn",
	Long: `Keeps the files in the session's scratch folder as unfiled files of the
project the session is working in. Reads the hook's JSON from standard input.

It always exits 0 and prints nothing, so it can never hold a turn open or
put text in front of Claude; problems are shown by 'KeroAgile artifacts status'.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		defer func() { _ = recover() }() // never fail the turn
		runStopHook(cmd.InOrStdin())
		return nil
	},
}

func runStopHook(stdin io.Reader) {
	raw, err := io.ReadAll(io.LimitReader(stdin, 1<<20))
	if err != nil {
		return
	}
	var in stopInput
	if err := json.Unmarshal(raw, &in); err != nil || in.ScratchpadDir == "" || in.CWD == "" {
		return // nothing to sweep, or not a hook's input
	}
	k, err := newKeeper()
	if err != nil {
		return // a bad [artifacts] section; status reports it
	}
	k.RecordHookError(sweepSession(k, in))
}

// sweepSession sweeps one session's scratch folder into its project.
func sweepSession(k *artifacts.Keeper, in stopInput) error {
	p, err := projectForDir(in.CWD)
	if err != nil {
		reason := "no project for this directory"
		if errors.Is(err, domain.ErrInvalid) {
			reason = "two projects match this directory equally"
		}
		if !errors.Is(err, domain.ErrNotFound) && !errors.Is(err, domain.ErrInvalid) {
			return err
		}
		if _, statErr := os.Stat(in.ScratchpadDir); statErr != nil {
			return nil // nothing would have been kept anyway
		}
		return k.RecordSkip(in.CWD, reason)
	}
	ctx, cancel := context.WithTimeout(context.Background(), hookSweepBudget)
	defer cancel()
	_, err = k.Sweep(ctx, artifacts.SweepInput{Dir: in.ScratchpadDir, SessionID: in.SessionID, Project: p})
	return err
}

// stopHookSnippet is what to add to the user-level Claude Code settings.
const stopHookSnippet = `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          { "type": "command", "command": "KeroAgile hook stop", "timeout": 60 }
        ]
      }
    ]
  }
}
`

var hookPrintCmd = &cobra.Command{
	Use:   "print",
	Short: "Print the Stop hook to add to your Claude Code settings",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.ErrOrStderr(), "Merge this into the \"hooks\" of ~/.claude/settings.json (user level, so it runs in every project):")
		fmt.Print(stopHookSnippet)
		return nil
	},
}

// isHookStop reports whether args run "hook stop", which must exit 0
// whatever happens, even when the database cannot be opened.
func isHookStop(args []string) bool {
	var words []string
	for _, a := range args {
		if len(a) > 0 && a[0] != '-' {
			words = append(words, a)
		}
	}
	return len(words) >= 2 && words[0] == "hook" && words[1] == "stop"
}

func init() {
	hookCmd.AddCommand(hookStopCmd, hookPrintCmd)
	rootCmd.AddCommand(hookCmd)
}
