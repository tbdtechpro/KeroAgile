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
internal/artifacts/ Files kept against tickets: holding area, intake, room, fetch, remote stores (imports domain)
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
- Validation lives in `domain.Service` and nowhere else, so it holds for the TUI, CLI, MCP and HTTP alike: status and priority must be known values; a ticket cannot block itself and a blocker link that would close a cycle is refused; an unknown project, ticket, assignee or sprint is `ErrNotFound` naming what is missing; project and ticket IDs are accepted in any letter case; a sprint must belong to the ticket's project. Rule violations wrap `domain.ErrInvalid`
- `domain.TaskPatch` — the one way to change an existing ticket: each field is a `domain.Change[T]`, either left alone (zero value) or set with `domain.To(v)`; `domain.Clear[T]()` empties an optional field. `Service.PatchTask(id, p)` validates the named fields and the store applies them in a single `UPDATE`, so no field the caller did not name is ever written. Never read a ticket, modify the struct and write it back: that reverts whatever another writer changed in between. `MoveTask`, `LinkBranch`, `LinkPR`, `MarkPRMerged` and `AssignTaskToSprint` are one-field patches
- `TaskPatch.IfUpdatedAt` makes a patch conditional on the ticket not having changed since the caller read it; a mismatch is `domain.ErrConflict` and nothing is written. The TUI does not use it yet (its form closes before the save completes, so a refusal would lose the typed text)
- `domain.Note` — a dated, signed note on a ticket (`task_notes`, removed with its ticket). `Service.AddNote(taskID, authorID, body)` trims the body, refuses an empty one or one over `MaxNoteLen` (20,000 characters), and checks a non-empty author is a user. Notes are never edited. `Service.GetTask` fills `Task.Notes` (and later links and files) through `withRecords`; listings leave them out. Records are local only: the API does not forward them to a sync primary
- `domain.Artifact` — a file kept against a ticket (`task_id` nil: *unfiled*, kept for its project until filed or discarded). Contents are a `domain.Blob` named by SHA-256, stored once however many artifacts use them; tables `artifacts` and `blobs`. `Service.RecordArtifact`, `GetArtifact`, `ListArtifacts(ArtifactQuery)`, `FileArtifact` (a ticket of the same project), `DiscardArtifact` (keeps the row with `discarded_at`, so the sweep does not keep that path again). `Artifact.Where` is `here | remote | both | unavailable`, from the blob's flags. `EffectivePolicy(project, default)`: a project's `artifact_policy` (`remote | local | off`) or the configured `default_policy` (shipped `local`)
- `artifacts.Keeper` (`internal/artifacts`) owns contents: `Attach(taskID, path, kind, note, author)` copies the file into the holding area (`<data dir>/artifacts/<aa>/<bb>/<sha256>`, beside the database) while hashing it; `Fetch(ctx, id)` returns a local path, correcting a blob row whose file has gone. Room: `local_max` is a hard limit and `min_free` a floor on free disk space; to make room it removes first contents nothing uses, then uploaded contents above `keep_local_under`; if that is not enough the file is refused with `ErrNoRoom`, and nothing already kept is dropped. Its database needs (`artifacts.Records`: blobs, leases, the `kv` table) are methods on `store.Store` outside `domain.Store`. The CLI and MCP build it with `newKeeper()` from the `[artifacts]` config (`config.ArtifactsConfig.Resolved()` applies the defaults)
- `domain.TaskQuery` — the one way to find tickets across projects: optional `Text` (ID, title or description), `ProjectID`, `Statuses`, `AssigneeID`, `Label`, `SprintID`, `Limit`. `Service.FindTasks(q)` returns the matching tickets (with blockers and blocking filled in) and the total that match; `Service.TaskCounts()` returns ticket counts per project and status
- `store.Store` — SQLite implementation; compile-time checked: `var _ domain.Store = (*Store)(nil)`
- `domain.Project` names where its code lives twice: `RepoDir` (the local checkout, which git runs in and a working directory is matched against) and `RepoRemote` (normalised `host/owner/repo`, which gh is asked about). `RepoPath` is kept as first given; `ClassifyRepo` sorts it, and `store.Open` backfills both fields from it for projects that have neither. `domain.MatchProject` / `Service.ProjectForDir(cwd, remote, home)` pick the project whose directory holds `cwd` (deepest wins), else the one whose remote is the clone's origin; two equal matches are `ErrInvalid`, never a guess. `Service.PatchProject` / `domain.ProjectPatch` change project settings field by field. CLI: `project set <id> --dir --remote --artifact-policy`, `project which [--dir]`
- `git.Repo{Dir, Remote}` (`git.ForProject(p)`) runs gh with `-R remote`, or inside the checkout when there is no remote. Branch names are checked with `domain.ValidBranchName` (no leading `-`, no spaces, nothing git refuses) by `PatchTask` and by the git wrappers, which also pass the branch after `--end-of-options` / `--`

