# KeroAgile — Core Implementation Plan (Plan A)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the domain, SQLite store, git integration, and full CLI — producing a working `KeroAgile` binary usable from the terminal and by Claude Code via `--json`.

**Architecture:** `internal/domain` (pure models + service interface) → `internal/store` (SQLite implementation) → `internal/git` (exec wrappers) → `internal/config` → `cmd/keroagile` (Cobra CLI). No TUI in this plan.

**Tech Stack:** Go 1.24, Cobra v1.9.1, modernc.org/sqlite (pure Go, no CGo), BurntSushi/toml, testify

---

## File Map

| File | Responsibility |
|------|---------------|
| `go.mod` | Module definition and dependencies |
| `Makefile` | build, install, test targets |
| `.gitignore` | ignore binary + config dir |
| `internal/domain/task.go` | Task model, Status, Priority, status machine |
| `internal/domain/project.go` | Project + Sprint models |
| `internal/domain/user.go` | User model |
| `internal/domain/store.go` | Store interface + TaskFilters |
| `internal/domain/service.go` | Service struct + all business logic |
| `internal/store/db.go` | Open(), migrate(), SQL schema |
| `internal/store/store.go` | Store struct, New(), compile-time interface check |
| `internal/store/project.go` | Project + Sprint CRUD methods on Store |
| `internal/store/user.go` | User CRUD methods on Store |
| `internal/store/task.go` | Task CRUD + deps + NextTaskSeq |
| `internal/git/client.go` | Client struct, execGit(), execGH() helpers |
| `internal/git/repo.go` | CurrentBranch(), CommitLog(), AutoLinkTasks() |
| `internal/git/github.go` | PRStatus(), ListOpenPRs(), AutoDetectPRs() |
| `internal/config/config.go` | Config struct, Load(), Save(), default path |
| `cmd/keroagile/main.go` | Cobra root, TUI stub launcher, --json flag |
| `cmd/keroagile/output.go` | printJSON(), printTable() helpers |
| `cmd/keroagile/cmd_project.go` | project subcommands |
| `cmd/keroagile/cmd_task.go` | task subcommands |
| `cmd/keroagile/cmd_sprint.go` | sprint subcommands |
| `cmd/keroagile/cmd_user.go` | user subcommands |

---

### Task 1: Project scaffold

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `.gitignore`
- Create: `cmd/keroagile/main.go` (stub)

- [ ] **Step 1: Create directory structure**

```bash
cd /home/matt/github/KeroAgile
mkdir -p cmd/keroagile internal/domain internal/store internal/git internal/config
```

- [ ] **Step 2: Write go.mod**

```
module keroagile

go 1.24.0

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/charmbracelet/bubbles v0.20.0
	github.com/charmbracelet/bubbletea v1.3.10
	github.com/charmbracelet/lipgloss v1.1.0
	github.com/spf13/cobra v1.9.1
	github.com/stretchr/testify v1.10.0
	modernc.org/sqlite v1.34.5
)
```

- [ ] **Step 3: Write Makefile**

```makefile
.PHONY: build install test clean

build:
	go build -o KeroAgile ./cmd/keroagile/

install:
	go build -o $(HOME)/.local/bin/KeroAgile ./cmd/keroagile/

test:
	go test ./...

clean:
	rm -f KeroAgile
```

- [ ] **Step 4: Write .gitignore**

```
KeroAgile
.superpowers/
*.db
```

- [ ] **Step 5: Write stub main.go**

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "KeroAgile",
	Short: "Terminal agile board",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("TUI coming in Plan B")
		return nil
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 6: Download dependencies**

```bash
cd /home/matt/github/KeroAgile && go mod tidy
```

Expected: go.sum created, no errors.

- [ ] **Step 7: Build smoke test**

```bash
go build -o KeroAgile ./cmd/keroagile/ && ./KeroAgile
```

Expected output: `TUI coming in Plan B`

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum Makefile .gitignore cmd/keroagile/main.go
git commit -m "feat: project scaffold"
```

---

### Task 2: Domain models

**Files:**
- Create: `internal/domain/task.go`
- Create: `internal/domain/project.go`
- Create: `internal/domain/user.go`
- Create: `internal/domain/task_test.go`

- [ ] **Step 1: Write failing test for status machine**

```go
// internal/domain/task_test.go
package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"keroagile/internal/domain"
)

func TestStatusNext(t *testing.T) {
	assert.Equal(t, domain.StatusTodo, domain.StatusBacklog.Next())
	assert.Equal(t, domain.StatusInProgress, domain.StatusTodo.Next())
	assert.Equal(t, domain.StatusReview, domain.StatusInProgress.Next())
	assert.Equal(t, domain.StatusDone, domain.StatusReview.Next())
	assert.Equal(t, domain.StatusDone, domain.StatusDone.Next()) // no-op at end
}

func TestStatusPrev(t *testing.T) {
	assert.Equal(t, domain.StatusBacklog, domain.StatusBacklog.Prev()) // no-op at start
	assert.Equal(t, domain.StatusBacklog, domain.StatusTodo.Prev())
	assert.Equal(t, domain.StatusTodo, domain.StatusInProgress.Prev())
}

func TestStatusLabel(t *testing.T) {
	assert.Equal(t, "In Progress", domain.StatusInProgress.Label())
	assert.Equal(t, "Backlog", domain.StatusBacklog.Label())
}

func TestPriorityColor(t *testing.T) {
	assert.NotEmpty(t, domain.PriorityCritical.Color())
}
```

- [ ] **Step 2: Run test — expect failure**

```bash
go test ./internal/domain/... 2>&1 | head -5
```

Expected: `cannot find package` or compile error.

- [ ] **Step 3: Write internal/domain/task.go**

```go
package domain

import "time"

type Status string

const (
	StatusBacklog    Status = "backlog"
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusReview     Status = "review"
	StatusDone       Status = "done"
)

var statusOrder = []Status{
	StatusBacklog, StatusTodo, StatusInProgress, StatusReview, StatusDone,
}

func (s Status) Next() Status {
	for i, st := range statusOrder {
		if st == s && i < len(statusOrder)-1 {
			return statusOrder[i+1]
		}
	}
	return s
}

func (s Status) Prev() Status {
	for i, st := range statusOrder {
		if st == s && i > 0 {
			return statusOrder[i-1]
		}
	}
	return s
}

func (s Status) Label() string {
	switch s {
	case StatusBacklog:
		return "Backlog"
	case StatusTodo:
		return "Todo"
	case StatusInProgress:
		return "In Progress"
	case StatusReview:
		return "Review"
	case StatusDone:
		return "Done"
	}
	return string(s)
}

type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

func (p Priority) Color() string {
	switch p {
	case PriorityLow:
		return "#6B7280"
	case PriorityMedium:
		return "#EAB308"
	case PriorityHigh:
		return "#F97316"
	case PriorityCritical:
		return "#EF4444"
	}
	return "#6B7280"
}

func (p Priority) Label() string {
	switch p {
	case PriorityLow:
		return "LOW"
	case PriorityMedium:
		return "MEDIUM"
	case PriorityHigh:
		return "HIGH"
	case PriorityCritical:
		return "CRITICAL"
	}
	return string(p)
}

