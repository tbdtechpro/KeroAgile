package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/syncsrv"
)

// isPrivateHost returns true for loopback, link-local, and RFC-1918 addresses.
func isPrivateHost(ctx context.Context, host string) bool {
	private := []string{
		"localhost",
		"127.", "10.", "192.168.",
		"172.16.", "172.17.", "172.18.", "172.19.",
		"172.20.", "172.21.", "172.22.", "172.23.",
		"172.24.", "172.25.", "172.26.", "172.27.",
		"172.28.", "172.29.", "172.30.", "172.31.",
		"169.254.", "::1",
	}
	h := strings.ToLower(host)
	for _, p := range private {
		if h == p || strings.HasPrefix(h, p) {
			return true
		}
	}
	// Also check if the host resolves to a private IP.
	resolver := &net.Resolver{}
	ips, err := resolver.LookupHost(ctx, h)
	if err != nil {
		return false
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
			return true
		}
	}
	return false
}

// handleSyncStatus returns the current sync connection state.
func (s *Server) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	if s.syncClient == nil {
		writeJSON(w, http.StatusOK, map[string]string{"state": "standalone"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"state": string(s.syncClient.State())})
}

// storeWithSyncOrigin is satisfied by *store.Store — it has SetSyncOrigin directly.
type storeWithSyncOrigin interface {
	SetSyncOrigin(projectID, origin string) error
}

// handleSyncJoin fetches a snapshot from a primary and writes it to the local store.
func (s *Server) handleSyncJoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PrimaryURL string `json:"primary_url"`
		APIToken   string `json:"api_token"`
		ProjectID  string `json:"project_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.PrimaryURL == "" || req.APIToken == "" || req.ProjectID == "" {
		writeErr(w, http.StatusBadRequest, "primary_url, api_token, and project_id are required")
		return
	}
	if s.rawStore == nil {
		writeErr(w, http.StatusInternalServerError, "store not available")
		return
	}

	// Normalize project ID to uppercase.
	req.ProjectID = strings.ToUpper(req.ProjectID)

	// Validate primary_url before making outbound request.
	parsed, err := url.Parse(req.PrimaryURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		writeErr(w, http.StatusBadRequest, "primary_url must be http or https")
		return
	}
	if isPrivateHost(r.Context(), parsed.Hostname()) {
		writeErr(w, http.StatusBadRequest, "primary_url must not be a private or loopback address")
		return
	}

	// Fetch snapshot from primary.
	snapshotURL := req.PrimaryURL + "/api/sync/snapshot?project_ids=" + req.ProjectID
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, snapshotURL, nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "build request: "+err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.APIToken)

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "primary unreachable: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeErr(w, http.StatusBadGateway, "primary returned "+resp.Status)
		return
	}

	var snap struct {
		Projects []*domain.Project `json:"projects"`
		Tasks    []*domain.Task    `json:"tasks"`
		Sprints  []*domain.Sprint  `json:"sprints"`
		Users    []*domain.User    `json:"users"`
		Cursor   int64             `json:"cursor"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		writeErr(w, http.StatusBadGateway, "decode snapshot: "+err.Error())
		return
	}

	// Write snapshot entities to local store (upsert pattern).
	for _, p := range snap.Projects {
		existing, _ := s.svc.GetProject(p.ID)
		if existing == nil {
			if err := s.svc.CreateProject(p.ID, p.Name, p.RepoPath); err != nil {
				writeErr(w, http.StatusInternalServerError, "write project "+p.ID+": "+err.Error())
				return
			}
		}
	}
	for _, u := range snap.Users {
		existing, _ := s.rawStore.GetUser(u.ID)
		if existing == nil {
			if err := s.rawStore.CreateUser(u); err != nil {
				writeErr(w, http.StatusInternalServerError, "write user "+u.ID+": "+err.Error())
				return
			}
		}
	}
	for _, sp := range snap.Sprints {
		existing, _ := s.rawStore.GetSprint(sp.ID)
		if existing == nil {
			if _, err := s.rawStore.CreateSprint(sp); err != nil {
				writeErr(w, http.StatusInternalServerError, "write sprint: "+err.Error())
				return
			}
		} else {
			if err := s.rawStore.UpdateSprint(sp); err != nil {
				writeErr(w, http.StatusInternalServerError, "update sprint: "+err.Error())
				return
			}
		}
	}
	for _, t := range snap.Tasks {
		existing, _ := s.rawStore.GetTask(t.ID)
		if existing == nil {
			if err := s.rawStore.CreateTask(t); err != nil {
				writeErr(w, http.StatusInternalServerError, "write task "+t.ID+": "+err.Error())
				return
			}
		} else {
			if err := s.rawStore.UpdateTask(t); err != nil {
				writeErr(w, http.StatusInternalServerError, "update task "+t.ID+": "+err.Error())
				return
			}
		}
	}

	// Record sync origin and cursor on the project.
	// SetSyncOrigin is on *store.Store directly; SetProjectSyncCursor is on syncsrv.SecondaryStore.
	if originSt, ok := s.rawStore.(storeWithSyncOrigin); ok {
		if err := originSt.SetSyncOrigin(req.ProjectID, req.PrimaryURL); err != nil {
			writeErr(w, http.StatusInternalServerError, "set sync origin: "+err.Error())
			return
		}
	}
	if secSt, ok := s.rawStore.(syncsrv.SecondaryStore); ok {
		_ = secSt.SetProjectSyncCursor(req.ProjectID, snap.Cursor)
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"project_id": req.ProjectID,
		"note":       "snapshot applied; restart server to begin streaming",
	})
}

