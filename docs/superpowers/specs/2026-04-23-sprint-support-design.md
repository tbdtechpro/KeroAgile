# Sprint Support — Design Spec

**Goal:** Full sprint support across CLI, TUI, and MCP — sprint creation, task assignment, and board filtering by sprint.

**Architecture:** No new layers. Changes flow through existing boundaries: domain service → SQLite store → CLI/TUI/MCP. One missing service method added (`AssignTaskToSprint`). Sidebar gains a second navigation state. Board gains a sprint filter. Task form gains a sprint field.

**Tech Stack:** Go, BubbleTea/Lipgloss (TUI), Cobra (CLI), SQLite via modernc.org/sqlite, JSON-RPC 2.0 MCP.

---

## 1. Domain / Service Layer

### 1.1 New service method

```go
// AssignTaskToSprint sets or clears the sprint assignment for a task.
// sprintID == nil clears the assignment.
func (s *Service) AssignTaskToSprint(taskID string, sprintID *int64) (*Task, error)
```

Implementation: load task via `store.GetTask`, update `SprintID`, call `store.UpdateTask`. Return updated task.

`store.UpdateTask` already persists `sprint_id` — no schema changes needed.

### 1.2 New store query: active sprint for project

```go
// GetActiveSprint returns the single sprint with status="active" for the given project.
// Returns domain.ErrNotFound if none is active.
GetActiveSprint(projectID string) (*Sprint, error)
```

Used by: TUI board header, MCP `get_sprint` (already works via `list_tasks` filter but needs this for auto-detection), `/keroagile-plan` slash command.

SQL: `SELECT ... FROM sprints WHERE project_id=? AND status='active' LIMIT 1`

### 1.3 ListTasks already supports SprintID filter

`store.ListTasks` already filters by `sprint_id` when `TaskFilters.SprintID != nil`. No changes needed here.

---

## 2. CLI

### 2.1 `sprint assign` command (missing — add it)

```
KeroAgile sprint assign <task-id> <sprint-id>
KeroAgile sprint assign <task-id> --clear
```

- `<sprint-id>` is an integer.
- `--clear` sets sprint to nil (removes assignment).
- Prints `assigned KA-008 to sprint 1` or `removed KA-008 from sprint`.
- `--json` prints updated task as JSON.
- Error on invalid task-id or sprint-id: exit code 1.

### 2.2 Wire `GetActiveSprint` into service

`service.GetActiveSprint` delegates to `store.GetActiveSprint`. Add to `domain.Store` interface and `store.Store` implementation.

---

## 3. TUI — Sidebar

### 3.1 Two navigation states

The `Sidebar` struct gains a `mode` field (`modeProjList` / `modeSprintList`) and a `sprints` slice.

**Project list mode (current behaviour + hint):**
- Bottom of inner content: dim text `↵ sprints`
- `enter` or mouse double-click on selected project: switch to sprint list mode, trigger `loadSprintsMsg` cmd

**Sprint list mode:**
- Header row: `◂ PROJECTS · <ProjectName>` in project accent colour
- Sprint rows: `All tasks` (always first), then one row per sprint
  - Active sprint: `●` in green, name, task count
  - Planning sprint: `○` in muted, name, task count
  - Completed sprint: `✓` in dim, name, task count
- Selected row highlighted with `▶`
- `↑`/`↓` navigate rows
- `↵` emits `sprintSelectedMsg{projectID, sprintID *int64}` (nil = All tasks) and returns to project list mode
- `esc` returns to project list mode without changing selection
- `n` emits `openSprintFormMsg{}` to open the sprint creation form

### 3.2 Sprint counts

`loadSprintsMsg` result includes a `[]SprintSummary`. Add to `internal/domain/project.go`:

```go
type SprintSummary struct {
    Sprint    *Sprint
    TaskCount int
}
```

The task count is fetched with a single SQL query per sprint using `COUNT(*)` with `sprint_id` filter.

Add `ListSprintsWithCounts(projectID string) ([]SprintSummary, error)` to store.

### 3.3 Status bar hint in sprint list mode

App passes current sidebar mode to the status bar renderer. When in sprint list mode: `[↵] select  [esc] back  [n] new sprint  [q] quit`

---

## 4. TUI — Board

### 4.1 Sprint filter state

`App` gains `selectedSprintID *int64` (nil = all tasks). Set by `sprintSelectedMsg`.

`loadTasks` passes `SprintID` filter when non-nil:
```go
tasks, err := a.svc.ListTasks(projectID, domain.TaskFilters{SprintID: a.selectedSprintID})
```

### 4.2 Board header

`Board` gains a `sprintHeader string` field set by App when a sprint is selected. Non-empty string is rendered as a dim header line above the first section inside the board panel.

