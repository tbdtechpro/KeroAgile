package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func toolList() []toolDef {
	str := func(desc string) map[string]any {
		return map[string]any{"type": "string", "description": desc}
	}
	obj := func(props map[string]any, required []string) map[string]any {
		m := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			m["required"] = required
		}
		return m
	}

	return []toolDef{
		{
			Name:        "list_projects",
			Description: "List all KeroAgile projects, each with its open (not done) and total ticket counts.",
			InputSchema: obj(map[string]any{}, nil),
		},
		{
			Name: "list_tasks",
			Description: "List a project's tickets as compact summaries: id, title, status, priority, assignee, points, labels, sprint, branch, PR, blockers, blocking, last update. " +
				"Descriptions are left out; use get_task for one ticket in full, or detail=\"full\" to include them here. " +
				"Auto-detects project_id from git remote if not provided.",
			InputSchema: obj(map[string]any{
				"project_id":  str("Project ID (auto-detected from git remote if omitted)"),
				"status":      str("Filter by status: backlog|todo|in_progress|review|done, or open for everything not done"),
				"assignee_id": str("Filter by assignee user ID"),
				"label":       str("Only tickets carrying this label"),
				"sprint_id":   map[string]any{"type": "integer", "description": "Only tickets in this sprint"},
				"detail":      str("summary (default) or full. full adds each ticket's description and can be very large; prefer get_task"),
				"limit":       map[string]any{"type": "integer", "description": "Most tickets to return (default 200, max 500). The reply says when the list was cut"},
			}, nil),
		},
		{
			Name: "search_tasks",
			Description: "Find tickets across every project, or one, as compact summaries. " +
				"query matches a ticket's ID, title or description. status, assignee_id and label narrow the result, and work without a query: " +
				"status=in_progress with assignee_id lists that person's work in flight on every board; status=open with assignee_id lists everything they have left. " +
				"Give at least one of query, status, assignee_id or label.",
			InputSchema: obj(map[string]any{
				"query":       str("Text to find in ticket IDs, titles and descriptions"),
				"project_id":  str("Search one project only (default: every project)"),
				"status":      str("backlog|todo|in_progress|review|done, or open for everything not done"),
				"assignee_id": str("Only tickets assigned to this user ID"),
				"label":       str("Only tickets carrying this label"),
				"limit":       map[string]any{"type": "integer", "description": "Most tickets to return (default 50, max 200). The reply says when the list was cut"},
			}, nil),
		},
		{
			Name:        "get_task",
			Description: "Get full details of a single task including blockers and PR info.",
			InputSchema: obj(map[string]any{
				"task_id": str("Task ID, e.g. KA-007"),
			}, []string{"task_id"}),
		},
		{
			Name:        "create_task",
			Description: "Create a new task. Auto-detects project_id from git remote if not provided. Assignee is auto-suggested from title keywords when omitted.",
			InputSchema: obj(map[string]any{
				"title":       str("Task title (required)"),
				"project_id":  str("Project ID (auto-detected if omitted)"),
				"description": str("Task description"),
				"priority":    str("low|medium|high|critical (default: medium)"),
				"status":      str("backlog|todo|in_progress|review|done (default: backlog)"),
				"assignee_id": str("Assignee user ID (auto-suggested from title if omitted)"),
				"points":      map[string]any{"type": "integer", "description": "Story points"},
				"labels":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Labels"},
				"sprint_id":   map[string]any{"type": "integer", "description": "Sprint ID to assign the task to"},
			}, []string{"title"}),
		},
		{
			Name:        "create_project",
			Description: "Create a new KeroAgile project.",
			InputSchema: obj(map[string]any{
				"id":        str("Project ID, e.g. KA (uppercase letters, required)"),
				"name":      str("Human-readable project name (required)"),
				"repo_path": str("Git remote URL or repo path for auto-detection"),
			}, []string{"id", "name"}),
		},
		{
			Name:        "create_sprint",
			Description: "Create a new sprint for a project.",
			InputSchema: obj(map[string]any{
				"name":       str("Sprint name, e.g. Sprint 1 (required)"),
				"project_id": str("Project ID (auto-detected from git remote if omitted)"),
				"start_date": str("Start date in YYYY-MM-DD format (optional)"),
				"end_date":   str("End date in YYYY-MM-DD format (optional)"),
			}, []string{"name"}),
		},
		{
			Name: "update_task",
			Description: "Change some fields of a ticket. Only the fields you pass are changed; everything else is left exactly as it is, including changes others have made since you read the ticket. " +
				"Empty strings are ignored, so to empty a field name it in clear. " +
				"Pass expected_updated_at (the updated_at from get_task) to have the update refused if the ticket has changed since you read it.",
			InputSchema: obj(map[string]any{
				"task_id":     str("Task ID (required)"),
				"title":       str("New title"),
				"description": str("New description"),
				"priority":    str("low|medium|high|critical"),
				"status":      str("backlog|todo|in_progress|review|done"),
				"assignee_id": str("Assignee user ID (empty string to clear)"),
				"points":      map[string]any{"type": "integer", "description": "Story points"},
				"labels":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Labels; replaces the current list"},
				"sprint_id":   map[string]any{"type": "integer", "description": "Sprint to put the ticket in (must belong to the ticket's project)"},
				"clear": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string", "enum": clearable},
					"description": "Fields to empty: description, points, labels, assignee_id, sprint_id",
				},
				"expected_updated_at": str("The updated_at you read from get_task. If the ticket has changed since, nothing is changed and the call fails; read it again and retry"),
			}, []string{"task_id"}),
		},
		{
			Name:        "move_task",
			Description: "Move a task to a different status column.",
			InputSchema: obj(map[string]any{
				"task_id": str("Task ID"),
				"status":  str("Target status: backlog|todo|in_progress|review|done"),
			}, []string{"task_id", "status"}),
		},
		{
			Name:        "delete_task",
			Description: "Delete a task permanently.",
			InputSchema: obj(map[string]any{
				"task_id": str("Task ID"),
			}, []string{"task_id"}),
		},
		{
			Name:        "link_branch",
			Description: "Link a git branch to a task.",
			InputSchema: obj(map[string]any{
				"task_id": str("Task ID"),
				"branch":  str("Branch name"),
			}, []string{"task_id", "branch"}),
		},
		{
			Name:        "list_users",
			Description: "List all registered users and agents.",
			InputSchema: obj(map[string]any{}, nil),
		},
		{
			Name:        "get_sprint",
			Description: "Get a sprint and its tickets as compact summaries (no descriptions; use get_task for one ticket in full). Pass sprint_id for a specific sprint, or omit to get the active sprint (project_id auto-detected from git remote).",
			InputSchema: obj(map[string]any{
				"sprint_id":  map[string]any{"type": "integer", "description": "Sprint ID (omit to get the active sprint)"},
				"project_id": str("Project ID for active-sprint lookup (auto-detected if omitted)"),
			}, nil),
		},
		{
			Name:        "add_blocker",
			Description: "Mark task A as blocking task B (A must be done before B can start).",
			InputSchema: obj(map[string]any{
				"task_id":    str("ID of the task that is blocking"),
				"blocked_by": str("ID of the task that is blocked"),
			}, []string{"task_id", "blocked_by"}),
		},
		{
			Name:        "remove_blocker",
			Description: "Remove a blocker relationship between two tasks.",
			InputSchema: obj(map[string]any{
				"task_id":    str("ID of the blocking task"),
				"blocked_by": str("ID of the blocked task"),
			}, []string{"task_id", "blocked_by"}),
		},
		{
			Name:        "assign_task_sprint",
			Description: "Assign a task to a sprint, or remove it from its current sprint.",
			InputSchema: obj(map[string]any{
				"task_id":   str("Task ID, e.g. KA-008"),
				"sprint_id": map[string]any{"type": "number", "description": "Sprint ID to assign, or omit/null to remove"},
			}, []string{"task_id"}),
		},
	}
}