## Database

- Path: `~/.config/keroagile/keroagile.db`
- Pure Go SQLite via `modernc.org/sqlite` (no CGo, no system libsqlite3)
- Single writer per process: `db.SetMaxOpenConns(1)`. Many processes share the file (every `KeroAgile mcp`, the TUI, the CLI, `serve`), so `store.Open` puts its settings in the DSN, where every pooled connection gets them: `busy_timeout(5000)` (wait for another process's lock instead of failing with `database is locked`), `foreign_keys(1)`, `journal_mode(WAL)`
- WAL keeps `keroagile.db-wal` and `keroagile.db-shm` next to the database. Back up with `sqlite3 <db> ".backup <dest>"`, never `cp`; when restoring a database file, move any `-wal`, `-shm` and `-journal` files aside first
- Schema migrations: append-only in `internal/store/db.go` `schema` const; `migrate()` runs at startup
- `updated_at` is written with sub-second precision (`stamp()` in `store/task.go`, RFC 3339 with nanoseconds) so `IfUpdatedAt` can tell apart two changes in the same second; rows written earlier hold whole seconds and still compare correctly. `store.UpdateTask` (whole row) remains only for sync replication, which applies a remote ticket's full state
- `task_deps` table has `ON DELETE CASCADE` — deleting a task removes its dependency rows
- `NextTaskSeq` uses INSERT + conflict update for race-free sequence generation

## CLI patterns

- All commands use `svc *domain.Service` (package-level in `cmd/keroagile/main.go`)
- `--json` flag or auto-detected non-TTY → `printJSON()` from `cmd/keroagile/output.go`
- Exit codes: 0 success, 1 not-found, 2 validation error
- Nullable int flags: use `cmd.Flags().Changed("points")` to distinguish "not provided" from `--points 0`
- Error handling: `errors.Is(err, domain.ErrNotFound)` → `exitNotFound()`, everything else → return err; `main` turns a returned `domain.ErrInvalid` into exit code 2 via `exitCode()`
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
- The task form records what it held when it opened on a ticket (`TaskForm.opened`) and reports in `SavedMsg.Changed` which fields differ at submit. `doUpdateTask` sends a `TaskPatch` holding only those fields, so the rest of the ticket keeps whatever the store has, and a value the form cannot show in full (a description longer than the 99-line text box) is not written back unless edited. An untouched sprint box is not re-validated
- `diffBlockers(oldIDs, newIDs []string) (toAdd, toRemove []string)` in `app.go` — pure helper; `doUpdateTask` uses it to reconcile dep changes via `svc.AddDep`/`svc.RemoveDep`, against the lists the form showed when it opened (`SavedMsg.BlocksWas` / `BlockedByWas`)
- PR polling: 60-second `tea.Tick` in `Init()`, rescheduled each `tickMsg`
- Mouse drag: `DragState` in `drag.go`; `computeSectionTops()` in `board.go` for hit-testing
- `tea.WithMouseCellMotion()` is required for per-cell motion during drag
- `Board.doneExpanded bool` — `z` key toggles the Done section between collapsed (N tasks ▸ collapsed) and expanded; all four code paths that branch on done-collapse (`lineOfCursor`, `computeSectionTops`, `totalContentLines`, `View`) check `!b.doneExpanded`
- Board `[→]` right-arrow handler is in `board.go` Update (not just `detail.go`); emits `jumpToTaskMsg{Blockers[0]}` so the key works when the board panel is focused
- `Board.SetSize` scales `filterInput.Width` and `blockerInput.Width` to the available panel inner width so they cannot overflow the panel when opened at narrow terminal widths
- Board `View()` divider is `strings.Repeat("┄", min(b.width-2, 31))` — dynamic so it never exceeds the panel inner width at narrow split-screen sizes
- Board `View()` strips the trailing `"\n"` from `content` before calling `Render()` — prevents lipgloss `Height()` (which acts as min-height) from expanding the board panel 1 row beyond the sidebar and detail panels
- `Detail.View()` wraps its content to the panel width and cuts it to `height - 2` lines with `clipLines`, ending with `↓ N more lines`
- Ticket records in the TUI: selecting a ticket shows the list's copy at once, then `loadRecords` reads it in full (`Service.GetTask`, with notes, links and files) and `taskRecordsMsg` replaces it if that ticket is still on show. `a` opens `forms.NoteForm` (overlay checked after the blocker picker): enter saves, ctrl+j starts a new line, esc cancels; the note is signed by the TUI's user when that user is on the board, otherwise unsigned
- A panel taller than its allotted height makes `App.View()` taller than the terminal, and bubbletea then drops lines from the **top** of the screen (header and top borders vanish). Every panel must keep its content within `height - 2` lines itself
- Sidebar lists scroll: `Sidebar.projScroll` / `sprintScroll` hold the first visible row. Each mode builds a `sidebarList{head, rows, foot}` (`projList()` / `sprintList()`), and `window()` is the single source of truth for which rows are on screen — `render`, `clampScroll`, `scrollBy` (mouse wheel) and `projectAtY` (click hit-testing) all call it, so they cannot disagree
- Sidebar `render()` shows `↑ N` on the heading and `↓ N more` on the line under the list when rows are hidden, drops the footer (board summary / key hint) when it would leave fewer than `minListRows` rows, and finally wraps then cuts the content to `height - 2` lines as a guarantee
- Sidebar click mapping: the first list row is at screen row `sidebarPanelTop` (2: header row + top border) plus the number of heading lines

## JSON output

All domain structs have `json:"snake_case"` tags. Field names in JSON output:
- `Task`: `id`, `project_id`, `title`, `status`, `priority`, `assignee_id`, `points`, `pr_number`, etc.
- `Project`: `id`, `name`, `repo_path`, `sprint_mode`
- `User`: `id`, `display_name`, `is_agent`

## HTTP API

- `PATCH /api/tasks/{id}` builds one `TaskPatch` from the request (`taskPatchFromJSON` in `internal/api/handlers.go`): an absent field is left alone; `null` empties `assignee_id`, `points`, `sprint_id` and `labels`; a status change is applied together with the other fields. `expected_updated_at` (the `updated_at` the client read) makes it conditional
- `statusFor(err)` maps service errors: `ErrInvalid` 400, `ErrNotFound` 404, `ErrConflict` 409

## MCP tools

- `internal/mcp/tools.go` defines the tools; `CallTool` dispatches them over `domain.Service`
- `update_task` changes only the fields passed. Empty strings are ignored (agents send them for fields they do not mean to touch); to empty a field, name it in `clear` (`description`, `points`, `labels`, `assignee_id`, `sprint_id`). It accepts `sprint_id`, and `expected_updated_at` from `get_task` for a conditional update
- `list_tasks` (one project) and `search_tasks` (every project, or one) both go through `Service.FindTasks`, so their filters behave identically: `status` takes one status or `open` (everything not done), plus `assignee_id`, `label`, and for `list_tasks` `sprint_id`. `search_tasks` adds `query` and needs at least one criterion; with no query it is the cross-project view (for example `status=in_progress` with `assignee_id`). A mistyped status is an error, not an empty list
- `add_note(task_id, body)` adds a note signed by `KEROAGILE_USER`, else the `claude` user when the board has one (`authorID`). Record tools live in `internal/mcp/records_tools.go` and are reached from `CallTool`'s default case through `callRecordTool`
- `set_project(project_id, repo_dir?, repo_remote?, artifact_policy?)` changes project settings through `Service.PatchProject`; `list_projects` shows `artifact_policy` as set and, when the server keeps files, `effective_policy`
- File tools: `attach_file(task_id, path, kind?, note?)`, `list_artifacts(task_id | project_id [+ unfiled])`, `fetch_artifact(artifact_id)` → local path, `file_artifact`, `discard_artifact`. `get_task` lists a ticket's files compactly (`artifactRow`). The server gets the keeper through `mcp.Env{Svc, Files, FilesErr}` (`CallToolEnv`, `DispatchEnv`); a bad `[artifacts]` config leaves `Files` nil and the file tools say why, rather than the server failing to start
- `list_projects` returns compact rows with `open` and `total` ticket counts
- Listings (`list_tasks`, `search_tasks`, `get_sprint`) return one compact `taskRow` per ticket, without descriptions, as unindented JSON inside `{project_id, total, returned, tasks}`; when cut to `limit` (`list_tasks` default 200, max 500; `search_tasks` default 50, max 200) the reply adds `truncated` and a `hint`. `get_task` is the full view; `list_tasks` with `detail: "full"` includes descriptions
- Keep listings small. MCP clients cap tool results (a 98-ticket project at 308,567 characters was refused), and whatever fits is paid for in the model's context on every call. A row must still carry what the `.claude/commands/keroagile-*.md` commands read: `blockers`, `blocking`, `branch`, `pr_number`, `assignee_id`, `points`

## Testing

- `internal/domain/` — `package domain_test`; tests use a `mockStore` with both `deps` and `blocking` reverse-index maps
- `internal/store/` — `package store_test`; every test uses `store.Open(":memory:")` via `testStore(t *testing.T)`
- `internal/tui/` — `package tui_test`; tests `NewSidebar`, `NewBoard` directly without running BubbleTea

Never use `t.Fatal` inside a goroutine. Prefer `require.NoError` over `assert.NoError` when the test cannot continue after failure.

## Concepts that don't exist

KeroAgile tasks are **flat within a project**. There is no concept of:

- parent tasks
- sub-tasks
- epics
- themes
- "umbrella" tasks
- hierarchical groupings of any kind

Do not invent these relationships when planning or describing work. Tasks within a project are siblings.

The only inter-task relationship is the **blocker DAG** (`task_deps` table), managed via the `add_blocker` / `remove_blocker` MCP tools or the `task block` / `task unblock` CLI commands. A blocker means "task A must reach `done` before task B can start" — it is a dependency, not a hierarchy.

To group related work, use:

- **shared labels** (free-form strings on each task), or
- **shared sprint** (assign tasks to the same sprint via `assign_task_sprint` or the sidebar)

This section is mirrored verbatim by `internal/mcp/instructions.go` (`conceptsInstructions`) so MCP clients see the same constraint at handshake time. Keep them in sync. The handshake also sends `recordsInstructions`: attach work products with `attach_file` before finishing a ticket, commit what belongs in the repo, record progress with `add_note`, and file or discard unfiled files without guessing a ticket.

## Known limitations

- `Board.panelTop` is hardcoded to 2 in `relayout()`; if the header ever becomes multi-line the drag ghost Y will be off again. `sidebarPanelTop` in `sidebar.go` makes the same assumption for click hit-testing

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

An `[artifacts]` table (sizes like `"1GiB"`, durations like `"14d"`; every key optional, with `omitempty` so `init` never writes an empty one) sets the holding area, its limits, `default_policy` and the remote store; `[artifacts.s3]` and `[artifacts.folder]` describe the remote. The same file holds `api_secret` (signs web sessions), `remote_url` / `remote_token` and the `[sync]` table. `config.Save` writes it with mode 0600. `KeroAgile init` changes only the two defaults above and must preserve everything else.