// handleSyncHeartbeat confirms the sync token is valid and the server is alive.
func (s *Server) handleSyncHeartbeat(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"ok": "true",
		"ts": time.Now().UTC().Format(time.RFC3339),
	})
}

// handleSyncSnapshot returns a full snapshot of all data for the granted projects.
// Query param: project_ids (repeated) — must all be granted to the calling secondary.
func (s *Server) handleSyncSnapshot(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeErr(w, http.StatusNotFound, "sync not configured")
		return
	}
	secID, _ := r.Context().Value(ctxKeySecondaryID).(string)

	projectIDs := r.URL.Query()["project_ids"]
	if len(projectIDs) == 0 {
		writeErr(w, http.StatusBadRequest, "project_ids is required")
		return
	}

	// Verify all requested projects are granted to this secondary.
	for _, pid := range projectIDs {
		ok, err := s.store.IsGranted(secID, pid)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeErr(w, http.StatusForbidden, fmt.Sprintf("project %s is not granted", pid))
			return
		}
	}

	projects, err := s.svc.ListProjects()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Filter to only the requested projects.
	grantedSet := make(map[string]bool, len(projectIDs))
	for _, pid := range projectIDs {
		grantedSet[pid] = true
	}

	var filteredProjects []*domain.Project
	for _, p := range projects {
		if grantedSet[p.ID] {
			filteredProjects = append(filteredProjects, p)
		}
	}

	// Gather tasks, sprints, and compute max cursor across all requested projects.
	var allTasks []*domain.Task
	var allSprints []*domain.Sprint
	var maxCursor int64

	for _, pid := range projectIDs {
		tasks, err := s.svc.ListTasks(pid, domain.TaskFilters{})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		allTasks = append(allTasks, tasks...)

		sprints, err := s.svc.ListSprints(pid)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		allSprints = append(allSprints, sprints...)

		changes, err := s.store.ReadChanges(pid, 0)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, c := range changes {
			if c.Cursor > maxCursor {
				maxCursor = c.Cursor
			}
		}
	}

	users, err := s.svc.ListUsers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if allTasks == nil {
		allTasks = []*domain.Task{}
	}
	if allSprints == nil {
		allSprints = []*domain.Sprint{}
	}
	if filteredProjects == nil {
		filteredProjects = []*domain.Project{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"projects": filteredProjects,
		"tasks":    allTasks,
		"sprints":  allSprints,
		"users":    users,
		"cursor":   maxCursor,
	})
}

