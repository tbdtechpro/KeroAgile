package api

import (
	"encoding/json"
	"net/http"

	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
)

// Records — notes, links and files — are written to this server's own
// database. They are not forwarded to a sync primary: the sync does not
// carry them (KA-072).

// SetFiles gives the server the keeper of files kept against tickets.
func (s *Server) SetFiles(k *artifacts.Keeper) { s.files = k }

func (s *Server) recordRoutes() {
	s.mux.HandleFunc("POST /api/tasks/{id}/notes", s.auth(s.handleAddNote))
}

// userID is the logged-in user of an authenticated request.
func userID(r *http.Request) string {
	id, _ := r.Context().Value(ctxKeyUserID).(string)
	return id
}

func (s *Server) handleAddNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	n, err := s.svc.AddNote(r.PathValue("id"), userID(r), req.Body)
	if err != nil {
		writeErr(w, statusFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, n)
}
