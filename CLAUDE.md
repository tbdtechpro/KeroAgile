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
internal/store/     SQLite implementation of domain.Store interface
internal/domain/    Pure types + service + Store interface (zero I/O, zero external imports)
```

`domain` imports nothing from this project. `store` imports only `domain`. The `domain.Store` interface is defined in `internal/domain/store.go` — not in the store package — so `domain` never imports `store` (dependency inversion).

## Key types

- `domain.Task` — 18 fields including `SprintID *int64`, `Points *int`, `AssigneeID *string`, `PRNumber *int` (nullable pointers, not zero values)
- `domain.Status` — string type: `backlog | todo | in_progress | review | done`; has `.Next()`, `.Prev()`, `.Label()`, `.Color()` methods
- `domain.Priority` — string type: `low | medium | high | critical`; has `.Label()`, `.Color()` methods
- `domain.Service` — business logic; wraps `domain.Store`; all mutations go through Service
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

## TUI patterns

BubbleTea Elm architecture — value-type models, immutable updates:

- `internal/tui/app.go` — root `App` model; owns sidebar, board, detail as value fields
- Panel focus is `panelFocus int` on `App`; `syncFocus()` propagates to sub-models
- All custom message types are unexported and defined in `internal/tui/msgs.go`
- `taskMovedMsg` is defined in `board.go` (where drag-and-drop emits it)
- Form overlay: `App.form *forms.TaskForm`; non-nil when open; all events route to form first
- PR polling: 60-second `tea.Tick` in `Init()`, rescheduled each `tickMsg`
- Mouse drag: `DragState` in `drag.go`; `computeSectionTops()` in `board.go` for hit-testing
- `tea.WithMouseCellMotion()` is required for per-cell motion during drag

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

## Known limitations (v0.1.1)

All v0.1.0 bugs were fixed in v0.1.1. Current known gaps:

- `doUpdateTask` in `app.go` does not persist Blocks/BlockedBy dep changes from the task form — tracked in roadmap §2.2, fix in v0.2.0
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
