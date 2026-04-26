# KeroAgile — Future Ideas

> Captured ideas not yet committed to a version. When an idea matures into scheduled
> work, it graduates to `docs/roadmap.md` as a numbered phase. Each idea links to its
> tracking task on the KA board.

## Origin

Drafted 2026-04-26 from morning notes; refined in conversation with Claude the same day.

---

## 1. UI/UX polish

### 1.1 Survey other agile/kanban web UIs for inspiration — [KA-031](../docs/roadmap.md)

Review Linear, Height, Plane, Jira, Basecamp, and others with fresh eyes focused on:
screen space efficiency, project listing/navigation patterns, card density, column headers,
sprint planning UX, and mobile-responsive patterns. Document screenshots and notes on what
works and what KeroAgile could borrow.

**Feeds:** KA-038 (screen space), KA-039 (project listing UX).

### 1.2 Make better use of screen space — [KA-038](../docs/roadmap.md)

Review and redesign the board layout to use available screen real estate more effectively.
Open questions: better density on large monitors vs. readable cards on small screens?
Collapsible sidebar? Column header height? Card compact mode? Blocked on KA-031 (survey).

### 1.3 Improve project listing & selection UX — [KA-039](../docs/roadmap.md)

The current project selector in the sidebar is functional but basic. Explore: a project
switcher in the nav bar, a project overview/home page, quick-switch keyboard shortcut,
project cards with summary stats (open task count, sprint progress).
Blocked on KA-031 (survey).

### 1.4 Prepare SVG icon for web interface — [KA-030](../docs/roadmap.md)

Design and export the KeroAgile SVG icon/logo for use in the web interface (favicon, nav
bar, PWA manifest). Should work at multiple sizes (16px, 32px, 192px, 512px). Consider
the frog motif consistent with the Kero Apps brand family. Ideally also usable as inline
SVG so it can inherit theme accent colours.

---

## 2. Theming

### 2.1 Multi-theme support — port KeroCareerPOC working impl — [KA-032](../docs/roadmap.md)

Add 6-theme support (each with light + dark = 12 colour sets):

**Colour tokens only — no font switching.** KeroAgile uses its own typeface
independently of themes. The Lora/Source Sans 3 pairing is specific to KeroCareer's
editorial identity and does not transfer.

| # | Name | Character |
|---|------|-----------|
| 1 | Default | Warm greens/greys — mirrors the existing TUI palette |
| 2 | KeroCareer | Warm earthy — KCP's "Default" theme |
| 3 | Corporate | Professional blue |
| 4 | Bonita | Soft purple on white |
| 5 | Muy Bonita | Deep crimson on pink |
| 6 | Geeky | Dracula dark / Alucard-adjacent light |

**Working reference:** `KeroCareerPOC/ui_tests/app-layout_2026-04-20/index.html` (vanilla
JS) + `app-layout_reference.md`. The implementation pattern: a `THEME_DEFS` object with
17 named tokens per colour set; `applyTheme()` writes them as CSS custom properties to
`document.documentElement`. Port to React via a `ThemeProvider` context + `useTheme()`
hook. **Omit the font-switching logic** from the KCP reference.

**Token source-of-truth for themes 2–6:**
`KeroCareerPOC/draft_assets/design_tokens/theme/theme-tokens.csv`

**Theme 1 (Default):** derive tokens from the existing TUI colour palette
(`internal/tui/styles/`).

**LocalStorage key:** `kero-prefs` — matching KCP for consistency.

---

## 3. README & marketing

### 3.1 Capture web UI screenshots — [KA-045](../docs/roadmap.md)

Take high-quality screenshots of the web UI for use in the README: board view with tasks
in multiple columns, task detail drawer/modal, sync settings page, mobile-viewport version.
Do after KA-038, KA-039, KA-032 are complete so screenshots reflect the polished UI.

### 3.2 Review/refresh README gifs — [KA-046](../docs/roadmap.md)

Review existing TUI gifs — are they still accurate and representative? Decide whether to
re-record (if TUI changed), trim, or add new web UI gifs. Do after KA-038, KA-039.

### 3.3 Matt: personal README revisions — [KA-033](../docs/roadmap.md)

Creative/editorial pass — tone, framing, section order, anything that feels off. Runs
in parallel with UI polish work; no specific prerequisite.

### 3.4 Claude: README final polish pass — [KA-047](../docs/roadmap.md)

