Generate a standup summary for the current KeroAgile project. $ARGUMENTS

## Steps

1. Use `list_tasks` with `status: "in_progress"` to get what is actively being worked on. Project is auto-detected from git remote.

2. Use `list_tasks` with `status: "review"` to get tasks waiting for review or PR merge.

3. Use `list_tasks` with `status: "todo"` to get what is queued up next.

4. Call `get_sprint` (no arguments) to include sprint context — name, dates, and task count.

5. For each task in in-progress and review:
   - If its `blockers` field is non-empty, flag it as BLOCKED and list the blocking task IDs.
   - If its `blocking` field is non-empty, note which downstream tasks it is currently holding up.
   - If its `pr_number` field is non-null, append "PR #N linked" to the task line.

6. Format the output as a standup report:

```
## Standup — <project name> — <today's date>

**Sprint:** <sprint name> (<start> → <end>) — if sprint is active

### In Progress
- [KA-007] Build login page (alice) — PR #42 linked
- [KA-012] Write API tests (bob) — BLOCKED by KA-009

### In Review
- [KA-005] Database migrations (alice)

### Up Next
- [KA-015] Add OAuth provider (unassigned)
- [KA-016] Rate limiting (bob)

### Blockers
- KA-009 (in progress) must finish before KA-012 can proceed
- KA-007 (in review) is holding up KA-014
```

## Notes

- Only show sections that have tasks — omit empty sections entirely.
- Show assignee as `(unassigned)` when `assignee_id` is null.
- If no tasks are in progress or review, say so explicitly — the board may be stale.
- If auto-detection fails, tell the user to run `KeroAgile project add --repo <remote-url>`.
- Keep the summary concise — one line per task.
