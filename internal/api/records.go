package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// Records — notes, links and files — are written to this server's own
// database. They are not forwarded to a sync primary: the sync does not
// carry them (KA-072).

// SetFiles gives the server the keeper of files kept against tickets.
func (s *Server) SetFiles(k *artifacts.Keeper) { s.files = k }

func (s *Server) recordRoutes() {
	s.mux.HandleFunc("POST /api/tasks/{id}/notes", s.auth(s.handleAddNote))
	s.mux.HandleFunc("GET /api/projects/{id}/artifacts", s.auth(s.handleListArtifacts))
	s.mux.HandleFunc("POST /api/artifacts/{id}/url", s.auth(s.handleArtifactURL))
	// No bearer token: a browser loads this from <a>, <img>, <video>. The
	// short-lived signature in the query string stands in for the login.
	s.mux.HandleFunc("GET /api/artifacts/{id}/content", s.handleArtifactContent)
}

// linkTTL is how long a signed file link works.
const linkTTL = 10 * time.Minute

// linkSig signs one file ID until exp with the server's secret.
func (s *Server) linkSig(id int64, exp int64) string {
	m := hmac.New(sha256.New, []byte(s.secret))
	fmt.Fprintf(m, "artifact|%d|%d", id, exp)
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Server) handleListArtifacts(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListArtifacts(domain.ArtifactQuery{ProjectID: r.PathValue("id"), Unfiled: r.URL.Query().Get("unfiled") != ""})
	if err != nil {
		writeErr(w, statusFor(err), err.Error())
		return
	}
	if list == nil {
		list = []*domain.Artifact{}
	}
	writeJSON(w, http.StatusOK, list)
}

// handleArtifactURL hands a logged-in user a link to one file, good for a
// few minutes.
func (s *Server) handleArtifactURL(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file ID must be a number")
		return
	}
	if _, err := s.svc.GetArtifact(id); err != nil {
		writeErr(w, statusFor(err), err.Error())
		return
	}
	exp := time.Now().Add(linkTTL).Unix()
	writeJSON(w, http.StatusOK, map[string]string{
		"url": fmt.Sprintf("/api/artifacts/%d/content?exp=%d&sig=%s", id, exp, s.linkSig(id, exp)),
	})
}

func (s *Server) handleArtifactContent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	q := r.URL.Query()
	exp, expErr := strconv.ParseInt(q.Get("exp"), 10, 64)
	if err != nil || expErr != nil || time.Now().Unix() > exp ||
		!hmac.Equal([]byte(q.Get("sig")), []byte(s.linkSig(id, exp))) {
		writeErr(w, http.StatusForbidden, "this link is not valid, or has expired")
		return
	}
	if s.files == nil {
		writeErr(w, http.StatusServiceUnavailable, "this server does not keep files")
		return
	}
	a, err := s.svc.GetArtifact(id)
	if err != nil {
		writeErr(w, statusFor(err), err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	path, err := s.files.Fetch(ctx, id)
	if errors.Is(err, artifacts.ErrUnavailable) {
		writeErr(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if err != nil {
		writeErr(w, statusFor(err), err.Error())
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "file could not be read")
		return
	}
	defer f.Close()
	disposition := "inline"
	if q.Get("download") != "" {
		disposition = "attachment"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": a.Name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// A kept file is not part of this site: it may not run script here.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; media-src 'self'; style-src 'unsafe-inline'; sandbox")
	w.Header().Set("Cache-Control", "private, max-age=600")
	if a.MediaType != "" {
		w.Header().Set("Content-Type", a.MediaType)
	}
	http.ServeContent(w, r, "", a.CreatedAt, f)
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
