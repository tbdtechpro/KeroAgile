package main

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

var sprintCmd = &cobra.Command{Use: "sprint", Short: "Manage sprints"}

var sprintAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a sprint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		startStr, _ := cmd.Flags().GetString("start")
		endStr, _ := cmd.Flags().GetString("end")

		var start, end *time.Time
		if startStr != "" {
			t, err := time.Parse("2006-01-02", startStr)
			if err != nil {
				exitValidation("--start must be YYYY-MM-DD")
			}
			start = &t
		}
		if endStr != "" {
			t, err := time.Parse("2006-01-02", endStr)
			if err != nil {
				exitValidation("--end must be YYYY-MM-DD")
			}
			end = &t
		}

		sp, err := svc.CreateSprint(args[0], project, start, end)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(sp)
		} else {
			fmt.Printf("created sprint %d: %s\n", sp.ID, sp.Name)
		}
		return nil
	},
}

var sprintListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sprints for a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		sprints, err := svc.ListSprints(project)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(sprints)
			return nil
		}
		for _, sp := range sprints {
			fmt.Printf("  #%d  %-20s  %s\n", sp.ID, sp.Name, string(sp.Status))
		}
		return nil
	},
}

var sprintActivateCmd = &cobra.Command{
	Use:   "activate <sprint-id>",
	Short: "Activate a sprint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			exitValidation("sprint-id must be an integer")
		}
		if err := svc.ActivateSprint(id); err != nil {
			return err
		}
		fmt.Printf("activated sprint %d\n", id)
		return nil
	},
}

var sprintAssignCmd = &cobra.Command{
	Use:   "assign <task-id> [<sprint-id>]",
	Short: "Assign a task to a sprint (or --clear to remove)",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		clear, _ := cmd.Flags().GetBool("clear")

		var sprintID *int64
		if !clear {
			if len(args) < 2 {
				exitValidation("provide <sprint-id> or --clear")
			}
			id, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				exitValidation("sprint-id must be an integer")
			}
			sprintID = &id
		}

		task, err := svc.AssignTaskToSprint(args[0], sprintID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				exitNotFound(fmt.Sprintf("task %s not found", args[0]))
			}
			return err
		}

		if jsonFlag {
			printJSON(task)
			return nil
		}
		if sprintID == nil {
			fmt.Printf("removed %s from sprint\n", task.ID)
		} else {
			fmt.Printf("assigned %s to sprint %d\n", task.ID, *sprintID)
		}
		return nil
	},
}

func init() {
	sprintAddCmd.Flags().String("project", "", "project ID")
	sprintAddCmd.Flags().String("start", "", "start date YYYY-MM-DD")
	sprintAddCmd.Flags().String("end", "", "end date YYYY-MM-DD")
	sprintListCmd.Flags().String("project", "", "project ID")
	sprintAssignCmd.Flags().Bool("clear", false, "remove task from its sprint")
	sprintCmd.AddCommand(sprintAddCmd, sprintListCmd, sprintActivateCmd, sprintAssignCmd)
}
