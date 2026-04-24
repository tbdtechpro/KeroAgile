Import tasks from a plan or spec file into KeroAgile. $ARGUMENTS

## What this does

Reads a structured plan (markdown with task lists, phases, or sprints) and creates the corresponding project, sprints, and tasks via MCP — no shell commands needed.

## Steps

### 1. Find the source file

If $ARGUMENTS names a file, use it. Otherwise look for:
- `docs/superpowers/plans/*.md` (most recent)
- `docs/superpowers/specs/*.md`
- `PLAN.md`, `TODO.md`, `TASKS.md` in the project root

Read the file and identify:
- Project name/ID (or ask the user if ambiguous)
- Phases or sprints (top-level groupings)
- Individual tasks within each phase

### 2. Resolve or create the project

Call `list_projects` to check if the target project already exists.

If it does not exist:
- Call `create_project` with the `id` and `name` from the plan
- Use `repo_path` if the plan references a git remote

If it exists, use the existing project ID.

### 3. Create sprints (one per phase/milestone)

For each phase or sprint section in the plan:
- Call `create_sprint` with `name` = the phase name and `project_id`
- Note the returned `id` — you will need it for task assignment

Phases map to sprints. If the plan has no phases, create a single sprint named after the plan.

### 4. Create tasks (one per checklist item)

For each task item in the plan, call `create_task` with:
- `title` — the task description (trim markdown checkbox syntax `- [ ]`)
- `project_id` — the project
- `sprint_id` — the sprint for this phase (eliminates the need for a separate assign call)
- `priority` — infer from plan labels: `critical`/`high`/`medium`/`low`; default `medium`
- `points` — use estimate from plan if present (e.g. `(3pts)`, `[5]`)
- `assignee_id` — omit; auto-suggestion from title keywords will handle it

Do NOT call `assign_task_sprint` separately — `sprint_id` on `create_task` handles it atomically.

### 5. Wire blockers

After all tasks are created, look for dependency language in the plan:
- "depends on", "after", "requires", "blocked by", "unblocks"

For each dependency, call `add_blocker`:
- `task_id` = the blocking task's ID
- `blocked_by` = the blocked task's ID

### 6. Confirm and report

Print a summary table:

```
## Import complete

Project: <ID> — <name>

| Sprint | Tasks | Points |
|--------|-------|--------|
| Phase 1: Setup | 8 | 21 |
| Phase 2: Core | 12 | 34 |

Total: 20 tasks imported across 2 sprints
```

If any `create_task` calls failed, list them so the user can retry.

## Notes

- This command is mechanical: follow the plan as written, do not interpret or restructure it.
- Assignee auto-suggestion (built into `create_task`) handles routing without explicit assignment.
- If the plan file is very large, process one phase at a time and confirm each before moving to the next.
- If auto-detection fails for `project_id`, tell the user to run `KeroAgile project add --repo <remote-url>` or pass the project ID explicitly in $ARGUMENTS.
