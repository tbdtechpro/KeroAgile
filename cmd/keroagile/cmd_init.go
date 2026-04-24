package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tbdtechpro/KeroAgile/internal/config"
)

var validID = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,29}[a-z0-9])?$`)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up KeroAgile for the first time",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := bufio.NewReader(os.Stdin)

		fmt.Println("Welcome to KeroAgile!")
		fmt.Println("─────────────────────────────────────────")

		name := prompt(r, "Your display name: ")
		if strings.TrimSpace(name) == "" {
			exitValidation("display name is required")
		}

		var id string
		for {
			id = strings.TrimSpace(prompt(r, "Your user ID (e.g. matt): "))
			if validID.MatchString(id) {
				break
			}
			fmt.Println("  ID must be lowercase letters, numbers, and hyphens (e.g. matt, john-doe)")
		}

		project := strings.TrimSpace(prompt(r, "Default project ID (optional, press enter to skip): "))

		addClaude := strings.ToLower(strings.TrimSpace(prompt(r, "Add Claude as an agent user for coding tasks? [Y/n]: ")))
		wantClaude := addClaude == "" || addClaude == "y" || addClaude == "yes"

		fmt.Println()

		if _, err := svc.CreateUser(id, name, false); err != nil {
			fmt.Printf("  note: skipping user creation (%v)\n", err)
		} else {
			fmt.Printf("  ✓ Created user %s (%s)\n", id, name)
		}

		if wantClaude {
			if _, err := svc.CreateUser("claude", "Claude", true); err != nil {
				fmt.Printf("  note: skipping agent creation (%v)\n", err)
			} else {
				fmt.Println("  ✓ Created agent user claude (Claude)")
			}
		}

		cfg = &config.Config{
			DefaultAssignee: id,
			DefaultProject:  project,
		}
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("  ✓ Set default assignee: %s\n", id)
		if project != "" {
			fmt.Printf("  ✓ Set default project: %s\n", project)
		}

		fmt.Println()
		fmt.Printf("You're all set, %s! Run `KeroAgile` to open the board.\n", name)
		return nil
	},
}

func prompt(r *bufio.Reader, label string) string {
	fmt.Print(label)
	line, _ := r.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}
