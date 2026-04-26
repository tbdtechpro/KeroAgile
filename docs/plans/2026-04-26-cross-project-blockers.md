# Cross-Project Blockers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add cross-project blocker creation/removal to the API, CLI, TUI, and web UI, and fix the pre-existing TUI form persistence bug (roadmap §2.2).

**Architecture:** The data layer (`task_deps` table, `Service.AddDep`/`RemoveDep`) already supports cross-project deps with no changes needed. This plan is additive: (1) add a `TaskSummary` type + `SearchTasks` to support autocomplete, (2) enrich `GetTask` with titled blocker details, (3) add three new API routes, (4) add two CLI commands, (5) add a TUI fuzzy-search overlay and fix form persistence, and (6) update the web UI with autocomplete in the modal and enriched chips in the detail drawer.

**Tech Stack:** Go 1.23 / SQLite (modernc.org/sqlite) / Cobra CLI / BubbleTea TUI (charmbracelet/bubbles v0.20.0) / React + TypeScript + TanStack Query v5 + Tailwind

---

## Files

| File | Change |
|---|---|
| `internal/domain/task.go` | Add `TaskSummary` struct; add `BlockerDetails`, `BlockingDetails` to `Task` |
| `internal/domain/store.go` | Add `SearchTasks(q string, limit int) ([]*TaskSummary, error)` to `Store` interface |
| `internal/domain/service.go` | Thin `SearchTasks` pass-through |
| `internal/domain/service_test.go` | Add `SearchTasks` to `mockStore`; add cross-project dep test |
| `internal/store/task.go` | Implement `SearchTasks`; enrich `GetTask` with batch blocker lookup |
| `internal/store/store_test.go` | `SearchTasks` + `GetTask` enrichment tests |
| `internal/api/handlers.go` | `handleSearchTasks`, `handleAddBlocker`, `handleRemoveBlocker` |
| `internal/api/server.go` | Register 3 new routes |
| `internal/api/server_test.go` | Tests for 3 new endpoints |
| `cmd/keroagile/cmd_task.go` | `task block` and `task unblock` subcommands |
| `internal/tui/forms/blocker_picker.go` | New BubbleTea model: fuzzy-search overlay |
| `internal/tui/forms/task_form.go` | Emit `OpenBlockerPickerMsg` on Enter in Blocks/BlockedBy; add `AppendToBlocker` method |
| `internal/tui/msgs.go` | Add `openBlockerPickerMsg` |
| `internal/tui/app.go` | Handle picker overlay; fix `doUpdateTask` to persist dep diffs; add `diffBlockers` helper |
| `web/src/api/types.ts` | Add `TaskSummary`; extend `Task` with `blocker_details`, `blocking_details` |
| `web/src/api/client.ts` | `searchTasks`, `addBlocker`, `removeBlocker` |
| `web/src/api/queries.ts` | `useAddBlocker`, `useRemoveBlocker` |
| `web/src/components/TaskModal.tsx` | Blocker autocomplete field |
| `web/src/components/TaskDetail.tsx` | Enriched chips, Blocking section, navigation |

---

### Task 1: Domain types + Store interface + Service pass-through

**Files:**
- Modify: `internal/domain/task.go`
- Modify: `internal/domain/store.go`
- Modify: `internal/domain/service.go`
- Modify: `internal/domain/service_test.go`

- [ ] **Step 1: Write failing test for cross-project dep + SearchTasks pass-through**

Add to `internal/domain/service_test.go`:

```go
func (m *mockStore) SearchTasks(q string, limit int) ([]*domain.TaskSummary, error) {
    var out []*domain.TaskSummary
    for _, t := range m.tasks {
        if strings.Contains(t.ID, q) || strings.Contains(t.Title, q) {
            out = append(out, &domain.TaskSummary{
                ID:        t.ID,
                Title:     t.Title,
                ProjectID: t.ProjectID,
                Status:    t.Status,
            })
            if len(out) >= limit {
                break
            }
        }
    }
    return out, nil
}

func TestSearchTasksPassThrough(t *testing.T) {
    svc := domain.NewService(newMock())
    require.NoError(t, svc.CreateProject("KA", "KeroAgile", ""))
    require.NoError(t, svc.CreateProject("KCP", "KeroCareer", ""))
    _, err := svc.CreateTask("Add JWT auth", "", "KA", domain.TaskCreateOpts{})
    require.NoError(t, err)
    _, err = svc.CreateTask("Career DB migration", "", "KCP", domain.TaskCreateOpts{})
    require.NoError(t, err)

    results, err := svc.SearchTasks("JWT", 10)
    require.NoError(t, err)
    require.Len(t, results, 1)
    assert.Equal(t, "KA", results[0].ProjectID)
}

func TestCrossProjectDep(t *testing.T) {
    mock := newMock()
    svc := domain.NewService(mock)
    require.NoError(t, svc.CreateProject("KA", "KeroAgile", ""))
    require.NoError(t, svc.CreateProject("KCP", "KeroCareer", ""))
    ka, err := svc.CreateTask("KA task", "", "KA", domain.TaskCreateOpts{})
    require.NoError(t, err)
    kcp, err := svc.CreateTask("KCP task", "", "KCP", domain.TaskCreateOpts{})
    require.NoError(t, err)

    // AddDep should accept IDs from different projects
    require.NoError(t, svc.AddDep(kcp.ID, ka.ID)) // kcp.ID blocks ka.ID

    t2, err := svc.GetTask(ka.ID)
    require.NoError(t, err)
    assert.Contains(t, t2.Blockers, kcp.ID)
}
```

