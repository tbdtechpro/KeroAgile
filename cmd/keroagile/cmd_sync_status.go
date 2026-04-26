package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/git"
)

type transition struct {
	newStatus  *domain.Status
	linkPR     *int
	markMerged bool
}

func (tr transition) isNoop() bool {
	return tr.newStatus == nil && tr.linkPR == nil && !tr.markMerged
}

func (tr transition) summary(taskID string) string {
	if tr.markMerged {
		return fmt.Sprintf("%s: moved to done (PR merged)", taskID)
	}
	if tr.newStatus != nil && tr.linkPR != nil {
		return fmt.Sprintf("%s: moved to %s, linked PR #%d", taskID, *tr.newStatus, *tr.linkPR)
	}
	if tr.newStatus != nil {
		return fmt.Sprintf("%s: moved to %s", taskID, *tr.newStatus)
	}
	return fmt.Sprintf("%s: no change", taskID)
}

func decideTransition(status domain.Status, currentPR *int, pr *git.PRStatus) transition {
	if status == domain.StatusDone {
		return transition{}
	}
	if pr == nil {
		return transition{}
	}

	var tr transition

	switch pr.State {
	case "OPEN":
		if status == domain.StatusReview {
			return transition{}
		}
		next := domain.StatusReview
		tr.newStatus = &next
		if currentPR == nil {
			tr.linkPR = &pr.Number
		}

	case "MERGED":
		tr.markMerged = true
		if currentPR == nil {
			tr.linkPR = &pr.Number
		}
	}

	return tr
}

func applyTransition(tr transition, taskID string) error {
	if tr.isNoop() {
		return nil
	}
	if tr.linkPR != nil {
		if err := svc.LinkPR(taskID, *tr.linkPR); err != nil {
			return err
		}
	}
	if tr.markMerged {
		return svc.MarkPRMerged(taskID)
	}
	if tr.newStatus != nil {
		_, err := svc.MoveTask(taskID, *tr.newStatus)
		return err
	}
	return nil
}

var syncStatusCmd = &cobra.Command{
	Use:   "sync-status",
	Short: "Sync the current branch's task status from PR state",
	RunE: func(cmd *cobra.Command, args []string) error {
		branch, _ := cmd.Flags().GetString("branch")
		quiet, _ := cmd.Flags().GetBool("quiet")

		if branch == "" {
			b, err := git.New(".").CurrentBranch()
			if err != nil {
				return nil // not a git repo — silent no-op
			}
			branch = b
		}

		task, err := svc.TaskByBranch(branch)
		if errors.Is(err, domain.ErrNotFound) {
			return nil // unlinked branch — silent no-op
		}
		if err != nil {
			return err
		}

		proj, err := svc.GetProject(task.ProjectID)
		if err != nil {
			return err
		}

		pr, err := git.PRForBranch(proj.RepoPath, branch)
		if err != nil {
			if !quiet {
				fmt.Fprintf(cmd.ErrOrStderr(), "warn: PR lookup failed: %v\n", err)
			}
			return nil
		}

		tr := decideTransition(task.Status, task.PRNumber, pr)
		if err := applyTransition(tr, task.ID); err != nil {
			return err
		}

		if !quiet || jsonFlag {
			type result struct {
				TaskID  string `json:"task_id"`
				Summary string `json:"summary"`
			}
			r := result{TaskID: task.ID, Summary: tr.summary(task.ID)}
			if jsonFlag {
				printJSON(r)
			} else {
				fmt.Println(r.Summary)
			}
		}
		return nil
	},
}

func init() {
	syncStatusCmd.Flags().String("branch", "", "branch to sync (defaults to current git branch)")
	syncStatusCmd.Flags().Bool("quiet", false, "suppress non-error output")
}
