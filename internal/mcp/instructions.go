package mcp

// serverInstructions is returned in the MCP initialize result so the LLM
// client receives this guidance once per session at handshake. The 2024-11-05
// MCP spec defines this field for exactly this purpose.
//
// Keep this string in sync with the "Concepts that don't exist" section of
// the project's CLAUDE.md.
const serverInstructions = `KeroAgile data model — important constraints:

Tasks are FLAT within a project. There is NO concept of:
  - parent tasks
  - sub-tasks
  - epics
  - themes
  - "umbrella" tasks
  - hierarchical groupings of any kind

Do not invent these relationships when describing or organizing work. Tasks
within a project are siblings.

The only inter-task relationship is the blocker DAG, managed via add_blocker
and remove_blocker. A blocker means "task A must reach 'done' before task B
can start" — it is a dependency, not a hierarchy.

To group related work, use:
  - shared labels (free-form strings on each task)
  - shared sprint (assign tasks to the same sprint via assign_task_sprint)

Never invent a parent task or umbrella task to "contain" related work.`
