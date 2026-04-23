package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"keroagile/internal/config"
	"keroagile/internal/domain"
	"keroagile/internal/store"
)

var (
	jsonFlag bool
	svc      *domain.Service
	cfg      *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "KeroAgile",
	Short: "Terminal agile board",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "help" {
			return nil
		}
		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("config: %w", err)
		}

		dbPath := config.DBPath()
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return err
		}
		db, err := store.Open(dbPath)
		if err != nil {
			return fmt.Errorf("db: %w", err)
		}
		svc = domain.NewService(store.New(db))

		if !isTerminal(os.Stdout) {
			jsonFlag = true
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("TUI launching... (Plan B)")
		return nil
	},
}

func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "output JSON")
	rootCmd.AddCommand(projectCmd, taskCmd, sprintCmd, userCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