type Task struct {
	ID          string
	ProjectID   string
	SprintID    *int64
	Title       string
	Description string
	Status      Status
	Priority    Priority
	Points      *int
	AssigneeID  *string
	Branch      string
	PRNumber    *int
	PRMerged    bool
	Labels      []string
	Blockers    []string // task IDs that block this task
	Blocking    []string // task IDs this task blocks
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
```

- [ ] **Step 4: Write internal/domain/project.go**

```go
package domain

import "time"

type Project struct {
	ID         string
	Name       string
	RepoPath   string
	SprintMode bool
}

type SprintStatus string

const (
	SprintPlanning  SprintStatus = "planning"
	SprintActive    SprintStatus = "active"
	SprintCompleted SprintStatus = "completed"
)

type Sprint struct {
	ID        int64
	ProjectID string
	Name      string
	StartDate *time.Time
	EndDate   *time.Time
	Status    SprintStatus
}
```

- [ ] **Step 5: Write internal/domain/user.go**

```go
package domain

type User struct {
	ID          string
	DisplayName string
	IsAgent     bool
}

func (u *User) DisplayPrefix() string {
	if u.IsAgent {
		return "🤖 " + u.DisplayName
	}
	return "👤 " + u.DisplayName
}
```

- [ ] **Step 6: Run tests — expect pass**

```bash
go test ./internal/domain/... -v
```

Expected: All tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/domain/
git commit -m "feat: domain models — Task, Project, Sprint, User, status machine"
```

---

### Task 3: Store interface

**Files:**
- Create: `internal/domain/store.go`

- [ ] **Step 1: Write internal/domain/store.go**

```go
package domain

// TaskFilters narrows ListTasks results.
type TaskFilters struct {
	Status     *Status
	AssigneeID *string
	SprintID   *int64
}

// Store is the persistence contract. internal/store provides the SQLite implementation.
type Store interface {
	// Projects
	CreateProject(p *Project) error
	ListProjects() ([]*Project, error)
	GetProject(id string) (*Project, error)
	UpdateProject(p *Project) error

	// Tasks
	CreateTask(t *Task) error
	ListTasks(projectID string, f TaskFilters) ([]*Task, error)
	GetTask(id string) (*Task, error)
	UpdateTask(t *Task) error
	DeleteTask(id string) error
	GetTaskDeps(taskID string) (blockers, blocking []string, err error)
	AddDep(blockerID, blockedID string) error
	RemoveDep(blockerID, blockedID string) error
	NextTaskSeq(projectID string) (int, error)

	// Sprints
	CreateSprint(s *Sprint) (*Sprint, error)
	ListSprints(projectID string) ([]*Sprint, error)
	GetSprint(id int64) (*Sprint, error)
	UpdateSprint(s *Sprint) error

	// Users
	CreateUser(u *User) error
	ListUsers() ([]*User, error)
	GetUser(id string) (*User, error)
}
```

- [ ] **Step 2: Verify compile**

```bash
go build ./internal/domain/...
```

Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/domain/store.go
git commit -m "feat: domain Store interface"
```

---

### Task 4: Domain service

**Files:**
- Create: `internal/domain/service.go`
- Create: `internal/domain/service_test.go`

- [ ] **Step 1: Write failing service test**

```go
// internal/domain/service_test.go
package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"keroagile/internal/domain"
)

// mockStore is a minimal in-memory Store for service tests.
type mockStore struct {
	projects  map[string]*domain.Project
	tasks     map[string]*domain.Task
	users     map[string]*domain.User
	sprints   map[int64]*domain.Sprint
	seqs      map[string]int
	deps      map[string][]string // blocked_id -> []blocker_id
}

func newMock() *mockStore {
	return &mockStore{
		projects: make(map[string]*domain.Project),
		tasks:    make(map[string]*domain.Task),
		users:    make(map[string]*domain.User),
		sprints:  make(map[int64]*domain.Sprint),
		seqs:     make(map[string]int),
		deps:     make(map[string][]string),
	}
}

