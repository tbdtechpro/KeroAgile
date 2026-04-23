Update the KeroAgile task board to reflect the work just completed. $ARGUMENTS

## Steps

1. Run `git branch --show-current` to get the current branch name.

2. Use the `list_tasks` MCP tool to list tasks for this project. The project is auto-detected from the git remote — no need to specify project_id unless auto-detection fails.

3. Find the task whose `branch` field matches the current branch name. If no task has a matching branch, search for a task whose title or description clearly relates to the work just completed and ask the user to confirm before proceeding.

4. Determine the appropriate new status:
   - If a PR was just opened or is under review: move the task to `review` using `move_task`
   - If the PR was merged or the work is fully complete: move the task to `done` using `move_task`
   - If no PR is involved and the work is done: move to `done`

5. If the task's `branch` field is empty, call `link_branch` with the current branch name.

6. Report what changed: task ID, title, old status → new status, and whether the branch was linked.

## Notes

- Do not delete the task or change its assignee.
- If the task is already in the target status, say so and skip the move.
- If auto-detection fails with an error about project_id, tell the user to run: `KeroAgile project add --repo <remote-url>` where `<remote-url>` is the output of `git remote get-url origin`.
