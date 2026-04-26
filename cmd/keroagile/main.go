package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tbdtechpro/KeroAgile/internal/config"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/store"
	"github.com/tbdtechpro/KeroAgile/internal/tui"
	"golang.org/x/term"
)

var (
	jsonFlag bool
	svc      *domain.Service
	st       *store.Store
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
		st = store.New(db)
		svc = domain.NewService(st)

		if !isTerminal(os.Stdout) {
			jsonFlag = true
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		app := tui.New(svc, cfg.DefaultAssignee)
		return app.Run()
	},
}

func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "output JSON")
	rootCmd.AddCommand(projectCmd, taskCmd, sprintCmd, userCmd, mcpCmd, initCmd, syncStatusCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