func (m *mockStore) CreateProject(p *domain.Project) error {
	m.projects[p.ID] = p
	return nil
}
func (m *mockStore) ListProjects() ([]*domain.Project, error) {
	var out []*domain.Project
	for _, p := range m.projects {
		out = append(out, p)
	}
	return out, nil
}
func (m *mockStore) GetProject(id string) (*domain.Project, error) {
	p, ok := m.projects[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return p, nil
}
func (m *mockStore) UpdateProject(p *domain.Project) error {
	m.projects[p.ID] = p
	return nil
}
func (m *mockStore) CreateTask(t *domain.Task) error {
	m.tasks[t.ID] = t
	return nil
}
func (m *mockStore) ListTasks(projectID string, f domain.TaskFilters) ([]*domain.Task, error) {
	var out []*domain.Task
	for _, t := range m.tasks {
		if t.ProjectID != projectID {
			continue
		}
		if f.Status != nil && t.Status != *f.Status {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}
func (m *mockStore) GetTask(id string) (*domain.Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}
func (m *mockStore) UpdateTask(t *domain.Task) error {
	m.tasks[t.ID] = t
	return nil
}
func (m *mockStore) DeleteTask(id string) error {
	delete(m.tasks, id)
	return nil
}
func (m *mockStore) GetTaskDeps(taskID string) (blockers, blocking []string, err error) {
	return m.deps[taskID], nil, nil
}
func (m *mockStore) AddDep(blockerID, blockedID string) error {
	m.deps[blockedID] = append(m.deps[blockedID], blockerID)
	return nil
}
func (m *mockStore) RemoveDep(blockerID, blockedID string) error { return nil }
func (m *mockStore) NextTaskSeq(projectID string) (int, error) {
	m.seqs[projectID]++
	return m.seqs[projectID], nil
}
func (m *mockStore) CreateSprint(s *domain.Sprint) (*domain.Sprint, error) {
	s.ID = int64(len(m.sprints) + 1)
	m.sprints[s.ID] = s
	return s, nil
}
func (m *mockStore) ListSprints(projectID string) ([]*domain.Sprint, error) { return nil, nil }
func (m *mockStore) GetSprint(id int64) (*domain.Sprint, error) {
	s, ok := m.sprints[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s, nil
}
func (m *mockStore) UpdateSprint(s *domain.Sprint) error {
	m.sprints[s.ID] = s
	return nil
}
func (m *mockStore) CreateUser(u *domain.User) error {
	m.users[u.ID] = u
	return nil
}
func (m *mockStore) ListUsers() ([]*domain.User, error) { return nil, nil }
func (m *mockStore) GetUser(id string) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func TestServiceCreateTask(t *testing.T) {
	svc := domain.NewService(newMock())
	require.NoError(t, svc.CreateProject("KA", "myapp", ""))

	task, err := svc.CreateTask("Fix login", "", "KA", domain.TaskCreateOpts{
		Priority:   domain.PriorityHigh,
		AssigneeID: "claude",
	})
	require.NoError(t, err)
	assert.Equal(t, "KA-001", task.ID)
	assert.Equal(t, domain.StatusBacklog, task.Status)
	assert.Equal(t, domain.PriorityHigh, task.Priority)
}

func TestServiceMoveTask(t *testing.T) {
	svc := domain.NewService(newMock())
	require.NoError(t, svc.CreateProject("KA", "myapp", ""))
	task, err := svc.CreateTask("Fix login", "", "KA", domain.TaskCreateOpts{})
	require.NoError(t, err)

	moved, err := svc.MoveTask(task.ID, domain.StatusTodo)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusTodo, moved.Status)
}

func TestServiceSecondTaskSeq(t *testing.T) {
	svc := domain.NewService(newMock())
	require.NoError(t, svc.CreateProject("KA", "myapp", ""))
	_, _ = svc.CreateTask("First", "", "KA", domain.TaskCreateOpts{})
	second, err := svc.CreateTask("Second", "", "KA", domain.TaskCreateOpts{})
	require.NoError(t, err)
	assert.Equal(t, "KA-002", second.ID)
}
```

- [ ] **Step 2: Run test — expect failure**

```bash
go test ./internal/domain/... 2>&1 | head -10
```

Expected: compile error — `ErrNotFound`, `NewService`, `TaskCreateOpts` undefined.

- [ ] **Step 3: Write internal/domain/service.go**

```go
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("not found")

type TaskCreateOpts struct {
	Priority   Priority
	Status     Status
	AssigneeID string
	Points     int
	Labels     []string
	SprintID   *int64
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) CreateProject(id, name, repoPath string) error {
	return s.store.CreateProject(&Project{
		ID:       strings.ToUpper(id),
		Name:     name,
		RepoPath: repoPath,
	})
}

func (s *Service) ListProjects() ([]*Project, error) {
	return s.store.ListProjects()
}

func (s *Service) GetProject(id string) (*Project, error) {
	return s.store.GetProject(id)
}

func (s *Service) SetSprintMode(projectID string, enabled bool) error {
	p, err := s.store.GetProject(projectID)
	if err != nil {
		return err
	}
	p.SprintMode = enabled
	return s.store.UpdateProject(p)
}

func (s *Service) CreateTask(title, description, projectID string, opts TaskCreateOpts) (*Task, error) {
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
	if opts.Points > 0 {
		t.Points = &opts.Points
	}
	return t, s.store.CreateTask(t)
}

func (s *Service) ListTasks(projectID string, f TaskFilters) ([]*Task, error) {
	tasks, err := s.store.ListTasks(projectID, f)
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
	t, err := s.store.GetTask(id)
	if err != nil {
		return nil, err
	}
	blockers, blocking, err := s.store.GetTaskDeps(id)
	if err != nil {
		return nil, err
	}
	t.Blockers = blockers
	t.Blocking = blocking
	return t, nil
}

func (s *Service) UpdateTask(t *Task) (*Task, error) {
	t.UpdatedAt = time.Now().UTC()
	return t, s.store.UpdateTask(t)
}

func (s *Service) MoveTask(id string, status Status) (*Task, error) {
	t, err := s.store.GetTask(id)
	if err != nil {
		return nil, err
	}
	t.Status = status
	t.UpdatedAt = time.Now().UTC()
	return t, s.store.UpdateTask(t)
}

func (s *Service) DeleteTask(id string) error {
	return s.store.DeleteTask(id)
}

func (s *Service) LinkBranch(taskID, branch string) error {
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return err
	}
	t.Branch = branch
	t.UpdatedAt = time.Now().UTC()
	return s.store.UpdateTask(t)
}

func (s *Service) LinkPR(taskID string, prNumber int) error {
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return err
	}
	t.PRNumber = &prNumber
	t.UpdatedAt = time.Now().UTC()
	return s.store.UpdateTask(t)
}

func (s *Service) MarkPRMerged(taskID string) error {
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return err
	}
	t.PRMerged = true
	t.Status = StatusDone
	t.UpdatedAt = time.Now().UTC()
	return s.store.UpdateTask(t)
}

func (s *Service) CreateSprint(name, projectID string, start, end *time.Time) (*Sprint, error) {
	return s.store.CreateSprint(&Sprint{
		ProjectID: projectID,
		Name:      name,
		StartDate: start,
		EndDate:   end,
		Status:    SprintPlanning,
	})
}

func (s *Service) ListSprints(projectID string) ([]*Sprint, error) {
	return s.store.ListSprints(projectID)
}

