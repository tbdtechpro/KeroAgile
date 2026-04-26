package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/syncsrv"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string `json:"user_id"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	user, err := s.svc.GetUser(req.UserID)
	if err != nil || user.PasswordHash == "" {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if CheckPassword(user.PasswordHash, req.Password) != nil {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := generateToken(req.UserID, s.secret)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "token error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.svc.ListProjects()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		RepoPath string `json:"repo_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.ID == "" || req.Name == "" {
		writeErr(w, http.StatusBadRequest, "id and name are required")
		return
	}
	if err := s.svc.CreateProject(req.ID, req.Name, req.RepoPath); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": req.ID, "name": req.Name})
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	filters := domain.TaskFilters{}
	if st := r.URL.Query().Get("status"); st != "" {
		v := domain.Status(st)
		filters.Status = &v
	}
	if aid := r.URL.Query().Get("assignee_id"); aid != "" {
		filters.AssigneeID = &aid
	}
	if sid := r.URL.Query().Get("sprint_id"); sid != "" {
		if id, err := strconv.ParseInt(sid, 10, 64); err == nil {
			filters.SprintID = &id
		}
	}
	tasks, err := s.svc.ListTasks(projectID, filters)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tasks == nil {
		tasks = []*domain.Task{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read body")
		return
	}
	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		ProjectID   string   `json:"project_id"`
		AssigneeID  string   `json:"assignee_id"`
		Priority    string   `json:"priority"`
		Status      string   `json:"status"`
		Points      *int     `json:"points"`
		Labels      []string `json:"labels"`
		SprintID    *int64   `json:"sprint_id"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Title == "" || req.ProjectID == "" {
		writeErr(w, http.StatusBadRequest, "title and project_id are required")
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if s.maybeSyncProxy(w, r, req.ProjectID) {
		return
	}
	opts := domain.TaskCreateOpts{
		AssigneeID: req.AssigneeID,
		Priority:   domain.Priority(req.Priority),
		Status:     domain.Status(req.Status),
		Labels:     req.Labels,
		Points:     req.Points,
		SprintID:   req.SprintID,
	}
	t, err := s.svc.CreateTask(req.Title, req.Description, req.ProjectID, opts)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logChange(r, t.ProjectID, syncsrv.EventTaskCreated, t)
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	t, err := s.svc.GetTask(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.svc.GetTask(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "task not found")
		return
	}
	if s.maybeSyncProxy(w, r, existing.ProjectID) {
		return
	}
	var req struct {
		Title       *string  `json:"title"`
		Description *string  `json:"description"`
		Status      *string  `json:"status"`
		Priority    *string  `json:"priority"`
		AssigneeID  *string  `json:"assignee_id"`
		Points      *int     `json:"points"`
		Labels      []string `json:"labels"`
		SprintID    *int64   `json:"sprint_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	// Status change goes through MoveTask for proper transition logic.
	if req.Status != nil && *req.Status != string(existing.Status) {
		t, err := s.svc.MoveTask(id, domain.Status(*req.Status))
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		s.logChange(r, t.ProjectID, syncsrv.EventTaskUpdated, t)
		writeJSON(w, http.StatusOK, t)
		return
	}
	updated := *existing
	if req.Title != nil {
		updated.Title = *req.Title
	}
	if req.Description != nil {
		updated.Description = *req.Description
	}
	if req.Priority != nil {
		updated.Priority = domain.Priority(*req.Priority)
	}
	if req.AssigneeID != nil {
		if *req.AssigneeID == "" {
			updated.AssigneeID = nil
		} else {
			updated.AssigneeID = req.AssigneeID
		}
	}
	if req.Points != nil {
		updated.Points = req.Points
	}
	if req.Labels != nil {
		updated.Labels = req.Labels
	}
	if req.SprintID != nil {
		updated.SprintID = req.SprintID
	}
	t, err := s.svc.UpdateTask(&updated)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logChange(r, t.ProjectID, syncsrv.EventTaskUpdated, t)
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := s.svc.GetTask(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "task not found")
		return
	}
	if s.maybeSyncProxy(w, r, task.ProjectID) {
		return
	}
	if err := s.svc.DeleteTask(id); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to delete task")
		return
	}
	s.logChange(r, task.ProjectID, syncsrv.EventTaskDeleted, map[string]string{"id": id})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.svc.ListUsers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if users == nil {
		users = []*domain.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleListSprints(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	summaries, err := s.svc.ListSprintsWithCounts(projectID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summaries)
}

func (s *Server) handleCreateSprint(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read body")
		return
	}
	var req struct {
		Name      string `json:"name"`
		ProjectID string `json:"project_id"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" || req.ProjectID == "" {
		writeErr(w, http.StatusBadRequest, "name and project_id are required")
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if s.maybeSyncProxy(w, r, req.ProjectID) {
		return
	}
	var start, end *time.Time
	if req.StartDate != "" {
		t, err := time.Parse("2006-01-02", req.StartDate)
		if err == nil {
			start = &t
		}
	}
	if req.EndDate != "" {
		t, err := time.Parse("2006-01-02", req.EndDate)
		if err == nil {
			end = &t
		}
	}
	sp, err := s.svc.CreateSprint(req.Name, req.ProjectID, start, end)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logChange(r, sp.ProjectID, syncsrv.EventSprintCreated, sp)
	writeJSON(w, http.StatusCreated, sp)
}

func (s *Server) handleGetSprint(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid sprint id")
		return
	}
	sp, err := s.svc.GetSprint(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "sprint not found")
		return
	}
	writeJSON(w, http.StatusOK, sp)
}

// maybeSyncProxy checks if projectID is a synced project and, if so, forwards the
// request to the primary instead. Returns true if the response has already been written
// (either proxied successfully or 503 written because primary is offline).
func (s *Server) maybeSyncProxy(w http.ResponseWriter, r *http.Request, projectID string) bool {
	if s.syncClient == nil {
		return false
	}
	proj, err := s.svc.GetProject(projectID)
	if err != nil || proj.SyncOrigin == "" {
		return false
	}
	if s.syncClient.State() == syncsrv.StateOffline {
		writeErr(w, http.StatusServiceUnavailable,
			"primary server unreachable — synced projects are read-only until reconnection")
		return true
	}
	s.syncClient.Proxy(w, r, r.URL.Path)
	return true
}

func (s *Server) handleSearchTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 100 {
		limit = n
	}
	hint := r.URL.Query().Get("hint_project_id")

	results, err := s.svc.SearchTasksWithHint(q, limit, hint)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []*domain.TaskSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": results})
}

func (s *Server) handleAddBlocker(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if _, err := s.svc.GetTask(taskID); err != nil {
		writeErr(w, http.StatusNotFound, "task not found")
		return
	}
	var req struct {
		BlockerID string `json:"blocker_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.BlockerID == "" {
		writeErr(w, http.StatusBadRequest, "blocker_id is required")
		return
	}
	if _, err := s.svc.GetTask(req.BlockerID); err != nil {
		writeErr(w, http.StatusNotFound, "blocker task not found")
		return
	}
	if err := s.svc.AddDep(req.BlockerID, taskID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRemoveBlocker(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	blockerID := r.PathValue("blocker_id")
	if err := s.svc.RemoveDep(blockerID, taskID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// logChange writes a change_log event for a completed mutation. Best-effort: errors are ignored.
func (s *Server) logChange(r *http.Request, projectID, eventType string, payload any) {
	if s.store == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	origin := r.Header.Get("X-Sync-Origin")
	_, _ = s.store.WriteChangeLog(projectID, eventType, data, origin)
}
