# KeroAgile — Claude Code Guide

## Build & test

```bash
make build        # go build -o KeroAgile ./cmd/keroagile/
make test         # go test ./...
make install      # installs to ~/.local/bin/KeroAgile
go vet ./...
```

Binary name is `KeroAgile` (capital K, capital A). Do not rename it.

## Architecture

Strict layering — each layer may only import layers below it:

```
cmd/keroagile/      Cobra CLI + BubbleTea entry point
internal/tui/       BubbleTea TUI (app, sidebar, board, detail, forms, styles)
internal/git/       git/gh CLI wrappers (no imports from tui or cmd)
internal/config/    TOML config (~/.config/keroagile/config.toml)
internal/store/     SQLite implementation of domain.Store + syncsrv interfaces
internal/syncsrv/   Sync types, interfaces, client, daemon (imports domain)
internal/domain/    Pure types + service + Store interface (zero I/O, zero external imports)
```

`domain` imports nothing from this project. `syncsrv` imports only `domain` (for entity types used by the daemon). `store` imports `domain` and `syncsrv`. The `domain.Store` interface is defined in `internal/domain/store.go` — not in the store package — so `domain` never imports `store` (dependency inversion). Similarly, `syncsrv.PrimaryStore` / `syncsrv.SecondaryStore` are defined in `internal/syncsrv/store.go` and implemented by `store.Store`.

## Key types

- `domain.Task` — 18 fields including `SprintID *int64`, `Points *int`, `AssigneeID *string`, `PRNumber *int` (nullable pointers, not zero values); also `BlockerDetails []*TaskSummary`, `BlockingDetails []*TaskSummary` (populated by `GetTask`, not `ListTasks`)
- `domain.TaskSummary` — lightweight `{ID, Title, ProjectID, Status}` used for blocker search results and `GetTask` enrichment
- `domain.Status` — string type: `backlog | todo | in_progress | review | done`; has `.Next()`, `.Prev()`, `.Label()`, `.Color()` methods
- `domain.Priority` — string type: `low | medium | high | critical`; has `.Label()`, `.Color()` methods
- `domain.Service` — business logic; wraps `domain.Store`; all mutations go through Service; has `SearchTasks(q, limit)` and `SearchTasksWithHint(q, limit, hintProjectID)` pass-throughs
- `store.Store` — SQLite implementation; compile-time checked: `var _ domain.Store = (*Store)(nil)`

## Database

- Path: `~/.config/keroagile/keroagile.db`
- Pure Go SQLite via `modernc.org/sqlite` (no CGo, no system libsqlite3)
- Single writer: `db.SetMaxOpenConns(1)`
- Schema migrations: append-only in `internal/store/db.go` `schema` const; `migrate()` runs at startup
- Foreign keys enabled via `PRAGMA foreign_keys = ON` at connection open
- `task_deps` table has `ON DELETE CASCADE` — deleting a task removes its dependency rows
- `NextTaskSeq` uses INSERT + conflict update for race-free sequence generation

## CLI patterns