Also add `"strings"` to the import if needed (it's likely already there). Add the `TestCrossProjectDep` import `"strings"` check.

- [ ] **Step 2: Run the tests — expect compile failure (TaskSummary undefined)**

```bash
cd /home/matt/github/KeroAgile && go test ./internal/domain/...
```

Expected: compile error `domain.TaskSummary undefined` and `svc.SearchTasks undefined`

- [ ] **Step 3: Add `TaskSummary` struct and enrichment fields to `internal/domain/task.go`**

Append to the end of `internal/domain/task.go` (after the `Task` struct):

```go
// TaskSummary is a lightweight task representation used in search results
// and blocker enrichment. It does not include body fields like description,
// labels, or sprint.
type TaskSummary struct {
    ID        string `json:"id"`
    Title     string `json:"title"`
    ProjectID string `json:"project_id"`
    Status    Status `json:"status"`
}
```

Also add these two fields to the `Task` struct, after the existing `Blocking []string` field (line 105):

```go
    BlockerDetails  []*TaskSummary `json:"blocker_details,omitempty"`
    BlockingDetails []*TaskSummary `json:"blocking_details,omitempty"`
```

- [ ] **Step 4: Add `SearchTasks` to `Store` interface in `internal/domain/store.go`**

Add one line to the Tasks section of the `Store` interface, after `RemoveDep`:

```go
    SearchTasks(q string, limit int) ([]*TaskSummary, error)
```

- [ ] **Step 5: Add `SearchTasks` pass-through to `internal/domain/service.go`**

Append after `RemoveDep` in `service.go`:

```go
func (s *Service) SearchTasks(q string, limit int) ([]*TaskSummary, error) {
    return s.store.SearchTasks(q, limit)
}
```

- [ ] **Step 6: Run tests — expect compile success, test pass**

```bash
go test ./internal/domain/...
```

Expected: all pass (the mock's `SearchTasks` is now satisfied)

- [ ] **Step 7: Commit**

```bash
git add internal/domain/task.go internal/domain/store.go internal/domain/service.go internal/domain/service_test.go
git commit -m "domain: add TaskSummary type, SearchTasks to Store interface, cross-project dep test"
```

---

### Task 2: Store SearchTasks implementation

**Files:**
- Modify: `internal/store/task.go`
- Modify: `internal/store/store_test.go`

- [ ] **Step 1: Write failing store tests**

Add to `internal/store/store_test.go`:

```go
func TestSearchTasksFuzzy(t *testing.T) {
    s := testStore(t)
    require.NoError(t, s.CreateProject(&domain.Project{ID: "KA", Name: "KeroAgile"}))
    require.NoError(t, s.CreateProject(&domain.Project{ID: "KCP", Name: "KeroCareer"}))

    // Sequence numbers required before task IDs are meaningful
    seq1, _ := s.NextTaskSeq("KA")
    kaID := fmt.Sprintf("KA-%03d", seq1)
    require.NoError(t, s.CreateTask(&domain.Task{
        ID: kaID, ProjectID: "KA", Title: "Add JWT auth",
        Status: domain.StatusBacklog, Priority: domain.PriorityMedium,
    }))

    seq2, _ := s.NextTaskSeq("KCP")
    kcpID := fmt.Sprintf("KCP-%03d", seq2)
    require.NoError(t, s.CreateTask(&domain.Task{
        ID: kcpID, ProjectID: "KCP", Title: "Career DB migration",
        Status: domain.StatusTodo, Priority: domain.PriorityMedium,
    }))

    // Title substring
    results, err := s.SearchTasks("JWT", 10)
    require.NoError(t, err)
    require.Len(t, results, 1)
    assert.Equal(t, kaID, results[0].ID)
    assert.Equal(t, "Add JWT auth", results[0].Title)
    assert.Equal(t, "KA", results[0].ProjectID)

    // ID prefix
    results, err = s.SearchTasks("KCP", 10)
    require.NoError(t, err)
    require.Len(t, results, 1)
    assert.Equal(t, kcpID, results[0].ID)

    // Cross-project: "a" matches both "JWT" and "Career"
    results, err = s.SearchTasks("a", 10)
    require.NoError(t, err)
    assert.GreaterOrEqual(t, len(results), 2)

    // limit respected
    results, err = s.SearchTasks("a", 1)
    require.NoError(t, err)
    assert.Len(t, results, 1)
}

func TestSearchTasksHintOrdering(t *testing.T) {
    s := testStore(t)
    require.NoError(t, s.CreateProject(&domain.Project{ID: "KA", Name: "KeroAgile"}))
    require.NoError(t, s.CreateProject(&domain.Project{ID: "KCP", Name: "KeroCareer"}))

    seq1, _ := s.NextTaskSeq("KA")
    kaID := fmt.Sprintf("KA-%03d", seq1)
    require.NoError(t, s.CreateTask(&domain.Task{
        ID: kaID, ProjectID: "KA", Title: "auth feature",
        Status: domain.StatusBacklog, Priority: domain.PriorityMedium,
    }))

    seq2, _ := s.NextTaskSeq("KCP")
    kcpID := fmt.Sprintf("KCP-%03d", seq2)
    require.NoError(t, s.CreateTask(&domain.Task{
        ID: kcpID, ProjectID: "KCP", Title: "auth middleware",
        Status: domain.StatusBacklog, Priority: domain.PriorityMedium,
    }))

    // With hint=KCP, KCP task should come first
    results, err := s.SearchTasksWithHint("auth", 10, "KCP")
    require.NoError(t, err)
    require.Len(t, results, 2)
    assert.Equal(t, kcpID, results[0].ID)
}
```

Also add `"fmt"` to the store_test.go imports if not present.

- [ ] **Step 2: Run tests — expect compile failure**

```bash
go test ./internal/store/...
```

Expected: `s.SearchTasks undefined`, `s.SearchTasksWithHint undefined`

- [ ] **Step 3: Implement `SearchTasks` and `SearchTasksWithHint` in `internal/store/task.go`**

Add these two methods to `internal/store/task.go` (append after `RemoveDep`):

```go
func (s *Store) SearchTasks(q string, limit int) ([]*domain.TaskSummary, error) {
    return s.SearchTasksWithHint(q, limit, "")
}

func (s *Store) SearchTasksWithHint(q string, limit int, hintProjectID string) ([]*domain.TaskSummary, error) {
    like := "%" + q + "%"
    rows, err := s.db.Query(
        `SELECT id, title, project_id, status FROM tasks
         WHERE id LIKE ? OR title LIKE ?
         ORDER BY (project_id = ?) DESC, rowid ASC
         LIMIT ?`,
        like, like, hintProjectID, limit,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var out []*domain.TaskSummary
    for rows.Next() {
        var ts domain.TaskSummary
        var status string
        if err := rows.Scan(&ts.ID, &ts.Title, &ts.ProjectID, &status); err != nil {
            return nil, err
        }
        ts.Status = domain.Status(status)
        out = append(out, &ts)
    }
    return out, rows.Err()
}
```

Note: `SearchTasksWithHint` is not on the `Store` interface — only `SearchTasks` is. The API handler will call `SearchTasks` and pass the hint via a different path (see Task 4). Alternatively, we expose `SearchTasksWithHint` via the service. For simplicity, add it to the Store interface now:

Add to `internal/domain/store.go` after `SearchTasks`:
```go
    SearchTasksWithHint(q string, limit int, hintProjectID string) ([]*TaskSummary, error)
```

Add to `internal/domain/service.go`:
```go
func (s *Service) SearchTasksWithHint(q string, limit int, hintProjectID string) ([]*TaskSummary, error) {
    return s.store.SearchTasksWithHint(q, limit, hintProjectID)
}
```

Add to `mockStore` in `internal/domain/service_test.go`:
```go
func (m *mockStore) SearchTasksWithHint(q string, limit int, hintProjectID string) ([]*domain.TaskSummary, error) {
    return m.SearchTasks(q, limit) // mock ignores hint
}
```

- [ ] **Step 4: Run tests — expect all pass**

```bash
go test ./internal/store/... ./internal/domain/...
```

Expected: all pass

- [ ] **Step 5: Commit**

```bash
git add internal/store/task.go internal/store/store_test.go internal/domain/store.go internal/domain/service.go internal/domain/service_test.go
git commit -m "store: implement SearchTasks fuzzy search with hint-based project ordering"
```

---

### Task 3: Store GetTask enrichment

**Files:**
- Modify: `internal/store/task.go`
- Modify: `internal/store/store_test.go`

- [ ] **Step 1: Write failing test for GetTask enrichment**

Add to `internal/store/store_test.go`:

```go
func TestGetTaskEnrichesBlockerDetails(t *testing.T) {
    s := testStore(t)
    require.NoError(t, s.CreateProject(&domain.Project{ID: "KA", Name: "KeroAgile"}))
    require.NoError(t, s.CreateProject(&domain.Project{ID: "KCP", Name: "KeroCareer"}))

    seq1, _ := s.NextTaskSeq("KA")
    kaID := fmt.Sprintf("KA-%03d", seq1)
    require.NoError(t, s.CreateTask(&domain.Task{
        ID: kaID, ProjectID: "KA", Title: "Implement handler",
        Status: domain.StatusInProgress, Priority: domain.PriorityHigh,
    }))

    seq2, _ := s.NextTaskSeq("KCP")
    kcpID := fmt.Sprintf("KCP-%03d", seq2)
    require.NoError(t, s.CreateTask(&domain.Task{
        ID: kcpID, ProjectID: "KCP", Title: "Career DB migration",
        Status: domain.StatusTodo, Priority: domain.PriorityMedium,
    }))

    require.NoError(t, s.AddDep(kcpID, kaID)) // kcpID blocks kaID

    task, err := s.GetTask(kaID)
    require.NoError(t, err)
    assert.Contains(t, task.Blockers, kcpID)
    require.Len(t, task.BlockerDetails, 1)
    assert.Equal(t, kcpID, task.BlockerDetails[0].ID)
    assert.Equal(t, "Career DB migration", task.BlockerDetails[0].Title)
    assert.Equal(t, "KCP", task.BlockerDetails[0].ProjectID)
    assert.Equal(t, domain.StatusTodo, task.BlockerDetails[0].Status)
    assert.Empty(t, task.BlockingDetails)
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
go test ./internal/store/... -run TestGetTaskEnrichesBlockerDetails
```

Expected: FAIL — `BlockerDetails` is nil

- [ ] **Step 3: Add batch enrichment helper to `internal/store/task.go`**

Add this helper after `SearchTasksWithHint`:

```go
// fetchSummaries returns TaskSummary for each id in ids, preserving order.
// Any id not found is silently skipped.
func (s *Store) fetchSummaries(ids []string) ([]*domain.TaskSummary, error) {
    if len(ids) == 0 {
        return nil, nil
    }
    placeholders := strings.Repeat("?,", len(ids))
    placeholders = placeholders[:len(placeholders)-1] // trim trailing comma
    args := make([]any, len(ids))
    for i, id := range ids {
        args[i] = id
    }
    rows, err := s.db.Query(
        `SELECT id, title, project_id, status FROM tasks WHERE id IN (`+placeholders+`) ORDER BY rowid ASC`,
        args...,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    byID := make(map[string]*domain.TaskSummary)
    for rows.Next() {
        var ts domain.TaskSummary
        var status string
        if err := rows.Scan(&ts.ID, &ts.Title, &ts.ProjectID, &status); err != nil {
            return nil, err
        }
        ts.Status = domain.Status(status)
        byID[ts.ID] = &ts
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }

    // Preserve input order
    out := make([]*domain.TaskSummary, 0, len(ids))
    for _, id := range ids {
        if ts, ok := byID[id]; ok {
            out = append(out, ts)
        }
    }
    return out, nil
}
```

- [ ] **Step 4: Update `GetTask` to call enrichment**

Replace the existing `GetTask` method body. Find this section (starting after the `scanTask` call):

```go
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
```

Replace with:

```go
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
	if err != nil {
		return nil, err
	}

	blockers, blocking, err := s.GetTaskDeps(id)
	if err != nil {
		return nil, err
	}
	t.Blockers = blockers
	t.Blocking = blocking

	t.BlockerDetails, err = s.fetchSummaries(blockers)
	if err != nil {
		return nil, err
	}
	t.BlockingDetails, err = s.fetchSummaries(blocking)
	if err != nil {
		return nil, err
	}
	return t, nil
}
```

Note: The existing `service.go` `GetTask` calls both `store.GetTask` (which doesn't set deps) and `store.GetTaskDeps` separately. Now that `store.GetTask` sets deps internally, the service's manual `GetTaskDeps` call becomes redundant. We'll keep it for now — the second call will be a no-op since it just overwrites the same data (correctness is preserved). Clean up is left to a follow-on refactor.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/store/... ./internal/domain/...
```

Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add internal/store/task.go internal/store/store_test.go
git commit -m "store: enrich GetTask with BlockerDetails and BlockingDetails via batch fetch"
```

---

### Task 4: API handlers + routes + tests

**Files:**
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/server.go`
- Modify: `internal/api/server_test.go`

- [ ] **Step 1: Write failing API tests**

Add to `internal/api/server_test.go`:

```go
func TestSearchTasks(t *testing.T) {
    srv, svc, _ := newTestServer(t)
    require.NoError(t, svc.CreateProject("KA", "KeroAgile", ""))
    require.NoError(t, svc.CreateProject("KCP", "KeroCareer", ""))

    _, err := svc.CreateUser("alice", "Alice", false)
    require.NoError(t, err)
    hash, _ := api.HashPassword("pw")
    require.NoError(t, svc.SetUserPasswordHash("alice", hash))

    _, err = svc.CreateTask("Add JWT auth", "", "KA", domain.TaskCreateOpts{})
    require.NoError(t, err)
    _, err = svc.CreateTask("Career DB migration", "", "KCP", domain.TaskCreateOpts{})
    require.NoError(t, err)

    loginBody, _ := json.Marshal(map[string]string{"user_id": "alice", "password": "pw"})
    req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(loginBody))
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)
    var lr map[string]string
    json.NewDecoder(w.Body).Decode(&lr)
    token := lr["token"]

    authed := func(method, path string, body []byte) *httptest.ResponseRecorder {
        var buf *bytes.Buffer
        if body != nil {
            buf = bytes.NewBuffer(body)
        } else {
            buf = &bytes.Buffer{}
        }
        r := httptest.NewRequest(method, path, buf)
        r.Header.Set("Authorization", "Bearer "+token)
        rr := httptest.NewRecorder()
        srv.ServeHTTP(rr, r)
        return rr
    }

    // Title match
    rr := authed(http.MethodGet, "/api/search/tasks?q=JWT", nil)
    require.Equal(t, http.StatusOK, rr.Code)
    var resp struct {
        Tasks []domain.TaskSummary `json:"tasks"`
    }
    require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
    require.Len(t, resp.Tasks, 1)
    assert.Equal(t, "KA", resp.Tasks[0].ProjectID)

    // Empty query returns all (up to limit)
    rr = authed(http.MethodGet, "/api/search/tasks?q=", nil)
    require.Equal(t, http.StatusOK, rr.Code)
    json.NewDecoder(rr.Body).Decode(&resp)
    assert.GreaterOrEqual(t, len(resp.Tasks), 2)
}

func TestAddAndRemoveBlocker(t *testing.T) {
    srv, svc, _ := newTestServer(t)
    require.NoError(t, svc.CreateProject("KA", "KeroAgile", ""))
    require.NoError(t, svc.CreateProject("KCP", "KeroCareer", ""))

    _, err := svc.CreateUser("bob", "Bob", false)
    require.NoError(t, err)
    hash, _ := api.HashPassword("pw")
    require.NoError(t, svc.SetUserPasswordHash("bob", hash))

    kaTask, err := svc.CreateTask("KA task", "", "KA", domain.TaskCreateOpts{})
    require.NoError(t, err)
    kcpTask, err := svc.CreateTask("KCP task", "", "KCP", domain.TaskCreateOpts{})
    require.NoError(t, err)

    loginBody, _ := json.Marshal(map[string]string{"user_id": "bob", "password": "pw"})
    req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(loginBody))
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, req)
    var lr map[string]string
    json.NewDecoder(w.Body).Decode(&lr)
    token := lr["token"]

    authed := func(method, path string, body []byte) *httptest.ResponseRecorder {
        var buf *bytes.Buffer
        if body != nil {
            buf = bytes.NewBuffer(body)
        } else {
            buf = &bytes.Buffer{}
        }
        r := httptest.NewRequest(method, path, buf)
        r.Header.Set("Authorization", "Bearer "+token)
        rr := httptest.NewRecorder()
        srv.ServeHTTP(rr, r)
        return rr
    }

    // Add cross-project blocker
    addBody, _ := json.Marshal(map[string]string{"blocker_id": kcpTask.ID})
    rr := authed(http.MethodPost, "/api/tasks/"+kaTask.ID+"/blockers", addBody)
    require.Equal(t, http.StatusOK, rr.Code)

    // Verify via GetTask
    rr = authed(http.MethodGet, "/api/tasks/"+kaTask.ID, nil)
    var task domain.Task
    json.NewDecoder(rr.Body).Decode(&task)
    assert.Contains(t, task.Blockers, kcpTask.ID)
    assert.Len(t, task.BlockerDetails, 1)

    // Remove the blocker
    rr = authed(http.MethodDelete, "/api/tasks/"+kaTask.ID+"/blockers/"+kcpTask.ID, nil)
    require.Equal(t, http.StatusOK, rr.Code)

    // Verify removed
    rr = authed(http.MethodGet, "/api/tasks/"+kaTask.ID, nil)
    json.NewDecoder(rr.Body).Decode(&task)
    assert.Empty(t, task.Blockers)

    // 400 on missing blocker_id
    rr = authed(http.MethodPost, "/api/tasks/"+kaTask.ID+"/blockers", []byte(`{}`))
    assert.Equal(t, http.StatusBadRequest, rr.Code)

    // 404 on unknown task
    rr = authed(http.MethodPost, "/api/tasks/FAKE-999/blockers", addBody)
    assert.Equal(t, http.StatusNotFound, rr.Code)
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```bash
go test ./internal/api/...
```