// A listing of a large project used to be several hundred kilobytes: every
// ticket's full description, pretty-printed. MCP clients cap tool results (a
// 98-ticket project was refused outright), and what does fit is paid for in
// the model's context on every call. Listings therefore carry a summary row
// per ticket, as compact JSON, and say so when they are cut short.

const (
	defaultListLimit = 200 // above the largest real project, so cuts are rare
	maxListLimit     = 500

	defaultSearchLimit = 50
	maxSearchLimit     = 200
)

// clearable lists the fields update_task can empty.
var clearable = []string{"description", "points", "labels", "assignee_id", "sprint_id"}

// projectRow is one project in list_projects.
type projectRow struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Open       int    `json:"open"`  // tickets not done
	Total      int    `json:"total"` // all tickets
	RepoPath   string `json:"repo_path,omitempty"`
	SprintMode bool   `json:"sprint_mode,omitempty"`
	SyncOrigin string `json:"sync_origin,omitempty"`
}

// taskRow is one ticket in a listing: enough to pick and triage tickets and
// to see what blocks what, without the description. get_task returns
// everything about one ticket.
type taskRow struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Status     domain.Status   `json:"status"`
	Priority   domain.Priority `json:"priority"`
	AssigneeID *string         `json:"assignee_id,omitempty"`
	Points     *int            `json:"points,omitempty"`
	Labels     []string        `json:"labels,omitempty"`
	SprintID   *int64          `json:"sprint_id,omitempty"`
	Branch     string          `json:"branch,omitempty"`
	PRNumber   *int            `json:"pr_number,omitempty"`
	Blockers   []string        `json:"blockers,omitempty"`
	Blocking   []string        `json:"blocking,omitempty"`
	UpdatedAt  string          `json:"updated_at"` // date only
}