After KA-033 (Matt's revisions), KA-045 (screenshots), and KA-046 (gifs): verify CLI
examples match current commands, update install steps, fix broken links, check feature
list reflects shipped work (sync, web UI, MCP), tighten prose.

---

## 4. TUI

### 4.1 Document TUI improvement opportunities — [KA-029](../docs/roadmap.md)

Use the TUI hands-on and document concrete friction points, visual inconsistencies, and
missing keyboard shortcuts. Produces a prioritised list (not implementation) for future
sprints. Focus areas: task form UX, drag-and-drop reliability, sprint/backlog mode
switching, truncation of long titles, BubbleTea refresh latency.

---

## 5. Personal / neurodivergent use case

### 5.1 Brainstorm: kanban for personal & neurodivergent executive function — [KA-034](../docs/roadmap.md)

Explore how an agile/kanban board could support personal (non-software) planning —
especially as an external memory and executive function scaffold for neurodivergent users
(ADHD, autism, etc.).

Questions to explore:
- What friction points does a standard kanban create for ND users?
- What would a low-friction capture flow look like?
- Should there be a "brain dump" backlog mode distinct from a sprint?
- How can Claude integration support task decomposition and prioritisation?
- What UI affordances reduce overwhelm (limiting visible tasks, daily focus mode)?

Output: written notes or a doc that feeds KA-040 (implementation).

### 5.2 Implement personal-project / neurodivergent-friendly enhancements — [KA-040](../docs/roadmap.md)

Scope TBD pending KA-034 (brainstorm). Likely candidates: brain-dump capture mode, daily
focus view, low-friction task creation (minimal required fields), Claude-assisted task
decomposition, gentle reminders rather than hard deadlines. Blocked on KA-034.

---

## 6. Cross-project features

### 6.1 Cross-project blockers — UI/UX — [KA-035](../docs/roadmap.md)

**The DB layer already supports this.** `task_deps` has no `project_id` constraint
(`internal/store/db.go:98-102`) and `Service.AddDep` is a pass-through
(`internal/domain/service.go:221-223`). The gap is entirely UI/UX:

1. Task form: blocker selector should allow searching/picking tasks from any project
2. Task detail view: cross-project blockers should render with full project-prefixed ID
   (e.g. `KCAL-007`) and a visual indicator that the blocking task is in another project
3. Navigation: click/hover on a cross-project blocker chip to jump to that task's project
4. CLI: verify `KeroAgile task block` accepts IDs from different projects (likely works
   already; add a test)

This unblocks KA-041 (KeroCalendar integration) and KA-042 (all-project planning view).

### 6.2 All-project daily standup assistant — [KA-042](../docs/roadmap.md)

A daily standup flow across all projects:

1. Pull open tasks assigned to the current user from every project (1–3 per project,
   ordered by task sequence)
2. Present as a simple list: "across your projects, here are your open tasks"
3. User declares intent: "I'll work on KA-031, KCP-012, and KA-035 today"
4. Claude moves the declared tasks to `todo` or `in_progress`

User-driven — Claude surfaces candidates, the user chooses, Claude executes status moves.
No autonomous prioritisation. Could be an MCP tool ("morning briefing"), a dedicated web
page, or both. Blocked on KA-035 so the view can also show which tasks are currently
blocked across projects.

---

## 7. Integrations (external apps)

### 7.1 KeroCalendar integration — [KA-041](../docs/roadmap.md)

Link KeroAgile tasks to KeroCalendar events via cross-project blockers. Example: a KCAL
task ("blocked waiting on KeroAgile API MVP") wired as a blocker on a KA integration task
and vice versa.

**External prereq:** KeroCalendar does not exist yet. Re-evaluate when it reaches MVP.
Also gated on KA-035 (cross-project blockers UI).

### 7.2 Obsidian integration

#### 7.2.1 Brainstorm integration scope — [KA-036](../docs/roadmap.md)

Map out what a KeroAgile ↔ Obsidian integration could look like beyond the obvious
(standups and diagrams). Starting ideas: task notes syncing to vault, sprint retrospective
templates, linking tasks to meeting notes, Obsidian graph view for project relationships.
Output feeds KA-043 and KA-044 and may spawn further tasks.

#### 7.2.2 Generate standup MD files — [KA-043](../docs/roadmap.md)

Generate Obsidian-compatible markdown standup files from KeroAgile task state. Each file
covers: done yesterday, in-progress today, blockers. Files land in a configurable vault
path using Obsidian's daily-note naming convention (`YYYY-MM-DD.md` or standup subfolder).
Blocked on KA-036 (brainstorm) to confirm vault integration approach.

#### 7.2.3 Render task dependency diagrams — [KA-044](../docs/roadmap.md)

Generate mermaid diagram blocks (or Dataview queries) in Obsidian MD files that visualise
KeroAgile task dependency chains. Could embed in sprint retrospectives, project overview
pages, or standup files. Blocked on KA-036 to confirm which approach Obsidian renders
best (mermaid is native; Dataview is a plugin).

### 7.3 KeroOle / KeroBooks research integration — [KA-037](../docs/roadmap.md)

When a KeroAgile task is tagged as a research task, Claude queries content in
KeroOle/KeroBooks and adds links to relevant ebook MD files in the task description.
Surfaces reading material without manual cross-referencing.

**KeroOle** exists at `/home/matt/github/KeroOle`. It currently downloads content from
O'Reilly Learning, logs it to a database, creates exports in multiple formats, and
provides AI-ready context. Non-O'Reilly ebook parsing is planned. **KeroBooks** is a
planned fork with the DB/export/context pipeline but without the O'Reilly-specific
grey-area features — the clean public version.

This task should wait until KeroOle's non-O'Reilly parsing is mature and the export API
is settled. May be token-intensive for large ebook indices — benchmark then.

---

## Dependency map

```
KA-031 (UI survey)
  └─ blocks KA-038 (screen space)
       └─ blocks KA-045 (screenshots) ◄─ also blocked by KA-039, KA-032
       └─ blocks KA-046 (gifs)        ◄─ also blocked by KA-039
  └─ blocks KA-039 (project listing)

KA-033 (Matt README) ──────────────────┐
KA-045 (screenshots) ──────────────────┤
KA-046 (gifs) ─────────────────────────┴─ blocks KA-047 (Claude README polish)

KA-034 (ND brainstorm) → blocks KA-040 (ND enhancements)

KA-035 (cross-project blockers UI)
  └─ blocks KA-041 (KeroCalendar)
  └─ blocks KA-042 (all-project view)

KA-036 (Obsidian brainstorm)
  └─ blocks KA-043 (standup MD files)
  └─ blocks KA-044 (task diagrams)
```
