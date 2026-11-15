package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("not found")

// ErrInvalid marks a request the board cannot accept as given: an unknown
// status, a ticket blocking itself, and so on. Errors wrap it together with
// the reason. Front ends report it as a validation error (CLI exit code 2).
var ErrInvalid = errors.New("invalid")

// ErrConflict means a conditional change was refused because the ticket has
// changed since the caller read it. The caller should read it again.
var ErrConflict = errors.New("conflict")

type TaskCreateOpts struct {
	Priority   Priority
	Status     Status
	AssigneeID string
	Points     *int
	Labels     []string
	SprintID   *int64
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

// The checks below are the one place the board's rules are enforced. Every
// front end (TUI, CLI, MCP, HTTP) reaches the store through Service, so a rule
// applied here holds for all of them.

// normID puts a project or ticket ID in its stored form: upper case.
func normID(id string) string { return strings.ToUpper(strings.TrimSpace(id)) }

func checkStatus(st Status) error {
	if st.Valid() {
		return nil
	}
	return fmt.Errorf("%w status %q: use backlog, todo, in_progress, review or done", ErrInvalid, string(st))
}

func checkPriority(p Priority) error {
	if p.Valid() {
		return nil
	}
	return fmt.Errorf("%w priority %q: use low, medium, high or critical", ErrInvalid, string(p))
}

// project looks a project up by ID in any letter case.
func (s *Service) project(id string) (*Project, error) {
	p, err := s.store.GetProject(normID(id))
	if errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("project %q: %w", normID(id), ErrNotFound)
	}
	return p, err
}

// task looks a ticket up by ID in any letter case.
func (s *Service) task(id string) (*Task, error) {
	t, err := s.store.GetTask(normID(id))
	if errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("ticket %q: %w", normID(id), ErrNotFound)
	}
	return t, err
}

func (s *Service) checkAssignee(id string) error {
	if id == "" {
		return nil
	}
	if _, err := s.store.GetUser(id); errors.Is(err, ErrNotFound) {
		return fmt.Errorf("assignee %q: %w", id, ErrNotFound)
	} else if err != nil {
		return err
	}
	return nil
}

// checkSprint accepts no sprint, or a sprint that belongs to projectID.
func (s *Service) checkSprint(projectID string, sprintID *int64) error {
	if sprintID == nil {
		return nil
	}
	sp, err := s.store.GetSprint(*sprintID)
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("sprint %d: %w", *sprintID, ErrNotFound)
	}
	if err != nil {
		return err
	}
	if sp.ProjectID != projectID {
		return fmt.Errorf("%w sprint: sprint %d belongs to project %s, not %s", ErrInvalid, sp.ID, sp.ProjectID, projectID)
	}
	return nil
}