Format: `Sprint 1  ·  Apr 21 – May 2` (dates omitted if nil). Active sprint shown in green, planning in yellow.

### 4.3 Quick-assign shortcut

In `App.handleKey`:
- `s` — assigns cursor task to `selectedSprintID`. If `selectedSprintID == nil`, status bar shows `select a sprint first`. If task already in sprint, no-op.
- `S` — removes cursor task from any sprint (`AssignTaskToSprint(id, nil)`).
- Both reload tasks after success.

Status bar (normal mode) adds: `[s] sprint  [S] unassign`

---

## 5. TUI — Task Form

### 5.1 New Sprint field

`TaskForm` gains a `sprintInput textinput.Model` field after `labelsInput`. Accepts sprint name or ID integer as a string. Blank = unassigned.

Pre-populated from `task.SprintID` when editing (display sprint name if available, else sprint ID as string).

On save, `SavedMsg` gains `SprintID *int64`. The form resolves the entered value against the available sprint list (passed in at form construction): match by integer ID first, then case-insensitive name match. If no match: inline error `"unknown sprint"`. If multiple name matches: inline error `"ambiguous sprint name — use ID"`. Blank field resolves to nil (unassigned).

### 5.2 Form construction

`forms.New` signature gains `sprints []*domain.Sprint`. App passes `a.sprints` (loaded alongside tasks). `a.sprints` is populated on project selection and kept in sync when `sprintSelectedMsg` arrives.

---

## 6. TUI — Sprint Creation Form

### 6.1 New `SprintForm` modal

New file: `internal/tui/forms/sprint_form.go`

Fields:
- `Name` (required, textinput)
- `Start` (optional, textinput, format hint `YYYY-MM-DD`)
- `End` (optional, textinput, format hint `YYYY-MM-DD`)

Navigation: `tab`/`shift+tab` cycles fields, `enter` on last field saves, `esc` cancels.

Emits `SprintSavedMsg{Name, Start, End}` or `SprintCancelledMsg{}`.

### 6.2 App wiring

`openSprintFormMsg{}` handled in `App.Update` — sets `a.sprintForm = &SprintForm{...}`. When form active, all events route to sprint form first (same pattern as task form). On `SprintSavedMsg`, call `svc.CreateSprint`, reload sprints for current project.

---

## 7. MCP — New Tool

### `assign_task_sprint`

```json
{
  "name": "assign_task_sprint",
  "description": "Assign a task to a sprint, or remove it from its current sprint.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "task_id":  { "type": "string", "description": "Task ID, e.g. KA-008" },
      "sprint_id": { "type": "number", "description": "Sprint ID to assign, or omit/null to remove" }
    },
    "required": ["task_id"]
  }
}
```

Returns updated task JSON. Tool count becomes 13.

---

## 8. Data Flow Summary

```
Sidebar: project selected → enter → loadSprints → sprintSelectedMsg
App: sprintSelectedMsg → selectedSprintID set → loadTasks(SprintID filter) → board updates
Board: s key → AssignTaskToSprint → loadTasks → board updates
Task form: sprint field → SavedMsg.SprintID → doUpdateTask → UpdateTask(sprint_id) → reload
Sprint form: SprintSavedMsg → CreateSprint → reload sprints
CLI: sprint assign <task> <id> → AssignTaskToSprint
MCP: assign_task_sprint → AssignTaskToSprint
```

---

## 9. Files Changed / Created

| File | Change |
|------|--------|
| `internal/domain/service.go` | Add `AssignTaskToSprint`, `GetActiveSprint` |
| `internal/domain/store.go` | Add `GetActiveSprint`, `ListSprintsWithCounts` to interface |
| `internal/store/sprint.go` | Implement `GetActiveSprint`, `ListSprintsWithCounts` |
| `internal/tui/sidebar.go` | Two-mode navigation, sprint list display |
| `internal/tui/app.go` | Sprint state, quick-assign keys, sprint form routing, status bar |
| `internal/tui/board.go` | `sprintHeader` field + render, no filter logic (filter is in loadTasks) |
| `internal/tui/msgs.go` | Add `sprintSelectedMsg`, `loadSprintsMsg`, `openSprintFormMsg`, `sprintsLoadedMsg` |
| `internal/tui/forms/task_form.go` | Add sprint field, sprints param, SprintID in SavedMsg |
| `internal/tui/forms/sprint_form.go` | New file — sprint creation modal |
| `internal/mcp/tools.go` | Add `assign_task_sprint` tool |
| `cmd/keroagile/cmd_sprint.go` | Add `sprint assign` subcommand |

No schema migrations needed — `sprint_id` column already exists on `tasks`.