Expected: routes not registered yet → 404 responses (or compile failure if types differ)

- [ ] **Step 3: Add handlers to `internal/api/handlers.go`**

Append to the end of `internal/api/handlers.go`:

```go
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
```

- [ ] **Step 4: Register routes in `internal/api/server.go`**

In the `routes()` method, add three lines after the existing task routes block:

```go
    s.mux.HandleFunc("GET /api/search/tasks", s.auth(s.handleSearchTasks))
    s.mux.HandleFunc("POST /api/tasks/{id}/blockers", s.auth(s.handleAddBlocker))
    s.mux.HandleFunc("DELETE /api/tasks/{id}/blockers/{blocker_id}", s.auth(s.handleRemoveBlocker))
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/api/... ./internal/store/... ./internal/domain/...
```

Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add internal/api/handlers.go internal/api/server.go internal/api/server_test.go
git commit -m "api: add search/tasks, POST/DELETE tasks/{id}/blockers endpoints"
```

---

### Task 5: CLI task block / task unblock commands

**Files:**
- Modify: `cmd/keroagile/cmd_task.go`

- [ ] **Step 1: Add the two commands to `cmd/keroagile/cmd_task.go`**

Append before the `func init()` at the bottom of `cmd_task.go`:

```go
var taskBlockCmd = &cobra.Command{
    Use:   "block <task-id> <blocker-id>",
    Short: "Mark a task as blocked by another task",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        taskID, blockerID := args[0], args[1]
        if err := svc.AddDep(blockerID, taskID); err != nil {
            if errors.Is(err, domain.ErrNotFound) {
                return exitNotFound(blockerID)
            }
            return err
        }
        if jsonFlag {
            printJSON(map[string]string{"blocked": taskID, "blocked_by": blockerID})
        } else {
            fmt.Printf("%s is now blocked by %s\n", taskID, blockerID)
        }
        return nil
    },
}