func (s *Service) ActivateSprint(sprintID int64) error {
	sp, err := s.store.GetSprint(sprintID)
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

func (s *Service) AddDependency(blockerID, blockedID string) error {
	return s.store.AddDep(blockerID, blockedID)
}

func (s *Service) RemoveDependency(blockerID, blockedID string) error {
	return s.store.RemoveDep(blockerID, blockedID)
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./internal/domain/... -v 2>&1 | tail -10
```

Expected: `PASS`, all 3 service tests green.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/service.go internal/domain/service_test.go
git commit -m "feat: domain service with full task/project/sprint/user methods"
```

---

### Task 5: SQLite store — DB + schema

**Files:**
- Create: `internal/store/db.go`
- Create: `internal/store/store.go`

- [ ] **Step 1: Write internal/store/db.go**

```go
package store

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}

const schema = `
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    repo_path   TEXT NOT NULL DEFAULT '',
    sprint_mode INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sequences (
    project_id TEXT PRIMARY KEY REFERENCES projects(id),
    next_seq   INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS users (
    id           TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    is_agent     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sprints (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id),
    name       TEXT NOT NULL,
    start_date TEXT,
    end_date   TEXT,
    status     TEXT NOT NULL DEFAULT 'planning'
);

CREATE TABLE IF NOT EXISTS tasks (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id),
    sprint_id   INTEGER REFERENCES sprints(id),
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'backlog',
    priority    TEXT NOT NULL DEFAULT 'medium',
    points      INTEGER,
    assignee_id TEXT REFERENCES users(id),
    branch      TEXT NOT NULL DEFAULT '',
    pr_number   INTEGER,
    pr_merged   INTEGER NOT NULL DEFAULT 0,
    labels      TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS task_deps (
    blocker_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    blocked_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    PRIMARY KEY (blocker_id, blocked_id)
);
`
```

- [ ] **Step 2: Write internal/store/store.go**

```go
package store

import (
	"database/sql"

	"keroagile/internal/domain"
)

// Store implements domain.Store using SQLite.
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// compile-time interface check
var _ domain.Store = (*Store)(nil)

func (s *Store) Close() error {
	return s.db.Close()
}
```

- [ ] **Step 3: Verify compile (will fail until all interface methods are implemented — that's expected)**

```bash
go build ./internal/store/... 2>&1 | head -5
```

Expected: errors listing unimplemented methods. That's correct — we'll add them in the next tasks.

- [ ] **Step 4: Commit**

```bash
git add internal/store/db.go internal/store/store.go
git commit -m "feat: SQLite store scaffold and schema"
```

---

### Task 6: Store — project and user methods

**Files:**
- Create: `internal/store/project.go`
- Create: `internal/store/user.go`
- Create: `internal/store/sprint.go`
- Create: `internal/store/store_test.go` (shared test helper)

- [ ] **Step 1: Write test helper**

```go
// internal/store/store_test.go
package store_test

import (
	"testing"

	"keroagile/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return store.New(db)
}
```

- [ ] **Step 2: Write failing project test**

```go
// internal/store/project_test.go
package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"keroagile/internal/domain"
)

func TestProjectCRUD(t *testing.T) {
	s := testStore(t)

	p := &domain.Project{ID: "KA", Name: "myapp", RepoPath: "/tmp/myapp"}
	require.NoError(t, s.CreateProject(p))

	got, err := s.GetProject("KA")
	require.NoError(t, err)
	assert.Equal(t, "myapp", got.Name)
	assert.Equal(t, "/tmp/myapp", got.RepoPath)

	got.SprintMode = true
	require.NoError(t, s.UpdateProject(got))

	list, err := s.ListProjects()
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.True(t, list[0].SprintMode)
}

func TestGetProjectNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.GetProject("NOPE")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
```

- [ ] **Step 3: Write internal/store/project.go**

```go
package store

import (
	"database/sql"
	"errors"

	"keroagile/internal/domain"
)

func (s *Store) CreateProject(p *domain.Project) error {
	_, err := s.db.Exec(
		`INSERT INTO projects(id, name, repo_path, sprint_mode) VALUES(?,?,?,?)`,
		p.ID, p.Name, p.RepoPath, boolInt(p.SprintMode),
	)
	return err
}

func (s *Store) ListProjects() ([]*domain.Project, error) {
	rows, err := s.db.Query(`SELECT id, name, repo_path, sprint_mode FROM projects ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetProject(id string) (*domain.Project, error) {
	row := s.db.QueryRow(`SELECT id, name, repo_path, sprint_mode FROM projects WHERE id=?`, id)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return p, err
}

func (s *Store) UpdateProject(p *domain.Project) error {
	_, err := s.db.Exec(
		`UPDATE projects SET name=?, repo_path=?, sprint_mode=? WHERE id=?`,
		p.Name, p.RepoPath, boolInt(p.SprintMode), p.ID,
	)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProject(r rowScanner) (*domain.Project, error) {
	var p domain.Project
	var sprintMode int
	err := r.Scan(&p.ID, &p.Name, &p.RepoPath, &sprintMode)
	if err != nil {
		return nil, err
	}
	p.SprintMode = sprintMode == 1
	return &p, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
```

- [ ] **Step 4: Write internal/store/user.go**

```go
package store

import (
	"database/sql"
	"errors"

	"keroagile/internal/domain"
)

func (s *Store) CreateUser(u *domain.User) error {
	_, err := s.db.Exec(
		`INSERT INTO users(id, display_name, is_agent) VALUES(?,?,?)`,
		u.ID, u.DisplayName, boolInt(u.IsAgent),
	)
	return err
}

func (s *Store) ListUsers() ([]*domain.User, error) {
	rows, err := s.db.Query(`SELECT id, display_name, is_agent FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) GetUser(id string) (*domain.User, error) {
	row := s.db.QueryRow(`SELECT id, display_name, is_agent FROM users WHERE id=?`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return u, err
}

func scanUser(r rowScanner) (*domain.User, error) {
	var u domain.User
	var isAgent int
	if err := r.Scan(&u.ID, &u.DisplayName, &isAgent); err != nil {
		return nil, err
	}
	u.IsAgent = isAgent == 1
	return &u, nil
}
```

- [ ] **Step 5: Write internal/store/sprint.go**

```go
package store

import (
	"database/sql"
	"errors"
	"time"

	"keroagile/internal/domain"
)

func (s *Store) CreateSprint(sp *domain.Sprint) (*domain.Sprint, error) {
	res, err := s.db.Exec(
		`INSERT INTO sprints(project_id, name, start_date, end_date, status) VALUES(?,?,?,?,?)`,
		sp.ProjectID, sp.Name, nullTime(sp.StartDate), nullTime(sp.EndDate), string(sp.Status),
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	sp.ID = id
	return sp, nil
}

func (s *Store) ListSprints(projectID string) ([]*domain.Sprint, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, name, start_date, end_date, status FROM sprints WHERE project_id=? ORDER BY id`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Sprint
	for rows.Next() {
		sp, err := scanSprint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (s *Store) GetSprint(id int64) (*domain.Sprint, error) {
	row := s.db.QueryRow(
		`SELECT id, project_id, name, start_date, end_date, status FROM sprints WHERE id=?`, id,
	)
	sp, err := scanSprint(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return sp, err
}

func (s *Store) UpdateSprint(sp *domain.Sprint) error {
	_, err := s.db.Exec(
		`UPDATE sprints SET name=?, start_date=?, end_date=?, status=? WHERE id=?`,
		sp.Name, nullTime(sp.StartDate), nullTime(sp.EndDate), string(sp.Status), sp.ID,
	)
	return err
}

func scanSprint(r rowScanner) (*domain.Sprint, error) {
	var sp domain.Sprint
	var startDate, endDate sql.NullString
	var status string
	if err := r.Scan(&sp.ID, &sp.ProjectID, &sp.Name, &startDate, &endDate, &status); err != nil {
		return nil, err
	}
	sp.Status = domain.SprintStatus(status)
	if startDate.Valid {
		t, _ := time.Parse(time.RFC3339, startDate.String)
		sp.StartDate = &t
	}
	if endDate.Valid {
		t, _ := time.Parse(time.RFC3339, endDate.String)
		sp.EndDate = &t
	}
	return &sp, nil
}

func nullTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.Format(time.RFC3339), Valid: true}
}
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/store/... -run TestProject -v
```

Expected: `TestProjectCRUD PASS`, `TestGetProjectNotFound PASS`.

- [ ] **Step 7: Commit**

```bash
git add internal/store/project.go internal/store/user.go internal/store/sprint.go \
        internal/store/store_test.go internal/store/project_test.go
git commit -m "feat: SQLite store — project, user, sprint CRUD"
```

---

### Task 7: Store — task CRUD + dependencies

**Files:**
- Create: `internal/store/task.go`
- Create: `internal/store/task_test.go`

- [ ] **Step 1: Write failing task store test**

```go
// internal/store/task_test.go
package store_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"keroagile/internal/domain"
)

func seedProject(t *testing.T, s interface {
	CreateProject(*domain.Project) error
}) {
	t.Helper()
	require.NoError(t, s.CreateProject(&domain.Project{ID: "KA", Name: "test"}))
}

func TestTaskCRUD(t *testing.T) {
	s := testStore(t)
	seedProject(t, s)

	pts := 3
	task := &domain.Task{
		ID:        "KA-001",
		ProjectID: "KA",
		Title:     "Fix login",
		Status:    domain.StatusBacklog,
		Priority:  domain.PriorityHigh,
		Points:    &pts,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, s.CreateTask(task))

	got, err := s.GetTask("KA-001")
	require.NoError(t, err)
	assert.Equal(t, "Fix login", got.Title)
	assert.Equal(t, domain.PriorityHigh, got.Priority)
	require.NotNil(t, got.Points)
	assert.Equal(t, 3, *got.Points)

	got.Status = domain.StatusTodo
	got.UpdatedAt = time.Now().UTC()
	require.NoError(t, s.UpdateTask(got))

	list, err := s.ListTasks("KA", domain.TaskFilters{})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, domain.StatusTodo, list[0].Status)

	require.NoError(t, s.DeleteTask("KA-001"))
	_, err = s.GetTask("KA-001")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTaskListFilter(t *testing.T) {
	s := testStore(t)
	seedProject(t, s)

	now := time.Now().UTC()
	assignee := "claude"
	for _, task := range []*domain.Task{
		{ID: "KA-001", ProjectID: "KA", Title: "A", Status: domain.StatusBacklog, Priority: domain.PriorityMedium, CreatedAt: now, UpdatedAt: now},
		{ID: "KA-002", ProjectID: "KA", Title: "B", Status: domain.StatusTodo, Priority: domain.PriorityMedium, AssigneeID: &assignee, CreatedAt: now, UpdatedAt: now},
	} {
		require.NoError(t, s.CreateTask(task))
	}

	status := domain.StatusTodo
	list, err := s.ListTasks("KA", domain.TaskFilters{Status: &status})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "KA-002", list[0].ID)
}

func TestTaskDeps(t *testing.T) {
	s := testStore(t)
	seedProject(t, s)

	now := time.Now().UTC()
	for _, task := range []*domain.Task{
		{ID: "KA-001", ProjectID: "KA", Title: "A", Status: domain.StatusBacklog, Priority: domain.PriorityMedium, CreatedAt: now, UpdatedAt: now},
		{ID: "KA-002", ProjectID: "KA", Title: "B", Status: domain.StatusBacklog, Priority: domain.PriorityMedium, CreatedAt: now, UpdatedAt: now},
	} {
		require.NoError(t, s.CreateTask(task))
	}

	require.NoError(t, s.AddDep("KA-001", "KA-002")) // KA-001 blocks KA-002
	blockers, blocking, err := s.GetTaskDeps("KA-001")
	require.NoError(t, err)
	assert.Empty(t, blockers)
	assert.Equal(t, []string{"KA-002"}, blocking)

	blockers2, _, err := s.GetTaskDeps("KA-002")
	require.NoError(t, err)
	assert.Equal(t, []string{"KA-001"}, blockers2)
}

func TestNextTaskSeq(t *testing.T) {
	s := testStore(t)
	seedProject(t, s)

	seq1, err := s.NextTaskSeq("KA")
	require.NoError(t, err)
	assert.Equal(t, 1, seq1)

	seq2, err := s.NextTaskSeq("KA")
	require.NoError(t, err)
	assert.Equal(t, 2, seq2)
}
```

- [ ] **Step 2: Run — expect failure**

```bash
go test ./internal/store/... -run TestTask 2>&1 | head -5
```

Expected: compile error — `CreateTask` undefined.

- [ ] **Step 3: Write internal/store/task.go**

```go
package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"keroagile/internal/domain"
)

func (s *Store) NextTaskSeq(projectID string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO sequences(project_id, next_seq) VALUES(?,1)
		 ON CONFLICT(project_id) DO UPDATE SET next_seq = next_seq + 1`,
		projectID,
	)
	if err != nil {
		return 0, err
	}

	var seq int
	if err = tx.QueryRow(`SELECT next_seq FROM sequences WHERE project_id=?`, projectID).Scan(&seq); err != nil {
		return 0, err
	}
	return seq, tx.Commit()
}

func (s *Store) CreateTask(t *domain.Task) error {
	_, err := s.db.Exec(
		`INSERT INTO tasks(id,project_id,sprint_id,title,description,status,priority,
		 points,assignee_id,branch,pr_number,pr_merged,labels,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.ProjectID, t.SprintID, t.Title, t.Description,
		string(t.Status), string(t.Priority),
		nullableInt(t.Points), nullableStr(t.AssigneeID), t.Branch,
		nullableInt(t.PRNumber), boolInt(t.PRMerged),
		strings.Join(t.Labels, ","),
		t.CreatedAt.UTC().Format(time.RFC3339),
		t.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) GetTask(id string) (*domain.Task, error) {
	row := s.db.QueryRow(
		`SELECT id,project_id,sprint_id,title,description,status,priority,
		 points,assignee_id,branch,pr_number,pr_merged,labels,created_at,updated_at
		 FROM tasks WHERE id=?`, id,
	)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return t, err
}

func (s *Store) ListTasks(projectID string, f domain.TaskFilters) ([]*domain.Task, error) {
	q := `SELECT id,project_id,sprint_id,title,description,status,priority,
	      points,assignee_id,branch,pr_number,pr_merged,labels,created_at,updated_at
	      FROM tasks WHERE project_id=?`
	args := []any{projectID}

	if f.Status != nil {
		q += ` AND status=?`
		args = append(args, string(*f.Status))
	}
	if f.AssigneeID != nil {
		q += ` AND assignee_id=?`
		args = append(args, *f.AssigneeID)
	}
	if f.SprintID != nil {
		q += ` AND sprint_id=?`
		args = append(args, *f.SprintID)
	}
	q += ` ORDER BY created_at ASC`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) UpdateTask(t *domain.Task) error {
	_, err := s.db.Exec(
		`UPDATE tasks SET sprint_id=?,title=?,description=?,status=?,priority=?,
		 points=?,assignee_id=?,branch=?,pr_number=?,pr_merged=?,labels=?,updated_at=?
		 WHERE id=?`,
		t.SprintID, t.Title, t.Description, string(t.Status), string(t.Priority),
		nullableInt(t.Points), nullableStr(t.AssigneeID), t.Branch,
		nullableInt(t.PRNumber), boolInt(t.PRMerged),
		strings.Join(t.Labels, ","),
		t.UpdatedAt.UTC().Format(time.RFC3339),
		t.ID,
	)
	return err
}

func (s *Store) DeleteTask(id string) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id=?`, id)
	return err
}

func (s *Store) GetTaskDeps(taskID string) (blockers, blocking []string, err error) {
	rows, err := s.db.Query(`SELECT blocker_id FROM task_deps WHERE blocked_id=?`, taskID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, nil, err
		}
		blockers = append(blockers, id)
	}

	rows2, err := s.db.Query(`SELECT blocked_id FROM task_deps WHERE blocker_id=?`, taskID)
	if err != nil {
		return nil, nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var id string
		if err := rows2.Scan(&id); err != nil {
			return nil, nil, err
		}
		blocking = append(blocking, id)
	}
	return blockers, blocking, nil
}

func (s *Store) AddDep(blockerID, blockedID string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO task_deps(blocker_id,blocked_id) VALUES(?,?)`,
		blockerID, blockedID,
	)
	return err
}

func (s *Store) RemoveDep(blockerID, blockedID string) error {
	_, err := s.db.Exec(
		`DELETE FROM task_deps WHERE blocker_id=? AND blocked_id=?`,
		blockerID, blockedID,
	)
	return err
}

func scanTask(r rowScanner) (*domain.Task, error) {
	var t domain.Task
	var status, priority, labels, createdAt, updatedAt string
	var points, prNumber sql.NullInt64
	var sprintID sql.NullInt64
	var assigneeID sql.NullString
	var prMerged int

	err := r.Scan(
		&t.ID, &t.ProjectID, &sprintID, &t.Title, &t.Description,
		&status, &priority, &points, &assigneeID, &t.Branch,
		&prNumber, &prMerged, &labels, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	t.Status = domain.Status(status)
	t.Priority = domain.Priority(priority)
	t.PRMerged = prMerged == 1
	if labels != "" {
		t.Labels = strings.Split(labels, ",")
	}
	if points.Valid {
		v := int(points.Int64)
		t.Points = &v
	}
	if sprintID.Valid {
		v := sprintID.Int64
		t.SprintID = &v
	}
	if assigneeID.Valid {
		t.AssigneeID = &assigneeID.String
	}
	if prNumber.Valid {
		v := int(prNumber.Int64)
		t.PRNumber = &v
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &t, nil
}

func nullableInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func nullableStr(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}
```

- [ ] **Step 4: Run all store tests**

```bash
go test ./internal/store/... -v 2>&1 | tail -15
```

Expected: all tests PASS. If the compile-time interface check fails, a missing method name is listed — add it.

- [ ] **Step 5: Commit**

```bash
git add internal/store/task.go internal/store/task_test.go
git commit -m "feat: SQLite task store — CRUD, deps, sequence generation"
```

---

### Task 8: Config

**Files:**
- Create: `internal/config/config.go`

- [ ] **Step 1: Write internal/config/config.go**

```go
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DefaultProject  string `toml:"default_project"`
	DefaultAssignee string `toml:"default_assignee"`
}

func DefaultPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "keroagile", "config.toml")
}

func DBPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "keroagile", "keroagile.db")
}

func Load() (*Config, error) {
	path := DefaultPath()
	cfg := &Config{}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	_, err := toml.DecodeFile(path, cfg)
	return cfg, err
}

func Save(cfg *Config) error {
	path := DefaultPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}
```

- [ ] **Step 2: Verify compile**

```bash
go build ./internal/config/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat: config — Load/Save TOML with default paths"
```

---

### Task 9: Git integration

**Files:**
- Create: `internal/git/client.go`
- Create: `internal/git/repo.go`
- Create: `internal/git/github.go`

- [ ] **Step 1: Write internal/git/client.go**

```go
package git

import (
	"bytes"
	"os/exec"
	"strings"
)

type Client struct {
	RepoPath string
}

func New(repoPath string) *Client {
	return &Client{RepoPath: repoPath}
}

func (c *Client) execGit(args ...string) (string, error) {
	fullArgs := append([]string{"-C", c.RepoPath}, args...)
	out, err := exec.Command("git", fullArgs...).Output()
	return strings.TrimSpace(string(out)), err
}

func execGH(args ...string) (string, error) {
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if ok := (err.Error() != ""); ok {
			_ = bytes.TrimSpace(exitErr.Stderr)
		}
	}
	return strings.TrimSpace(string(out)), err
}

