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

Adds an optional HTTP API server that exposes all domain service operations over a local REST API. This is the foundation for network access from other machines and for the eventual web interface.

**New binary target** (or `--serve` flag on the existing binary): `KeroAgile serve --addr :7432`

API surface mirrors the CLI:
```
GET    /api/projects
POST   /api/projects
GET    /api/projects/:id
GET    /api/projects/:id/tasks
POST   /api/projects/:id/tasks
GET    /api/tasks/:id
PATCH  /api/tasks/:id
DELETE /api/tasks/:id
POST   /api/tasks/:id/move
POST   /api/tasks/:id/link-branch
POST   /api/tasks/:id/link-pr
GET    /api/users
POST   /api/users
GET    /api/sprints?project=:id
POST   /api/sprints
POST   /api/sprints/:id/activate
```

All responses use the existing JSON serialisation (snake_case tags already in place).

**Remote client mode:** The CLI and TUI gain a `--remote http://homelab:7432` flag (or `remote_url` in config). When set, all store calls are HTTP requests to the API server instead of local SQLite. The domain layer stays unchanged — a new `internal/store/remote/client.go` implementing `domain.Store` over HTTP.

This enables the full multi-machine workflow: server runs `KeroAgile serve`, any machine on the network runs `KeroAgile --remote http://homelab:7432` to get the full TUI or CLI against the shared board.

**Auth:** For homelab use, simple shared token via `Authorization: Bearer <token>` header is sufficient. Token set in server config.

**docker-compose with server mode:**
```yaml
services:
  keroagile:
    build: .
    command: ["KeroAgile", "serve", "--addr", "0.0.0.0:7432"]
    ports:
      - "7432:7432"
    volumes:
      - keroagile-data:/data
```

### 3.4 Mode D — TUI in browser via terminal proxy (optional convenience)

For users who want the TUI accessible from a browser without SSH, wrap the TUI with `ttyd` or `gotty` in the Docker image:

```dockerfile
# Optional browser-TUI mode
CMD ["ttyd", "-p", "7433", "KeroAgile"]
```

Exposes the full interactive TUI at `http://homelab:7433` in any browser. Zero JavaScript, zero frontend code — the TUI renders exactly as in a local terminal. This is a convenience layer, not a first-class feature.

---

## 4. Claude Code integration (v0.4.0)

KeroAgile should integrate natively with Claude Code so that an AI agent working in a repo can read and update the board without leaving its coding context.

### 4.1 MCP server

Implement KeroAgile as an MCP (Model Context Protocol) server. When running, Claude Code can call KeroAgile tools directly:

Proposed tools:
- `keroagile_list_tasks` — list tasks for a project with optional filters
- `keroagile_get_task` — get full task detail including blockers and git context
- `keroagile_create_task` — create a task from a title/description
- `keroagile_move_task` — advance task status
- `keroagile_link_branch` — link current git branch to a task
- `keroagile_add_blocker` — mark one task as blocking another
- `keroagile_get_sprint` — get active sprint and its tasks

The MCP server wraps `domain.Service` directly (same binary, `KeroAgile mcp` subcommand). If the API server (Mode C) is running, the MCP server can optionally proxy to it instead of hitting SQLite directly.

**MCP server mode:**
```bash
KeroAgile mcp          # stdio transport (for Claude Code local config)
KeroAgile mcp --http :7434  # HTTP transport (for remote Claude Code instances)
```

### 4.2 Claude Code plugin / skill

A superpowers skill (or Claude Code slash command plugin) that teaches Claude how to use KeroAgile in a project context:

- **Skill: `keroagile-update`** — on completing a task, finds the matching KeroAgile task by branch name, moves it to `done`, and links the PR
- **Skill: `keroagile-standup`** — summarises in-progress tasks for the current project with assignees and blockers
- **Skill: `keroagile-plan`** — reads the backlog and suggests a sprint composition based on priorities and points

### 4.3 Configuration documentation

Once MCP support ships, add a dedicated **Claude Code Integration** section to the README:

```markdown
## Claude Code integration

### Local setup (stdio MCP)

Add to your Claude Code config (~/.claude/settings.json or project .claude/settings.json):

\`\`\`json
{
  "mcpServers": {
    "keroagile": {
      "command": "KeroAgile",
      "args": ["mcp"],
      "env": {}
    }
  }
}
\`\`\`

### Remote setup (API server + HTTP MCP)

If KeroAgile runs on a homelab server:

\`\`\`json
{
  "mcpServers": {
    "keroagile": {
      "command": "KeroAgile",
      "args": ["mcp", "--remote", "http://homelab:7432"],
      "env": { "KEROAGILE_TOKEN": "your-token" }
    }
  }
}
\`\`\`
```

*(Configuration syntax is a placeholder — update when MCP server is implemented and Claude Code plugin API is finalised.)*

---

## 5. Web interface (v1.0.0)

A browser-based UI that matches the TUI feature set, making KeroAgile accessible without a terminal. The goal is that CLI, TUI, and Web are three equal interfaces to the same data — no feature exclusive to any one.

### 5.1 Architecture

Prerequisites: API server (§3.3) must exist first — the web UI is a pure frontend against the REST API.

**Tech choices to decide:**
- **Vanilla HTML + htmx** — minimal JS, server-rendered fragments, pairs well with Go templates. Fastest to ship, lowest maintenance. Recommended for v1.
- **React/Vue SPA** — more interactive but adds a full frontend build pipeline.
- **Templ + Go** — type-safe Go HTML templates, stays in the Go ecosystem.

Recommended path: Go templates + htmx for v1, migrate to Templ as a refactor.

### 5.2 Feature parity with TUI

| TUI feature | Web equivalent |
|-------------|---------------|
| Three-panel layout | Responsive three-column layout (sidebar collapses on mobile) |
| Keyboard nav | Keyboard shortcuts via JS `keydown` listeners |
| Drag-and-drop task move | HTML5 drag-and-drop or a library (SortableJS) |
| Task form overlay | Modal dialog |
| Detail panel | Slide-in drawer or right column |
| PR auto-transition | Server-Sent Events or WebSocket push from server |
| Sprint filter | Dropdown or tab bar |
| Status bar / notifs | Toast notifications |

### 5.3 Served from the same binary

Add a `web` subcommand that serves both the API and the web UI from one process:

```bash
KeroAgile web --addr :7435   # serves UI at / and API at /api/
```

Static assets embedded via `go:embed`. No separate frontend build step in the binary — assets are pre-built and committed, or built as part of the goreleaser pipeline.

### 5.4 Mobile / home network access

With the web interface running on a homelab server, any device on the home network (phone, tablet, laptop) can access the board at `http://homelab:7435` in a browser. Responsive layout required.

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
| 4.1 | MCP server (`KeroAgile mcp`) | v0.4.0 | L |
| 4.2 | Claude Code plugin / skill | v0.4.0 | M |
| 4.3 | Claude Code configuration docs | v0.4.0 | S |
| 5.1 | Web UI architecture + API prereq | v1.0.0 | XL |
| 5.2 | Web UI feature parity with TUI | v1.0.0 | XL |
| 5.3 | `KeroAgile web` unified server | v1.0.0 | M |
| 5.4 | Mobile-responsive layout | v1.0.0 | M |

**Effort key:** XS < 1 day · S 1–2 days · M 3–5 days · L 1–2 weeks · XL 2–4 weeks
