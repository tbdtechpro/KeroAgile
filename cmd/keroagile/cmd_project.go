package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{Use: "project", Short: "Manage projects"}

var projectAddCmd = &cobra.Command{
	Use:   "add <id> <name>",
	Short: "Create a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		if err := svc.CreateProject(args[0], args[1], repo); err != nil {
			return err
		}
		p, _ := svc.GetProject(args[0])
		if jsonFlag {
			printJSON(p)
		} else {
			fmt.Printf("created project %s (%s)\n", p.ID, p.Name)
		}
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		projects, err := svc.ListProjects()
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(projects)
			return nil
		}
		for _, p := range projects {
			sprint := ""
			if p.SprintMode {
				sprint = " [sprints]"
			}
			fmt.Printf("  %s  %s%s  %s\n", p.ID, p.Name, sprint, p.RepoPath)
		}
		return nil
	},
}

var projectSprintCmd = &cobra.Command{
	Use:   "set-sprint <project-id> on|off",
	Short: "Enable or disable sprint mode for a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		enabled := args[1] == "on"
		if err := svc.SetSprintMode(args[0], enabled); err != nil {
			return err
		}
		fmt.Printf("sprint mode %s for %s\n", args[1], args[0])
		return nil
	},
}

func init() {
	projectAddCmd.Flags().String("repo", "", "absolute path to git repo")
	projectCmd.AddCommand(projectAddCmd, projectListCmd, projectSprintCmd)
}
