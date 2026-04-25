package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/tbdtechpro/KeroAgile/internal/api"
	webstatic "github.com/tbdtechpro/KeroAgile/internal/web"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP API + web UI server (port 7432)",
	Long: `Starts the KeroAgile REST API and embedded web UI.

  API routes are served at /api/ (JWT auth required).
  The React board UI is served at / (login page handles auth).

API endpoints:
  POST /api/auth/login        Obtain a JWT token
  GET  /api/projects          List projects
  POST /api/projects          Create a project
  GET  /api/tasks             List tasks
  POST /api/tasks             Create a task
  GET  /api/tasks/{id}        Get a task
  PATCH /api/tasks/{id}       Update / move a task
  DELETE /api/tasks/{id}      Delete a task
  GET  /api/users             List users
  GET  /api/sprints           List sprints
  POST /api/sprints           Create a sprint
  GET  /api/sprints/{id}      Get a sprint

Authentication:
  Set a user password with: KeroAgile user set-password <user-id>
  Log in via the web UI or: POST /api/auth/login {"user_id":"...","password":"..."}

Configuration:
  Add to ~/.config/keroagile/config.toml:
    api_secret = "your-random-secret"   # makes tokens persistent across restarts`,
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _ := cmd.Flags().GetString("addr")

		secret := cfg.APISecret
		if secret == "" {
			secret = api.RandomSecret()
			fmt.Println("⚠  No api_secret configured — tokens will expire on restart.")
			fmt.Println("   Add to ~/.config/keroagile/config.toml:")
			fmt.Printf("   api_secret = %q\n\n", secret)
		}

		mux := http.NewServeMux()

		// API routes
		apiSrv := api.New(svc, secret)
		mux.Handle("/api/", apiSrv)

		// Embedded React web UI — SPA with client-side routing
		mux.Handle("/", spaHandler(webstatic.FS()))

		log.Printf("KeroAgile listening on http://%s", addr)
		return http.ListenAndServe(addr, mux)
	},
}

// spaHandler serves static files; falls back to index.html for any path not
// found (so React Router's client-side routes work without 404s).
func spaHandler(fsys http.FileSystem) http.Handler {
	fileServer := http.FileServer(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try serving the requested file
		f, err := fsys.Open(r.URL.Path)
		if err != nil {
			// File not found: serve index.html for SPA routing
			r2 := *r
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, &r2)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
}

func init() {
	serveCmd.Flags().String("addr", ":7432", "address to listen on")
	rootCmd.AddCommand(serveCmd)
}