// blocks reports whether ticket from blocks ticket target, directly or
// through a chain of other tickets.
func (s *Service) blocks(from, target string) (bool, error) {
	seen := map[string]bool{from: true}
	queue := []string{from}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		_, blocking, err := s.store.GetTaskDeps(id)
		if err != nil {
			return false, err
		}
		for _, next := range blocking {
			if next == target {
				return true, nil
			}
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false, nil
}

// CreateProject creates a project. repoPath may be a local directory or a
// remote; it is kept as given and also sorted into RepoDir or RepoRemote.
func (s *Service) CreateProject(id, name, repoPath string) error {
	dir, remote := ClassifyRepo(repoPath)
	return s.store.CreateProject(&Project{
		ID:         strings.ToUpper(id),
		Name:       name,
		RepoPath:   repoPath,
		RepoDir:    dir,
		RepoRemote: remote,
	})
}

// PatchProject changes the settings p names and returns the project as it
// then stands. A directory is trimmed; a remote is normalised.
func (s *Service) PatchProject(id string, p ProjectPatch) (*Project, error) {
	pr, err := s.project(id)
	if err != nil {
		return nil, err
	}
	if p.ArtifactPolicy.Set && !p.ArtifactPolicy.Value.Valid() {
		return nil, fmt.Errorf("%w artifact policy %q: use remote, local, off, or empty for the default", ErrInvalid, p.ArtifactPolicy.Value)
	}
	if p.RepoDir.Set {
		p.RepoDir.Value = strings.TrimSpace(p.RepoDir.Value)
		if p.RepoDir.Value != "" && !looksLikeDir(p.RepoDir.Value) {
			return nil, fmt.Errorf("%w directory %q: give a full path", ErrInvalid, p.RepoDir.Value)
		}
	}
	if p.RepoRemote.Set {
		p.RepoRemote.Value = NormalizeRemote(p.RepoRemote.Value)
	}
	if err := s.store.PatchProject(pr.ID, p); err != nil {
		return nil, err
	}
	return s.store.GetProject(pr.ID)
}

// ProjectForDir finds the one project a working directory belongs to; see
// MatchProject. remote is the clone's origin URL, or empty.
func (s *Service) ProjectForDir(cwd, remote, home string) (*Project, error) {
	projects, err := s.store.ListProjects()
	if err != nil {
		return nil, err
	}
	return MatchProject(projects, cwd, remote, home)
}

func (s *Service) ListProjects() ([]*Project, error) {
	return s.store.ListProjects()
}

// GetProject resolves id exactly as the mutating methods do (any letter
// case). Callers use it to decide how to treat a write, such as the HTTP
// layer's sync gate, so it must never disagree with them about which project
// an ID names.
func (s *Service) GetProject(id string) (*Project, error) {
	return s.project(id)
}

func (s *Service) SetSprintMode(projectID string, enabled bool) error {
	_, err := s.PatchProject(projectID, ProjectPatch{SprintMode: To(enabled)})
	return err
}

func (s *Service) CreateTask(title, description, projectID string, opts TaskCreateOpts) (*Task, error) {
	// Check everything before taking a ticket number, so a rejected request
	// does not leave a gap in the sequence.
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("%w ticket: a title is required", ErrInvalid)
	}
	if opts.Status != "" {
		if err := checkStatus(opts.Status); err != nil {
			return nil, err
		}
	}
	if opts.Priority != "" {
		if err := checkPriority(opts.Priority); err != nil {
			return nil, err
		}
	}
	project, err := s.project(projectID)
	if err != nil {
		return nil, err
	}
	projectID = project.ID
	if err := s.checkAssignee(opts.AssigneeID); err != nil {
		return nil, err
	}
	if err := s.checkSprint(projectID, opts.SprintID); err != nil {
		return nil, err
	}

	seq, err := s.store.NextTaskSeq(projectID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	t := &Task{
		ID:          fmt.Sprintf("%s-%03d", projectID, seq),
		ProjectID:   projectID,
		Title:       title,
		Description: description,
		Status:      StatusBacklog,
		Priority:    PriorityMedium,
		SprintID:    opts.SprintID,
		Labels:      opts.Labels,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if opts.Priority != "" {
		t.Priority = opts.Priority
	}
	if opts.Status != "" {
		t.Status = opts.Status
	}
	if opts.AssigneeID != "" {
		t.AssigneeID = &opts.AssigneeID
	}
	if opts.Points != nil {
		t.Points = opts.Points
	}
	return t, s.store.CreateTask(t)
}

func (s *Service) ListTasks(projectID string, f TaskFilters) ([]*Task, error) {
	// An unknown project is an error, not an empty list: "no tickets" for a
	// mistyped ID reads as "nothing to do".
	project, err := s.project(projectID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.store.ListTasks(project.ID, f)
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		blockers, blocking, err := s.store.GetTaskDeps(t.ID)
		if err != nil {
			return nil, err
		}
		t.Blockers = blockers
		t.Blocking = blocking
	}
	return tasks, nil
}

func (s *Service) GetTask(id string) (*Task, error) {
	t, err := s.task(id)
	if err != nil {
		return nil, err
	}
	blockers, blocking, err := s.store.GetTaskDeps(t.ID)
	if err != nil {
		return nil, err
	}
	t.Blockers = blockers
	t.Blocking = blocking
	return t, nil
}

func (s *Service) MoveTask(id string, status Status) (*Task, error) {
	return s.PatchTask(id, TaskPatch{Status: To(status)})
}

func (s *Service) DeleteTask(id string) error {
	t, err := s.task(id)
	if err != nil {
		return err
	}
	return s.store.DeleteTask(t.ID)
}

func (s *Service) TaskByBranch(branch string) (*Task, error) {
	return s.store.TaskByBranch(branch)
}

func (s *Service) LinkBranch(taskID, branch string) error {
	_, err := s.PatchTask(taskID, TaskPatch{Branch: To(branch)})
	return err
}

func (s *Service) LinkPR(taskID string, prNumber int) error {
	_, err := s.PatchTask(taskID, TaskPatch{PRNumber: To(&prNumber)})
	return err
}

func (s *Service) MarkPRMerged(taskID string) error {
	_, err := s.PatchTask(taskID, TaskPatch{PRMerged: To(true), Status: To(StatusDone)})
	return err
}

func (s *Service) CreateSprint(name, projectID string, start, end *time.Time) (*Sprint, error) {
	project, err := s.project(projectID)
	if err != nil {
		return nil, err
	}
	return s.store.CreateSprint(&Sprint{
		ProjectID: project.ID,
		Name:      name,
		StartDate: start,
		EndDate:   end,
		Status:    SprintPlanning,
	})
}

func (s *Service) ListSprints(projectID string) ([]*Sprint, error) {
	return s.store.ListSprints(normID(projectID))
}

func (s *Service) GetSprint(id int64) (*Sprint, error) {
	return s.store.GetSprint(id)
}

func (s *Service) ActivateSprint(id int64) error {
	sp, err := s.store.GetSprint(id)
	if err != nil {
		return err
	}
	sp.Status = SprintActive
	return s.store.UpdateSprint(sp)
}

func (s *Service) CreateUser(id, displayName string, isAgent bool) (*User, error) {
	u := &User{ID: id, DisplayName: displayName, IsAgent: isAgent}
	return u, s.store.CreateUser(u)
}

func (s *Service) ListUsers() ([]*User, error) {
	return s.store.ListUsers()
}

func (s *Service) GetUser(id string) (*User, error) {
	return s.store.GetUser(id)
}

// SetUserPasswordHash stores a pre-computed bcrypt hash for a user.
func (s *Service) SetUserPasswordHash(id, hash string) error {
	return s.store.SetUserPassword(id, hash)
}

// AddDep records that blockerID must be done before blockedID can start.
// The blocker graph stays acyclic: a ticket cannot block itself, and a new
// link is refused when the two tickets would end up waiting on each other.
func (s *Service) AddDep(blockerID, blockedID string) error {
	blocker, err := s.task(blockerID)
	if err != nil {
		return err
	}
	blocked, err := s.task(blockedID)
	if err != nil {
		return err
	}
	if blocker.ID == blocked.ID {
		return fmt.Errorf("%w blocker: %s cannot block itself", ErrInvalid, blocker.ID)
	}
	loops, err := s.blocks(blocked.ID, blocker.ID)
	if err != nil {
		return err
	}
	if loops {
		return fmt.Errorf("%w blocker: %s already blocks %s, directly or through other tickets, so the two would wait on each other",
			ErrInvalid, blocked.ID, blocker.ID)
	}
	return s.store.AddDep(blocker.ID, blocked.ID)
}

func (s *Service) RemoveDep(blockerID, blockedID string) error {
	return s.store.RemoveDep(normID(blockerID), normID(blockedID))
}

func (s *Service) SearchTasks(q string, limit int) ([]*TaskSummary, error) {
	return s.store.SearchTasks(q, limit)
}

func (s *Service) SearchTasksWithHint(q string, limit int, hintProjectID string) ([]*TaskSummary, error) {
	return s.store.SearchTasksWithHint(q, limit, hintProjectID)
}

// AssignTaskToSprint sets or clears the sprint assignment for a task.
// sprintID == nil clears the assignment.
func (s *Service) AssignTaskToSprint(taskID string, sprintID *int64) (*Task, error) {
	return s.PatchTask(taskID, TaskPatch{SprintID: Change[*int64]{Set: true, Value: sprintID}})
}

func (s *Service) GetActiveSprint(projectID string) (*Sprint, error) {
	return s.store.GetActiveSprint(normID(projectID))
}

func (s *Service) ListSprintsWithCounts(projectID string) ([]SprintSummary, error) {
	return s.store.ListSprintsWithCounts(normID(projectID))
}

// FindTasks finds tickets across projects, or within one when q.ProjectID is
// set. It returns at most q.Limit tickets, each with what blocks it and what
// it blocks, and the number of tickets that match in all.
func (s *Service) FindTasks(q TaskQuery) ([]*Task, int, error) {
	q.Text = strings.TrimSpace(q.Text)
	q.Label = strings.TrimSpace(q.Label)
	for _, st := range q.Statuses {
		if err := checkStatus(st); err != nil {
			return nil, 0, err
		}
	}
	if q.ProjectID != "" {
		project, err := s.project(q.ProjectID)
		if err != nil {
			return nil, 0, err
		}
		q.ProjectID = project.ID
	}
	if err := s.checkAssignee(q.AssigneeID); err != nil {
		return nil, 0, err
	}

	tasks, total, err := s.store.FindTasks(q)
	if err != nil {
		return nil, 0, err
	}
	for _, t := range tasks {
		blockers, blocking, err := s.store.GetTaskDeps(t.ID)
		if err != nil {
			return nil, 0, err
		}
		t.Blockers = blockers
		t.Blocking = blocking
	}
	return tasks, total, nil
}

// TaskCounts returns the number of tickets per project and status.
func (s *Service) TaskCounts() (map[string]map[Status]int, error) {
	return s.store.TaskCounts()
}

// PatchTask changes only the fields p names and returns the ticket as it then
// stands. Every other field keeps whatever value the store holds at that
// moment, so a change another writer made since the caller read the ticket is
// never reverted. Set p.IfUpdatedAt to refuse the change (ErrConflict) when
// the ticket has changed at all since it was read.
//
// This is the only way Service changes an existing ticket's fields.
func (s *Service) PatchTask(id string, p TaskPatch) (*Task, error) {
	t, err := s.task(id)
	if err != nil {
		return nil, err
	}
	if p.Title.Set && strings.TrimSpace(p.Title.Value) == "" {
		return nil, fmt.Errorf("%w ticket: a title is required", ErrInvalid)
	}
	if p.Status.Set {
		if err := checkStatus(p.Status.Value); err != nil {
			return nil, err
		}
	}
	if p.Priority.Set {
		if err := checkPriority(p.Priority.Value); err != nil {
			return nil, err
		}
	}
	if p.AssigneeID.Set {
		if p.AssigneeID.Value != nil && *p.AssigneeID.Value == "" {
			p.AssigneeID.Value = nil // front ends send "" for "nobody"
		}
		if p.AssigneeID.Value != nil {
			if err := s.checkAssignee(*p.AssigneeID.Value); err != nil {
				return nil, err
			}
		}
	}
	if p.SprintID.Set {
		if err := s.checkSprint(t.ProjectID, p.SprintID.Value); err != nil {
			return nil, err
		}
	}
	if p.Branch.Set {
		p.Branch.Value = strings.TrimSpace(p.Branch.Value)
		if err := ValidBranchName(p.Branch.Value); err != nil {
			return nil, err
		}
	}

	if !p.Empty() {
		err := s.store.PatchTask(t.ID, p, time.Now().UTC())
		if errors.Is(err, ErrConflict) {
			return nil, fmt.Errorf("%w: %s was changed by someone else after you read it; read it again and reapply your change", ErrConflict, t.ID)
		}
		if err != nil {
			return nil, err
		}
	}
	return s.GetTask(t.ID)
}
