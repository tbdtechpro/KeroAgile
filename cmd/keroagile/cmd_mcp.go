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

KeroAgile finds the active project from the working directory (the project
whose directory holds it) or, failing that, the git remote. Set a project's
directory with 'KeroAgile project set <id> --dir <path>'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		env := mcp.Env{Svc: svc}
		// A bad [artifacts] section must not take the whole server down:
		// the file tools report it instead.
		env.Files, env.FilesErr = newKeeper()
		return mcp.Serve(env)
	},
}
