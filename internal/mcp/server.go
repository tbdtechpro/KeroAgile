package mcp

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"strings"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// Request is a JSON-RPC 2.0 request (exported for testing).
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// Response is a JSON-RPC 2.0 response (exported for testing).
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	codeParseError     = -32700
	codeInvalidReq     = -32600
	codeMethodNotFound = -32601
	codeInternalError  = -32603 //nolint:deadcode,unused
	codeAppError       = -32000
)

// Serve runs the MCP JSON-RPC 2.0 server loop over stdin/stdout.
func Serve(svc *domain.Service) error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	enc := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			enc.Encode(Response{JSONRPC: "2.0", Error: &rpcError{Code: codeParseError, Message: err.Error()}}) //nolint:errcheck
			continue
		}
		resp := Dispatch(svc, req)
		enc.Encode(resp) //nolint:errcheck
	}
	return scanner.Err()
}

// Dispatch handles a single JSON-RPC request (exported for testing).
func Dispatch(svc *domain.Service, req Request) Response {
	base := Response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		base.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "keroagile", "version": "0.2.0"},
		}
	case "notifications/initialized":
		// No response needed for notifications.
		return Response{JSONRPC: "2.0"}
	case "tools/list":
		base.Result = map[string]any{"tools": toolList()}
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			base.Error = &rpcError{Code: codeInvalidReq, Message: err.Error()}
			return base
		}
		result, err := CallTool(svc, p.Name, p.Arguments)
		if err != nil {
			base.Error = &rpcError{Code: codeAppError, Message: err.Error()}
			return base
		}
		base.Result = map[string]any{
			"content": []map[string]any{{"type": "text", "text": result}},
		}
	default:
		base.Error = &rpcError{Code: codeMethodNotFound, Message: "method not found: " + req.Method}
	}
	return base
}

// DetectProjectID finds the KeroAgile project whose repo_path matches the current git remote.
func DetectProjectID(svc *domain.Service) string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	remote := strings.TrimSpace(string(out))
	projects, err := svc.ListProjects()
	if err != nil {
		return ""
	}
	for _, p := range projects {
		if p.RepoPath != "" && strings.Contains(remote, p.RepoPath) {
			return p.ID
		}
	}
	return ""
}
