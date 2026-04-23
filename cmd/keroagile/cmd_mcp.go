package main

import (
	"github.com/spf13/cobra"
	"github.com/tbdtechpro/KeroAgile/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for Claude Code integration",
	Long: `Start a JSON-RPC 2.0 MCP server over stdio.

Add to ~/.claude/settings.json for global Claude Code integration:

  {
    "mcpServers": {
      "keroagile": {
        "type": "stdio",
        "command": "/path/to/KeroAgile",
        "args": ["mcp"]
      }
    }
  }

KeroAgile auto-detects the active project from the git remote URL.
Run 'KeroAgile project add --repo <remote-url>' to link a project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return mcp.Serve(svc)
	},
}