var taskUnblockCmd = &cobra.Command{
    Use:   "unblock <task-id> <blocker-id>",
    Short: "Remove a blocker from a task",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        taskID, blockerID := args[0], args[1]
        if err := svc.RemoveDep(blockerID, taskID); err != nil {
            return err
        }
        if jsonFlag {
            printJSON(map[string]string{"unblocked": taskID, "removed_blocker": blockerID})
        } else {
            fmt.Printf("%s is no longer blocked by %s\n", taskID, blockerID)
        }
        return nil
    },
}
```

- [ ] **Step 2: Register both commands in `init()`**

Change the `taskCmd.AddCommand(...)` line to include the two new commands:

```go
    taskCmd.AddCommand(taskAddCmd, taskListCmd, taskGetCmd, taskMoveCmd,
        taskLinkBranchCmd, taskLinkPRCmd, taskDeleteCmd,
        taskBlockCmd, taskUnblockCmd)
```

- [ ] **Step 3: Build and smoke-test**

```bash
make build
./KeroAgile task block --help
./KeroAgile task unblock --help
```

Expected: help text shows `block <task-id> <blocker-id>` and `unblock <task-id> <blocker-id>`

- [ ] **Step 4: Run all tests**

```bash
make test
```

Expected: all pass

- [ ] **Step 5: Commit**

```bash
git add cmd/keroagile/cmd_task.go
git commit -m "cli: add task block and task unblock subcommands"
```

---

### Task 6: TUI blocker picker model

**Files:**
- Create: `internal/tui/forms/blocker_picker.go`

The picker is a BubbleTea overlay model (value type, like SprintForm). It holds a `*domain.Service` for live search, a `textinput.Model` for the query, and a plain slice + cursor for results. Debounce uses a version counter: each keystroke increments the counter and schedules `tea.Tick(300ms)`; the tick only fires a search command if its version matches the current counter.

- [ ] **Step 1: Create `internal/tui/forms/blocker_picker.go`**

```go
package forms

import (
    "fmt"
    "time"

    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/tbdtechpro/KeroAgile/internal/domain"
    "github.com/tbdtechpro/KeroAgile/internal/tui/styles"
)

// BlockerPickedMsg is emitted when the user selects a task.
type BlockerPickedMsg struct{ ID string }

// BlockerPickerCancelledMsg is emitted when the user presses Esc.
type BlockerPickerCancelledMsg struct{}

// OpenBlockerPickerMsg is emitted by TaskForm when the user activates the
// blocker picker on a blocks/blockedBy field. Field is "blocks" or "blockedBy".
type OpenBlockerPickerMsg struct{ Field string }

type pickerSearchTickMsg struct{ version int }
type pickerResultsMsg struct {
    results []*domain.TaskSummary
    version int
}

// BlockerPicker is a modal overlay for fuzzy-searching tasks across all projects.
type BlockerPicker struct {
    svc      *domain.Service
    search   textinput.Model
    results  []*domain.TaskSummary
    cursor   int
    debounce int
    width    int
    height   int
}

// NewBlockerPicker creates a ready-to-use picker overlay.
func NewBlockerPicker(svc *domain.Service, width, height int) BlockerPicker {
    ti := textinput.New()
    ti.Placeholder = "Search tasks…"
    ti.Width = width - 16
    ti.Focus()
    return BlockerPicker{svc: svc, search: ti, width: width, height: height}
}

func (f BlockerPicker) Init() tea.Cmd { return textinput.Blink }

func (f BlockerPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "esc":
            return f, func() tea.Msg { return BlockerPickerCancelledMsg{} }
        case "enter":
            if len(f.results) > 0 && f.cursor < len(f.results) {
                id := f.results[f.cursor].ID
                return f, func() tea.Msg { return BlockerPickedMsg{ID: id} }
            }
            return f, nil
        case "up":
            if f.cursor > 0 {
                f.cursor--
            }
            return f, nil
        case "down":
            if f.cursor < len(f.results)-1 {
                f.cursor++
            }
            return f, nil
        }
        // All other keys update the text input and schedule a debounced search.
        var tiCmd tea.Cmd
        f.search, tiCmd = f.search.Update(msg)
        f.debounce++
        v := f.debounce
        return f, tea.Batch(tiCmd, tea.Tick(300*time.Millisecond, func(_ time.Time) tea.Msg {
            return pickerSearchTickMsg{version: v}
        }))

    case pickerSearchTickMsg:
        if msg.version != f.debounce {
            return f, nil
        }
        q := f.search.Value()
        v := f.debounce
        return f, func() tea.Msg {
            results, _ := f.svc.SearchTasksWithHint(q, 10, "")
            return pickerResultsMsg{results: results, version: v}
        }

    case pickerResultsMsg:
        if msg.version != f.debounce {
            return f, nil
        }
        f.results = msg.results
        if f.cursor >= len(f.results) {
            f.cursor = 0
        }
        return f, nil
    }
    return f, nil
}

