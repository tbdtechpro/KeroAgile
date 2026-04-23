Update the KeroAgile task board to reflect the work just completed. $ARGUMENTS

## Steps

1. Run `git branch --show-current` to get the current branch name.

2. Use `list_tasks` with `status: "in_progress"`, then with `status: "review"`, and then with `status: "todo"` to collect all active tasks. Project is auto-detected from git remote — no need to specify project_id unless auto-detection fails.

3. Find the task whose `branch` field matches the current branch name across all collected tasks. If no task matches, search the collected task titles and descriptions for a clear match and ask the user to confirm before proceeding.

4. Determine the appropriate new status:
   - If $ARGUMENTS contains "merged" or "done": move to `done`
   - If $ARGUMENTS contains "review" or "pr" or "opened": move to `review`
   - Otherwise: check the task's `pr_number` field. If non-null, run `gh pr view <pr_number> --json state --jq .state` to get the PR state. A `MERGED` state means move to `done`; `OPEN` means move to `review`.
   - If PR state still cannot be determined, ask the user: "Should I move this to 'review' (PR open) or 'done' (work complete)?"

5. Call `move_task` with the task ID and the determined status.

6. If the task's `branch` field is empty, call `link_branch` with the current branch name.

7. Report what changed: task ID, title, old status → new status, and whether the branch was linked.

## Notes

- Do not delete the task or change its assignee.
- If the task is already in the target status, say so and skip the move.
- There is no MCP tool for linking a PR number — if the user wants to link a PR, tell them to run: `KeroAgile task link-pr <task-id> <pr-number>`
- If auto-detection fails with an error about project_id, tell the user to run: `KeroAgile project add --repo <remote-url>` where `<remote-url>` is the output of `git remote get-url origin`.
