# KeroAgile — Design Spec

**Date:** 2026-04-22
**Status:** Approved

---

## Overview

KeroAgile is a self-hostable agile board that runs in the Linux terminal. It mirrors core Jira functionality — task tracking, sprint management, git integration — with a first-class Claude Code integration so Claude can create, assign, and update tasks autonomously. Built with Go, BubbleTea, and Lipgloss.

---

## Goals

- Full-featured Kanban/Sprint board operable entirely from the terminal
- Bright, polished TUI with keyboard-only and keyboard+mouse (including drag-and-drop) support
- Machine-readable CLI mode (`--json`) for Claude Code automation
- Per-project git integration: branch/PR linking, auto-task-transition on merge
- Multi-project support with a single central SQLite store
- Small team use (named identities, no auth/passwords)

---

## Workflow Style

**Hybrid: Kanban by default, optional Sprints per project.**

- Every project starts in Kanban mode (continuous flow)
- Sprint mode can be enabled per-project (`KeroAgile project set-sprint <id> on`)
- Story points exist at the task level regardless of sprint mode — enabling sprints later requires no back-filling

**Task status machine:**
```
backlog → todo → in_progress → review → done
                  ↑_________________________| (re-open from review)
backlog ←──────────────────────────────────── (re-open from any)
```

Auto-transition: when a linked PR is merged, task moves `review → done` automatically.

**Blocker enforcement:** Advisory only. A task with open blockers can still be moved to any status — the detail panel displays a warning (`⚠ blocked by KA-005`) but does not prevent the move. This keeps the tool flexible for real-world async work.

---

## Architecture

**Layered Go architecture — Option B (approved):**

```
KeroAgile/
├── cmd/keroagile/
│   └── main.go              # Cobra root command; launches TUI when no subcommand
├── internal/
│   ├── domain/              # Pure business logic, zero I/O
│   │   ├── task.go          # Task model, status machine, validation
│   │   ├── project.go       # Project model, sprint model
│   │   ├── user.go          # User/identity model
│   │   └── service.go       # Business rules (move task, link branch, etc.)
│   ├── store/               # SQLite persistence via database/sql
│   │   ├── db.go            # Connection, migrations
│   │   ├── task_store.go
│   │   └── project_store.go
│   ├── git/                 # Git/GitHub integration
│   │   ├── repo.go          # Branch detection, commit log (exec git)
│   │   └── github.go        # gh CLI wrapper — PR status, auto-transition
│   ├── tui/                 # BubbleTea application
│   │   ├── app.go           # Root model, key/mouse dispatch, panel focus
│   │   ├── sidebar.go       # Project tree panel
│   │   ├── board.go         # Task list panel (all statuses, vertical)
│   │   ├── detail.go        # Task detail + git info panel
│   │   ├── forms/           # New/edit task form overlay
│   │   └── styles/          # Lipgloss palette and shared styles
│   └── config/              # User identities, defaults (~/.config/keroagile/)
├── go.mod
└── Makefile
```

**Key conventions:**
- `domain` has zero imports from `store`, `tui`, or `git` — pure models and rules
- `store` implements interfaces defined in `domain` (dependency inversion)
- `tui` calls `domain.Service` only — never touches SQL directly
- CLI commands in `cmd/keroagile/` call the same `domain.Service`
- `git` package wraps `exec.Command("git", ...)` and `exec.Command("gh", ...)` — no git library dependency

**Binary name:** The Makefile produces a binary named `KeroAgile`:
```makefile
build:
    go build -o KeroAgile ./cmd/keroagile/

install:
    go build -o $(HOME)/.local/bin/KeroAgile ./cmd/keroagile/
```

---

## Data Model

**Storage:** Single SQLite file at `~/.config/keroagile/keroagile.db`

