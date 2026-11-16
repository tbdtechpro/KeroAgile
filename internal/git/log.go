package git

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
)

// LogEntry is one commit as the scan reads it.
type LogEntry struct {
	SHA     string
	Subject string
	Body    string
}

func (c *Client) gitCtx(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", c.RepoPath}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// HeadBranch is the checked-out branch, or "" when HEAD is detached.
func (c *Client) HeadBranch(ctx context.Context) (string, error) {
	return c.gitCtx(ctx, "branch", "--show-current")
}

// TopLevel is the root of the checkout holding dir.
func TopLevel(ctx context.Context, dir string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	return strings.TrimSpace(string(out)), err
}

// LogSince returns the commits on HEAD after since, newest first. When since
// is empty, or is no longer an ancestor of HEAD (history was rewritten), it
// returns the last max commits instead.
func (c *Client) LogSince(ctx context.Context, since string, max int) ([]LogEntry, error) {
	if _, err := c.gitCtx(ctx, "rev-parse", "--verify", "-q", "HEAD"); err != nil {
		return nil, nil // no commits yet
	}
	args := []string{"log", "--format=%H%x1f%s%x1f%b%x1e", "-n", strconv.Itoa(max)}
	if since != "" && c.IsAncestor(ctx, since) {
		args = append(args, since+"..HEAD")
	} else {
		args = append(args, "HEAD")
	}
	out, err := c.gitCtx(ctx, args...)
	if err != nil {
		return nil, err
	}
	var entries []LogEntry
	for _, rec := range strings.Split(out, "\x1e") {
		parts := strings.SplitN(strings.TrimLeft(rec, "\n"), "\x1f", 3)
		if len(parts) < 2 || parts[0] == "" {
			continue
		}
		e := LogEntry{SHA: parts[0], Subject: parts[1]}
		if len(parts) == 3 {
			e.Body = strings.TrimSpace(parts[2])
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// IsAncestor reports whether sha is a commit HEAD contains.
func (c *Client) IsAncestor(ctx context.Context, sha string) bool {
	if !isHex(sha) {
		return false
	}
	_, err := c.gitCtx(ctx, "merge-base", "--is-ancestor", sha, "HEAD")
	return err == nil
}

// Orphaned reports whether sha is a commit this clone has that no branch,
// local or remote-tracking, contains: rebased or squashed away. A commit the
// clone does not have is not orphaned; it is unknown here.
func (c *Client) Orphaned(ctx context.Context, sha string) (bool, error) {
	if !isHex(sha) {
		return false, nil
	}
	if _, err := c.gitCtx(ctx, "cat-file", "-e", sha+"^{commit}"); err != nil {
		return false, nil
	}
	out, err := c.gitCtx(ctx, "branch", "-a", "--contains", sha, "--format=%(refname)")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

func isHex(s string) bool {
	return len(s) >= 7 && strings.Trim(strings.ToLower(s), "0123456789abcdef") == ""
}

// PR is a pull request as the scan reads it.
type PR struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	HeadRefName string `json:"headRefName"`
	State       string `json:"state"` // OPEN, MERGED, CLOSED
	URL         string `json:"url"`
}

// ListPRs returns the repository's latest pull requests in any state.
func (r Repo) ListPRs(ctx context.Context, limit int) ([]PR, error) {
	if r.Remote == "" && r.Dir == "" {
		return nil, nil
	}
	args := []string{"pr", "list", "--state", "all", "--limit", strconv.Itoa(limit),
		"--json", "number,title,headRefName,state,url"}
	if r.Remote != "" {
		args = append(args, "-R", r.Remote)
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	if r.Remote == "" {
		cmd.Dir = r.Dir
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var prs []PR
	return prs, json.Unmarshal(out, &prs)
}
