package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/syncsrv"
)

type ctxKey int

const (
	ctxKeyUserID      ctxKey = 0
	ctxKeySecondaryID ctxKey = 1
)

// Server is the KeroAgile HTTP API server.
type Server struct {
	svc        *domain.Service
	store      syncsrv.PrimaryStore
	rawStore   domain.Store          // for entity upserts during snapshot ingestion
	syncClient *syncsrv.Client // non-nil on secondary installs; nil for standalone/primary
	secret     string
	syncMode   syncsrv.Mode
	mux        *http.ServeMux
}

// New creates a Server and registers all routes. syncClient must be non-nil when mode is
// ModeSecondary so that write proxying and 503-on-offline work; pass nil for standalone/primary.
func New(svc *domain.Service, st syncsrv.PrimaryStore, rawSt domain.Store, secret string, mode syncsrv.Mode, syncClient *syncsrv.Client) *Server {
	s := &Server{svc: svc, store: st, rawStore: rawSt, syncClient: syncClient, secret: secret, syncMode: mode, mux: http.NewServeMux()}
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

	s.mux.HandleFunc("GET /api/search/tasks", s.auth(s.handleSearchTasks))
	s.mux.HandleFunc("POST /api/tasks/{id}/blockers", s.auth(s.handleAddBlocker))
	s.mux.HandleFunc("DELETE /api/tasks/{id}/blockers/{blocker_id}", s.auth(s.handleRemoveBlocker))

	s.mux.HandleFunc("GET /api/users", s.auth(s.handleListUsers))

	s.mux.HandleFunc("GET /api/sprints", s.auth(s.handleListSprints))
	s.mux.HandleFunc("POST /api/sprints", s.auth(s.handleCreateSprint))
	s.mux.HandleFunc("GET /api/sprints/{id}", s.auth(s.handleGetSprint))

	// Sync routes — secondary-side status and join
	s.mux.HandleFunc("GET /api/sync/status", s.auth(s.handleSyncStatus))
	s.mux.HandleFunc("POST /api/sync/join", s.auth(s.handleSyncJoin))

	// Sync routes
	s.mux.HandleFunc("GET /api/sync/heartbeat", s.syncAuth(s.handleSyncHeartbeat))
	s.mux.HandleFunc("GET /api/sync/snapshot", s.syncAuth(s.handleSyncSnapshot))
	s.mux.HandleFunc("GET /api/sync/stream", s.syncAuth(s.handleSyncStream))
	s.mux.HandleFunc("GET /api/sync/secondaries", s.auth(s.handleListSecondaries))
	s.mux.HandleFunc("POST /api/sync/secondaries", s.auth(s.handleAddSecondary))
	s.mux.HandleFunc("DELETE /api/sync/secondaries/{id}", s.auth(s.handleRevokeSecondary))
	s.mux.HandleFunc("GET /api/sync/secondaries/{id}/grants", s.auth(s.handleListGrants))
	s.mux.HandleFunc("PUT /api/sync/grants/{secondary}/{project}", s.auth(s.handleGrantProject))
	s.mux.HandleFunc("DELETE /api/sync/grants/{secondary}/{project}", s.auth(s.handleRevokeGrant))
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
			// Also accept sync tokens so proxied writes from a secondary can reach
			// this primary's task/sprint endpoints using the secondary's sync token.
			if s.store != nil {
				sec, serr := s.store.GetSecondaryByTokenHash(syncsrv.SHA256Hex(token))
				if serr == nil && sec != nil {
					_ = s.store.TouchSecondary(sec.ID)
					// Only allow proxied writes (secondary always sets X-Sync-Origin).
					if r.Header.Get("X-Sync-Origin") == "" {
						writeErr(w, http.StatusForbidden, "sync tokens cannot authenticate user routes directly")
						return
					}
					r = r.WithContext(context.WithValue(r.Context(), ctxKeySecondaryID, sec.ID))
					next(w, r)
					return
				}
			}
			writeErr(w, http.StatusUnauthorized, "invalid token")
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyUserID, userID))
		next(w, r)
	}
}

func (s *Server) syncAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.store == nil {
			writeErr(w, http.StatusNotFound, "sync not configured")
			return
		}
		bearer := r.Header.Get("Authorization")
		token := strings.TrimPrefix(bearer, "Bearer ")
		if token == "" || token == bearer {
			writeErr(w, http.StatusUnauthorized, "missing token")
			return
		}
		sec, err := s.store.GetSecondaryByTokenHash(syncsrv.SHA256Hex(token))
		if err != nil || sec == nil {
			writeErr(w, http.StatusUnauthorized, "invalid sync token")
			return
		}
		_ = s.store.TouchSecondary(sec.ID)
		r = r.WithContext(context.WithValue(r.Context(), ctxKeySecondaryID, sec.ID))
		next(w, r)
	}
}

// StoreForTest returns the primary store for test assertions.
func (s *Server) StoreForTest() syncsrv.PrimaryStore { return s.store }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
