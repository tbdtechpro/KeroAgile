# KeroAgile

A self-hostable agile board for the terminal. Mirrors the core Jira workflow — projects, sprints, tasks, assignees, blockers — without the browser.

Runs two ways:
- **TUI** — three-panel kanban board (sidebar / board / detail) with keyboard and mouse support
- **CLI** — every operation as a subcommand, with `--json` output for scripting

Data lives in a single SQLite file at `~/.config/keroagile/keroagile.db`. No server, no network, no account.

## Requirements

- Go 1.25+ (for building from source)
- Optional: `git` for branch auto-link, `gh` for PR polling

## Install

### From a release binary (recommended)

Download the archive for your platform from [Releases](https://github.com/tbdtechpro/KeroAgile/releases/latest), extract, and place the binary in your PATH:

```bash
# macOS (Apple Silicon)
curl -L https://github.com/tbdtechpro/KeroAgile/releases/download/v0.2.0/KeroAgile_0.2.0_darwin_arm64.tar.gz | tar xz
sudo mv KeroAgile /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/tbdtechpro/KeroAgile/releases/download/v0.2.0/KeroAgile_0.2.0_darwin_amd64.tar.gz | tar xz
sudo mv KeroAgile /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/tbdtechpro/KeroAgile/releases/download/v0.2.0/KeroAgile_0.2.0_linux_amd64.tar.gz | tar xz
sudo mv KeroAgile /usr/local/bin/

# Linux (arm64)
curl -L https://github.com/tbdtechpro/KeroAgile/releases/download/v0.2.0/KeroAgile_0.2.0_linux_arm64.tar.gz | tar xz
sudo mv KeroAgile /usr/local/bin/
```

Binaries are fully self-contained — no CGo, no system dependencies.

### From source

```bash
git clone https://github.com/tbdtechpro/KeroAgile
cd KeroAgile
make install          # builds and installs to ~/.local/bin/KeroAgile
```

Verify the install:

```bash
KeroAgile --help
```

## Quick start

```bash
# Create a project linked to your git repo's remote URL
KeroAgile project add MYAPP "My App" --repo https://github.com/you/my-app

# Add team members
KeroAgile user add alice "Alice"
KeroAgile user add bot "CI Bot" --agent

# Create tasks
KeroAgile task add "Build login page" --project MYAPP --assignee alice --priority high --points 3
KeroAgile task add "Write tests"      --project MYAPP --assignee alice --priority medium

# Move tasks through the board
KeroAgile task move MYAPP-001 in_progress
KeroAgile task move MYAPP-001 review

# Launch the TUI
KeroAgile
```

The `--repo` flag is the remote URL of your git repo. It is used by the MCP integration to auto-detect which project Claude is working on — set it now to avoid configuring it later.

## TUI keyboard shortcuts

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Cycle panel focus (sidebar → board → detail) |
| `↑` / `↓` or `j` / `k` | Navigate within focused panel |
| `n` | New task form |
| `e` | Edit selected task |
| `m` / `M` | Move task forward / backward one status |
| `d` | Delete selected task |
| `r` | Refresh tasks + git |
| `q` / `ctrl+c` | Quit |

Mouse: click to focus a panel, click-and-drag a task row to move it to a different status column.

## Claude Code integration

KeroAgile works as a native Claude Code tool via MCP. Once configured, Claude can create tasks, query the board, move cards, and link branches in plain English — from any repo you are working in.

### Step 1 — Install the binary

Follow the [Install](#install) section above. Confirm the path:

```bash
which KeroAgile
# e.g. /usr/local/bin/KeroAgile  or  /home/you/.local/bin/KeroAgile
```

### Step 2 — Register the MCP server with Claude Code

Add a `keroagile` entry to your Claude Code settings. There are two places you can put it:

**Global** — works in every repo:

```json
// ~/.claude/settings.json
{
  "mcpServers": {
    "keroagile": {
      "type": "stdio",
      "command": "/usr/local/bin/KeroAgile",
      "args": ["mcp"]
    }
  }
}
```

Replace `/usr/local/bin/KeroAgile` with the actual path from `which KeroAgile`.

**Per-repo** — only active in one project:

```json
// <repo-root>/.mcp.json
{
  "mcpServers": {
    "keroagile": {
      "type": "stdio",
      "command": "/usr/local/bin/KeroAgile",
      "args": ["mcp"]
    }
  }
}
```

Restart Claude Code after editing the settings file.

### Step 3 — Create a KeroAgile project linked to your repo

Claude auto-detects the active project by reading the repo's git remote and matching it against the project's `--repo` URL. Create the project with `--repo` set to the remote URL:

```bash
KeroAgile project add MYAPP "My App" --repo https://github.com/you/my-app
```

To find your repo's remote URL:

```bash
git remote get-url origin
```

### Step 4 — Add yourself as a user (optional but recommended)

```bash
KeroAgile user add you "Your Name"
```

Set yourself as the default assignee in `~/.config/keroagile/config.toml`:

```toml
default_project  = "MYAPP"
default_assignee = "you"
```

### Step 5 — Use it

Open Claude Code in the repo directory and start asking in plain English:

```
"Add a high-priority task: implement OAuth login"
"What's in progress?"
"Move MYAPP-003 to done"
"What's blocking the login task?"
"Show me the current sprint"
```

Claude auto-detects the project from the repo's remote URL — no project ID needed.

### Slash commands

Three slash commands ship with KeroAgile for common workflows. They are available in any repo where the `.claude/commands/` directory is on Claude's load path (either the KeroAgile repo itself, or copied into your project's `.claude/commands/`):

| Command | What it does |
|---------|-------------|
| `/keroagile-update` | Finds the task matching your current branch and moves it to `review` or `done` |
| `/keroagile-standup` | Summarises in-progress and review tasks with assignees and blockers |
| `/keroagile-plan` | Reads the backlog and suggests a sprint composition by priority and points |

To make these commands available in any project, copy them to `~/.claude/commands/`:

```bash
cp /path/to/KeroAgile/.claude/commands/keroagile-*.md ~/.claude/commands/
```

### Available MCP tools

| Tool | Description |
|------|-------------|
| `list_projects` | List all projects |
| `list_tasks` | List tasks, optionally filtered by status or assignee |
| `get_task` | Get full details of one task |
| `create_task` | Create a new task |
| `update_task` | Update a task's fields |
| `move_task` | Move a task to a different status |
| `delete_task` | Delete a task |
| `link_branch` | Link a git branch to a task |
| `list_users` | List all users and agents |
| `get_sprint` | Get the active sprint (or a specific sprint by ID) |
| `add_blocker` | Mark one task as blocking another |
| `remove_blocker` | Remove a blocker relationship |

## CLI reference

```
KeroAgile project add <id> <name> [--repo <remote-url>]
KeroAgile project list
KeroAgile project set-sprint <project-id> on|off

KeroAgile user add <id> <name> [--agent]
KeroAgile user list

KeroAgile task add <title> --project <id> [--assignee <id>] [--priority low|medium|high|critical]
                            [--status backlog|todo|in_progress|review|done] [--points N]
                            [--labels a,b,c] [--description "..."]
KeroAgile task list [--project <id>] [--status <s>] [--assignee <id>]
KeroAgile task get <task-id>
KeroAgile task move <task-id> <status>
KeroAgile task link-branch <task-id> <branch>
KeroAgile task link-pr <task-id> <pr-number>
KeroAgile task delete <task-id>

KeroAgile sprint add <name> --project <id> [--start YYYY-MM-DD] [--end YYYY-MM-DD]
KeroAgile sprint list --project <id>
KeroAgile sprint activate <sprint-id>
KeroAgile sprint assign <task-id> <sprint-id>

KeroAgile mcp                              # start MCP server (Claude Code integration)
```

Every command accepts `--json` for structured output. When stdout is not a TTY (e.g. piped to `jq`), JSON is emitted automatically.

```bash
KeroAgile task list --project MYAPP --json | jq '.[].title'
```

## Status values

`backlog` → `todo` → `in_progress` → `review` → `done`

## Priority values

`low` · `medium` · `high` · `critical`

## Configuration

Config file: `~/.config/keroagile/config.toml`

```toml
default_project  = "MYAPP"
default_assignee = "alice"
```

Setting `default_project` lets you omit `--project` on most task commands.

## PR auto-transition

If a task is in `review` status and has a linked PR number, KeroAgile polls GitHub every 60 seconds in the TUI. When the PR merges, the task automatically moves to `done`.

```bash
KeroAgile task link-pr MYAPP-001 42
```

## Troubleshooting

**Auto-detection fails / "project_id required" error** — the project must have a `--repo` URL set that matches the repo's git remote. Check with:
```bash
git remote get-url origin
KeroAgile project list --json | jq '.[] | {id, repo_path}'
```
The `repo_path` must be a substring of the remote URL. Re-create the project with `--repo` if needed.

**MCP server not showing up in Claude Code** — restart Claude Code after editing `settings.json`. Confirm the `command` path is correct with `which KeroAgile`. The MCP server writes nothing to stdout at startup; it only responds when Claude sends it a request.

**`gh: command not found`** — PR polling is disabled; tasks still work normally. Install the [GitHub CLI](https://cli.github.com) to enable PR auto-transition.

**`database is locked`** — only one KeroAgile process should write at a time. If a TUI session crashed, close any lingering processes and retry.

**Terminal too narrow** — the TUI needs at least 80 columns and 24 rows. Resize the window or reduce font size.

**Task IDs are wrong** — task IDs are `<project-id>-<seq>` (e.g. `MYAPP-001`). Project ID is case-sensitive.

## Architecture

```
internal/
  domain/     pure domain types + service (no I/O)
  store/      SQLite via modernc.org/sqlite (pure Go, no CGo)
  config/     TOML config load/save
  git/        git + gh CLI wrappers
  mcp/        MCP server (JSON-RPC 2.0 over stdio)
  tui/        BubbleTea TUI (app, sidebar, board, detail, forms)
cmd/keroagile/  Cobra CLI entry point
.claude/commands/  Claude Code slash commands
```

## Development

```bash
make test     # go test ./...
make build    # go build -o KeroAgile ./cmd/keroagile/
make install  # installs to ~/.local/bin/KeroAgile
```
