Update the KeroAgile task board to reflect the work just completed. $ARGUMENTS

## Steps

1. Run `KeroAgile sync-status` and report what changed (task ID, old status → new status, PR linked).

2. If `sync-status` reports "no change" and $ARGUMENTS provides a hint (e.g. "merged", "done", "review"), use `move_task` to apply that status manually to the task on the current branch.
   - Get the current branch: `git branch --show-current`
   - Find the task by calling `list_tasks` for `in_progress`, `review`, and `todo` statuses; match by `branch` field.
   - Call `move_task` with the task ID and the hinted status.

3. Report the final state: task ID, title, status after the command.

## Notes

- Do not delete the task or change its assignee.
- If the task is already in the target status, say so and skip the move.
- `sync-status` handles PR linking automatically; you do not need to call `link_branch` separately.
- If `sync-status` fails because the branch has no linked task, tell the user to link it first:
  `KeroAgile task link-branch <task-id> <branch-name>`
