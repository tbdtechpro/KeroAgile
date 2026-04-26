# KeroAgile

A self-hostable agile board for your terminal and browser, with first-class Claude Code integration.

**License:** [BWL | AI-C2S | AI-T0](LICENSE) — free for workers and individuals; corporations require permission; the code may not be used to train AI models.

![KeroAgile board overview](docs/gifs/board-overview.gif)

KeroAgile gives you Jira-style project tracking (projects, sprints, tasks, assignees, blockers, PR auto-close) with zero cloud account and zero vendor lock-in. Everything stores in a single SQLite file. Run the keyboard-first TUI in your terminal, or open the React web UI in any browser on your network. The MCP integration means Claude Code can read your board, create tasks, move cards, and link branches in plain English — from inside any repo you're working in.

---

> **Early release** — KeroAgile works well for personal projects and solo/small-team use. The core board, CLI, MCP server, API, and web UI are solid; test coverage is present but not exhaustive. Feedback and issues welcome.

---

## Why KeroAgile?

Most project management tools are designed for teams of dozens, cost money, and live in a browser tab you forget to update. KeroAgile is the opposite: it runs where you already work, stores data locally, and plugs directly into your AI coding workflow.

The MCP integration is the part that makes it genuinely different. When you add KeroAgile to Claude Code as an MCP server, Claude can:

- **Create tasks while it codes** — "I'm implementing OAuth, I'll create a task for the follow-up work" happens automatically
- **Route tasks to the right assignee** — coding tasks go to Claude, design/research tasks go to you, based on title keywords
- **Import entire project plans in one shot** — `/keroagile-import` reads a markdown plan file and turns it into a sprint-organised board with one command
- **Run your standup** — `/keroagile-standup` summarises what's in progress, what's blocked, and what's ready for review
- **Plan your next sprint** — `/keroagile-plan` reads the backlog and proposes a sprint composition by priority and points

---

## Features

- **Terminal TUI** — three-panel layout (sidebar, kanban board, task detail); keyboard + mouse; drag-and-drop between columns
- **React web UI** — the same board in any browser; drag-and-drop, task modal, detail drawer, sprint sidebar, mobile-responsive
- **Sprints** — create sprint phases, filter the board to a sprint, assign tasks, track per-sprint progress
- **Blockers** — mark tasks as blocking/blocked-by across projects; blocked tasks shown with ⚠; cross-project blockers display with project prefix and ↗ indicator in both TUI and web UI
- **PR auto-close** — link a PR number to a task; when it merges, the task moves to `done` automatically
- **Claude Code MCP** — 15 MCP tools covering every board operation; auto-detects project from git remote
- **Smart assignee** — infers the right assignee from title keywords (no need to specify every time)
- **REST API** — JWT-authenticated HTTP API on port 7432; powers the web UI and enables scripting
- **CLI + JSON** — every operation works as a subcommand; pipe to `jq` when stdout isn't a TTY
- **Zero dependencies** — pure Go, pure-Go SQLite (no CGo, no system libraries)

---

## Requirements

- Go 1.21+ (to build from source)
- Node.js 18+ (to build the web UI from source; not needed for release binaries)
- Optional: `git` for branch auto-link, `gh` for PR polling

## Install

### From source

```bash
git clone https://github.com/tbdtechpro/KeroAgile
cd KeroAgile
make build-full   # builds React UI, embeds it, then builds the Go binary
make install      # installs to ~/.local/bin/KeroAgile
```

### From a release binary

