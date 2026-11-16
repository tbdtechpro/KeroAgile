package mcp

// serverInstructions is returned in the MCP initialize result so the LLM
// client receives this guidance once per session at handshake. The 2024-11-05
// MCP spec defines this field for exactly this purpose.
//
// The first part is kept in sync with the "Concepts that don't exist"
// section of the project's CLAUDE.md. The second tells Claude to keep what
// its work produces with the ticket.
const serverInstructions = conceptsInstructions + "\n\n" + recordsInstructions

const conceptsInstructions = `KeroAgile data model — important constraints:

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

const recordsInstructions = `Keeping work products with the ticket:

When work on a ticket produces files outside the repository — test scripts,
logs, screenshots, audio, video, rendered output, reference material — attach
each one worth keeping with attach_file before finishing the ticket, with a
short note saying what it is. Session scratch folders are wiped, and a file
not attached has to be recreated later. A file that belongs in the repository
is committed instead, and the commit mentions the ticket ID.

Record what you found, decided or tried with add_note rather than rewriting
the description. Notes are dated, signed and never overwritten.

Files you do not attach may be kept by a sweep as unfiled files of the
project. list_artifacts with project_id and unfiled=true shows them; file
each on its ticket with file_artifact, or discard_artifact what is not worth
keeping. Never guess a ticket for a file.`