// handleSyncStream pushes change_log rows to the secondary via SSE.
// Query param: project_ids (repeated), since (optional int64 cursor).
func (s *Server) handleSyncStream(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeErr(w, http.StatusNotFound, "sync not configured")
		return
	}
	secID, _ := r.Context().Value(ctxKeySecondaryID).(string)

	projectIDs := r.URL.Query()["project_ids"]
	if len(projectIDs) == 0 {
		writeErr(w, http.StatusBadRequest, "project_ids is required")
		return
	}

	// Verify grants.
	for _, pid := range projectIDs {
		ok, err := s.store.IsGranted(secID, pid)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeErr(w, http.StatusForbidden, fmt.Sprintf("project %s is not granted", pid))
			return
		}
	}

	var cursor int64
	if since := r.URL.Query().Get("since"); since != "" {
		cursor, _ = strconv.ParseInt(since, 10, 64)
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	pollTicker := time.NewTicker(500 * time.Millisecond)
	defer pollTicker.Stop()
	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeatTicker.C:
			fmt.Fprintf(w, "event: heartbeat\ndata: {\"ts\":%q}\n\n", time.Now().UTC().Format(time.RFC3339))
			flusher.Flush()
		case <-pollTicker.C:
			for _, pid := range projectIDs {
				changes, err := s.store.ReadChanges(pid, cursor)
				if err != nil {
					continue
				}
				for _, c := range changes {
					data, err := json.Marshal(c)
					if err != nil {
						continue
					}
					fmt.Fprintf(w, "event: change\ndata: %s\n\n", data)
					if c.Cursor > cursor {
						cursor = c.Cursor
					}
				}
			}
			flusher.Flush()
		}
	}
}

// handleListSecondaries returns all registered secondaries.
func (s *Server) handleListSecondaries(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeErr(w, http.StatusNotFound, "sync not configured")
		return
	}
	secondaries, err := s.store.ListSecondaries()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if secondaries == nil {
		secondaries = []*syncsrv.Secondary{}
	}
	writeJSON(w, http.StatusOK, secondaries)
}

// handleAddSecondary registers a new secondary and returns its token.
func (s *Server) handleAddSecondary(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeErr(w, http.StatusNotFound, "sync not configured")
		return
	}
	var req struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.ID == "" || req.DisplayName == "" {
		writeErr(w, http.StatusBadRequest, "id and display_name are required")
		return
	}
	token, err := s.store.AddSecondary(req.ID, req.DisplayName)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"id":    req.ID,
		"token": token,
	})
}

// handleRevokeSecondary deletes a secondary registration.
func (s *Server) handleRevokeSecondary(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeErr(w, http.StatusNotFound, "sync not configured")
		return
	}
	id := r.PathValue("id")
	if err := s.store.RevokeSecondary(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListGrants lists the projects granted to a secondary.
func (s *Server) handleListGrants(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeErr(w, http.StatusNotFound, "sync not configured")
		return
	}
	id := r.PathValue("id")
	projects, err := s.store.ListGrantedProjects(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if projects == nil {
		projects = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"project_ids": projects})
}

// handleGrantProject grants a project to a secondary.
func (s *Server) handleGrantProject(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeErr(w, http.StatusNotFound, "sync not configured")
		return
	}
	secondaryID := r.PathValue("secondary")
	projectID := r.PathValue("project")
	if err := s.store.GrantProject(secondaryID, projectID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleRevokeGrant removes a project grant from a secondary.
func (s *Server) handleRevokeGrant(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeErr(w, http.StatusNotFound, "sync not configured")
		return
	}
	secondaryID := r.PathValue("secondary")
	projectID := r.PathValue("project")
	if err := s.store.RevokeGrant(secondaryID, projectID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