func (f BlockerPicker) View() string {
    w := f.width - 8
    if w < 20 {
        w = 20
    }

    title := lipgloss.NewStyle().
        Foreground(styles.CAccentLt).Bold(true).
        Render("Add blocker — type to search all projects")

    searchLine := f.search.View()

    var rows []string
    for i, ts := range f.results {
        label := fmt.Sprintf("%s · %s", ts.ID, ts.Title)
        if ts.ProjectID != "" {
            // Prefix cross-project tasks with [PROJECT]
            label = fmt.Sprintf("[%s] %s", ts.ProjectID, label)
        }
        if len(label) > w-2 {
            label = label[:w-5] + "…"
        }
        style := lipgloss.NewStyle().Width(w - 2)
        if i == f.cursor {
            style = style.
                Background(styles.CAccent).
                Foreground(lipgloss.Color("#ffffff"))
        }
        rows = append(rows, style.Render(label))
    }
    if len(rows) == 0 {
        rows = append(rows, lipgloss.NewStyle().Foreground(styles.CMuted).Render("No results"))
    }

    hint := lipgloss.NewStyle().Foreground(styles.CMuted).
        Render("↑↓ navigate · Enter select · Esc cancel")

    content := lipgloss.JoinVertical(lipgloss.Left,
        title, searchLine, "",
        lipgloss.JoinVertical(lipgloss.Left, rows...),
        "", hint,
    )

    modal := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(styles.CAccent).
        Padding(1, 2).
        Width(w).
        Render(content)

    return lipgloss.Place(f.width, f.height, lipgloss.Center, lipgloss.Center, modal)
}
```

- [ ] **Step 2: Build to verify compile**

```bash
go build ./internal/tui/...
```

Expected: success (the picker is not yet wired to the form, but it compiles)

- [ ] **Step 3: Run all tests**

```bash
make test
```

Expected: all pass

- [ ] **Step 4: Commit**

```bash
git add internal/tui/forms/blocker_picker.go
git commit -m "tui: add BlockerPicker fuzzy-search overlay model"
```

---

### Task 7: TUI form wiring + doUpdateTask persistence fix

**Files:**
- Modify: `internal/tui/forms/task_form.go`
- Modify: `internal/tui/msgs.go`
- Modify: `internal/tui/app.go`

- [ ] **Step 1: Modify `internal/tui/forms/task_form.go` to emit OpenBlockerPickerMsg on Enter**

In `TaskForm.Update`, find the `"enter"` case and add field-specific handling **before** the existing validate/submit logic:

Current code at line ~184:
```go
        case "enter":
            if f.focus == fieldDesc {
                break
            }
            if errStr := f.validate(); errStr != "" {
```

Replace with:

```go
        case "enter":
            if f.focus == fieldDesc {
                break
            }
            if f.focus == fieldBlockedBy {
                return f, func() tea.Msg { return OpenBlockerPickerMsg{Field: "blockedBy"} }
            }
            if f.focus == fieldBlocks {
                return f, func() tea.Msg { return OpenBlockerPickerMsg{Field: "blocks"} }
            }
            if errStr := f.validate(); errStr != "" {
```

- [ ] **Step 2: Add `AppendToBlocker` method to `internal/tui/forms/task_form.go`**

Append to the end of `task_form.go`:

```go
// AppendToBlocker appends id to the blocks or blockedBy input field.
// field must be "blocks" or "blockedBy".
func (f TaskForm) AppendToBlocker(field, id string) TaskForm {
    switch field {
    case "blockedBy":
        existing := strings.TrimSpace(f.blockedByIn.Value())
        if existing == "" {
            f.blockedByIn.SetValue(id)
        } else {
            f.blockedByIn.SetValue(existing + ", " + id)
        }
    case "blocks":
        existing := strings.TrimSpace(f.blocksInput.Value())
        if existing == "" {
            f.blocksInput.SetValue(id)
        } else {
            f.blocksInput.SetValue(existing + ", " + id)
        }
    }
    return f
}
```

- [ ] **Step 3: Add `openBlockerPickerMsg` to `internal/tui/msgs.go`**

Add to `msgs.go`:

```go
// openBlockerPickerMsg tells App to open the blocker picker overlay.
// Field is "blocks" or "blockedBy".
type openBlockerPickerMsg struct{ field string }
```

Wait — `OpenBlockerPickerMsg` is already defined in `forms/blocker_picker.go` as a public type. The `tui` package will receive it directly from the form's command. So we do NOT need a separate type in `msgs.go`. The App's `Update` method will match on `forms.OpenBlockerPickerMsg`.

Skip this step.

- [ ] **Step 4: Update `App` struct and `Update` in `internal/tui/app.go`**

**4a.** Add `blockerPicker` and `blockerPickerField` to the `App` struct (after the `sprintForm` field):

```go
    blockerPicker      *forms.BlockerPicker
    blockerPickerField string // "blocks" or "blockedBy"
```

**4b.** In `App.Update`, add picker routing **before** form routing. Find the section that checks `a.sprintForm != nil` and add the picker check first:

```go
// Picker overlay (within task form flow) — takes priority over form and main UI
if a.blockerPicker != nil {
    newPicker, cmd := a.blockerPicker.Update(msg)
    bp := newPicker.(forms.BlockerPicker)
    a.blockerPicker = &bp
    return a, cmd
}
```

**4c.** In `App.Update`, handle the messages emitted by the picker. Find the large `switch msg := msg.(type)` block and add these cases:

```go
case forms.OpenBlockerPickerMsg:
    picker := forms.NewBlockerPicker(a.svc, a.width, a.height)
    a.blockerPicker = &picker
    a.blockerPickerField = msg.Field
    return a, picker.Init()

case forms.BlockerPickedMsg:
    if a.form != nil && a.blockerPicker != nil {
        updated := a.form.AppendToBlocker(a.blockerPickerField, msg.ID)
        a.form = &updated
    }
    a.blockerPicker = nil
    a.blockerPickerField = ""
    return a, nil

case forms.BlockerPickerCancelledMsg:
    a.blockerPicker = nil
    a.blockerPickerField = ""
    return a, nil
```

**4d.** In `App.View()`, render the picker when active. Find the section that renders the sprint form overlay. After it (or in the same pattern), add:

```go
if a.blockerPicker != nil {
    return a.blockerPicker.View()
}
```

This should come before (or alongside) the sprint form check so that the picker gets priority when both are somehow open.

- [ ] **Step 5: Fix `doUpdateTask` in `internal/tui/app.go`**

**5a.** Add the `diffBlockers` helper function to `app.go` (append near the end, before `refreshGit`):

```go
// diffBlockers returns the IDs to add (in newIDs but not oldIDs) and to remove
// (in oldIDs but not newIDs).
func diffBlockers(oldIDs, newIDs []string) (toAdd, toRemove []string) {
    oldSet := make(map[string]bool, len(oldIDs))
    for _, id := range oldIDs {
        oldSet[id] = true
    }
    newSet := make(map[string]bool, len(newIDs))
    for _, id := range newIDs {
        newSet[id] = true
    }
    for id := range newSet {
        if !oldSet[id] {
            toAdd = append(toAdd, id)
        }
    }
    for id := range oldSet {
        if !newSet[id] {
            toRemove = append(toRemove, id)
        }
    }
    return
}
```

**5b.** Replace the `doUpdateTask` method to persist blocker diffs. The current implementation (lines ~569–595) has the TODO comment. Replace the entire function:

```go
func (a App) doUpdateTask(msg forms.SavedMsg, t *domain.Task) tea.Cmd {
    updated := *t
    updated.Title = msg.Title
    updated.Description = msg.Description
    updated.Priority = msg.Priority
    updated.Status = msg.Status
    updated.Labels = msg.Labels
    updated.Points = msg.Points
    updated.SprintID = msg.SprintID
    if msg.AssigneeID != "" {
        s := msg.AssigneeID
        updated.AssigneeID = &s
    } else {
        updated.AssigneeID = nil
    }
    taskID := t.ID
    projectID := t.ProjectID
    oldBlockers := append([]string(nil), t.Blockers...)
    oldBlocking := append([]string(nil), t.Blocking...)
    newBlockedBy := msg.BlockedBy
    newBlocks := msg.Blocks

    return func() tea.Msg {
        if _, err := a.svc.UpdateTask(&updated); err != nil {
            return statusNotifMsg{fmt.Sprintf("error: %v", err)}
        }

        // Reconcile blocked-by (tasks that block this task)
        add, remove := diffBlockers(oldBlockers, newBlockedBy)
        for _, id := range remove {
            if err := a.svc.RemoveDep(id, taskID); err != nil {
                return statusNotifMsg{fmt.Sprintf("error removing blocker %s: %v", id, err)}
            }
        }
        for _, id := range add {
            if err := a.svc.AddDep(id, taskID); err != nil {
                return statusNotifMsg{fmt.Sprintf("error adding blocker %s: %v", id, err)}
            }
        }

        // Reconcile blocks (tasks that this task blocks)
        add, remove = diffBlockers(oldBlocking, newBlocks)
        for _, id := range remove {
            if err := a.svc.RemoveDep(taskID, id); err != nil {
                return statusNotifMsg{fmt.Sprintf("error removing block %s: %v", id, err)}
            }
        }
        for _, id := range add {
            if err := a.svc.AddDep(taskID, id); err != nil {
                return statusNotifMsg{fmt.Sprintf("error adding block %s: %v", id, err)}
            }
        }

        return reloadTasksMsg{projectID}
    }
}
```

- [ ] **Step 6: Write a unit test for `diffBlockers`**

Add to `internal/tui/board_test.go` (or create `internal/tui/app_test.go`):

```go
// app_test.go or board_test.go addition
func TestDiffBlockers(t *testing.T) {
    // This tests the unexported diffBlockers — it lives in the same tui_test package.
    // diffBlockers is exported as a test helper via a shim or tested indirectly.
    // Since it's unexported, test via integration: use store + service directly.
    //
    // We test the logic separately: old=[A,B], new=[B,C] → add=[C], remove=[A]
    s := store.New(func() *sql.DB {
        db, _ := store.Open(":memory:")
        return db
    }())
    // ... this approach is complex; instead test via the service integration test
}
```

Actually `diffBlockers` is unexported. The simplest test is via the store integration test that verifies `doUpdateTask`'s net effect. Instead of a TUI unit test (which requires a running BubbleTea loop), add a store integration test that verifies the dep reconciliation pattern: add A and B, then "update" to have only B and C, verify A removed and C added.

Add to `internal/store/store_test.go`:

```go
func TestDepReconciliation(t *testing.T) {
    s := testStore(t)
    require.NoError(t, s.CreateProject(&domain.Project{ID: "KA", Name: "KeroAgile"}))

    seq1, _ := s.NextTaskSeq("KA")
    t1 := fmt.Sprintf("KA-%03d", seq1)
    seq2, _ := s.NextTaskSeq("KA")
    t2 := fmt.Sprintf("KA-%03d", seq2)
    seq3, _ := s.NextTaskSeq("KA")
    t3 := fmt.Sprintf("KA-%03d", seq3)
    seq4, _ := s.NextTaskSeq("KA")
    t4 := fmt.Sprintf("KA-%03d", seq4)

    for _, id := range []string{t1, t2, t3, t4} {
        require.NoError(t, s.CreateTask(&domain.Task{
            ID: id, ProjectID: "KA", Title: id,
            Status: domain.StatusBacklog, Priority: domain.PriorityMedium,
        }))
    }

    // t1 is blocked by t2 and t3
    require.NoError(t, s.AddDep(t2, t1))
    require.NoError(t, s.AddDep(t3, t1))

    // "Update" to: t1 blocked by t3 and t4 (remove t2, add t4)
    require.NoError(t, s.RemoveDep(t2, t1))
    require.NoError(t, s.AddDep(t4, t1))

    blockers, _, err := s.GetTaskDeps(t1)
    require.NoError(t, err)
    assert.ElementsMatch(t, []string{t3, t4}, blockers)
    assert.NotContains(t, blockers, t2)
}
```

- [ ] **Step 7: Build and run tests**

```bash
make build && make test
```

Expected: all pass

- [ ] **Step 8: Commit**

```bash
git add internal/tui/forms/task_form.go internal/tui/app.go internal/store/store_test.go
git commit -m "tui: wire blocker picker to task form; fix doUpdateTask to persist dep diffs"
```

---

### Task 8: TUI detail enrichment

**Files:**
- Modify: `internal/tui/detail.go`

- [ ] **Step 1: Locate the current blocker rendering in `internal/tui/detail.go`**

The current code (around line 149):
```go
    if len(t.Blockers) > 0 {
        sb.WriteString("\n" + styles.Muted.Render("Blockers") + "\n")
        for _, b := range t.Blockers {
            sb.WriteString(styles.Danger.Render("⚠ ") + styles.NormalRow.Render(b) + "\n")
        }
    }
```

- [ ] **Step 2: Replace with enriched rendering**

Replace the blockers block with:

```go
    if len(t.Blockers) > 0 {
        sb.WriteString("\n" + styles.Muted.Render("Blocked by") + "\n")
        if len(t.BlockerDetails) > 0 {
            for _, bd := range t.BlockerDetails {
                label := formatBlockerChip(bd)
                sb.WriteString(styles.Danger.Render("⚠ ") + styles.NormalRow.Render(label) + "\n")
            }
        } else {
            // Fallback to ID-only rendering if enrichment unavailable
            for _, b := range t.Blockers {
                sb.WriteString(styles.Danger.Render("⚠ ") + styles.NormalRow.Render(b) + "\n")
            }
        }
    }

    if len(t.Blocking) > 0 {
        sb.WriteString("\n" + styles.Muted.Render("Blocking") + "\n")
        if len(t.BlockingDetails) > 0 {
            for _, bd := range t.BlockingDetails {
                label := formatBlockerChip(bd)
                sb.WriteString(styles.NormalRow.Render("► " + label) + "\n")
            }
        } else {
            for _, b := range t.Blocking {
                sb.WriteString(styles.NormalRow.Render("► " + b) + "\n")
            }
        }
    }
```

- [ ] **Step 3: Add `formatBlockerChip` helper in `internal/tui/detail.go`**

Append to the end of `detail.go`:

```go
// formatBlockerChip returns a display string for a TaskSummary in the detail panel.
// Cross-project tasks are prefixed with [PROJECT].
func formatBlockerChip(ts *domain.TaskSummary) string {
    title := ts.Title
    if len(title) > 40 {
        title = title[:37] + "…"
    }
    if ts.ProjectID != "" {
        return fmt.Sprintf("[%s] %s · %s", ts.ProjectID, ts.ID, title)
    }
    return fmt.Sprintf("%s · %s", ts.ID, title)
}
```

Note: the `detail.go` file already imports `"fmt"`, `"strings"`, and the domain package — verify this with the existing imports before assuming. If `domain` is not imported, add it.

- [ ] **Step 4: Build**

```bash
go build ./internal/tui/...
```

Expected: success

- [ ] **Step 5: Run all tests**

```bash
make test
```

Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add internal/tui/detail.go
git commit -m "tui: enrich detail panel with titled blocker chips and Blocking reverse-index section"
```

---

### Task 9: Web types.ts + client.ts + queries.ts

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/client.ts`
- Modify: `web/src/api/queries.ts`

- [ ] **Step 1: Add `TaskSummary` and extend `Task` in `web/src/api/types.ts`**

Add `TaskSummary` interface after the existing type definitions. Also add two optional fields to the `Task` interface.

In `types.ts`, add after the `Status` / `Priority` type definitions (before `Task`):

```typescript
export interface TaskSummary {
  id: string
  title: string
  project_id: string
  status: Status
}
```

In the `Task` interface, add after `blocking: string[] | null`:

```typescript
  blocker_details?: TaskSummary[] | null
  blocking_details?: TaskSummary[] | null
```

- [ ] **Step 2: Add `searchTasks`, `addBlocker`, `removeBlocker` to `web/src/api/client.ts`**

Add to the `api` object export in `client.ts`, after the existing task methods:

```typescript
  searchTasks: (q: string, hintProjectId?: string): Promise<{ tasks: TaskSummary[] }> => {
    const qs = new URLSearchParams({ q, limit: '20' })
    if (hintProjectId) qs.set('hint_project_id', hintProjectId)
    return request('GET', `/api/search/tasks?${qs}`)
  },

  addBlocker: (taskId: string, blockerId: string): Promise<{ status: string }> =>
    request('POST', `/api/tasks/${taskId}/blockers`, { blocker_id: blockerId }),

  removeBlocker: (taskId: string, blockerId: string): Promise<{ status: string }> =>
    request('DELETE', `/api/tasks/${taskId}/blockers/${blockerId}`),
```

Also update the import line in `client.ts` to include `TaskSummary`:

```typescript
import type { Task, Project, User, Sprint, SprintSummary, CreateTaskInput, UpdateTaskInput, Secondary, TaskSummary } from './types'
```

- [ ] **Step 3: Add `useAddBlocker` and `useRemoveBlocker` to `web/src/api/queries.ts`**

Append to `queries.ts`:

```typescript
export function useAddBlocker() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ taskId, blockerId }: { taskId: string; blockerId: string }) =>
      api.addBlocker(taskId, blockerId),
    onSuccess: (_data, { taskId }) => {
      qc.invalidateQueries({ queryKey: ['tasks'] })
      qc.invalidateQueries({ queryKey: ['task', taskId] })
    },
  })
}

