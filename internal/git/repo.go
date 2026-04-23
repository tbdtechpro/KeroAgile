package git

import (
	"fmt"
	"strings"
)

type Commit struct {
	Hash    string
	Subject string
	When    string
}

// CurrentBranch returns the current branch name in the repo.
func (c *Client) CurrentBranch() (string, error) {
	return c.execGit("branch", "--show-current")
}

// CommitLog returns the last n commits on the given branch.
func (c *Client) CommitLog(branch string, n int) ([]Commit, error) {
	out, err := c.execGit(
		"log", branch,
		"--oneline",
		"-"+fmt.Sprintf("%d", n),
		`--format=%h|||%s|||%cr`,
	)
	if err != nil || out == "" {
		return nil, err
	}
	var commits []Commit
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|||", 3)
		if len(parts) != 3 {
			continue
		}
		commits = append(commits, Commit{
			Hash:    parts[0],
			Subject: parts[1],
			When:    parts[2],
		})
	}
	return commits, nil
}

// AutoLinkTaskID checks if the branch name contains a task ID (e.g. feature/KA-001-foo → "KA-001").
func AutoLinkTaskID(branch string) string {
	parts := strings.Split(branch, "/")
	for _, part := range parts {
		segments := strings.Split(part, "-")
		if len(segments) >= 2 {
			prefix := segments[0]
			if prefix == strings.ToUpper(prefix) && len(prefix) >= 2 {
				if isDigits(segments[1]) {
					return prefix + "-" + segments[1]
				}
			}
		}
	}
	return ""
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
