package mcp

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

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
	}
}

// callRecordTool runs one of the record tools. ok is false when name is not
// one of them.
func callRecordTool(svc *domain.Service, name string, args map[string]any) (out string, ok bool, err error) {
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
	}
	return "", false, nil
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
