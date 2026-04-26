# KeroAgile Roadmap

This document captures all known future work, from immediate polish through long-horizon features. Items are grouped by theme, not timeline — priority within each section runs roughly top-to-bottom.

---

## 1. Immediate polish (v0.1.x)

These are known issues in the shipped v0.1.0 code, documented in CLAUDE.md. None are blockers for daily use but should be cleared before a v0.2.0 release.

### 1.1 Fix `go.mod` module path

**Current:** `module keroagile` (local name only)  
**Required:** `module github.com/tbdtechpro/KeroAgile`

All internal import paths must be updated to match. One-line change to `go.mod`, then a global find-and-replace across `*.go` files. Affects every file with an internal import. Required before anyone else can use the library or before `go install` works from GitHub.

### 1.2 Fix TUI sync I/O in Update loop

`App.Update` calls `svc.GetTask`, `svc.MoveTask`, `svc.DeleteTask`, `svc.GetProject`, and `svc.MarkPRMerged` synchronously on the BubbleTea event-loop goroutine. On fast local SSDs this is invisible, but under any disk pressure it causes visible stutter.

Fix: wrap each store call in a `tea.Cmd` closure, emit a result message, and handle state updates in `Update` when the message arrives — the same pattern already used for `loadTasks` and `loadProjects`.

### 1.3 Fix drag ghost rendering

Two related rendering bugs:

- **Wrong Y coordinate:** `drag.CurrentY` is in raw terminal coordinates; the board panel starts below the header row. Subtract the board panel's top offset before comparing against `lines` index.
- **Extra line:** the ghost is inserted *before* the task row rather than *replacing* it, growing panel content by one line per render while dragging. Replace the original row with the ghost instead of prepending.

### 1.4 Add form validation for priority and status

The task form accepts arbitrary text in the Priority and Status fields. Bad values (e.g. `"urgent"`, `"wip"`) are cast directly to `domain.Priority`/`domain.Status` and reach the store.

Fix: add cases to `TaskForm.validate()` checking against the known enum values. Surface a red error line (`✗ invalid priority — use: low medium high critical`) before allowing submit.

### 1.5 Rune-aware string truncation

`truncate()` in `detail.go` and `board.go` slices bytes, not runes. Any multi-byte UTF-8 character (emoji, CJK, accented letters) that lands on the cut boundary renders as `U+FFFD` replacement character.

Fix: replace the byte slice with `[]rune(s)[:n]` and convert back, or use `go.uber.org/runes` / `golang.org/x/text` for width-aware truncation (important for CJK which is double-width).

### 1.6 Expanded README with ASCII screenshots and user guide

The current README is accurate but sparse. Expand to include:

- ASCII art mockup of the three-panel TUI layout with labelled regions
- Annotated ASCII "screenshot" of the task form overlay
- ASCII table showing the status state machine (`backlog → todo → in_progress → review → done`)
- Fuller user guide: config file reference, sprint workflow end-to-end, git integration walk-through, PR auto-transition example, blocker graph usage
- Troubleshooting section (no `gh` installed, SQLite locked, terminal too narrow)
- Contributing guide stub

---

## 2. Core workflow completion (v0.2.0)

Features that are partially present (data model + CLI) but missing from the TUI, or missing entirely.

### 2.1 Sprint panel in TUI

Sprints are fully modelled in the domain and CLI but invisible in the TUI. The board should support a sprint-aware mode:

- Sidebar shows active sprint name + dates when the project has `sprint_mode = true`
- Board filter toggle: `All tasks` vs `Current sprint` (toggle with `s` key)
- Sprint summary row in sidebar: story points committed vs completed
- Command palette or form for activating a sprint and assigning tasks to it

The `TaskFilters.SprintID` filter already exists in the store — the TUI just needs to pass it.

### 2.2 Interactive blocker management in TUI

Blockers are shown in the detail panel and can be set via the task form, but there is no way to:

- Add/remove a blocker from the board without opening the full edit form
- See a visual indicator on a task row that it is blocked (e.g. `⚠` prefix in red)
- Navigate from a blocked task to its blocker with a single keypress