// Available returns true if the git binary is found.
func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// GHAvailable returns true if the gh CLI is found.
func GHAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}
```

- [ ] **Step 2: Write internal/git/repo.go**

```go
package git

import (
	"strings"
)

type Commit struct {
	Hash    string
	Subject string
	When    string
}

// CurrentBranch returns the current branch name in the repo.
func (c *Client) CurrentBranch() (string, error) {
	return c.execGit("branch", "--show-current")
}

// CommitLog returns the last n commits on the given branch.
func (c *Client) CommitLog(branch string, n int) ([]Commit, error) {
	out, err := c.execGit(
		"log", branch,
		"--oneline",
		"-"+string(rune('0'+n)),
		`--format=%h|||%s|||%cr`,
	)
	if err != nil || out == "" {
		return nil, err
	}
	var commits []Commit
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|||", 3)
		if len(parts) != 3 {
			continue
		}
		commits = append(commits, Commit{
			Hash:    parts[0],
			Subject: parts[1],
			When:    parts[2],
		})
	}
	return commits, nil
}

// AutoLinkTaskID checks if the branch name contains a task ID (e.g. feature/KA-001-foo → "KA-001").
func AutoLinkTaskID(branch string) string {
	parts := strings.Split(branch, "/")
	for _, part := range parts {
		segments := strings.Split(part, "-")
		if len(segments) >= 2 {
			// Check pattern: UPPERCASE-digits
			prefix := segments[0]
			if prefix == strings.ToUpper(prefix) && len(prefix) >= 2 {
				if isDigits(segments[1]) {
					return prefix + "-" + segments[1]
				}
			}
		}
	}
	return ""
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
```

- [ ] **Step 3: Write internal/git/github.go**

```go
package git

import (
	"encoding/json"
	"fmt"
)

type PRStatus struct {
	Number int
	State  string // OPEN, MERGED, CLOSED
	Title  string
	Body   string
	URL    string
	Comments int
}

type OpenPR struct {
	Number      int    `json:"number"`
	HeadRefName string `json:"headRefName"`
	Title       string `json:"title"`
	URL         string `json:"url"`
}

// PRView fetches the state of a single PR by number.
func PRView(repoPath string, prNumber int) (*PRStatus, error) {
	out, err := execGH("pr", "view", fmt.Sprintf("%d", prNumber),
		"--json", "number,state,title,body,url,comments",
		"-R", repoPath,
	)
	if err != nil {
		return nil, err
	}
	var result struct {
		Number   int    `json:"number"`
		State    string `json:"state"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		URL      string `json:"url"`
		Comments []any  `json:"comments"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, err
	}
	return &PRStatus{
		Number:   result.Number,
		State:    result.State,
		Title:    result.Title,
		URL:      result.URL,
		Comments: len(result.Comments),
	}, nil
}

// ListOpenPRs returns open PRs for the repo, used for auto-detect.
func ListOpenPRs(repoPath string) ([]OpenPR, error) {
	out, err := execGH("pr", "list",
		"--json", "number,headRefName,title,url",
		"--state", "open",
		"-R", repoPath,
	)
	if err != nil {
		return nil, err
	}
	var prs []OpenPR
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return nil, err
	}
	return prs, nil
}
```

- [ ] **Step 4: Write AutoLinkTaskID test**

```go
// internal/git/repo_test.go
package git_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"keroagile/internal/git"
)

