package main

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/tbdtechpro/KeroAgile/internal/api"
	"golang.org/x/term"
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

var userSetPasswordCmd = &cobra.Command{
	Use:   "set-password <user-id>",
	Short: "Set a user's API password (for KeroAgile serve)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		userID := args[0]
		if _, err := svc.GetUser(userID); err != nil {
			return fmt.Errorf("user %q not found", userID)
		}
		fmt.Printf("New password for %s: ", userID)
		raw, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return err
		}
		if len(raw) == 0 {
			return fmt.Errorf("password cannot be empty")
		}
		hash, err := api.HashPassword(string(raw))
		if err != nil {
			return err
		}
		if err := svc.SetUserPasswordHash(userID, hash); err != nil {
			return err
		}
		fmt.Printf("password set for %s\n", userID)
		return nil
	},
}

func init() {
	userAddCmd.Flags().Bool("agent", false, "mark as AI agent")
	userCmd.AddCommand(userAddCmd, userListCmd, userSetPasswordCmd)
}
