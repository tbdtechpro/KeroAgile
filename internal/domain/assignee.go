package domain

import "strings"

var humanKeywords = []string{
	"design", "plan", "research", "qa", "quality", "asset", "mockup",
	"wireframe", "spec", "document", "review", "interview", "meeting",
	"analyze", "discovery", "ux",
}

var agentKeywords = []string{
	"implement", "build", "code", "refactor", "fix", "debug", "integrate",
	"develop", "create", "update", "add", "migrate", "deploy", "optimize",
	"parse", "generate", "render",
}

// SuggestAssignee returns the ID of the most appropriate assignee for a task
// with the given title. Human-signal keywords take priority; agent-signal keywords
// route to the first agent user when one exists; everything else returns defaultID.
func SuggestAssignee(title string, users []*User, defaultID string) string {
	lower := strings.ToLower(title)

	for _, kw := range humanKeywords {
		if strings.Contains(lower, kw) {
			return defaultID
		}
	}

	isAgentTitle := false
	for _, kw := range agentKeywords {
		if strings.Contains(lower, kw) {
			isAgentTitle = true
			break
		}
	}
	if !isAgentTitle && strings.Contains(lower, "write") && strings.Contains(lower, "test") {
		isAgentTitle = true
	}

	if isAgentTitle {
		for _, u := range users {
			if u.IsAgent {
				return u.ID
			}
		}
	}

	return defaultID
}