```sql
CREATE TABLE projects (
    id          TEXT PRIMARY KEY,           -- e.g. "KA"
    name        TEXT NOT NULL,
    repo_path   TEXT,                       -- absolute path to git repo
    sprint_mode INTEGER NOT NULL DEFAULT 0  -- 0=kanban, 1=sprint-enabled
);

CREATE TABLE users (
    id           TEXT PRIMARY KEY,          -- e.g. "matt", "claude"
    display_name TEXT NOT NULL,
    is_agent     INTEGER NOT NULL DEFAULT 0 -- 1 = AI agent (renders 🤖)
);

CREATE TABLE sprints (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id),
    name       TEXT NOT NULL,
    start_date TEXT,
    end_date   TEXT,
    status     TEXT NOT NULL DEFAULT 'planning' -- planning|active|completed
);

CREATE TABLE tasks (
    id          TEXT PRIMARY KEY,           -- e.g. "KA-001"
    project_id  TEXT NOT NULL REFERENCES projects(id),
    sprint_id   INTEGER REFERENCES sprints(id), -- NULL = backlog
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'backlog',
    -- backlog | todo | in_progress | review | done
    priority    TEXT NOT NULL DEFAULT 'medium',
    -- low | medium | high | critical
    points      INTEGER,                    -- story points, nullable
    assignee_id TEXT REFERENCES users(id),
    branch      TEXT,                       -- linked git branch name
    pr_number   INTEGER,                    -- linked GitHub PR number
    pr_merged   INTEGER NOT NULL DEFAULT 0,
    labels      TEXT NOT NULL DEFAULT '',   -- comma-separated
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE task_deps (
    blocker_id TEXT NOT NULL REFERENCES tasks(id),
    blocked_id TEXT NOT NULL REFERENCES tasks(id),
    PRIMARY KEY (blocker_id, blocked_id)
);
```

**Task IDs:** Generated as `<PROJECT_ID>-<sequence>` (e.g. `KA-001`). Stored as text — stable and human-readable in CLI output.

**Config file** (`~/.config/keroagile/config.toml`) — defaults only, not user storage:
```toml
default_project  = "KA"
default_assignee = "claude"
```

Users are stored in the SQLite `users` table (managed via `KeroAgile user add`). The config only sets which project/assignee to default to when flags are omitted.

---

## TUI Design

**Layout: Three-panel (sidebar · board · detail)**

```
╔══════════════╦════════════════════════════════════╦══════════════════════════════════╗
║ ⬡ KeroAgile ║  myapp  ›  main  ·  2 open PRs     ║  Task Detail                     ║
╠══════════════╬════════════════════════════════════╬══════════════════════════════════╣
║              ║  ◆ BACKLOG  (2)                    ║                                  ║
║  PROJECTS    ║  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  ║  API layer refactor              ║
║  ▶ myapp     ║    Auth flow redesign  KA-007       ║  KA-001  ·  HIGH  ·  IN PROGRESS ║
║    backend   ║    Dark mode toggle    KA-012       ║  🤖 claude  ·  3 pts             ║
║    infra     ║                                    ║                                  ║
║              ║  ◆ TODO  (1)                       ║  Branch   feature/api-layer      ║
║  ──────────  ║  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  ║  PR       #42  ·  3 comments     ║
║  BOARD       ║    Fix login redirect  KA-003       ║                                  ║
║  Backlog   2 ║                                    ║  Build the REST API layer...      ║
║  Todo      1 ║  ◆ IN PROGRESS  (1)                ║                                  ║
║  In Prog   1 ║  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  ║  Blockers                        ║
║  Review    0 ║  ▶ API layer refactor  KA-001       ║  ⚠ KA-005 JWT middleware         ║
║  Done      8 ║                                    ║                                  ║
║              ║  ◆ REVIEW  (0)                     ║  Recent commits                  ║
║              ║                                    ║  a3f9c12 Add route handlers  2h  ║
║              ║  ◆ DONE  (8)  ▸ collapsed          ║  b8d2e45 Init API package    5h  ║
╠══════════════╩════════════════════════════════════╩══════════════════════════════════╣
║  [n]ew  [e]dit  [m]ove  [d]elete  [/]search  [tab]focus  [p]rojects  [?]help  [q]quit ║
╚══════════════════════════════════════════════════════════════════════════════════════╝
```

**Panel focus:** `tab` / `shift+tab` cycles focus between sidebar → board → detail. The focused panel's border renders in bright violet; unfocused panels render in muted grey.

**Task create/edit form:** Rendered as a modal overlay on top of the board. Fields: title (text), description (textarea), assignee (dropdown), priority (dropdown), points (number), status (dropdown), labels (text), blocks (task ID list), blocked-by (task ID list). Navigation: `tab` / `shift+tab` between fields, `enter` to save, `esc` to cancel.

### Color Palette

| Color | Hex | Used for |
|-------|-----|----------|
| Accent violet | `#7C3AED` | Borders, panel focus, logo |
| Accent light | `#A78BFA` | Key hints, selected items |
| Green | `#22C55E` | In Progress, Done, current branch |
| Orange | `#F97316` | Todo status |
| Yellow | `#EAB308` | Backlog status, PR warnings |
| Red | `#EF4444` | Critical priority, blockers |
| Muted | `#6B7280` | Unfocused borders, secondary text |
| Background | `#0F172A` | Base background |
| White | `#F8FAFC` | Primary text |

