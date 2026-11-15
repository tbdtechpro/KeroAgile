package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// Tools for a ticket's records: notes, links and preserved files.

func strSchema(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func objSchema(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

func recordToolList() []toolDef {
	return []toolDef{
		{
			Name: "add_note",
			Description: "Add a note to a ticket: what was found, decided or tried. Notes are dated, signed and kept in order; " +
				"they are never edited, so use them instead of rewriting the description to record progress.",
			InputSchema: objSchema(map[string]any{
				"task_id": strSchema("Ticket ID, e.g. KA-008"),
				"body":    strSchema("The note"),
			}, "task_id", "body"),
		},
		{
			Name: "add_link",
			Description: "Link a ticket to a commit, branch, pull request or tag. A ticket can have many links; linking the same thing twice is harmless. " +
				"Commits whose message names the ticket are linked automatically by the Stop hook's scan.",
			InputSchema: objSchema(map[string]any{
				"task_id": strSchema("Ticket ID"),
				"kind":    strSchema("commit | branch | pr | tag"),
				"ref":     strSchema("The commit SHA, branch name, PR number or tag"),
				"repo":    strSchema("host/owner/repo (default: the ticket's project's remote)"),
				"title":   strSchema("Commit subject or PR title"),
			}, "task_id", "kind", "ref"),
		},
		{
			Name: "set_project",
			Description: "Change a project's settings: its local checkout directory, its forge repository, and what happens to files kept for its tickets " +
				"(artifact_policy: remote = keep and upload to the remote store, local = keep on this machine only, off = keep nothing, default = follow the configured default).",
			InputSchema: objSchema(map[string]any{
				"project_id":      strSchema("Project ID"),
				"repo_dir":        strSchema("Local checkout directory"),
				"repo_remote":     strSchema("Forge repository, e.g. github.com/owner/repo"),
				"artifact_policy": strSchema("remote | local | off | default (empty strings are ignored)"),
			}, "project_id"),
		},
		{
			Name: "attach_file",
			Description: "Keep a file against a ticket: a test script, a log, a screenshot, audio or video produced while working it outside the repo. " +
				"The file is copied in, so it survives the session's scratch folder being wiped. Files that belong in the repo are committed instead.",
			InputSchema: objSchema(map[string]any{
				"task_id": strSchema("Ticket ID, e.g. KA-008"),
				"path":    strSchema("Path to the file on this machine"),
				"kind":    strSchema("script, log, image, audio, video, document or other (default: from the file name)"),
				"note":    strSchema("A caption: what the file is or shows"),
			}, "task_id", "path"),
		},
		{
			Name:        "list_artifacts",
			Description: "List the files kept on a ticket, or a project's unfiled files (kept by the sweep, waiting to be filed on a ticket or discarded).",
			InputSchema: objSchema(map[string]any{
				"task_id":    strSchema("A ticket's files"),
				"project_id": strSchema("A project's files; with unfiled=true only those on no ticket"),
				"unfiled":    map[string]any{"type": "boolean", "description": "Only files on no ticket (with project_id)"},
				"limit":      map[string]any{"type": "integer", "description": "Most files to return (default 100, max 500). The reply says when the list was cut"},
			}),
		},
		{
			Name:        "fetch_artifact",
			Description: "Get a kept file onto this machine and return its local path; files held only on the remote store are downloaded.",
			InputSchema: objSchema(map[string]any{
				"artifact_id": map[string]any{"type": "integer", "description": "File ID from list_artifacts or get_task"},
			}, "artifact_id"),
		},
		{
			Name:        "file_artifact",
			Description: "Put a kept file (usually an unfiled one) on a ticket of its project.",
			InputSchema: objSchema(map[string]any{
				"artifact_id": map[string]any{"type": "integer", "description": "File ID"},
				"task_id":     strSchema("Ticket ID"),
			}, "artifact_id", "task_id"),
		},
		{
			Name:        "discard_artifact",
			Description: "Discard kept files that are not worth keeping: one (artifact_id) or many (artifact_ids). The sweep will not keep the same files again.",
			InputSchema: objSchema(map[string]any{
				"artifact_id":  map[string]any{"type": "integer", "description": "File ID"},
				"artifact_ids": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Several file IDs"},
			}),
		},
	}
}

// artifactRow is one file in get_task and list_artifacts.
type artifactRow struct {
	ID     int64               `json:"id"`
	Name   string              `json:"name"`
	Kind   domain.ArtifactKind `json:"kind"`
	Size   int64               `json:"size"`
	Where  string              `json:"where"`
	TaskID *string             `json:"task_id,omitempty"`
	Note   string              `json:"note,omitempty"`
	Added  string              `json:"added"` // date only
}

func artifactRows(as []*domain.Artifact) []artifactRow {
	rows := make([]artifactRow, len(as))
	for i, a := range as {
		rows[i] = artifactRow{ID: a.ID, Name: a.Name, Kind: a.Kind, Size: a.Size, Where: a.Where, TaskID: a.TaskID,
			Note: a.Note, Added: a.CreatedAt.Format("2006-01-02")}
	}
	return rows
}

// taskView is get_task's reply: the ticket in full, with its files listed
// compactly.
type taskView struct {
	*domain.Task
	Artifacts []artifactRow `json:"artifacts,omitempty"`
}

func intArg(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), n == float64(int64(n))
	case int:
		return int64(n), true
	case int64:
		return n, true
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		return i, err == nil
	}
	return 0, false
}