func TestAutoLinkTaskID(t *testing.T) {
	cases := []struct {
		branch string
		want   string
	}{
		{"feature/KA-001-api-layer", "KA-001"},
		{"feature/KA-012", "KA-012"},
		{"main", ""},
		{"fix/some-bug", ""},
		{"PROJ-42-thing", "PROJ-42"},
	}
	for _, tc := range cases {
		t.Run(tc.branch, func(t *testing.T) {
			assert.Equal(t, tc.want, git.AutoLinkTaskID(tc.branch))
		})
	}
}
```

- [ ] **Step 5: Run test**

```bash
go test ./internal/git/... -v
```

Expected: all cases PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/git/
git commit -m "feat: git/gh integration — branch, commits, PR status, auto-link"
```

---

### Task 10: CLI — root wiring + output helpers

**Files:**
- Modify: `cmd/keroagile/main.go`
- Create: `cmd/keroagile/output.go`

- [ ] **Step 1: Rewrite main.go**

```go
package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"keroagile/internal/config"
	"keroagile/internal/domain"
	"keroagile/internal/store"
)

var (
	jsonFlag bool
	svc      *domain.Service
	cfg      *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "KeroAgile",
	Short: "Terminal agile board",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip DB init for help commands
		if cmd.Name() == "help" {
			return nil
		}
		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("config: %w", err)
		}

		dbPath := config.DBPath()
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return err
		}
		db, err := store.Open(dbPath)
		if err != nil {
			return fmt.Errorf("db: %w", err)
		}
		svc = domain.NewService(store.New(db))

		// Non-TTY stdout → imply --json
		if !isTerminal(os.Stdout) {
			jsonFlag = true
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// No subcommand — launch TUI (stub for now)
		fmt.Println("TUI launching... (Plan B)")
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "output JSON")
	rootCmd.AddCommand(projectCmd, taskCmd, sprintCmd, userCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Add missing import and isTerminal helper**

Add to main.go:
```go
import (
	// ... existing imports ...
	"path/filepath"

	"golang.org/x/term"
)

