# KeroAgile

A self-hostable agile board for the terminal. Mirrors the core Jira workflow — projects, sprints, tasks, assignees, blockers — without the browser.

Runs two ways:
- **TUI** — three-panel kanban board (sidebar / board / detail) with keyboard and mouse support
- **CLI** — every operation as a subcommand, with `--json` output for scripting

Data lives in a single SQLite file at `~/.config/keroagile/keroagile.db`. No server, no network, no account.

## Requirements

- Go 1.21+
- Optional: `git` for branch auto-link, `gh` for PR polling

## Install

```bash
git clone https://github.com/tbdtechpro/KeroAgile
cd KeroAgile
make install          # installs to ~/.local/bin/KeroAgile
```

Or just build locally:

```bash
make build            # produces ./KeroAgile
```

## Quick start

```bash
# Create a project (optionally linked to a git repo)
KeroAgile project add MYAPP "My App" --repo /path/to/repo

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

## CLI reference

```
KeroAgile project add <id> <name> [--repo <path>]
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

## PR auto-transition

If a task is in `review` status and has a linked PR number, KeroAgile polls GitHub every 60 seconds in the TUI. When the PR merges, the task automatically moves to `done`.

```bash
KeroAgile task link-pr MYAPP-001 42
```

## Configuration

Config file: `~/.config/keroagile/config.toml`

```toml
default_project  = "MYAPP"
default_assignee = "alice"
```

Setting `default_project` means you can omit `--project` on most task commands.

## Claude Code integration

KeroAgile works as a native Claude Code tool via MCP. Once installed, you can manage tasks in plain English from any repo — no CLI commands required.

One-time setup:

```json
// ~/.claude/settings.json  (global — works in every repo)
{
  "mcpServers": {
    "keroagile": {
      "type": "stdio",
      "command": "/home/you/.local/bin/KeroAgile",
      "args": ["mcp"]
    }
  }
}
```

After that, Claude Code auto-detects the active KeroAgile project from the git remote URL of whichever repo you are working in. No project argument needed.

```
"Add a high-priority task: implement OAuth login"
"What's in progress on this project?"
"Mark KA-007 as done and link PR #42"
```

For this to work, the project must have been created with `--repo` pointing at the repo's remote URL:

```bash
KeroAgile project add MYAPP "My App" --repo https://github.com/you/my-app
```

## Troubleshooting

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
  tui/        BubbleTea TUI (app, sidebar, board, detail, forms)
cmd/keroagile/  Cobra CLI entry point
```

## Development

```bash
make test     # go test ./...
make build    # go build -o KeroAgile ./cmd/keroagile/
```
