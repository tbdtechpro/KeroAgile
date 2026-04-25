package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/tbdtechpro/KeroAgile/internal/api"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP API server (port 7432)",
	Long: `Starts the KeroAgile REST API server.

Endpoints:
  POST /api/auth/login        Obtain a JWT token
  GET  /api/projects          List projects
  POST /api/projects          Create a project
  GET  /api/tasks             List tasks (filters: project_id, status, assignee_id, sprint_id)
  POST /api/tasks             Create a task
  GET  /api/tasks/{id}        Get a task
  PATCH /api/tasks/{id}       Update a task
  DELETE /api/tasks/{id}      Delete a task
  GET  /api/users             List users
  GET  /api/sprints           List sprints (filter: project_id)
  POST /api/sprints           Create a sprint
  GET  /api/sprints/{id}      Get a sprint

Authentication:
  Users log in with POST /api/auth/login using their user ID and password.
  Set a user password with: KeroAgile user set-password <user-id>
  All other endpoints require: Authorization: Bearer <token>

Configuration:
  Set api_secret in ~/.config/keroagile/config.toml for persistent tokens.
  Without it, a random secret is generated on each startup (tokens expire on restart).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _ := cmd.Flags().GetString("addr")

		secret := cfg.APISecret
		if secret == "" {
			secret = api.RandomSecret()
			fmt.Println("⚠  No api_secret configured — tokens will expire on restart.")
			fmt.Println("   Add to ~/.config/keroagile/config.toml:")
			fmt.Printf("   api_secret = %q\n\n", secret)
		}

		srv := api.New(svc, secret)
		log.Printf("KeroAgile API listening on %s", addr)
		return http.ListenAndServe(addr, srv)
	},
}

func init() {
	serveCmd.Flags().String("addr", ":7432", "address to listen on")
	rootCmd.AddCommand(serveCmd)
}
