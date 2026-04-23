package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "KeroAgile",
	Short: "Terminal agile board",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("TUI coming in Plan B")
		return nil
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