// callRecordTool runs one of the record tools. ok is false when name is not
// one of them.
func callRecordTool(env Env, name string, args map[string]any) (out string, ok bool, err error) {
	svc := env.Svc
	str := func(key string) string {
		v, _ := args[key].(string)
		return strings.TrimSpace(v)
	}
	switch name {
	case "add_note":
		if str("task_id") == "" {
			return "", true, errors.New("task_id is required")
		}
		body, _ := args["body"].(string)
		n, err := svc.AddNote(str("task_id"), authorID(svc), body)
		if err != nil {
			return "", true, err
		}
		out, err := indentJSON(n)
		return out, true, err

	case "add_link":
		l, created, err := svc.AddLink(str("task_id"), domain.Link{Kind: domain.LinkKind(strings.ToLower(str("kind"))),
			Ref: str("ref"), Repo: str("repo"), Title: str("title")})
		if err != nil {
			return "", true, err
		}
		out, err := indentJSON(map[string]any{"created": created, "link": l})
		return out, true, err

	case "set_project":
		if str("project_id") == "" {
			return "", true, errors.New("project_id is required")
		}
		// Empty strings are ignored: agents send them for fields they do
		// not mean to touch. "default" resets the policy.
		var p domain.ProjectPatch
		if v := str("repo_dir"); v != "" {
			p.RepoDir = domain.To(v)
		}
		if v := str("repo_remote"); v != "" {
			p.RepoRemote = domain.To(v)
		}
		if v := strings.ToLower(str("artifact_policy")); v != "" {
			if v == "default" {
				v = ""
			}
			p.ArtifactPolicy = domain.To(domain.ArtifactPolicy(v))
		}
		proj, err := svc.PatchProject(str("project_id"), p)
		if err != nil {
			return "", true, err
		}
		out, err := compactJSON(proj)
		return out, true, err

	case "list_artifacts":
		unfiled, _ := args["unfiled"].(bool)
		list, err := svc.ListArtifacts(domain.ArtifactQuery{TaskID: str("task_id"), ProjectID: str("project_id"), Unfiled: unfiled})
		if err != nil {
			return "", true, err
		}
		limit := listLimit(args["limit"], 100, 500)
		reply := map[string]any{"total": len(list)}
		if len(list) > limit {
			list = list[:limit]
			reply["truncated"] = true
			reply["hint"] = "file or discard these (discard_artifact takes artifact_ids for many at once), then list again; or raise limit (max 500)"
		}
		reply["returned"] = len(list)
		reply["files"] = artifactRows(list)
		out, err := compactJSON(reply)
		return out, true, err

	case "discard_artifact":
		var ids []int64
		if id, ok := intArg(args["artifact_id"]); ok {
			ids = append(ids, id)
		}
		if many, ok := args["artifact_ids"].([]any); ok {
			for _, v := range many {
				id, ok := intArg(v)
				if !ok {
					return "", true, fmt.Errorf("artifact_ids: %v is not a file ID", v)
				}
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return "", true, errors.New("give artifact_id or artifact_ids")
		}
		for _, id := range ids {
			if err := svc.DiscardArtifact(id); err != nil {
				return "", true, err
			}
		}
		out, err := compactJSON(map[string]any{"discarded": ids})
		return out, true, err

	case "file_artifact", "fetch_artifact":
		id, ok := intArg(args["artifact_id"])
		if !ok {
			return "", true, errors.New("artifact_id is required: a file ID from list_artifacts or get_task")
		}
		switch name {
		case "file_artifact":
			a, err := svc.FileArtifact(id, str("task_id"))
			if err != nil {
				return "", true, err
			}
			out, err := compactJSON(artifactRows([]*domain.Artifact{a})[0])
			return out, true, err
		default:
			if env.Files == nil {
				return "", true, filesUnavailable(env)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			path, err := env.Files.Fetch(ctx, id)
			if err != nil {
				return "", true, err
			}
			out, err := compactJSON(map[string]any{"id": id, "path": path})
			return out, true, err
		}

	case "attach_file":
		if env.Files == nil {
			return "", true, filesUnavailable(env)
		}
		if str("task_id") == "" || str("path") == "" {
			return "", true, errors.New("task_id and path are required")
		}
		a, err := env.Files.Attach(str("task_id"), str("path"), domain.ArtifactKind(str("kind")), str("note"), authorID(svc))
		if err != nil {
			return "", true, err
		}
		out, err := compactJSON(artifactRows([]*domain.Artifact{a})[0])
		return out, true, err
	}
	return "", false, nil
}

func filesUnavailable(env Env) error {
	if env.FilesErr != nil {
		return fmt.Errorf("keeping files is not available in this server: %w", env.FilesErr)
	}
	return errors.New("keeping files is not available in this server")
}

func indentJSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	return string(b), err
}

// authorID is who notes and files added through MCP are signed by: the user
// named by KEROAGILE_USER, else the "claude" user when the board has one,
// else nobody. MCP has no login of its own.
func authorID(svc *domain.Service) string {
	if id := strings.TrimSpace(os.Getenv("KEROAGILE_USER")); id != "" {
		return id
	}
	if _, err := svc.GetUser("claude"); err == nil {
		return "claude"
	}
	return ""
}