- All commands use `svc *domain.Service` (package-level in `cmd/keroagile/main.go`)
- `--json` flag or auto-detected non-TTY → `printJSON()` from `cmd/keroagile/output.go`
- Exit codes: 0 success, 1 not-found, 2 validation error
- Nullable int flags: use `cmd.Flags().Changed("points")` to distinguish "not provided" from `--points 0`
- Error handling: `errors.Is(err, domain.ErrNotFound)` → `exitNotFound()`, everything else → return err
- `task block <task-id> <blocker-id>` / `task unblock <task-id> <blocker-id>` — cross-project IDs work naturally (no project constraint at DB layer)
- `GetTaskDeps` filters the `blockers` direction via `JOIN tasks ON status != 'done'`; done tasks are not returned as active blockers anywhere (board ⚠ indicator, detail panel, edit form). The `blocking` direction (what I'm blocking) is unfiltered.

## TUI patterns

BubbleTea Elm architecture — value-type models, immutable updates:

- `internal/tui/app.go` — root `App` model; owns sidebar, board, detail as value fields
- Panel focus is `panelFocus int` on `App`; `syncFocus()` propagates to sub-models
- All custom message types are unexported and defined in `internal/tui/msgs.go`
- `taskMovedMsg` is defined in `board.go` (where drag-and-drop emits it)
- Form overlays: `App.blockerPicker *forms.BlockerPicker` (checked first), `App.sprintForm *forms.SprintForm`, `App.form *forms.TaskForm`; non-nil when open; all events route to active overlay first in that order
- Blocker picker: pressing Enter on the Blocks/BlockedBy field in the task form opens `forms.BlockerPicker` — a fuzzy-search overlay over `Service.SearchTasksWithHint`; same debounce pattern (version counter + `tea.Tick(300ms)`) as PR polling
- `diffBlockers(oldIDs, newIDs []string) (toAdd, toRemove []string)` in `app.go` — pure helper; `doUpdateTask` uses it to reconcile dep changes via `svc.AddDep`/`svc.RemoveDep`
- PR polling: 60-second `tea.Tick` in `Init()`, rescheduled each `tickMsg`
- Mouse drag: `DragState` in `drag.go`; `computeSectionTops()` in `board.go` for hit-testing
- `tea.WithMouseCellMotion()` is required for per-cell motion during drag
- `Board.doneExpanded bool` — `z` key toggles the Done section between collapsed (N tasks ▸ collapsed) and expanded; all four code paths that branch on done-collapse (`lineOfCursor`, `computeSectionTops`, `totalContentLines`, `View`) check `!b.doneExpanded`
- Board `[→]` right-arrow handler is in `board.go` Update (not just `detail.go`); emits `jumpToTaskMsg{Blockers[0]}` so the key works when the board panel is focused
- `Board.SetSize` scales `filterInput.Width` and `blockerInput.Width` to the available panel inner width so they cannot overflow the panel when opened at narrow terminal widths
- Board `View()` divider is `strings.Repeat("┄", min(b.width-2, 31))` — dynamic so it never exceeds the panel inner width at narrow split-screen sizes
- Board `View()` strips the trailing `"\n"` from `content` before calling `Render()` — prevents lipgloss `Height()` (which acts as min-height) from expanding the board panel 1 row beyond the sidebar and detail panels

## JSON output

All domain structs have `json:"snake_case"` tags. Field names in JSON output:
- `Task`: `id`, `project_id`, `title`, `status`, `priority`, `assignee_id`, `points`, `pr_number`, etc.
- `Project`: `id`, `name`, `repo_path`, `sprint_mode`
- `User`: `id`, `display_name`, `is_agent`

## Testing

- `internal/domain/` — `package domain_test`; tests use a `mockStore` with both `deps` and `blocking` reverse-index maps
- `internal/store/` — `package store_test`; every test uses `store.Open(":memory:")` via `testStore(t *testing.T)`
- `internal/tui/` — `package tui_test`; tests `NewSidebar`, `NewBoard` directly without running BubbleTea

Never use `t.Fatal` inside a goroutine. Prefer `require.NoError` over `assert.NoError` when the test cannot continue after failure.

## Known limitations

- `Board.panelTop` is hardcoded to 2 in `relayout()`; if the header ever becomes multi-line the drag ghost Y will be off again

## Roadmap

All planned future work is documented in `docs/roadmap.md`. Key upcoming phases:

- **v0.1.x** — fix module path, TUI async I/O, drag ghost, form validation, rune-aware truncation
- **v0.2.0** — MCP server (`KeroAgile mcp` subcommand), Claude Code skills, CI, goreleaser
- **v0.3.0** — API server (port 7432, JWT auth, small-team multi-user), Docker, remote client mode
- **v1.0.0** — React web UI (Vite + TypeScript + TanStack Query + dnd kit + Tailwind), embedded via `go:embed`

## Config file

`~/.config/keroagile/config.toml`:

```toml
default_project  = "KA"
default_assignee = "matt"
```
