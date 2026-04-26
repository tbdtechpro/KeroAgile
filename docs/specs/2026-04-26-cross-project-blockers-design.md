# Cross-Project Blockers — Design Spec

> **For agentic workers:** Use `superpowers:subagent-driven-development` or
> `superpowers:executing-plans` to implement this spec task-by-task.

**KA board task:** KA-035  
**Date:** 2026-04-26  
**Goal:** Surface cross-project blocker relationships in both the web UI and TUI, add the API and CLI infrastructure to create/remove them, and fix the pre-existing TUI form persistence bug (roadmap §2.2).

---

## Background

The `task_deps(blocker_id, blocked_id)` table has no `project_id` constraint. `Service.AddDep` and `Service.RemoveDep` are pass-throughs with no project validation — cross-project blockers already work at the data layer. The gap is entirely UI/UX: no way to create or remove blocker relationships from the web UI, no API endpoints for it, no CLI command, and a TUI form that silently drops blocker changes on save.

---

## Architecture

Additive changes across four layers. No schema migrations.

```
internal/domain/   — TaskSummary type; SearchTasks on Store interface + Service
internal/store/    — SearchTasks implementation; enrich GetTask with blocker details
internal/api/      — 3 new routes; enriched GetTask response
internal/tui/      — fuzzy-search overlay; form persistence fix; enriched detail view
cmd/keroagile/     — task block / task unblock subcommands
web/src/           — autocomplete in TaskModal; enriched chips + Blocking section
                     + navigation in TaskDetail
```

---

## Files

| File | Change |
|---|---|
| `internal/domain/task.go` | Add `TaskSummary` struct; add `BlockerDetails`, `BlockingDetails []TaskSummary` to `Task` |
| `internal/domain/store.go` | Add `SearchTasks(q string, limit int) ([]*TaskSummary, error)` to `Store` interface |
| `internal/domain/service.go` | Thin `SearchTasks` pass-through |
| `internal/store/task.go` | Implement `SearchTasks`; enrich `GetTask` with batch blocker lookup |
| `internal/store/store_test.go` | `SearchTasks` tests |
| `internal/domain/domain_test.go` | `AddDep`/`RemoveDep` cross-project tests |
| `internal/api/handlers.go` | `handleSearchTasks`, `handleAddBlocker`, `handleRemoveBlocker`; enrich `handleGetTask` |
| `internal/api/server.go` | Register 3 new routes |
| `internal/api/server_test.go` | Tests for new endpoints |
| `internal/tui/forms/blocker_picker.go` | New BubbleTea model: fuzzy-search overlay |
| `internal/tui/forms/task_form.go` | Wire blocker picker; accept cross-project IDs |
| `internal/tui/app.go` | Fix `doUpdateTask` to persist Blocks/BlockedBy diffs |
| `internal/tui/detail.go` | Enrich blocker rendering; add Blocking reverse-index section |
| `internal/tui/tui_test.go` | `doUpdateTask` persistence test |
| `cmd/keroagile/cmd_task.go` | `task block` and `task unblock` subcommands |
| `web/src/api/types.ts` | Add `TaskSummary`; extend `Task` with `blocker_details`, `blocking_details` |
| `web/src/api/client.ts` | `searchTasks`, `addBlocker`, `removeBlocker` |
| `web/src/components/TaskModal.tsx` | Blocker autocomplete field |
| `web/src/components/TaskDetail.tsx` | Enriched chips, Blocking section, navigation |

---

## API Design

### `GET /api/search/tasks?q=<term>&limit=20&hint_project_id=KA`

Purpose-built for autocomplete. Searches task ID and title across all projects the
authenticated user can see. Returns lightweight `TaskSummary` objects — not the full
`Task` payload. `hint_project_id` is an optional ranking hint (not a filter): tasks
from that project sort first, everything else follows by task sequence. Omitting it
returns all projects sorted by sequence. Forward-compatible with per-user project
visibility scoping.

**Response:**
```json
{
  "tasks": [
    { "id": "KA-021", "title": "Add JWT auth", "project_id": "KA", "status": "done" },
    { "id": "KCP-008", "title": "Career DB schema migration", "project_id": "KCP", "status": "in_progress" }
  ]
}
```