export function useRemoveBlocker() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ taskId, blockerId }: { taskId: string; blockerId: string }) =>
      api.removeBlocker(taskId, blockerId),
    onSuccess: (_data, { taskId }) => {
      qc.invalidateQueries({ queryKey: ['tasks'] })
      qc.invalidateQueries({ queryKey: ['task', taskId] })
    },
  })
}
```

- [ ] **Step 4: Build the web app**

```bash
cd /home/matt/github/KeroAgile/web && npm run build 2>&1 | tail -20
```

Expected: no TypeScript errors

- [ ] **Step 5: Commit**

```bash
git add web/src/api/types.ts web/src/api/client.ts web/src/api/queries.ts
git commit -m "web: add TaskSummary type, searchTasks/addBlocker/removeBlocker API calls, useAddBlocker/useRemoveBlocker hooks"
```

---

### Task 10: Web TaskModal — blocker autocomplete field

**Files:**
- Modify: `web/src/components/TaskModal.tsx`

The TaskModal is an edit/create form. For edit mode, it should allow adding/removing blockers with live autocomplete. For create mode, blockers can be set but mutations fire after creation — simplified flow: the modal only manages blockers when `isEdit` is true (same approach used for other post-creation operations).

- [ ] **Step 1: Add blocker state and debounce to `web/src/components/TaskModal.tsx`**

Add imports at the top:

```typescript
import { useEffect, useState, useRef } from 'react'
import type { Priority, Status, Task, TaskSummary } from '../api/types'
import type { CreateTaskInput, UpdateTaskInput } from '../api/types'
import { useCreateTask, useUpdateTask, useUsers, useSprints, useAddBlocker, useRemoveBlocker } from '../api/queries'
import { api } from '../api/client'
```

Add state variables inside the component function (after existing `useState` calls):

```typescript
  const [blockerQuery, setBlockerQuery] = useState('')
  const [blockerResults, setBlockerResults] = useState<TaskSummary[]>([])
  const [selectedBlockers, setSelectedBlockers] = useState<TaskSummary[]>(
    task?.blocker_details?.filter(Boolean) as TaskSummary[] ?? []
  )
  const [showDropdown, setShowDropdown] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const addBlocker = useAddBlocker()
  const removeBlocker = useRemoveBlocker()
