Generate a standup summary for the current KeroAgile project. $ARGUMENTS

## Steps

1. Use `list_tasks` with `status: "in_progress"` to get what is actively being worked on. Project is auto-detected from git remote.

2. Use `list_tasks` with `status: "review"` to get tasks waiting for review or PR merge.

3. Use `list_tasks` with `status: "todo"` to get what is queued up next.

4. Optionally call `get_sprint` (no arguments) to include sprint context — name, dates, and progress.

5. For each in-progress task, check if `blockers` is non-empty. If so, flag it as blocked.

6. Format the output as a standup report:

```
## Standup — <project name> — <today's date>

**Sprint:** <sprint name> (<start> → <end>) — if sprint is active

### In Progress
- [KA-007] Build login page (alice) — in review, PR linked
- [KA-012] Write API tests (bob) — BLOCKED by KA-009

### In Review
- [KA-005] Database migrations (alice) — waiting on approval

### Up Next
- [KA-015] Add OAuth provider (unassigned)
- [KA-016] Rate limiting (bob)

### Blockers
- KA-009 must be done before KA-012 can proceed
```

## Notes

- Only show sections that have tasks (omit empty sections).
- If no tasks are in progress or review, say so explicitly — it may mean the board is stale.
- If auto-detection fails, tell the user to run `KeroAgile project add --repo <remote-url>`.
- Keep the summary concise — one line per task.