func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
```

Then add to go.mod (run `go get golang.org/x/term` and `go mod tidy`).

- [ ] **Step 3: Write cmd/keroagile/output.go**

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "json encode: %v\n", err)
		os.Exit(1)
	}
}

func printError(code int, msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+msg+"\n", args...)
	os.Exit(code)
}

func exitNotFound(id string) {
	printError(1, "%s not found", id)
}

func exitValidation(msg string) {
	printError(2, msg)
}
```

- [ ] **Step 4: Build check**

```bash
go build ./cmd/keroagile/... 2>&1
```

Fix any import or missing-symbol errors until it compiles.

- [ ] **Step 5: Commit**

```bash
git add cmd/keroagile/main.go cmd/keroagile/output.go
git commit -m "feat: CLI root — db wiring, --json flag, TTY detection, output helpers"
```

---

### Task 11: CLI — project + user commands

**Files:**
- Create: `cmd/keroagile/cmd_project.go`
- Create: `cmd/keroagile/cmd_user.go`

- [ ] **Step 1: Write cmd/keroagile/cmd_project.go**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{Use: "project", Short: "Manage projects"}

var projectAddCmd = &cobra.Command{
	Use:   "add <id> <name>",
	Short: "Create a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		if err := svc.CreateProject(args[0], args[1], repo); err != nil {
			return err
		}
		p, _ := svc.GetProject(args[0])
		if jsonFlag {
			printJSON(p)
		} else {
			fmt.Printf("created project %s (%s)\n", p.ID, p.Name)
		}
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		projects, err := svc.ListProjects()
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(projects)
			return nil
		}
		for _, p := range projects {
			sprint := ""
			if p.SprintMode {
				sprint = " [sprints]"
			}
			fmt.Printf("  %s  %s%s  %s\n", p.ID, p.Name, sprint, p.RepoPath)
		}
		return nil
	},
}

var projectSprintCmd = &cobra.Command{
	Use:   "set-sprint <project-id> on|off",
	Short: "Enable or disable sprint mode for a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		enabled := args[1] == "on"
		if err := svc.SetSprintMode(args[0], enabled); err != nil {
			return err
		}
		fmt.Printf("sprint mode %s for %s\n", args[1], args[0])
		return nil
	},
}

func init() {
	projectAddCmd.Flags().String("repo", "", "absolute path to git repo")
	projectCmd.AddCommand(projectAddCmd, projectListCmd, projectSprintCmd)
}
```

- [ ] **Step 2: Write cmd/keroagile/cmd_user.go**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{Use: "user", Short: "Manage team members"}

var userAddCmd = &cobra.Command{
	Use:   "add <id> <display-name>",
	Short: "Add a team member",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		isAgent, _ := cmd.Flags().GetBool("agent")
		u, err := svc.CreateUser(args[0], args[1], isAgent)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(u)
		} else {
			fmt.Printf("added user %s (%s)\n", u.ID, u.DisplayName)
		}
		return nil
	},
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List team members",
	RunE: func(cmd *cobra.Command, args []string) error {
		users, err := svc.ListUsers()
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(users)
			return nil
		}
		for _, u := range users {
			prefix := "👤"
			if u.IsAgent {
				prefix = "🤖"
			}
			fmt.Printf("  %s %s  %s\n", prefix, u.ID, u.DisplayName)
		}
		return nil
	},
}

func init() {
	userAddCmd.Flags().Bool("agent", false, "mark as AI agent")
	userCmd.AddCommand(userAddCmd, userListCmd)
}
```

- [ ] **Step 3: Build and smoke test**

```bash
go build -o KeroAgile ./cmd/keroagile/ && \
  ./KeroAgile project add KA "myapp" --repo /tmp/myapp && \
  ./KeroAgile project list && \
  ./KeroAgile user add matt "Matt" && \
  ./KeroAgile user add claude "Claude" --agent && \
  ./KeroAgile user list
```

Expected:
```
created project KA (myapp)
  KA  myapp  /tmp/myapp
added user matt (Matt)
added user claude (Claude)
  👤 matt  Matt
  🤖 claude  Claude
```

- [ ] **Step 4: Commit**

```bash
git add cmd/keroagile/cmd_project.go cmd/keroagile/cmd_user.go
git commit -m "feat: CLI project and user commands"
```

---

### Task 12: CLI — task commands

**Files:**
- Create: `cmd/keroagile/cmd_task.go`

- [ ] **Step 1: Write cmd/keroagile/cmd_task.go**

