package main

import (
	"bufio"
	"fmt"
	"io"
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

		name, err := promptLine(r, "Your display name: ")
		if err != nil {
			return fmt.Errorf("stdin closed before display name was provided")
		}
		if strings.TrimSpace(name) == "" {
			exitValidation("display name is required")
		}

		var id string
		for {
			raw, err := promptLine(r, "Your user ID (e.g. matt): ")
			if err != nil {
				return fmt.Errorf("stdin closed before user ID was provided")
			}
			id = strings.TrimSpace(raw)
			if validID.MatchString(id) {
				break
			}
			fmt.Println("  ID must be lowercase letters, numbers, and hyphens (e.g. matt, john-doe)")
		}

		var project string
		for {
			raw, err := promptLine(r, "Default project ID (optional, press enter to skip): ")
			if err != nil {
				break
			}
			project = strings.TrimSpace(raw)
			if project == "" || validID.MatchString(strings.ToLower(project)) {
				break
			}
			fmt.Println("  Project ID must be letters, numbers, and hyphens (e.g. KA, MY-APP) — or press enter to skip")
		}

		raw, _ := promptLine(r, "Add Claude as an agent user for coding tasks? [Y/n]: ")
		addClaude := strings.ToLower(strings.TrimSpace(raw))
		wantClaude := addClaude == "" || addClaude == "y" || addClaude == "yes"

		fmt.Println()

		if _, err := svc.CreateUser(id, name, false); err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				fmt.Printf("  note: user %q already exists\n", id)
			} else {
				fmt.Printf("  note: skipping user creation (%v)\n", err)
			}
		} else {
			fmt.Printf("  ✓ Created user %s (%s)\n", id, name)
		}

		if wantClaude {
			if _, err := svc.CreateUser("claude", "Claude", true); err != nil {
				if strings.Contains(err.Error(), "UNIQUE constraint failed") {
					fmt.Println("  note: agent user 'claude' already exists")
				} else {
					fmt.Printf("  note: skipping agent creation (%v)\n", err)
				}
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

func promptLine(r *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return strings.TrimRight(line, "\r\n"), err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
