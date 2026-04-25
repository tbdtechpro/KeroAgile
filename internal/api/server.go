package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

type ctxKey int

const ctxKeyUserID ctxKey = 0

// Server is the KeroAgile HTTP API server.
type Server struct {
	svc    *domain.Service
	secret string
	mux    *http.ServeMux
}

// New creates a Server and registers all routes.
func New(svc *domain.Service, secret string) *Server {
	s := &Server{svc: svc, secret: secret, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)

	s.mux.HandleFunc("GET /api/projects", s.auth(s.handleListProjects))
	s.mux.HandleFunc("POST /api/projects", s.auth(s.handleCreateProject))

	s.mux.HandleFunc("GET /api/tasks", s.auth(s.handleListTasks))
	s.mux.HandleFunc("POST /api/tasks", s.auth(s.handleCreateTask))
	s.mux.HandleFunc("GET /api/tasks/{id}", s.auth(s.handleGetTask))
	s.mux.HandleFunc("PATCH /api/tasks/{id}", s.auth(s.handleUpdateTask))
	s.mux.HandleFunc("DELETE /api/tasks/{id}", s.auth(s.handleDeleteTask))

	s.mux.HandleFunc("GET /api/users", s.auth(s.handleListUsers))

	s.mux.HandleFunc("GET /api/sprints", s.auth(s.handleListSprints))
	s.mux.HandleFunc("POST /api/sprints", s.auth(s.handleCreateSprint))
	s.mux.HandleFunc("GET /api/sprints/{id}", s.auth(s.handleGetSprint))
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bearer := r.Header.Get("Authorization")
		token := strings.TrimPrefix(bearer, "Bearer ")
		if token == "" || token == bearer {
			writeErr(w, http.StatusUnauthorized, "missing token")
			return
		}
		userID, err := validateToken(token, s.secret)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid token")
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyUserID, userID))
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
