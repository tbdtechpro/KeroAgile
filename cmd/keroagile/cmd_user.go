package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{Use: "user", Short: "Manage team members"}

var userAddCmd = &cobra.Command{
	Use:   "add <id> <display-name>",
	Short: "Add a team member",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		isAgent, _ := cmd.Flags().GetBool("agent")
		u, err := svc.CreateUser(args[0], args[1], isAgent)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(u)
		} else {
			fmt.Printf("added user %s (%s)\n", u.ID, u.DisplayName)
		}
		return nil
	},
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List team members",
	RunE: func(cmd *cobra.Command, args []string) error {
		users, err := svc.ListUsers()
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(users)
			return nil
		}
		for _, u := range users {
			prefix := "👤"
			if u.IsAgent {
				prefix = "🤖"
			}
			fmt.Printf("  %s %s  %s\n", prefix, u.ID, u.DisplayName)
		}
		return nil
	},
}

func init() {
	userAddCmd.Flags().Bool("agent", false, "mark as AI agent")
	userCmd.AddCommand(userAddCmd, userListCmd)
}