### `POST /api/tasks/{id}/blockers`

**Body:** `{ "blocker_id": "KCP-008" }`  
Calls `Service.AddDep(blockerID, taskID)`. Returns 200. Returns 404 if either task
doesn't exist, 400 if `blocker_id` is missing.

### `DELETE /api/tasks/{id}/blockers/{blocker_id}`

Calls `Service.RemoveDep`. Returns 200. Returns 404 if the relationship doesn't exist.

### `GET /api/tasks/{id}` (enhanced)

Adds two new fields to the response. The existing `blockers`/`blocking` string arrays
remain for backward compatibility. `ListTasks` is unchanged — no enrichment, no
performance impact on the board view.

```json
{
  "id": "KA-028",
  "blockers": ["KA-021", "KCP-008"],
  "blocking": ["KA-030"],
  "blocker_details": [
    { "id": "KA-021", "title": "Add JWT auth", "project_id": "KA", "status": "done" },
    { "id": "KCP-008", "title": "Career DB schema migration", "project_id": "KCP", "status": "in_progress" }
  ],
  "blocking_details": [
    { "id": "KA-030", "title": "Sync settings page", "project_id": "KA", "status": "backlog" }
  ]
}
```

---

## Domain

### `TaskSummary`

```go
type TaskSummary struct {
    ID        string `json:"id"`
    Title     string `json:"title"`
    ProjectID string `json:"project_id"`
    Status    Status `json:"status"`
}
```

### `Task` additions

```go
BlockerDetails  []*TaskSummary `json:"blocker_details,omitempty"`
BlockingDetails []*TaskSummary `json:"blocking_details,omitempty"`
```

### `Store` interface addition

```go
SearchTasks(q string, limit int) ([]*TaskSummary, error)
```

### `Service` addition

Thin pass-through to `Store.SearchTasks`. `AddDep` and `RemoveDep` already exist.

---

## Store Implementation

### `SearchTasks`

SQLite query: fuzzy match on `id LIKE '%q%' OR title LIKE '%q%'` across all projects,
ordered by `(project_id = hint_project_id) DESC, seq ASC` (hint defaults to `""` when
not supplied, so all projects sort equally by sequence), limited to `limit` rows. No cross-project constraint — returns everything the authenticated user can see
(all projects for now; scoped by user membership when that feature exists).

### `GetTask` enrichment

After fetching the task and its `blockers`/`blocking` ID arrays, batch-fetch
`TaskSummary` for each referenced task ID using a single
`WHERE id IN (...)` query. Populate `BlockerDetails` and `BlockingDetails`.
`ListTasks` does not call this enrichment path.

---

## Web UI

### `TaskModal.tsx` — blocker autocomplete field

New "Blocked by" section below existing fields:

- Selected blockers render as chips: same-project chips are red
  (`KA-021 Add JWT auth`); cross-project chips are blue with ↗ and a project badge
  (`↗ KCP-008 Career DB migration [KCP]`).
- A text input below the chips triggers `searchTasks(q)` with 300ms debounce on
  keystroke. Dropdown lists results with project badge; current-project results appear
  first.
- Clicking a dropdown result adds it as a chip and clears the input.
- Clicking × on a chip removes it.
- On save: added blockers → `POST /api/tasks/{id}/blockers`; removed → `DELETE`.
- Uses TanStack Query mutations; checkboxes disabled while mutation is pending (same
  pattern as `SyncSettingsPage`).

### `TaskDetail.tsx` — enriched chips + Blocking section + navigation

- "Blocked by" section: uses `task.blocker_details` instead of `task.blockers`.
  Same-project chips: red. Cross-project chips: blue, ↗, project badge.
  Clicking any chip calls the React router to navigate to `/<project_id>` (switching
  the active project in the sidebar) and opens that task's detail drawer via the
  existing `selectedTaskId` state — the same mechanism used when clicking a task card
  on the board.
  × button on each chip calls `removeBlocker` mutation.
- New "Blocking" section: green chips using `task.blocking_details`, same structure.
  Appears only when `blocking_details.length > 0`.
- Existing ⚠ warning icon in title when `blockers.length > 0` is unchanged.

---

## TUI

### `internal/tui/forms/blocker_picker.go` — new model