```

- [ ] **Step 2: Add search effect with debounce**

Add after the existing `useEffect` for Escape key:

```typescript
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current)
    if (!blockerQuery.trim()) {
      setBlockerResults([])
      setShowDropdown(false)
      return
    }
    debounceRef.current = setTimeout(async () => {
      try {
        const resp = await api.searchTasks(blockerQuery, projectId)
        const filtered = (resp.tasks ?? []).filter(
          ts => !selectedBlockers.some(b => b.id === ts.id) && ts.id !== task?.id
        )
        setBlockerResults(filtered)
        setShowDropdown(filtered.length > 0)
      } catch {
        // ignore search errors
      }
    }, 300)
    return () => { if (debounceRef.current) clearTimeout(debounceRef.current) }
  }, [blockerQuery, projectId, selectedBlockers, task?.id])
```

- [ ] **Step 3: Add blocker helper functions inside the component**

```typescript
  function selectBlocker(ts: TaskSummary) {
    setSelectedBlockers(prev => [...prev, ts])
    setBlockerQuery('')
    setBlockerResults([])
    setShowDropdown(false)
    if (task) {
      addBlocker.mutate({ taskId: task.id, blockerId: ts.id })
    }
  }

  function deselectBlocker(ts: TaskSummary) {
    setSelectedBlockers(prev => prev.filter(b => b.id !== ts.id))
    if (task) {
      removeBlocker.mutate({ taskId: task.id, blockerId: ts.id })
    }
  }