Extends the existing KeroOle palette — same violet accent and status colors, deeper background.

### Keyboard Bindings

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Cycle panel focus |
| `↑` `↓` / `j` `k` | Navigate within focused panel |
| `n` | New task |
| `e` | Edit focused task |
| `m` | Move task to next status |
| `M` | Move task to previous status |
| `d` | Delete task (confirm prompt) |
| `/` | Search tasks |
| `p` | Switch project |
| `r` | Refresh git / PR status |
| `?` | Help overlay |
| `q` / `ctrl+c` | Quit |

### Mouse Support

- Click to focus a panel or select a task
- Scroll wheel to navigate within a panel
- **Drag and drop** to move tasks between status sections:
  - Mouse down on a task row → enters drag state
  - Mouse motion (using `tea.EnableMouseCellMotion`) → renders floating ghost of task title at cursor, highlights target status section header
  - Mouse release → calls `domain.Service.MoveTask()` with target status, clears drag state

---

## CLI Interface

**Entry point behaviour:**
- No subcommand → launches TUI
- Any subcommand → runs CLI mode, exits after command
- Non-TTY stdout → implies `--json` automatically

**Command tree:**
```
KeroAgile project add <name> --repo <path>
KeroAgile project list
KeroAgile project set-sprint <project-id> on|off

KeroAgile task add <title> --project <id> [--assignee <id>] [--priority <p>]
                            [--points <n>] [--status <s>] [--labels <l>]
KeroAgile task list --project <id> [--status <s>] [--assignee <a>] [--sprint <id>]
KeroAgile task get <task-id>
KeroAgile task update <task-id> [same flags as add]
KeroAgile task move <task-id> <status>
KeroAgile task link-branch <task-id> <branch>
KeroAgile task link-pr <task-id> <pr-number>
KeroAgile task delete <task-id>

KeroAgile sprint add <name> --project <id> [--start <date>] [--end <date>]
KeroAgile sprint list --project <id>
KeroAgile sprint activate <sprint-id>

KeroAgile user add <id> <display-name> [--agent]
KeroAgile user list
```

**`--json` flag:** Available on every command. Writes structured JSON to stdout. On writes, returns the created/updated entity.

**Exit codes:**
- `0` — success
- `1` — not found
- `2` — validation error
- `3` — git/gh error

---

## Git Integration

### Branch Linking

Two paths:
1. **Explicit:** `KeroAgile task link-branch <task-id> <branch>` or TUI `b` key
2. **Auto-detect:** On TUI open, `internal/git` runs `git -C <repo_path> branch --show-current` for each linked repo. If the branch name contains a task ID (e.g. `feature/KA-001-api-layer`), it links automatically.

### PR Linking

1. **Explicit:** `KeroAgile task link-pr <task-id> <pr-number>` or TUI `p` key
2. **Auto-detect:** `gh pr list --json number,headRefName` matched against linked branch names.

### Auto-Transition on Merge

A `tea.Tick` fires every 60 seconds in the TUI. For each task with a linked PR in `review` status:
```bash
gh pr view <pr_number> --json state,mergedAt
```
When `state == "MERGED"` → task moves `review → done`. A status-bar notification flashes: `✓ KA-001 auto-closed via PR #42`. The `r` key forces an immediate refresh.

### Commit Log

When a task has a linked branch, the detail panel shows the 5 most recent commits:
```bash
git -C <repo_path> log <branch> --oneline -5 --format="%h %s %cr"
```

### Graceful Degradation

| Tool | Required | Fallback |
|------|----------|---------|
| `git` | No | Branch/commit features show "git not available"; board fully functional |
| `gh` | No | PR features show "gh not available"; board fully functional |

---

## Team / Identity

- Users are named identities defined in `~/.config/keroagile/config.toml` — no passwords, no auth
- `is_agent = true` marks AI agents — rendered with `🤖` prefix in all views
- `default_assignee` in config lets Claude's environment pre-set itself as assignee when adding tasks
- Initial users: `matt` and `claude`; additional team members added via `KeroAgile user add`

---

## Out of Scope (v1)

- Real-time multi-user sync (no daemon, no sockets)
- Web UI
- Email / Slack notifications
- Full Jira import/export
- The Claude Code skill/plugin (built separately after the app is complete)