func rowsOf(tasks []*domain.Task) []taskRow {
	rows := make([]taskRow, len(tasks))
	for i, t := range tasks {
		rows[i] = taskRow{
			ID: t.ID, Title: t.Title, Status: t.Status, Priority: t.Priority,
			AssigneeID: t.AssigneeID, Points: t.Points, Labels: t.Labels,
			SprintID: t.SprintID, Branch: t.Branch, PRNumber: t.PRNumber,
			Blockers: t.Blockers, Blocking: t.Blocking,
			UpdatedAt: t.UpdatedAt.Format("2006-01-02"),
		}
	}
	return rows
}

// listDetail reads the detail argument: "summary" (the default) or "full".
func listDetail(v string) (full bool, err error) {
	switch v {
	case "", "summary":
		return false, nil
	case "full":
		return true, nil
	}
	return false, fmt.Errorf("detail %q: use \"summary\" or \"full\"", v)
}

// listLimit reads the limit argument, falling back to def and capping at max.
func listLimit(v any, def, max int) int {
	n, ok := v.(float64)
	if !ok || n < 1 {
		return def
	}
	return min(int(n), max)
}

// statusFilter reads a status argument: one status, "open" for everything
// not done, or empty for any status. A mistyped status is an error rather
// than an empty result, which would read as "nothing in that state".
func statusFilter(v string) ([]domain.Status, error) {
	switch {
	case v == "":
		return nil, nil
	case v == "open":
		return []domain.Status{domain.StatusBacklog, domain.StatusTodo, domain.StatusInProgress, domain.StatusReview}, nil
	case domain.Status(v).Valid():
		return []domain.Status{domain.Status(v)}, nil
	}
	return nil, fmt.Errorf("status %q: use backlog, todo, in_progress, review or done, or open for everything not done", v)
}

// int64Arg reads an optional integer argument.
func int64Arg(v any) *int64 {
	n, ok := v.(float64)
	if !ok {
		return nil
	}
	id := int64(n)
	return &id
}

// listReply renders tickets found by FindTasks: total is how many matched,
// tasks the ones returned. When some were left out it says so and adds
// narrowHint, the tool's advice on getting a shorter list.
func listReply(projectID string, tasks []*domain.Task, total int, full bool, narrowHint string) (string, error) {
	reply := map[string]any{"total": total}
	if projectID != "" {
		reply["project_id"] = strings.ToUpper(projectID)
	}
	if total > len(tasks) {
		reply["truncated"] = true
		reply["hint"] = fmt.Sprintf("%d more not shown: %s", total-len(tasks), narrowHint)
	}
	reply["returned"] = len(tasks)
	if full {
		if tasks == nil {
			tasks = []*domain.Task{}
		}
		reply["tasks"] = tasks
	} else {
		reply["tasks"] = rowsOf(tasks)
	}
	return compactJSON(reply)
}

func compactJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

func CallTool(svc *domain.Service, name string, args map[string]any) (string, error) {
	str := func(key string) string {
		if v, ok := args[key].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}
	toJSON := func(v any) (string, error) {
		b, err := json.MarshalIndent(v, "", "  ")
		return string(b), err
	}

	switch name {
	case "list_projects":
		projects, err := svc.ListProjects()
		if err != nil {
			return "", err
		}
		counts, err := svc.TaskCounts()
		if err != nil {
			return "", err
		}
		rows := make([]projectRow, len(projects))
		for i, p := range projects {
			rows[i] = projectRow{ID: p.ID, Name: p.Name, RepoPath: p.RepoPath, SprintMode: p.SprintMode, SyncOrigin: p.SyncOrigin}
			for status, n := range counts[p.ID] {
				rows[i].Total += n
				if status != domain.StatusDone {
					rows[i].Open += n
				}
			}
		}
		return compactJSON(map[string]any{"total": len(rows), "projects": rows})

	case "list_tasks":
		pid := str("project_id")
		if pid == "" {
			pid = DetectProjectID(svc)
		}
		if pid == "" {
			return "", errors.New("project_id required — provide it or run `KeroAgile project add --repo <remote-url>` to enable auto-detection")
		}
		statuses, err := statusFilter(str("status"))
		if err != nil {
			return "", err
		}
		full, err := listDetail(str("detail"))
		if err != nil {
			return "", err
		}
		tasks, total, err := svc.FindTasks(domain.TaskQuery{
			ProjectID:  pid,
			Statuses:   statuses,
			AssigneeID: str("assignee_id"),
			Label:      str("label"),
			SprintID:   int64Arg(args["sprint_id"]),
			Limit:      listLimit(args["limit"], defaultListLimit, maxListLimit),
		})
		if err != nil {
			return "", err
		}
		hint := fmt.Sprintf("narrow with status, assignee_id, label or sprint_id, or raise limit (max %d)", maxListLimit)
		return listReply(pid, tasks, total, full, hint)

	case "search_tasks":
		statuses, err := statusFilter(str("status"))
		if err != nil {
			return "", err
		}
		q := domain.TaskQuery{
			Text:       str("query"),
			ProjectID:  str("project_id"),
			Statuses:   statuses,
			AssigneeID: str("assignee_id"),
			Label:      str("label"),
			Limit:      listLimit(args["limit"], defaultSearchLimit, maxSearchLimit),
		}
		if q.Text == "" && len(q.Statuses) == 0 && q.AssigneeID == "" && q.Label == "" {
			// Otherwise the answer is every ticket on the board.
			return "", errors.New("give at least one of query, status, assignee_id or label; to list a whole project use list_tasks")
		}
		tasks, total, err := svc.FindTasks(q)
		if err != nil {
			return "", err
		}
		hint := fmt.Sprintf("make the search more specific, or raise limit (max %d)", maxSearchLimit)
		return listReply(q.ProjectID, tasks, total, false, hint)

	case "get_task":
		tid := str("task_id")
		if tid == "" {
			return "", errors.New("task_id is required")
		}
		task, err := svc.GetTask(tid)
		if err != nil {
			return "", err
		}
		return toJSON(task)

	case "create_task":
		title := str("title")
		if title == "" {
			return "", errors.New("title is required")
		}
		pid := str("project_id")
		if pid == "" {
			pid = DetectProjectID(svc)
		}
		if pid == "" {
			return "", errors.New("project_id required — provide it or run `KeroAgile project add --repo <remote-url>` to enable auto-detection")
		}
		opts := domain.TaskCreateOpts{
			Priority: domain.Priority(str("priority")),
			Status:   domain.Status(str("status")),
		}
		if aid := str("assignee_id"); aid != "" {
			opts.AssigneeID = aid
		} else {
			users, _ := svc.ListUsers()
			opts.AssigneeID = domain.SuggestAssignee(title, users, "")
		}
		if pts, ok := args["points"].(float64); ok {
			n := int(pts)
			opts.Points = &n
		}
		if lbls, ok := args["labels"].([]any); ok {
			for _, l := range lbls {
				if s, ok := l.(string); ok && s != "" {
					opts.Labels = append(opts.Labels, s)
				}
			}
		}
		if sid, ok := args["sprint_id"].(float64); ok {
			id := int64(sid)
			opts.SprintID = &id
		}
		task, err := svc.CreateTask(title, str("description"), pid, opts)
		if err != nil {
			return "", err
		}
		return toJSON(task)

	case "create_project":
		id := str("id")
		name := str("name")
		if id == "" || name == "" {
			return "", errors.New("id and name are required")
		}
		if err := svc.CreateProject(id, name, str("repo_path")); err != nil {
			return "", err
		}
		return toJSON(map[string]any{"created": strings.ToUpper(id), "name": name})

	case "create_sprint":
		name := str("name")
		if name == "" {
			return "", errors.New("name is required")
		}
		pid := str("project_id")
		if pid == "" {
			pid = DetectProjectID(svc)
		}
		if pid == "" {
			return "", errors.New("project_id required — provide it or run `KeroAgile project add --repo <remote-url>` to enable auto-detection")
		}
		var start, end *time.Time
		if s := str("start_date"); s != "" {
			t, err := time.Parse("2006-01-02", s)
			if err != nil {
				return "", fmt.Errorf("invalid start_date: %w", err)
			}
			start = &t
		}
		if s := str("end_date"); s != "" {
			t, err := time.Parse("2006-01-02", s)
			if err != nil {
				return "", fmt.Errorf("invalid end_date: %w", err)
			}
			end = &t
		}
		sprint, err := svc.CreateSprint(name, pid, start, end)
		if err != nil {
			return "", err
		}
		return toJSON(sprint)

	case "update_task":
		tid := str("task_id")
		if tid == "" {
			return "", errors.New("task_id is required")
		}
		// Build a patch from what was passed. Fields left out are not sent to
		// the store at all, so they keep whatever value it holds now.
		var p domain.TaskPatch
		named := map[string]bool{} // fields this call sets, to catch set-and-clear
		if v := str("title"); v != "" {
			p.Title = domain.To(v)
		}
		if v := str("description"); v != "" {
			p.Description, named["description"] = domain.To(v), true
		}
		if v := str("priority"); v != "" {
			p.Priority = domain.To(domain.Priority(v))
		}
		if v := str("status"); v != "" {
			p.Status = domain.To(domain.Status(v))
		}
		if _, ok := args["assignee_id"]; ok {
			v := str("assignee_id") // "" means nobody
			p.AssigneeID, named["assignee_id"] = domain.To(&v), true
		}
		if pts, ok := args["points"].(float64); ok {
			n := int(pts)
			p.Points, named["points"] = domain.To(&n), true
		}
		if lbls, ok := args["labels"].([]any); ok {
			var labels []string
			for _, l := range lbls {
				if s, ok := l.(string); ok && s != "" {
					labels = append(labels, s)
				}
			}
			p.Labels, named["labels"] = domain.To(labels), true
		}
		if id := int64Arg(args["sprint_id"]); id != nil {
			p.SprintID, named["sprint_id"] = domain.To(id), true
		}
		if list, ok := args["clear"].([]any); ok {
			for _, item := range list {
				field, _ := item.(string)
				if named[field] {
					return "", fmt.Errorf("%s is both set and listed in clear", field)
				}
				switch field {
				case "description":
					p.Description = domain.To("")
				case "points":
					p.Points = domain.Clear[int]()
				case "labels":
					p.Labels = domain.To([]string(nil))
				case "assignee_id":
					p.AssigneeID = domain.Clear[string]()
				case "sprint_id":
					p.SprintID = domain.Clear[int64]()
				default:
					return "", fmt.Errorf("clear %q: only %s can be cleared", field, strings.Join(clearable, ", "))
				}
			}
		}
		if v := str("expected_updated_at"); v != "" {
			when, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				return "", fmt.Errorf("expected_updated_at %q: pass the updated_at exactly as get_task returned it", v)
			}
			p.IfUpdatedAt = &when
		}
		updated, err := svc.PatchTask(tid, p)
		if err != nil {
			return "", err
		}
		return toJSON(updated)

	case "move_task":
		tid := str("task_id")
		status := str("status")
		if tid == "" || status == "" {
			return "", errors.New("task_id and status are required")
		}
		task, err := svc.MoveTask(tid, domain.Status(status))
		if err != nil {
			return "", err
		}
		return toJSON(task)

	case "delete_task":
		tid := str("task_id")
		if tid == "" {
			return "", errors.New("task_id is required")
		}
		if err := svc.DeleteTask(tid); err != nil {
			return "", err
		}
		return toJSON(map[string]any{"deleted": tid})

	case "link_branch":
		tid := str("task_id")
		branch := str("branch")
		if tid == "" || branch == "" {
			return "", errors.New("task_id and branch are required")
		}
		if err := svc.LinkBranch(tid, branch); err != nil {
			return "", err
		}
		return toJSON(map[string]any{"linked": tid, "branch": branch})

	case "list_users":
		users, err := svc.ListUsers()
		if err != nil {
			return "", err
		}
		return toJSON(users)

	case "get_sprint":
		// Direct lookup by sprint_id if provided.
		if sid, ok := args["sprint_id"].(float64); ok {
			id := int64(sid)
			sp, err := svc.GetSprint(id)
			if err != nil {
				return "", err
			}
			tasks, err := svc.ListTasks(sp.ProjectID, domain.TaskFilters{SprintID: &sp.ID})
			if err != nil {
				return "", err
			}
			return compactJSON(map[string]any{"sprint": sp, "tasks": rowsOf(tasks)})
		}
		// Fall back to active-sprint lookup via project_id.
		pid := str("project_id")
		if pid == "" {
			pid = DetectProjectID(svc)
		}
		if pid == "" {
			return "", errors.New("sprint_id or project_id required — provide sprint_id, pass project_id, or run `KeroAgile project add --repo <remote-url>` to enable auto-detection")
		}
		sprints, err := svc.ListSprints(pid)
		if err != nil {
			return "", err
		}
		var active *domain.Sprint
		for _, sp := range sprints {
			if sp.Status == domain.SprintActive {
				active = sp
				break
			}
		}
		if active == nil {
			return `{"active_sprint": null, "message": "no active sprint"}`, nil
		}
		tasks, err := svc.ListTasks(pid, domain.TaskFilters{SprintID: &active.ID})
		if err != nil {
			return "", err
		}
		return compactJSON(map[string]any{"sprint": active, "tasks": rowsOf(tasks)})

	case "add_blocker":
		taskID := str("task_id")
		blockedBy := str("blocked_by")
		if taskID == "" || blockedBy == "" {
			return "", errors.New("task_id and blocked_by are required")
		}
		if err := svc.AddDep(taskID, blockedBy); err != nil {
			return "", err
		}
		return toJSON(map[string]any{"blocker": taskID, "blocked": blockedBy, "added": true})

	case "remove_blocker":
		taskID := str("task_id")
		blockedBy := str("blocked_by")
		if taskID == "" || blockedBy == "" {
			return "", errors.New("task_id and blocked_by are required")
		}
		if err := svc.RemoveDep(taskID, blockedBy); err != nil {
			return "", err
		}
		return toJSON(map[string]any{"blocker": taskID, "blocked": blockedBy, "removed": true})

	case "assign_task_sprint":
		taskID := str("task_id")
		if taskID == "" {
			return "", fmt.Errorf("task_id is required")
		}
		var sprintID *int64
		if v, ok := args["sprint_id"]; ok && v != nil {
			switch n := v.(type) {
			case float64:
				id := int64(n)
				sprintID = &id
			case int64:
				sprintID = &n
			}
		}
		task, err := svc.AssignTaskToSprint(taskID, sprintID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return "", fmt.Errorf("task %s not found", taskID)
			}
			return "", err
		}
		return toJSON(task)

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}