```go
package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"keroagile/internal/domain"
)

var taskCmd = &cobra.Command{Use: "task", Short: "Manage tasks"}

var taskAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Create a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			project = cfg.DefaultProject
		}
		if project == "" {
			exitValidation("--project is required (or set default_project in config)")
		}

		assignee, _ := cmd.Flags().GetString("assignee")
		if assignee == "" {
			assignee = cfg.DefaultAssignee
		}
		priorityStr, _ := cmd.Flags().GetString("priority")
		statusStr, _ := cmd.Flags().GetString("status")
		points, _ := cmd.Flags().GetInt("points")
		labelsStr, _ := cmd.Flags().GetString("labels")
		desc, _ := cmd.Flags().GetString("description")

		var labels []string
		if labelsStr != "" {
			labels = strings.Split(labelsStr, ",")
		}

		opts := domain.TaskCreateOpts{
			AssigneeID: assignee,
			Points:     points,
			Labels:     labels,
		}
		if priorityStr != "" {
			opts.Priority = domain.Priority(priorityStr)
		}
		if statusStr != "" {
			opts.Status = domain.Status(statusStr)
		}

		t, err := svc.CreateTask(args[0], desc, project, opts)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(t)
		} else {
			fmt.Printf("created %s: %s\n", t.ID, t.Title)
		}
		return nil
	},
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			project = cfg.DefaultProject
		}
		statusStr, _ := cmd.Flags().GetString("status")
		assignee, _ := cmd.Flags().GetString("assignee")

		f := domain.TaskFilters{}
		if statusStr != "" {
			s := domain.Status(statusStr)
			f.Status = &s
		}
		if assignee != "" {
			f.AssigneeID = &assignee
		}

		tasks, err := svc.ListTasks(project, f)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(tasks)
			return nil
		}
		for _, t := range tasks {
			assigneeStr := ""
			if t.AssigneeID != nil {
				assigneeStr = " @" + *t.AssigneeID
			}
			fmt.Printf("  %s  %-30s  %-12s  %-8s%s\n",
				t.ID, t.Title, t.Status.Label(), t.Priority.Label(), assigneeStr)
		}
		return nil
	},
}

var taskGetCmd = &cobra.Command{
	Use:   "get <task-id>",
	Short: "Show task details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		t, err := svc.GetTask(args[0])
		if err != nil {
			exitNotFound(args[0])
		}
		if jsonFlag {
			printJSON(t)
			return nil
		}
		fmt.Printf("%s  %s\n", t.ID, t.Title)
		fmt.Printf("  Status:   %s\n", t.Status.Label())
		fmt.Printf("  Priority: %s\n", t.Priority.Label())
		if t.AssigneeID != nil {
			fmt.Printf("  Assignee: %s\n", *t.AssigneeID)
		}
		if t.Points != nil {
			fmt.Printf("  Points:   %d\n", *t.Points)
		}
		if t.Branch != "" {
			fmt.Printf("  Branch:   %s\n", t.Branch)
		}
		if len(t.Blockers) > 0 {
			fmt.Printf("  Blockers: %s\n", strings.Join(t.Blockers, ", "))
		}
		return nil
	},
}

var taskMoveCmd = &cobra.Command{
	Use:   "move <task-id> <status>",
	Short: "Move task to a new status",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		t, err := svc.MoveTask(args[0], domain.Status(args[1]))
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(t)
		} else {
			fmt.Printf("%s → %s\n", t.ID, t.Status.Label())
		}
		return nil
	},
}

var taskLinkBranchCmd = &cobra.Command{
	Use:   "link-branch <task-id> <branch>",
	Short: "Link a git branch to a task",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := svc.LinkBranch(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("linked %s → branch %s\n", args[0], args[1])
		return nil
	},
}

var taskLinkPRCmd = &cobra.Command{
	Use:   "link-pr <task-id> <pr-number>",
	Short: "Link a GitHub PR to a task",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		prNum, err := strconv.Atoi(args[1])
		if err != nil {
			exitValidation("pr-number must be an integer")
		}
		if err := svc.LinkPR(args[0], prNum); err != nil {
			return err
		}
		fmt.Printf("linked %s → PR #%d\n", args[0], prNum)
		return nil
	},
}

var taskDeleteCmd = &cobra.Command{
	Use:   "delete <task-id>",
	Short: "Delete a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := svc.DeleteTask(args[0]); err != nil {
			return err
		}
		fmt.Printf("deleted %s\n", args[0])
		return nil
	},
}

func init() {
	taskAddCmd.Flags().String("project", "", "project ID")
	taskAddCmd.Flags().String("assignee", "", "assignee user ID")
	taskAddCmd.Flags().String("priority", "medium", "low|medium|high|critical")
	taskAddCmd.Flags().String("status", "backlog", "backlog|todo|in_progress|review|done")
	taskAddCmd.Flags().Int("points", 0, "story points")
	taskAddCmd.Flags().String("labels", "", "comma-separated labels")
	taskAddCmd.Flags().String("description", "", "task description")

	taskListCmd.Flags().String("project", "", "project ID")
	taskListCmd.Flags().String("status", "", "filter by status")
	taskListCmd.Flags().String("assignee", "", "filter by assignee")

	taskCmd.AddCommand(taskAddCmd, taskListCmd, taskGetCmd, taskMoveCmd,
		taskLinkBranchCmd, taskLinkPRCmd, taskDeleteCmd)
}
```

- [ ] **Step 2: Build and end-to-end smoke test**

```bash
go build -o KeroAgile ./cmd/keroagile/

# Setup
./KeroAgile project add KA myapp --repo /tmp/myapp
./KeroAgile user add claude "Claude" --agent

# Task lifecycle
./KeroAgile task add "Build login" --project KA --assignee claude --priority high --points 3
./KeroAgile task list --project KA
./KeroAgile task move KA-001 todo
./KeroAgile task get KA-001

# JSON output
./KeroAgile task list --project KA --json
```

Expected: tasks created, listed, moved; JSON output is valid JSON array.

- [ ] **Step 3: Commit**

```bash
git add cmd/keroagile/cmd_task.go
git commit -m "feat: CLI task commands — add, list, get, move, link-branch, link-pr, delete"
```

---

### Task 13: CLI — sprint commands + final build

**Files:**
- Create: `cmd/keroagile/cmd_sprint.go`

- [ ] **Step 1: Write cmd/keroagile/cmd_sprint.go**

```go
package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var sprintCmd = &cobra.Command{Use: "sprint", Short: "Manage sprints"}

var sprintAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a sprint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		startStr, _ := cmd.Flags().GetString("start")
		endStr, _ := cmd.Flags().GetString("end")

		var start, end *time.Time
		if startStr != "" {
			t, err := time.Parse("2006-01-02", startStr)
			if err != nil {
				exitValidation("--start must be YYYY-MM-DD")
			}
			start = &t
		}
		if endStr != "" {
			t, err := time.Parse("2006-01-02", endStr)
			if err != nil {
				exitValidation("--end must be YYYY-MM-DD")
			}
			end = &t
		}

		sp, err := svc.CreateSprint(args[0], project, start, end)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(sp)
		} else {
			fmt.Printf("created sprint %d: %s\n", sp.ID, sp.Name)
		}
		return nil
	},
}

var sprintListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sprints for a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		sprints, err := svc.ListSprints(project)
		if err != nil {
			return err
		}
		if jsonFlag {
			printJSON(sprints)
			return nil
		}
		for _, sp := range sprints {
			fmt.Printf("  #%d  %-20s  %s\n", sp.ID, sp.Name, string(sp.Status))
		}
		return nil
	},
}

var sprintActivateCmd = &cobra.Command{
	Use:   "activate <sprint-id>",
	Short: "Activate a sprint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			exitValidation("sprint-id must be an integer")
		}
		if err := svc.ActivateSprint(id); err != nil {
			return err
		}
		fmt.Printf("activated sprint %d\n", id)
		return nil
	},
}

func init() {
	sprintAddCmd.Flags().String("project", "", "project ID")
	sprintAddCmd.Flags().String("start", "", "start date YYYY-MM-DD")
	sprintAddCmd.Flags().String("end", "", "end date YYYY-MM-DD")
	sprintListCmd.Flags().String("project", "", "project ID")
	sprintCmd.AddCommand(sprintAddCmd, sprintListCmd, sprintActivateCmd)
}
```

- [ ] **Step 2: Full build + all tests**

```bash
go build -o KeroAgile ./cmd/keroagile/ && go test ./...
```

Expected: build succeeds, all tests pass.

- [ ] **Step 3: Commit**

```bash
git add cmd/keroagile/cmd_sprint.go
git commit -m "feat: CLI sprint commands — add, list, activate"
```

- [ ] **Step 4: Tag Plan A complete**

```bash
git tag v0.1.0-core
```

---

*Plan A complete. The binary is fully functional from the CLI. Proceed to Plan B for the TUI.*
