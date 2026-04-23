package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"keroagile/internal/domain"
)

var taskCmd = &cobra.Command{Use: "task", Short: "Manage tasks"}

var taskAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Create a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			project = cfg.DefaultProject
		}
		if project == "" {
			exitValidation("--project is required (or set default_project in config)")
		}

		assignee, _ := cmd.Flags().GetString("assignee")
		if assignee == "" {
			assignee = cfg.DefaultAssignee
		}
		priorityStr, _ := cmd.Flags().GetString("priority")
		statusStr, _ := cmd.Flags().GetString("status")
		labelsStr, _ := cmd.Flags().GetString("labels")
		desc, _ := cmd.Flags().GetString("description")

		var labels []string
		if labelsStr != "" {
			labels = strings.Split(labelsStr, ",")
		}

		opts := domain.TaskCreateOpts{
			AssigneeID: assignee,
			Labels:     labels,
		}
		if priorityStr != "" {
			opts.Priority = domain.Priority(priorityStr)
		}
		if statusStr != "" {
			opts.Status = domain.Status(statusStr)
		}
		if cmd.Flags().Changed("points") {
			pts, _ := cmd.Flags().GetInt("points")
			opts.Points = &pts
		}

		t, err := svc.CreateTask(args[0], desc, project, opts)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(t)
		} else {
			fmt.Printf("created %s: %s\n", t.ID, t.Title)
		}
		return nil
	},
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			project = cfg.DefaultProject
		}
		statusStr, _ := cmd.Flags().GetString("status")
		assignee, _ := cmd.Flags().GetString("assignee")

		f := domain.TaskFilters{}
		if statusStr != "" {
			s := domain.Status(statusStr)
			f.Status = &s
		}
		if assignee != "" {
			f.AssigneeID = &assignee
		}

		tasks, err := svc.ListTasks(project, f)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(tasks)
			return nil
		}
		for _, t := range tasks {
			assigneeStr := ""
			if t.AssigneeID != nil {
				assigneeStr = " @" + *t.AssigneeID
			}
			fmt.Printf("  %s  %-30s  %-12s  %-8s%s\n",
				t.ID, t.Title, t.Status.Label(), t.Priority.Label(), assigneeStr)
		}
		return nil
	},
}

var taskGetCmd = &cobra.Command{
	Use:   "get <task-id>",
	Short: "Show task details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		t, err := svc.GetTask(args[0])
		if err != nil {
			exitNotFound(args[0])
		}
		if jsonFlag {
			printJSON(t)
			return nil
		}
		fmt.Printf("%s  %s\n", t.ID, t.Title)
		fmt.Printf("  Status:   %s\n", t.Status.Label())
		fmt.Printf("  Priority: %s\n", t.Priority.Label())
		if t.AssigneeID != nil {
			fmt.Printf("  Assignee: %s\n", *t.AssigneeID)
		}
		if t.Points != nil {
			fmt.Printf("  Points:   %d\n", *t.Points)
		}
		if t.Branch != "" {
			fmt.Printf("  Branch:   %s\n", t.Branch)
		}
		if len(t.Blockers) > 0 {
			fmt.Printf("  Blockers: %s\n", strings.Join(t.Blockers, ", "))
		}
		return nil
	},
}

var taskMoveCmd = &cobra.Command{
	Use:   "move <task-id> <status>",
	Short: "Move task to a new status",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		t, err := svc.MoveTask(args[0], domain.Status(args[1]))
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(t)
		} else {
			fmt.Printf("%s → %s\n", t.ID, t.Status.Label())
		}
		return nil
	},
}

var taskLinkBranchCmd = &cobra.Command{
	Use:   "link-branch <task-id> <branch>",
	Short: "Link a git branch to a task",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := svc.LinkBranch(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("linked %s → branch %s\n", args[0], args[1])
		return nil
	},
}

var taskLinkPRCmd = &cobra.Command{
	Use:   "link-pr <task-id> <pr-number>",
	Short: "Link a GitHub PR to a task",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		prNum, err := strconv.Atoi(args[1])
		if err != nil {
			exitValidation("pr-number must be an integer")
		}
		if err := svc.LinkPR(args[0], prNum); err != nil {
			return err
		}
		fmt.Printf("linked %s → PR #%d\n", args[0], prNum)
		return nil
	},
}

var taskDeleteCmd = &cobra.Command{
	Use:   "delete <task-id>",
	Short: "Delete a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := svc.DeleteTask(args[0]); err != nil {
			return err
		}
		fmt.Printf("deleted %s\n", args[0])
		return nil
	},
}

func init() {
	taskAddCmd.Flags().String("project", "", "project ID")
	taskAddCmd.Flags().String("assignee", "", "assignee user ID")
	taskAddCmd.Flags().String("priority", "", "low|medium|high|critical")
	taskAddCmd.Flags().String("status", "", "backlog|todo|in_progress|review|done")
	taskAddCmd.Flags().Int("points", 0, "story points")
	taskAddCmd.Flags().String("labels", "", "comma-separated labels")
	taskAddCmd.Flags().String("description", "", "task description")

	taskListCmd.Flags().String("project", "", "project ID")
	taskListCmd.Flags().String("status", "", "filter by status")
	taskListCmd.Flags().String("assignee", "", "filter by assignee")

	taskCmd.AddCommand(taskAddCmd, taskListCmd, taskGetCmd, taskMoveCmd,
		taskLinkBranchCmd, taskLinkPRCmd, taskDeleteCmd)
}
