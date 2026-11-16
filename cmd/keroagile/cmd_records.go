package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

var taskNoteCmd = &cobra.Command{
	Use:   "note <task-id> <text...>",
	Short: "Add a note to a ticket",
	Long:  "Add a dated note to a ticket, signed by default_assignee from the config when that user is on the board. Notes are never edited.",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		author := cfg.DefaultAssignee
		if _, err := svc.GetUser(author); err != nil {
			author = "" // not on the board: keep the note unsigned
		}
		n, err := svc.AddNote(args[0], author, strings.Join(args[1:], " "))
		if err != nil { // a missing ticket or author exits 1 through exitCode
			return err
		}
		if jsonFlag {
			printJSON(n)
		} else {
			fmt.Printf("noted on %s\n", n.TaskID)
		}
		return nil
	},
}

var taskLinkCmd = &cobra.Command{
	Use:   "link <task-id> (--commit <sha> | --pr <n> | --branch <name> | --tag <name>)",
	Short: "Link a ticket to a commit, pull request, branch or tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		var l domain.Link
		for _, k := range []domain.LinkKind{domain.LinkCommit, domain.LinkPR, domain.LinkBranch, domain.LinkTag} {
			if v, _ := cmd.Flags().GetString(string(k)); v != "" {
				if l.Kind != "" {
					return fmt.Errorf("%w link: give one of --commit, --pr, --branch, --tag", domain.ErrInvalid)
				}
				l.Kind, l.Ref = k, v
			}
		}
		if l.Kind == "" {
			return fmt.Errorf("%w link: give one of --commit, --pr, --branch, --tag", domain.ErrInvalid)
		}
		l.Repo = repo
		got, created, err := svc.AddLink(args[0], l)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(map[string]any{"created": created, "link": got})
		} else {
			fmt.Printf("linked %s %s %s\n", got.TaskID, got.Kind, got.Ref)
		}
		return nil
	},
}

// printRecords prints a ticket's notes, links and files under task get.
func printRecords(t *domain.Task) {
	if len(t.Notes) > 0 {
		fmt.Println("  Notes:")
		for _, n := range t.Notes {
			who := n.AuthorID
			if who == "" {
				who = "—"
			}
			fmt.Printf("    %s  %s\n", n.CreatedAt.Local().Format("2006-01-02 15:04"), who)
			for _, line := range strings.Split(n.Body, "\n") {
				fmt.Printf("      %s\n", line)
			}
		}
	}
	if len(t.Links) > 0 {
		fmt.Println("  Links:")
		for _, l := range t.Links {
			fmt.Printf("    %-6s %-14s %-10s %s\n", l.Kind, shortRef(l), l.State, l.Title)
		}
	}
	if len(t.Artifacts) > 0 {
		fmt.Println("  Files:")
		for _, a := range t.Artifacts {
			fmt.Printf("    %-5d %-9s %-7s %-11s %s\n", a.ID, artifacts.HumanSize(a.Size), a.Kind, a.Where, a.Name)
		}
	}
}

func shortRef(l *domain.Link) string {
	switch {
	case l.Kind == domain.LinkCommit && len(l.Ref) > 10:
		return l.Ref[:10]
	case l.Kind == domain.LinkPR:
		return "#" + l.Ref
	}
	return l.Ref
}

func init() {
	for _, k := range []string{"commit", "pr", "branch", "tag"} {
		taskLinkCmd.Flags().String(k, "", "the "+k+" to link")
	}
	taskLinkCmd.Flags().String("repo", "", "host/owner/repo (default: the project's remote)")
	taskCmd.AddCommand(taskNoteCmd, taskLinkCmd)
}
