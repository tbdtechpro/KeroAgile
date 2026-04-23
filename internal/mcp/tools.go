package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// toolDef describes one MCP tool.
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
			Description: "List all KeroAgile projects.",
			InputSchema: obj(map[string]any{}, nil),
		},
		{
			Name:        "list_tasks",
			Description: "List tasks. Auto-detects project_id from git remote if not provided.",
			InputSchema: obj(map[string]any{
				"project_id":  str("Project ID (auto-detected from git remote if omitted)"),
				"status":      str("Filter by status: backlog|todo|in_progress|review|done"),
				"assignee_id": str("Filter by assignee user ID"),
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
			Description: "Create a new task. Auto-detects project_id from git remote if not provided.",
			InputSchema: obj(map[string]any{
				"title":       str("Task title (required)"),
				"project_id":  str("Project ID (auto-detected if omitted)"),
				"description": str("Task description"),
				"priority":    str("low|medium|high|critical (default: medium)"),
				"status":      str("backlog|todo|in_progress|review|done (default: backlog)"),
				"assignee_id": str("Assignee user ID"),
				"points":      map[string]any{"type": "integer", "description": "Story points"},
				"labels":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Labels"},
			}, []string{"title"}),
		},
		{
			Name:        "update_task",
			Description: "Update an existing task's fields.",
			InputSchema: obj(map[string]any{
				"task_id":     str("Task ID (required)"),
				"title":       str("New title"),
				"description": str("New description"),
				"priority":    str("low|medium|high|critical"),
				"status":      str("backlog|todo|in_progress|review|done"),
				"assignee_id": str("Assignee user ID (empty string to clear)"),
				"points":      map[string]any{"type": "integer", "description": "Story points"},
				"labels":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Labels"},
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
			Description: "Get the active sprint for a project and its tasks. Auto-detects project_id from git remote.",
			InputSchema: obj(map[string]any{
				"project_id": str("Project ID (auto-detected if omitted)"),
			}, nil),
		},
		{
			Name:        "add_blocker",
			Description: "Mark task A as blocking task B (A must be done before B can start).",
			InputSchema: obj(map[string]any{
				"blocker_id": str("ID of the task that is blocking"),
				"blocked_id": str("ID of the task that is blocked"),
			}, []string{"blocker_id", "blocked_id"}),
		},
		{
			Name:        "remove_blocker",
			Description: "Remove a blocker relationship between two tasks.",
			InputSchema: obj(map[string]any{
				"blocker_id": str("ID of the blocking task"),
				"blocked_id": str("ID of the blocked task"),
			}, []string{"blocker_id", "blocked_id"}),
		},
	}
}

// CallTool dispatches a tool call by name and returns JSON text (exported for testing).
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
		return toJSON(projects)

	case "list_tasks":
		pid := str("project_id")
		if pid == "" {
			pid = DetectProjectID(svc)
		}
		if pid == "" {
			return "", errors.New("project_id required — provide it or run `KeroAgile project add --repo <remote-url>` to enable auto-detection")
		}
		filters := domain.TaskFilters{}
		if s := str("status"); s != "" {
			st := domain.Status(s)
			filters.Status = &st
		}
		if a := str("assignee_id"); a != "" {
			filters.AssigneeID = &a
		}
		tasks, err := svc.ListTasks(pid, filters)
		if err != nil {
			return "", err
		}
		return toJSON(tasks)

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
			Priority:   domain.Priority(str("priority")),
			Status:     domain.Status(str("status")),
			AssigneeID: str("assignee_id"),
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
		task, err := svc.CreateTask(title, str("description"), pid, opts)
		if err != nil {
			return "", err
		}
		return toJSON(task)

	case "update_task":
		tid := str("task_id")
		if tid == "" {
			return "", errors.New("task_id is required")
		}
		task, err := svc.GetTask(tid)
		if err != nil {
			return "", err
		}
		if v := str("title"); v != "" {
			task.Title = v
		}
		if v := str("description"); v != "" {
			task.Description = v
		}
		if v := str("priority"); v != "" {
			task.Priority = domain.Priority(v)
		}
		if v := str("status"); v != "" {
			task.Status = domain.Status(v)
		}
		if _, ok := args["assignee_id"]; ok {
			if v := str("assignee_id"); v != "" {
				task.AssigneeID = &v
			} else {
				task.AssigneeID = nil
			}
		}
		if pts, ok := args["points"].(float64); ok {
			n := int(pts)
			task.Points = &n
		}
		if lbls, ok := args["labels"].([]any); ok {
			task.Labels = nil
			for _, l := range lbls {
				if s, ok := l.(string); ok && s != "" {
					task.Labels = append(task.Labels, s)
				}
			}
		}
		updated, err := svc.UpdateTask(task)
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
		return fmt.Sprintf(`{"deleted": "%s"}`, tid), nil

	case "link_branch":
		tid := str("task_id")
		branch := str("branch")
		if tid == "" || branch == "" {
			return "", errors.New("task_id and branch are required")
		}
		if err := svc.LinkBranch(tid, branch); err != nil {
			return "", err
		}
		return fmt.Sprintf(`{"linked": "%s", "branch": "%s"}`, tid, branch), nil

	case "list_users":
		users, err := svc.ListUsers()
		if err != nil {
			return "", err
		}
		return toJSON(users)

	case "get_sprint":
		pid := str("project_id")
		if pid == "" {
			pid = DetectProjectID(svc)
		}
		if pid == "" {
			return "", errors.New("project_id required — provide it or run `KeroAgile project add --repo <remote-url>` to enable auto-detection")
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
		return toJSON(map[string]any{"sprint": active, "tasks": tasks})

	case "add_blocker":
		blockerID := str("blocker_id")
		blockedID := str("blocked_id")
		if blockerID == "" || blockedID == "" {
			return "", errors.New("blocker_id and blocked_id are required")
		}
		if err := svc.AddDep(blockerID, blockedID); err != nil {
			return "", err
		}
		return fmt.Sprintf(`{"blocker": "%s", "blocked": "%s", "added": true}`, blockerID, blockedID), nil

	case "remove_blocker":
		blockerID := str("blocker_id")
		blockedID := str("blocked_id")
		if blockerID == "" || blockedID == "" {
			return "", errors.New("blocker_id and blocked_id are required")
		}
		if err := svc.RemoveDep(blockerID, blockedID); err != nil {
			return "", err
		}
		return fmt.Sprintf(`{"blocker": "%s", "blocked": "%s", "removed": true}`, blockerID, blockedID), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}