Download from [Releases](https://github.com/tbdtechpro/KeroAgile/releases/latest):

```bash
# macOS (Apple Silicon)
curl -L https://github.com/tbdtechpro/KeroAgile/releases/download/v0.2.0/KeroAgile_0.2.0_darwin_arm64.tar.gz | tar xz
sudo mv KeroAgile /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/tbdtechpro/KeroAgile/releases/download/v0.2.0/KeroAgile_0.2.0_linux_amd64.tar.gz | tar xz
sudo mv KeroAgile /usr/local/bin/
```

Binaries are fully self-contained — the React web UI is embedded in the binary; no Node.js, no CGo, no system dependencies.

---

## Quick start

```bash
# First-time setup — creates your user, optionally adds Claude as an agent, writes config
KeroAgile init

# Create a project linked to your git repo
KeroAgile project add MYAPP "My App" --repo https://github.com/you/my-app

# Add tasks — assignee is inferred automatically from the title
KeroAgile task add "Implement OAuth login"   --project MYAPP --priority high --points 3
KeroAgile task add "Design onboarding flow"  --project MYAPP --priority medium

# Open the terminal board
KeroAgile

# — or — open the web UI in a browser
KeroAgile user set-password alice    # set a login password first
KeroAgile serve                      # starts API + web UI on :7432
# then open http://localhost:7432
```

---

## TUI

![Creating a new task](docs/gifs/new-task.gif)

The board has three panels: projects/sprints on the left, kanban columns in the middle, task detail on the right. Tab cycles focus between them.

### Keyboard shortcuts

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Cycle panel focus |
| `↑` / `↓` or `j` / `k` | Navigate |
| `enter` | Open sprint list for selected project |
| `esc` | Return to project list / close filter |
| `n` | New task (or new sprint in sprint list) |
| `e` | Edit selected task |
| `m` / `M` | Move task forward / backward one status |
| `s` | Assign selected task to the active sprint filter |
| `S` | Remove selected task from its sprint |
| `b` | Open blocker input — type a task ID to mark this task as blocked by it |
| `→` | In task detail: jump to the first blocker task |
| `/` | Open filter bar — filter by title, status, priority, assignee, or label |
| `d` | Delete selected task |
| `r` | Refresh tasks + git |
| `q` / `ctrl+c` | Quit |

Mouse: click to focus a panel, click-and-drag a task row to drop it into a different status column.

### Sprint workflow

![Sprint filtering](docs/gifs/sprint-filter.gif)

Press `enter` on a project in the sidebar to open its sprint list. Select a sprint and press `enter` — the board filters to that sprint and the header shows the sprint name and date range. Press `s` on any task to pull it into the selected sprint; `S` removes it. Press `n` in the sprint list view to create a new sprint from the TUI.

### Blocker workflow

Press `b` on a selected task to open the blocker input. Type the blocking task's ID and press `enter`. The blocked task shows a red `⚠` prefix in the board. In the detail panel, press `→` to jump directly to the blocker task.

When editing a task (`e`), tab to the **Blocks** or **Blocked by** field and press `enter` to open the fuzzy-search picker — type any part of a task ID or title and the picker filters across all projects live. Cross-project blockers are shown with a `[PROJECT]` prefix in the detail panel.

---

## Web UI

`KeroAgile serve` embeds a full React kanban board at `http://localhost:7432`. Open it in any browser on your network — phones, tablets, and desktops all work.

### Starting

```bash
# Set a password for your user (required for web login)
KeroAgile user set-password alice

# Add a JWT signing secret to config so tokens survive restarts
# ~/.config/keroagile/config.toml:
#   api_secret = "your-random-secret-here"

KeroAgile serve                         # listens on :7432
KeroAgile serve --addr 0.0.0.0:7432     # bind all interfaces for LAN access
```

Then open `http://localhost:7432` in any browser and log in with your user ID and password.

### Features

- **Kanban board** — same five columns as the TUI; drag tasks between columns (8px drag threshold prevents accidental moves)
- **Task modal** — create and edit tasks from the board; title, description, priority, status, assignee, story points, labels, sprint; live blocker autocomplete searches across all projects with cross-project chips (blue ↗ for other projects)
- **Task detail drawer** — click any task card to open its full details on the right; blocked tasks shown with ⚠; two-click delete to prevent accidents
- **Sprint sidebar** — filter by sprint, "no sprint", or all tasks; "My tasks" toggle filters to the logged-in user
- **Toast notifications** — success/error feedback on all mutations (4-second auto-dismiss, click to dismiss early)
- **Mobile-responsive** — sidebar collapses to a hamburger drawer on narrow screens; task detail becomes a full-screen overlay; works on any device accessing the board over your home network

---

## Claude Code integration

KeroAgile runs as a native Claude Code MCP server. Once registered, Claude can manage your board in plain English from any repo it's working in.

### Setup

**1. Register the MCP server** (global — works in every repo):

```bash
claude mcp add --scope user keroagile $(which KeroAgile) mcp
```

**2. Link your project to its git repo:**

```bash
KeroAgile project add MYAPP "My App" --repo https://github.com/you/my-app
```

Claude reads the repo's git remote and matches it to this URL — no project ID needed in prompts.

**3. Run first-time setup** (if you haven't already):

```bash
KeroAgile init
```

This creates your user, optionally adds Claude as an agent user, and writes `~/.config/keroagile/config.toml`.

**4. Use it:**

```
"What's in my backlog?"
"Add a high-priority task: rate limiting on the API"
"What's blocking the auth task?"
"Move MYAPP-003 to review"
"Show me the current sprint"
```

### Smart assignee

When `assignee_id` is omitted, `create_task` infers the right person from the title:

| Title keywords | Assigned to |
|----------------|-------------|
| `implement`, `build`, `fix`, `refactor`, `migrate`, `develop`, … | First agent user (Claude) |
| `design`, `plan`, `research`, `qa`, `review`, `document`, … | `default_assignee` from config |
| No match | `default_assignee` |

Claude never needs to reason about routing — the server handles it.

### Slash commands

Copy these to `~/.claude/commands/` to make them available in any project:

```bash
cp /path/to/KeroAgile/.claude/commands/keroagile-*.md ~/.claude/commands/
```

| Command | What it does |
|---------|-------------|
| `/keroagile-import` | Reads a plan/spec markdown file and creates the project, sprints, and tasks — one command to import a full roadmap |
| `/keroagile-plan` | Reads the backlog and proposes a sprint by priority and points |
| `/keroagile-standup` | Summarises in-progress and review tasks with assignees and blockers |
| `/keroagile-update` | Finds the task matching your current branch and moves it to `review` or `done` |

### MCP tools

| Tool | Description |
|------|-------------|
| `list_projects` | List all projects |
| `create_project` | Create a new project |
| `list_tasks` | List tasks, filtered by status or assignee |
| `get_task` | Get full task details including blockers and PR info |
| `create_task` | Create a task (auto-detects project; auto-suggests assignee; accepts `sprint_id`) |
| `update_task` | Update task fields |
| `move_task` | Move a task to a different status |
| `delete_task` | Delete a task |
| `link_branch` | Link a git branch to a task |
| `list_users` | List all users and agents |
| `get_sprint` | Get the active sprint or a specific sprint by ID |
| `create_sprint` | Create a new sprint |
| `assign_task_sprint` | Assign a task to a sprint or remove it |
| `add_blocker` | Mark one task as blocking another |
| `remove_blocker` | Remove a blocker relationship |

---

## CLI reference

```
KeroAgile init

KeroAgile project add <id> <name> [--repo <remote-url>]
KeroAgile project list
KeroAgile project set-sprint <project-id> on|off

KeroAgile user add <id> <name> [--agent]
KeroAgile user list
KeroAgile user set-password <user-id>

KeroAgile task add <title> --project <id> [--assignee <id>] [--priority low|medium|high|critical]
                            [--status backlog|todo|in_progress|review|done] [--points N]
                            [--labels a,b,c] [--description "..."]
KeroAgile task list [--project <id>] [--status <s>] [--assignee <id>]
KeroAgile task get <task-id>
KeroAgile task move <task-id> <status>
KeroAgile task link-branch <task-id> <branch>
KeroAgile task link-pr <task-id> <pr-number>
KeroAgile task block <task-id> <blocker-id>    # mark task as blocked by blocker (cross-project IDs work)
KeroAgile task unblock <task-id> <blocker-id>  # remove that blocker relationship
KeroAgile task delete <task-id>

KeroAgile sprint add <name> --project <id> [--start YYYY-MM-DD] [--end YYYY-MM-DD]
KeroAgile sprint list --project <id>
KeroAgile sprint activate <sprint-id>
KeroAgile sprint assign <task-id> <sprint-id>

KeroAgile serve [--addr :7432]    # REST API + embedded React web UI
KeroAgile mcp                     # MCP server (stdio, for Claude Code)
```

Every command accepts `--json`. When stdout is not a TTY (piped), JSON is emitted automatically:

```bash
KeroAgile task list --project MYAPP | jq '.[].title'
```

---

## Status values

`backlog` → `todo` → `in_progress` → `review` → `done`

## Priority values

`low` · `medium` · `high` · `critical`

## Configuration

`~/.config/keroagile/config.toml`:

```toml
default_project  = "MYAPP"
default_assignee = "alice"
api_secret       = "your-random-secret-here"   # JWT signing key; tokens expire on restart if unset
```

---

## Deployment

KeroAgile is a single static binary with no dependencies. The React web UI is embedded inside it — no separate web server needed.

### SSH + tmux (terminal board on a shared server)

Install the binary on any Linux server, then open the board from your laptop with a single command:

```bash
# ~/.ssh/config
Host keroagile
  HostName myserver.local
  User alice
  RequestTTY yes
  RemoteCommand tmux new-session -A -s keroagile 'KeroAgile'
```

```bash
ssh keroagile          # opens (or reattaches to) the board session
```

`tmux new-session -A` attaches to an existing session if one is running, or creates a new one. Your board stays alive between SSH sessions — close the terminal and re-connect where you left off.

For screen users:

```bash
RemoteCommand screen -DR keroagile KeroAgile
```

### One-shot SSH alias (read-only friendly)

```bash
# ~/.bashrc / ~/.zshrc
alias ka='ssh myserver.local KeroAgile'
alias ka-tasks='ssh myserver.local KeroAgile task list --project MYAPP'
```

### Web UI on your home network

Run the server on any machine on your LAN — phones, tablets, and laptops can all open the board in a browser:

```bash
KeroAgile serve --addr 0.0.0.0:7432
# then open http://<server-ip>:7432 on any device
```

### Database location

The SQLite database lives at `~/.config/keroagile/keroagile.db` on whichever machine runs the binary. The `KEROAGILE_DATA_DIR` environment variable overrides this path — useful for Docker volumes or CI:

```bash
KEROAGILE_DATA_DIR=/mnt/shared KeroAgile task list
```

### Docker

A `Dockerfile` and `docker-compose.yml` are included. The React web UI and API run together in a single container; the database is on a named volume.

```bash
# Build the image (embeds the React UI)
docker build -t keroagile .

# Start the web UI + API
docker compose up -d

# CLI commands against the containerised DB
docker compose run --rm keroagile task list --project MYAPP
docker compose run --rm keroagile task add "Fix login bug" --project MYAPP
```

To run the web UI (rather than the default MCP mode), override the command:

```yaml
# docker-compose.yml snippet
services:
  keroagile:
    image: keroagile:latest
    command: ["serve", "--addr", ":7432"]
    environment:
      KEROAGILE_DATA_DIR: /data
    ports:
      - "7432:7432"
    volumes:
      - keroagile-data:/data
```

**TUI in the browser** — `Dockerfile.ttyd` wraps the TUI with [ttyd](https://github.com/tsl0922/ttyd), exposing a fully interactive terminal at `http://homelab:7433`. No JavaScript, no frontend:

```bash
# Start the browser-accessible TUI (alongside the main service)
docker compose --profile browser up keroagile-browser
# then open http://localhost:7433
```

Both services share the same `keroagile-data` volume.

---

## PR auto-transition

Link a PR number to a task; KeroAgile polls GitHub every 60 seconds while the TUI is open. When the PR merges, the task moves to `done` automatically.

```bash
KeroAgile task link-pr MYAPP-001 42
```

---

## Troubleshooting

**Auto-detection fails / "project_id required"** — the project's `--repo` URL must match the repo's git remote. Check with:
```bash
git remote get-url origin
KeroAgile project list --json | jq '.[] | {id, repo_path}'
```

**MCP not showing up** — run `claude mcp list`. Confirm the binary path with `which KeroAgile`.

**`gh: command not found`** — PR polling is disabled. Install the [GitHub CLI](https://cli.github.com) to enable PR auto-transition.

**`database is locked`** — only one KeroAgile process should write at a time. Close any stale TUI sessions.

**Terminal too narrow** — the TUI needs at least 80 columns and 24 rows.

**Web UI login fails** — ensure you've set a password with `KeroAgile user set-password <user-id>` and that `api_secret` is set in `config.toml` (if unset, tokens expire when the server restarts).

---

## REST API

The `KeroAgile serve` command exposes a JSON REST API at `/api/` alongside the web UI. Useful for scripting or building integrations.

### Authentication

```bash
# Login to get a JWT token (valid 24h)
curl -s -X POST http://localhost:7432/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"user_id":"alice","password":"mypassword"}' | jq -r .token

# Use the token in subsequent requests
TOKEN=<paste-token>
curl -s http://localhost:7432/api/tasks?project_id=MYAPP \
  -H "Authorization: Bearer $TOKEN" | jq .
```

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/auth/login` | Login → JWT token |
| `GET` | `/api/projects` | List projects |
| `POST` | `/api/projects` | Create project |
| `GET` | `/api/tasks` | List tasks (`?project_id=`, `?status=`, `?assignee_id=`, `?sprint_id=`) |
| `POST` | `/api/tasks` | Create task |
| `GET` | `/api/tasks/{id}` | Get task |
| `PATCH` | `/api/tasks/{id}` | Update / move task |
| `DELETE` | `/api/tasks/{id}` | Delete task |
| `GET` | `/api/users` | List users |
| `GET` | `/api/sprints` | List sprints (`?project_id=`) |
| `POST` | `/api/sprints` | Create sprint |
| `GET` | `/api/sprints/{id}` | Get sprint |

---

## Architecture

```
internal/
  domain/     pure domain types + service (no I/O)
  store/      SQLite via modernc.org/sqlite (pure Go, no CGo)
  config/     TOML config load/save
  git/        git + gh CLI wrappers
  mcp/        MCP server (JSON-RPC 2.0 over stdio)
  api/        REST API server (JWT auth, net/http ServeMux)
  web/        embedded React build (go:embed all:dist)
  tui/        BubbleTea TUI (app, sidebar, board, detail, forms)
cmd/keroagile/        Cobra CLI entry point
web/                  React source (Vite + TypeScript + TanStack Query + dnd-kit + Tailwind)
.claude/commands/     Claude Code slash commands
```

## Development

```bash
make test        # go test ./...
make build       # go build -o KeroAgile (uses existing embedded UI if present)
make build-web   # npm run build + copy dist into internal/web/dist
make build-full  # build-web then go build (use this after changing web/ sources)
make install     # installs to ~/.local/bin/KeroAgile
go vet ./...
```

The React source lives in `web/`. `make build-full` compiles it and embeds the output into the Go binary via `//go:embed all:dist` in `internal/web/static.go`. Release binaries include the pre-built UI — Node.js is only needed when modifying the frontend.
