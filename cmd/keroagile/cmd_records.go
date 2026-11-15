package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
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
}

func init() {
	taskCmd.AddCommand(taskNoteCmd)
}