Proposed: `b` key on the board opens a minimal "block by" input; blocker tasks show a `⚠` prefix in the board row; pressing `→` in the detail panel on a blocker ID jumps to that task.

### 2.3 Task filter / search in TUI

No way to filter the board without using the CLI. Proposed:

- `/` key opens an inline filter bar (similar to Vim's `/` search)
- Filter by: free-text substring of title, status, priority, assignee, label
- Active filter shown in board header; `esc` clears
- Filter state persists until explicitly cleared (survives task reloads)

### 2.4 GitHub Actions CI

`.github/workflows/ci.yml` that runs on every push and PR to `master`:

```yaml
- go test ./...
- go vet ./...
- go build ./cmd/keroagile/
```

Matrix: latest Go release + one prior. Required before accepting external contributions.

### 2.5 Binary releases via goreleaser

`.goreleaser.yaml` that produces signed binaries for:
- `linux/amd64`, `linux/arm64`
- `darwin/amd64`, `darwin/arm64`
- `windows/amd64`

Triggered on `git tag v*`. Publishes to GitHub Releases. Enables `go install github.com/tbdtechpro/KeroAgile/cmd/keroagile@latest` to work once the module path is fixed.

---

## 3. Deployment modes (v0.3.0)

KeroAgile currently only runs as a local binary against a local SQLite file. This section covers making it easy to run on a homelab server and access from any machine on the home network, with progressively more capable modes.

### 3.1 Mode A — SSH (zero new code, document now)

The simplest "remote" mode: install KeroAgile on the server, SSH in, run the TUI. Works today. The TUI is already designed for terminal use so it works perfectly over SSH including mouse support (most SSH clients forward mouse events).

Document in README:
- Server install via `make install` or goreleaser binary
- `~/.ssh/config` alias for one-command launch: `ssh homelab -t KeroAgile`
- `tmux`/`screen` session persistence so the board stays open between connections

No code changes required. This should be documented alongside the Docker option.

### 3.2 Mode B — Docker container (self-contained, single machine)

A `Dockerfile` and `docker-compose.yml` that packages KeroAgile with its SQLite database persisted on a named volume:

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o KeroAgile ./cmd/keroagile/

FROM alpine:3.21
COPY --from=builder /app/KeroAgile /usr/local/bin/KeroAgile
VOLUME /data
ENV KEROAGILE_DATA_DIR=/data
ENTRYPOINT ["KeroAgile"]
```

```yaml
# docker-compose.yml
services:
  keroagile:
    build: .
    volumes:
      - keroagile-data:/data
    tty: true
    stdin_open: true
volumes:
  keroagile-data:
```

This requires adding a `KEROAGILE_DATA_DIR` environment variable to `internal/config/config.go` so the DB and config paths can be overridden. With Docker, users connect via `docker exec -it keroagile KeroAgile` or via the SSH mode above (install an SSH daemon in the container).

### 3.3 Mode C — API server mode (network-accessible, homelab)

Adds an optional HTTP API server that exposes all domain service operations over a REST API. This is the architectural prerequisite for the MCP remote mode (§4.1), the web interface (§5), and multi-machine access from anywhere on the home network.

**Design decisions (confirmed):**
- Port: **7432**
- Deployment target: **small team** — designed for a handful of named users (not anonymous public access). The current solo user will add team members over time; the auth system should support this from day one without needing migration.
- Data model: **one shared board** — all users see the same projects and tasks; users filter to their own work via a `--mine` / `@me` shorthand
- Auth: **per-user identity with admin management** — no email validation, no 2FA, no OAuth

**Launch:**
```bash
KeroAgile serve --addr :7432
```

#### Auth design

Username/password authentication issues a short-lived JWT bearer token. All API requests carry the token in `Authorization: Bearer <token>`. The domain layer is unchanged — the server layer validates the token and passes the caller's user ID as context.

**Bootstrap:** the first `user add` call made while no users exist (or via a `--admin` flag) creates the admin account. Subsequent user creation requires an admin token.

**Password storage:** bcrypt hashes stored in a new `credentials` table alongside the existing `users` table. The current `User` type (display name, agent flag) stays unchanged — credentials are a separate concern.

**Admin capabilities:**
- `KeroAgile user add <id> <name> --password <initial>` — create user with a temporary password
- `KeroAgile user reset-password <id> <new-password>` — admin resets any user's password
- `KeroAgile user list --reset-requested` — see who has flagged a forgotten password

**User password reset flow:**
1. User runs `KeroAgile user forgot-password` against the server — sets a `reset_requested` flag in the DB
2. Admin sees the flag via `user list --reset-requested` or a notification in the TUI/web
3. Admin resets the password; user logs in with the new one

No email required. Simple, works entirely within the tool.

**Token expiry:** configurable in `config.toml` (`token_ttl_hours`, default 168 = 7 days). No refresh token complexity.

#### API surface

```
POST   /api/auth/login              { user_id, password } → { token }

GET    /api/projects
POST   /api/projects
GET    /api/projects/:id
PATCH  /api/projects/:id

GET    /api/projects/:id/tasks
POST   /api/projects/:id/tasks
GET    /api/tasks/:id
PATCH  /api/tasks/:id
DELETE /api/tasks/:id
POST   /api/tasks/:id/move
POST   /api/tasks/:id/link-branch
POST   /api/tasks/:id/link-pr

GET    /api/users
POST   /api/users                   (admin only)
POST   /api/users/:id/reset-password (admin only)
POST   /api/users/forgot-password

GET    /api/sprints?project=:id
POST   /api/sprints
POST   /api/sprints/:id/activate
POST   /api/sprints/:id/assign-task
```

All responses use the existing snake_case JSON tags. All routes except `/api/auth/login` require a valid bearer token.

#### "My tasks" filter

The current `TaskFilters.AssigneeID` already exists in the store. The API and CLI expose it as:

```bash
KeroAgile task list --mine          # CLI shorthand for --assignee <current user>
```

In the TUI, a toggle (e.g. `f` key) switches between "all tasks" and "my tasks" for the active project. The server resolves `@me` to the authenticated user's ID.

#### Remote client mode

The CLI and TUI gain a `--remote http://homelab:7432` flag (or `remote_url` + `remote_token` in `config.toml`). When set, all store calls become HTTP requests to the API server instead of local SQLite. Implementation: a new `internal/store/remote/client.go` that implements `domain.Store` over HTTP. The domain and service layers are unchanged.

```toml
# ~/.config/keroagile/config.toml
remote_url   = "http://homelab:7432"
remote_token = "your-jwt-token"
```

This enables the full multi-machine workflow: server runs on the homelab, any machine on the network runs `KeroAgile` or `KeroAgile mcp` against the shared board with their own identity.

#### docker-compose with server mode

```yaml
services:
  keroagile:
    build: .
    command: ["KeroAgile", "serve", "--addr", "0.0.0.0:7432"]
    ports:
      - "7432:7432"
    volumes:
      - keroagile-data:/data
    environment:
      - KEROAGILE_DATA_DIR=/data
volumes:
  keroagile-data:
```

### 3.4 Mode D — TUI in browser via terminal proxy (optional convenience)

For users who want the TUI accessible from a browser without SSH, wrap the TUI with `ttyd` or `gotty` in the Docker image:

```dockerfile
# Optional browser-TUI mode
CMD ["ttyd", "-p", "7433", "KeroAgile"]
```

Exposes the full interactive TUI at `http://homelab:7433` in any browser. Zero JavaScript, zero frontend code — the TUI renders exactly as in a local terminal. This is a convenience layer, not a first-class feature.

---

## 4. Claude Code integration (v0.2.0)

KeroAgile should integrate natively with Claude Code so that an AI agent working in any repo can read and update the board in plain English — "add a high-priority task called X to project KA", "what's in progress?", "mark KA-007 as done" — without the user ever typing a CLI command.

**Status:** MCP (Model Context Protocol) is fully supported in Claude Code today. It is not blocked on any external API stabilisation. The only missing piece is the `KeroAgile mcp` subcommand in the binary. Once that exists and the user drops a `.mcp.json` file in their project (or sets it globally), Claude Code discovers and calls KeroAgile tools automatically in every conversation.

### 4.1 MCP server (`KeroAgile mcp`)

**Ready to implement now.**

Add a `mcp` subcommand that starts a JSON-RPC 2.0 server over stdio, implementing the MCP tool protocol that Claude Code speaks. The server wraps `domain.Service` directly — no new data layer required.

New package: `internal/mcp/server.go` — tool registration, request dispatch, response formatting.

**Project auto-detection (confirmed behavior):**

When a KeroAgile MCP tool is invoked from within a git repo, the MCP server resolves the active project automatically:

1. Read the git remote URL of the current working directory (via `git remote get-url origin`)
2. Match against `projects.repo_path` in the database (exact or suffix match)
3. If a match is found, use that project as the default `project_id` for tools that accept it
4. If no match is found (project exists but `--repo` was not set), return a structured error:
   ```
   Could not auto-detect project. Run:
     KeroAgile project add <id> --repo <path-or-remote-url>
   to link this repository to a project.
   ```
5. If `project_id` is explicitly provided in the tool call, skip detection and use it directly

This means `create_task` and `list_tasks` work without a `project_id` argument when called from a linked repo — Claude never needs to ask the user which project they mean.

**Tools to expose:**

| Tool | Description |
|------|-------------|
| `list_projects` | List all projects |
| `list_tasks` | List tasks; filters: `project_id`, `status`, `assignee_id`, `sprint_id` |
| `get_task` | Full task detail including blockers, branch, PR |
| `create_task` | Create task; params: title, project_id, assignee_id, priority, points, labels |
| `update_task` | Edit title, description, priority, assignee, points, labels |
| `move_task` | Change status; params: task_id, status |
| `delete_task` | Delete task by ID |
| `link_branch` | Link a git branch to a task |
| `list_users` | List all users |
| `get_sprint` | Get sprint by project; returns active sprint and its tasks |
| `add_blocker` | Mark task A as blocking task B |
| `remove_blocker` | Remove a blocker relationship |

**Launch modes:**
```bash
KeroAgile mcp                        # stdio (local Claude Code config)
KeroAgile mcp --remote http://homelab:7432  # proxy to API server (Phase 3.3)
```

The `--remote` flag lets a local Claude Code instance talk to a KeroAgile server on the home network — no SQLite file needed on the user's machine.

### 4.2 Claude Code skills

Skills that teach Claude when and how to use KeroAgile naturally during development work:

- **`keroagile-update`** — when Claude finishes a task, auto-finds the matching KeroAgile task by branch name, moves it to `done`, and links the PR number
- **`keroagile-standup`** — summarises in-progress tasks for the current project with assignees and blockers; useful as a morning context-setter
- **`keroagile-plan`** — reads the backlog and suggests sprint composition based on priority and story points

Skills use the MCP tools above — they are prompt-engineering guides for *when* to call the tools, not additional code.

### 4.3 Configuration

Add a **Claude Code Integration** section to the README once 4.1 ships. The exact configuration format is confirmed — Claude Code uses `.mcp.json` at the project root (or `~/.claude/settings.json` for global config):

**Local setup** (KeroAgile binary on the same machine as Claude Code):

```json
// .mcp.json in any project root, or ~/.claude/settings.json globally
{
  "mcpServers": {
    "keroagile": {
      "type": "stdio",
      "command": "/home/matt/.local/bin/KeroAgile",
      "args": ["mcp"]
    }
  }
}
```

**Remote setup** (KeroAgile running on a homelab server, requires Phase 3.3):

```json
{
  "mcpServers": {
    "keroagile": {
      "type": "stdio",
      "command": "/home/matt/.local/bin/KeroAgile",
      "args": ["mcp", "--remote", "http://homelab:7432"],
      "env": { "KEROAGILE_TOKEN": "your-token" }
    }
  }
}
```

Once configured, Claude Code discovers all KeroAgile tools automatically. No per-project setup, no explaining the CLI — plain English works from any repo.

---

## 5. Web interface (v1.0.0)

A browser-based UI that matches the TUI feature set, making KeroAgile accessible without a terminal. The goal is that CLI, TUI, and Web are three equal interfaces to the same data — no feature exclusive to any one.

### 5.1 Architecture

**Prerequisites:** API server (§3.3) must exist first — the web UI is a pure frontend against the REST API.

**Tech stack: React (confirmed)**

React is chosen for consistency with the broader Kero Apps ecosystem, where React is the standard web UX layer across all projects. Shared patterns, components, and developer familiarity outweigh the simplicity advantage of htmx for a long-lived multi-app family.

**React adds zero complexity for end users.** The distinction matters:

- The React app is built once at release time (`npm run build`), producing static files
- Those files are embedded into the Go binary at compile time via `go:embed`
- Users download one binary or run `docker compose up` — no Node.js, no npm, nothing JavaScript-related is visible or required
- Contributors working on the frontend need Node, which is expected

**Tooling decisions:**
- **Build tool:** Vite (fast, minimal config, standard for modern React)
- **Language:** TypeScript (safer for a long-lived codebase)
- **API state:** TanStack Query (React Query) for server state, caching, and background refetch
- **Drag-and-drop:** dnd kit (actively maintained, accessible, works with React 18)
- **Styling:** Tailwind CSS (consistent with self-contained builds, no runtime CSS-in-JS overhead)

**Source layout:**
```
web/                    React app source (Vite project)
  src/
    components/         Board, Sidebar, Detail, TaskForm, etc.
    api/                typed fetch wrappers for each /api/ endpoint
    hooks/              useProjects, useTasks, useSprint, etc.
  dist/                 built output (git-ignored; produced by npm run build)
cmd/keroagile/
  web.go               go:embed web/dist → serves / and /api/
```

**Multi-stage Docker build** (fully transparent to the user):

```dockerfile
# Stage 1 — build React (Node never reaches the final image)
FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2 — build Go binary with embedded frontend
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN go build -o KeroAgile ./cmd/keroagile/

# Stage 3 — minimal runtime (~20MB image, no Node, no npm)
FROM alpine:3.21
COPY --from=builder /app/KeroAgile /usr/local/bin/KeroAgile
ENTRYPOINT ["KeroAgile"]
```

**goreleaser** runs `npm ci && npm run build` as a `before.hooks` step before `go build`, so pre-built binaries on GitHub Releases also have the frontend baked in. Users who install via binary or `go install` get the full web UI with no extra steps.

### 5.2 Feature parity with TUI

| TUI feature | Web equivalent | Notes |
|-------------|---------------|-------|
| Three-panel layout | Responsive three-column layout | Sidebar collapses on mobile |
| Keyboard nav (j/k, tab, n/e/m/d) | `keydown` shortcuts via React hook | Same bindings where sensible |
| Drag-and-drop task move | dnd kit | Easier in React than TUI |
| Task form overlay | Modal dialog | React portal |
| Detail panel | Right column / slide-in drawer | |
| PR auto-transition | Server-Sent Events from API server | Push, not poll |
| Sprint filter toggle | Tab bar or dropdown | |
| "My tasks" filter | Toggle button, resolves via JWT identity | |
| Status bar / notifs | Toast notifications | |
| User display (🤖/👤) | Same prefix in UI | |

### 5.3 Served from the same binary

The `serve` subcommand (§3.3) already handles the API. Extend it to also serve the embedded React app:

```bash
KeroAgile serve --addr :7432   # API at /api/, web UI at /
```

Or a dedicated subcommand for local-only use without the full server auth stack:

```bash
KeroAgile web --addr :7435     # single-user local mode
```

Static assets embedded via `go:embed`. Zero external dependencies at runtime.

### 5.4 Mobile / home network access

With the web interface running on the homelab server, any device on the home network — phone, tablet, laptop — can access the full board at `http://homelab:7432` in a browser. Tailwind's responsive utilities handle the layout shift for narrower screens; the sidebar collapses to a drawer on mobile.

---

## 6. Cross-network sync (v0.4.0) ✓ Shipped

Multi-install sync so a primary KeroAgile server (e.g. homelab) can replicate selected projects to secondary installs (e.g. laptop), with write-through proxy keeping the primary authoritative.

### 6.1 Primary-side sync API ✓

New `/api/sync/*` endpoints on the primary: heartbeat, snapshot, SSE stream, secondary registration, grant management. Secondaries authenticate with long-lived SHA-256 tokens (not user JWTs). Change log (`change_log` table) records every mutation as an append-only event stream that secondaries consume.

### 6.2 Secondary sync daemon ✓

Background goroutine on the secondary that:
- Polls the primary's heartbeat every 15 seconds; transitions `online → reconnecting → offline` after threshold misses
- Opens an SSE stream for each synced project, applying incoming events (task/sprint/user upsert, task delete) to the local SQLite store
- Write-through proxy: mutations to synced projects are forwarded to the primary; returns 503 if primary is offline (read-only degraded mode)

### 6.3 Sync Settings web UI ✓

Primary-mode Sync Settings page (header nav → "Sync"):
- Register secondaries (generates one-time display token)
- Per-secondary project grant management (checkbox list)
- Revoke secondaries

Secondary-mode features wired into the board:
- Sync status indicator in header (green/yellow/red dot)
- "Add Synced Project" modal — provide primary URL, token, project ID to pull initial snapshot
- Frozen project banner when primary revokes a grant

### 6.4 End-to-end integration tests ✓

In-process two-server test cluster (`internal/syncsrv/integration_test.go`):
- `TestBidirectionalSync`: primary→secondary SSE delivery + secondary→primary write-proxy
- `TestOfflineDetection`: heartbeat threshold transitions to `StateOffline`

---

## Summary table

| # | Item | Phase | Effort |
|---|------|-------|--------|
| 1.1 | Fix `go.mod` module path | Immediate | XS |
| 1.2 | Fix TUI sync I/O in Update | Immediate | M |
| 1.3 | Fix drag ghost rendering | Immediate | S |
| 1.4 | Form validation for priority/status | Immediate | S |
| 1.5 | Rune-aware string truncation | Immediate | S |
| 1.6 | Expanded README + ASCII screenshots | Immediate | M |
| 2.1 | Sprint panel in TUI | v0.2.0 | L |
| 2.2 | Interactive blocker management in TUI | v0.2.0 | M |
| 2.3 | Task filter/search in TUI | v0.2.0 | M |
| 2.4 | GitHub Actions CI | v0.2.0 | XS |
| 2.5 | Binary releases via goreleaser | v0.2.0 | S |
| 3.1 | SSH deployment documentation | v0.3.0 | XS |
| 3.2 | Docker container + compose | v0.3.0 | S |
| 3.3 | API server mode + remote client | v0.3.0 | XL |
| 3.4 | TUI-in-browser via ttyd (optional) | v0.3.0 | S |
| 4.1 | MCP server (`KeroAgile mcp`) | v0.2.0 | L |
| 4.2 | Claude Code skills | v0.2.0 | M |
| 4.3 | Claude Code configuration docs | v0.2.0 | S |
| 5.1 | Web UI architecture + API prereq | v1.0.0 | XL |
| 5.2 | Web UI feature parity with TUI | v1.0.0 | XL |
| 5.3 | `KeroAgile web` unified server | v1.0.0 | M |
| 5.4 | Mobile-responsive layout | v1.0.0 | M |
| 6.1 | Primary sync API (SSE stream, change log) | v0.4.0 ✓ | XL |
| 6.2 | Secondary sync daemon + write-through proxy | v0.4.0 ✓ | XL |
| 6.3 | Sync Settings UI + secondary board features | v0.4.0 ✓ | L |
| 6.4 | End-to-end integration tests | v0.4.0 ✓ | M |

**Effort key:** XS < 1 day · S 1–2 days · M 3–5 days · L 1–2 weeks · XL 2–4 weeks
