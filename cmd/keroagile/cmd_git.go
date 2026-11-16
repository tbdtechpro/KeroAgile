package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/tbdtechpro/KeroAgile/internal/git"
	"github.com/tbdtechpro/KeroAgile/internal/gitscan"
)

var gitCmd = &cobra.Command{Use: "git", Short: "Link tickets to git history"}

var gitScanCmd = &cobra.Command{
	Use:   "scan [--dir <checkout>]",
	Short: "Link this repository's commits and pull requests to the tickets they name",
	Long: `Reads the checked-out branch since the last scan and links each commit whose
message names a ticket of this repository's project (for example "(KA-062)").
At most every ten minutes it also asks gh for pull requests whose title or
branch names a ticket, and records their state. Tickets of other projects are
not linked; links are never removed, and a commit rewritten away is marked
superseded. The Stop hook runs this after every turn.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		res, err := scanDir(ctx, dir)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(res)
		} else {
			fmt.Printf("linked %d commits, %d pull requests; %d commits superseded\n", res.Commits, res.PRs, res.Superseded)
			for _, w := range res.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
			}
		}
		return nil
	},
}

// scanDir runs the scan for the checkout holding dir (the working directory
// when empty), as its project.
func scanDir(ctx context.Context, dir string) (gitscan.Result, error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return gitscan.Result{}, err
		}
		dir = wd
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	top, err := git.TopLevel(ctx, dir)
	if err != nil {
		return gitscan.Result{}, fmt.Errorf("%s is not in a git checkout", dir)
	}
	p, err := projectForDir(dir)
	if err != nil {
		return gitscan.Result{}, err
	}
	repo := git.ForProject(p)
	repo.Dir = top
	return gitscan.Run(ctx, svc, st, repo, p, time.Now().UTC())
}

func init() {
	gitScanCmd.Flags().String("dir", "", "a directory in the checkout (default: the working directory)")
	gitCmd.AddCommand(gitScanCmd)
	rootCmd.AddCommand(gitCmd)
}