BubbleTea model with two sub-components:
- `textinput.Model` for the search query
- `list.Model` for filtered results

On each keystroke, a `tea.Tick`-debounced message fires `Service.SearchTasks(q, 20)`.
Results render as `KA-021 · Add JWT auth` with cross-project tasks shown as
`[KCP] KCP-008 · Career DB migration`. Arrow keys navigate the list. Enter returns
the selected task ID to the parent form. Esc returns without selection.

Activated from the task form's Blocks/BlockedBy field when the user presses Enter —
the same overlay pattern already used by `SprintForm`.

### `internal/tui/forms/task_form.go`

The Blocks/BlockedBy fields accept both direct ID entry (power users) and the
fuzzy-search overlay (the default assisted flow). Pressing Enter on the field opens
the picker. Typing an ID directly and pressing Tab validates it inline against the
service.

### `internal/tui/app.go` — fix `doUpdateTask` (§2.2)

Current bug: Blocks/BlockedBy changes from the task form are read but discarded.

Fix: diff the form's blocker ID list against `task.Blockers`:
- IDs in form but not in task → `Service.AddDep(newID, taskID)`
- IDs in task but not in form → `Service.RemoveDep(removedID, taskID)`

Errors (e.g. ID not found) surface as a flash message (`errMsg`) rather than silently
swallowing.

### `internal/tui/detail.go` — enriched rendering

- Blockers render as `[KCP] KCP-008 · Career DB migration` for cross-project tasks,
  `KA-021 · Add JWT auth` for same-project.
- New "Blocking" section renders the `BlockingDetails` reverse-index.
- Both sections only appear when non-empty.

---

## CLI

### `KeroAgile task block <task-id> <blocker-id>`

```bash
KeroAgile task block KA-028 KCP-008
# → KA-028 is now blocked by KCP-008
```

Calls `Service.AddDep(blockerID, taskID)`. `errors.Is(err, domain.ErrNotFound)` →
`exitNotFound()`. Supports `--json`. Cross-project IDs work naturally.

### `KeroAgile task unblock <task-id> <blocker-id>`

```bash
KeroAgile task unblock KA-028 KCP-008
# → KCP-008 removed as blocker from KA-028
```

Calls `Service.RemoveDep`. Same error handling pattern.

Both commands follow the existing CLI conventions in `cmd_task.go`: package-level `svc`,
exit codes 0/1/2.

---

## Testing

### `internal/store/store_test.go` — `SearchTasks`

- Fuzzy match on ID prefix (`KA-0` returns KA tasks)
- Title substring match
- Cross-project results included (tasks from multiple projects in result)
- `limit` respected
- Results ordered: current-project tasks before others

### `internal/domain/domain_test.go` — cross-project deps

- `AddDep("KA-001", "KCP-007")` succeeds; no project constraint enforced
- `AddDep` with non-existent ID returns `ErrNotFound`
- `RemoveDep` on non-existent relationship is a no-op (idempotent)

### `internal/api/server_test.go` — new endpoints

- `GET /api/search/tasks?q=sync` returns `TaskSummary` list with correct project IDs
- `POST /api/tasks/KA-028/blockers` with `{"blocker_id":"KCP-008"}` returns 200;
  subsequent `GET /api/tasks/KA-028` includes KCP-008 in `blocker_details`
- `DELETE /api/tasks/KA-028/blockers/KCP-008` returns 200; relationship removed
- `GET /api/tasks/KA-028` without prior blockers: `blocker_details` is empty array,
  not null

### `internal/tui/tui_test.go` — persistence fix

- Create task, set a blocker ID in form, call `doUpdateTask`, verify `GetTask` returns
  the blocker in `Blockers` list
- Remove a blocker via form, call `doUpdateTask`, verify it's gone

### End-to-end smoke test (manual)

1. Web UI: open task form → type "career" in blocker field → select KCP-008 → save →
   verify blue ↗ chip in detail view → click chip → verify navigation to KCP board,
   KCP-008 detail open
2. TUI: open task form → tab to Blocks field → Enter → type "career" in picker →
   select KCP-008 → save → verify cross-project prefix shown in detail view
3. CLI: `KeroAgile task block KA-028 KCP-008` → verify with
   `KeroAgile task get KA-028 --json` → blockers includes KCP-008