```

- [ ] **Step 4: Add blocker UI section to the form JSX**

In the `return` block, add the "Blocked by" section after the sprint input field and before the error display / submit button. Insert:

```tsx
          {/* Blocked by — edit mode only (blockers require an existing task ID) */}
          {isEdit && (
            <div>
              <label className="block text-xs mb-1" style={{ color: 'var(--ka-muted)' }}>
                Blocked by
              </label>
              {/* Chips */}
              <div className="flex flex-wrap gap-1 mb-1">
                {selectedBlockers.map(b => (
                  <span
                    key={b.id}
                    className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded"
                    style={{
                      background: b.project_id !== projectId ? '#1e3a5f' : '#7f1d1d',
                      color: b.project_id !== projectId ? '#93c5fd' : 'var(--ka-red)',
                    }}
                  >
                    {b.project_id !== projectId && <span>↗</span>}
                    {b.id} {b.title}
                    <button
                      type="button"
                      onClick={() => deselectBlocker(b)}
                      className="ml-1 opacity-60 hover:opacity-100"
                    >
                      ×
                    </button>
                  </span>
                ))}
              </div>
              {/* Search input */}
              <div className="relative">
                <input
                  type="text"
                  placeholder="Search tasks to add as blocker…"
                  value={blockerQuery}
                  onChange={e => setBlockerQuery(e.target.value)}
                  onFocus={() => blockerResults.length > 0 && setShowDropdown(true)}
                  onBlur={() => setTimeout(() => setShowDropdown(false), 150)}
                  className="w-full text-xs px-2 py-1 rounded border"
                  style={{
                    background: 'var(--ka-inset)',
                    borderColor: '#1e293b',
                    color: 'var(--ka-text)',
                    outline: 'none',
                  }}
                />
                {showDropdown && (
                  <div
                    className="absolute z-10 w-full mt-1 rounded border shadow-lg"
                    style={{ background: 'var(--ka-panel)', borderColor: '#1e293b', maxHeight: 200, overflowY: 'auto' }}
                  >
                    {blockerResults.map(ts => (
                      <button
                        key={ts.id}
                        type="button"
                        onMouseDown={() => selectBlocker(ts)}
                        className="w-full text-left text-xs px-3 py-1.5 hover:bg-blue-900"
                        style={{ color: 'var(--ka-text)' }}
                      >
                        {ts.project_id !== projectId && (
                          <span className="text-blue-400 mr-1">[{ts.project_id}]</span>
                        )}
                        <span className="font-mono mr-1">{ts.id}</span>
                        {ts.title}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}
```

- [ ] **Step 5: Build and check TypeScript**

```bash
cd /home/matt/github/KeroAgile/web && npm run build 2>&1 | tail -20
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add web/src/components/TaskModal.tsx
git commit -m "web: add blocker autocomplete field to TaskModal with live search, chips, and add/remove mutations"
```

---

### Task 11: Web TaskDetail — enriched chips + Blocking section + navigation

**Files:**
- Modify: `web/src/components/TaskDetail.tsx`

- [ ] **Step 1: Add imports and mutation hooks to `web/src/components/TaskDetail.tsx`**

Check and add to the imports at the top of the file:

```typescript
import { useRemoveBlocker } from '../api/queries'
import { useNavigate } from 'react-router-dom'
```

Inside the component, add:

```typescript
  const removeBlocker = useRemoveBlocker()
  const navigate = useNavigate()

  function handleBlockerClick(projectId: string, taskId: string) {
    navigate(`/${projectId}`)
    // Ideally also open the task detail — pass taskId to parent or use URL state
    // For now, navigating to the project is the MVP behaviour
    onClose()
  }
```

Note: if `useNavigate` is not already available (depends on whether react-router-dom is used), check `web/src/App.tsx` or `web/src/main.tsx` for the router setup. If not present, omit the `navigate` call and just call `onClose()`.

- [ ] **Step 2: Replace the existing "Blocked by" chips section**

Find the current "Blocked by" section (around line 165):

```tsx
        {task.blockers && task.blockers.length > 0 && (
          <Field label="Blocked by">
            <div className="flex flex-wrap gap-1">
              {task.blockers.map(b => (
                <span
                  key={b}
                  className="text-xs px-2 py-0.5 rounded"
                  style={{ background: '#7f1d1d', color: 'var(--ka-red)' }}
                >
                  {b}
                </span>
              ))}
            </div>
          </Field>
        )}
```

Replace with:

```tsx
        {task.blockers && task.blockers.length > 0 && (
          <Field label="Blocked by">
            <div className="flex flex-wrap gap-1">
              {(task.blocker_details && task.blocker_details.length > 0
                ? task.blocker_details
                : task.blockers.map(id => ({ id, title: '', project_id: task.project_id, status: task.status }))
              ).map(b => {
                const isCross = b.project_id !== task.project_id
                return (
                  <span
                    key={b.id}
                    className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded cursor-pointer"
                    style={{
                      background: isCross ? '#1e3a5f' : '#7f1d1d',
                      color: isCross ? '#93c5fd' : 'var(--ka-red)',
                    }}
                    onClick={() => handleBlockerClick(b.project_id || task.project_id, b.id)}
                    title={b.title || b.id}
                  >
                    {isCross && <span>↗</span>}
                    <span>{b.id}</span>
                    {b.title && <span className="opacity-75">{b.title.length > 24 ? b.title.slice(0, 21) + '…' : b.title}</span>}
                    {isCross && (
                      <span
                        className="text-xs px-1 rounded"
                        style={{ background: 'rgba(255,255,255,0.1)', fontSize: '0.65rem' }}
                      >
                        {b.project_id}
                      </span>
                    )}
                    <button
                      type="button"
                      onClick={e => {
                        e.stopPropagation()
                        removeBlocker.mutate({ taskId: task.id, blockerId: b.id })
                      }}
                      className="ml-1 opacity-50 hover:opacity-100"
                    >
                      ×
                    </button>
                  </span>
                )
              })}
            </div>
          </Field>
        )}
```

- [ ] **Step 3: Add "Blocking" reverse-index section after "Blocked by"**

Add immediately after the "Blocked by" block:

```tsx
        {task.blocking_details && task.blocking_details.length > 0 && (
          <Field label="Blocking">
            <div className="flex flex-wrap gap-1">
              {task.blocking_details.map(b => {
                const isCross = b.project_id !== task.project_id
                return (
                  <span
                    key={b.id}
                    className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded cursor-pointer"
                    style={{
                      background: isCross ? '#1e3a5f' : '#064e3b',
                      color: isCross ? '#93c5fd' : '#6ee7b7',
                    }}
                    onClick={() => handleBlockerClick(b.project_id || task.project_id, b.id)}
                    title={b.title || b.id}
                  >
                    {isCross && <span>↗</span>}
                    <span>►</span>
                    <span>{b.id}</span>
                    {b.title && <span className="opacity-75">{b.title.length > 24 ? b.title.slice(0, 21) + '…' : b.title}</span>}
                    {isCross && (
                      <span
                        className="text-xs px-1 rounded"
                        style={{ background: 'rgba(255,255,255,0.1)', fontSize: '0.65rem' }}
                      >
                        {b.project_id}
                      </span>
                    )}
                  </span>
                )
              })}
            </div>
          </Field>
        )}
```

- [ ] **Step 4: Build and verify TypeScript**

```bash
cd /home/matt/github/KeroAgile/web && npm run build 2>&1 | tail -20
```

Expected: no errors

- [ ] **Step 5: Run all Go tests**

```bash
cd /home/matt/github/KeroAgile && make test
```

Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add web/src/components/TaskDetail.tsx
git commit -m "web: enrich TaskDetail with titled blocker chips, cross-project indicator, Blocking section, and × removal"
```

---

## Self-Review Checklist

### Spec coverage

| Spec requirement | Task |
|---|---|
| `TaskSummary` type | Task 1 |
| `SearchTasks` Store + Service | Task 1, 2 |
| `GetTask` enrichment with `blocker_details`/`blocking_details` | Task 3 |
| `GET /api/search/tasks` with `hint_project_id` ordering | Task 4 |
| `POST /api/tasks/{id}/blockers` | Task 4 |
| `DELETE /api/tasks/{id}/blockers/{blocker_id}` | Task 4 |
| CLI `task block` / `task unblock` | Task 5 |
| TUI fuzzy-search picker overlay | Task 6 |
| TUI form blocker picker activation | Task 7 |
| Fix `doUpdateTask` dep persistence | Task 7 |
| TUI detail titled chips + Blocking section | Task 8 |
| Web `TaskSummary` type, API client methods | Task 9 |
| Web `TaskModal` autocomplete | Task 10 |
| Web `TaskDetail` enriched chips, navigation, Blocking | Task 11 |

### Consistency check

- `BlockerPickedMsg`, `BlockerPickerCancelledMsg`, `OpenBlockerPickerMsg` are defined in `internal/tui/forms/blocker_picker.go` and referenced in `app.go` — consistent.
- `diffBlockers(oldIDs, newIDs []string) (toAdd, toRemove []string)` is defined in `app.go` and called only there — consistent.
- `AppendToBlocker(field, id string) TaskForm` on `TaskForm` — called in `app.go`, defined in `task_form.go` — consistent.
- `SearchTasksWithHint` is on both `Store` interface and `Service` — must add to `mockStore` in `service_test.go` (covered in Task 2, Step 3).
- `fetchSummaries` is unexported, used only within `store/task.go` — consistent.
- `formatBlockerChip` is unexported, used only within `tui/detail.go` — consistent.
- Web `TaskSummary` exported from `types.ts`, imported in `TaskModal.tsx` and `TaskDetail.tsx` — consistent.
