Help plan the next sprint for the current KeroAgile project. $ARGUMENTS

## Steps

1. Use `list_tasks` with `status: "backlog"` to get all unstarted work. Project is auto-detected from git remote.

2. Use `get_sprint` (no arguments) to check whether a sprint is currently active and how many tasks it already contains.

3. Use `list_users` to know who is available on the team.

4. Analyse the backlog:
   - Group tasks by priority: `critical`, `high`, `medium`, `low`
   - Note which tasks have story points and which do not
   - Identify any blocker relationships — a blocked task cannot start until its blocker is done

5. Suggest a sprint composition:
   - Lead with `critical` and `high` priority tasks
   - Prefer tasks with no blockers (or whose blockers are already in the sprint)
   - Aim for a reasonable total point count — if the user specified a target (e.g. "plan a 20-point sprint") in $ARGUMENTS, use that; otherwise suggest 20–30 points as a default
   - Flag tasks without points and ask the user to estimate them before finalising

6. Present the suggestion as a table:

```
## Proposed Sprint: <suggested name>

| Task | Title | Priority | Points | Assignee | Notes |
|------|-------|----------|--------|----------|-------|
| KA-003 | Add OAuth login | critical | 5 | alice | |
| KA-007 | Rate limiting | high | 3 | bob | blocks KA-011 |
| KA-009 | DB index | high | 2 | — | needs estimate |

**Total:** 10 points assigned, 2 unestimated

**Left in backlog:** 8 tasks
```

7. Ask the user: "Should I create this sprint and assign these tasks?" If they confirm:
   - Note: sprint creation and task assignment require the CLI (`KeroAgile sprint add`, `KeroAgile sprint assign`) because the MCP tools do not expose sprint creation. Provide the exact commands to run.

## Notes

- Do not create or modify anything without explicit user confirmation.
- If the backlog is empty, say so and suggest moving tasks from `todo` to plan instead.
- If $ARGUMENTS contains a point target, assignee preference, or focus area, honour it.
- If auto-detection fails, tell the user to run `KeroAgile project add --repo <remote-url>`.
