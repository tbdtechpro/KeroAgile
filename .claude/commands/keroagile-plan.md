Help plan the next sprint for the current KeroAgile project. $ARGUMENTS

## Steps

1. Use `list_tasks` with `status: "backlog"` to get all unstarted work. Project is auto-detected from git remote.

2. Call `get_sprint` (no arguments) to check whether a sprint is currently active and sum its current task points.

3. Use `list_users` to know who is available on the team.

4. Analyse the backlog:
   - Group tasks by priority: `critical`, `high`, `medium`, `low`
   - Note which tasks have story points and which do not
   - Identify blocker relationships — a blocked task cannot start until its blocker is done

5. Determine the point budget:
   - If $ARGUMENTS specifies a target (e.g. "plan a 20-point sprint"), use that
   - Otherwise default to 20–30 points
   - If there is an active sprint with tasks already in it, subtract its current total points from the target before selecting backlog tasks

6. Suggest a sprint composition:
   - Lead with `critical` and `high` priority tasks
   - Prefer tasks with no blockers, or tasks whose blockers are also included in the suggestion
   - Flag tasks without points and ask the user to estimate them before finalising
   - Respect any assignee or focus area constraints from $ARGUMENTS

7. Present the suggestion as a table:

```
## Proposed Sprint: <suggested name>

| Task | Title | Priority | Points | Assignee | Notes |
|------|-------|----------|--------|----------|-------|
| KA-003 | Add OAuth login | critical | 5 | alice | |
| KA-007 | Rate limiting | high | 3 | bob | also unblocks KA-011 |
| KA-009 | DB index | high | — | — | needs estimate |

**Total:** 8 points assigned, 1 task needs estimate

**Left in backlog:** 6 tasks
```

8. Ask the user: "Should I create this sprint and assign these tasks?" If they confirm:
   - Call `create_sprint` with the suggested name and `project_id` to create the sprint
   - For each selected task, call `update_task` with `sprint_id` set to the new sprint's ID
   - Then ask: "Sprint created. Should I activate it?" If yes, the user must run:
     ```
     KeroAgile sprint activate <sprint-id>
     ```
     (Sprint activation is not yet available via MCP.)

## Notes

- Do not create or modify anything without explicit user confirmation.
- If the backlog is empty, suggest moving `todo` tasks into the sprint instead.
- If auto-detection fails, tell the user to run `KeroAgile project add --repo <remote-url>`.
