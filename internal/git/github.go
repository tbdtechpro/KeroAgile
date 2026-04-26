package git

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

type PRStatus struct {
	Number   int
	State    string // OPEN, MERGED, CLOSED
	Title    string
	URL      string
	Comments int
}

type OpenPR struct {
	Number      int    `json:"number"`
	HeadRefName string `json:"headRefName"`
	Title       string `json:"title"`
	URL         string `json:"url"`
}

// PRView fetches the state of a single PR by number.
func PRView(repoPath string, prNumber int) (*PRStatus, error) {
	out, err := execGH("pr", "view", fmt.Sprintf("%d", prNumber),
		"--json", "number,state,title,url,comments",
		"-R", repoPath,
	)
	if err != nil {
		return nil, err
	}
	var result struct {
		Number   int    `json:"number"`
		State    string `json:"state"`
		Title    string `json:"title"`
		URL      string `json:"url"`
		Comments []any  `json:"comments"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, err
	}
	return &PRStatus{
		Number:   result.Number,
		State:    result.State,
		Title:    result.Title,
		URL:      result.URL,
		Comments: len(result.Comments),
	}, nil
}

// PRForBranch returns the PR for the given branch, or nil if no PR exists.
// Returns (nil, nil) when gh reports no PR found.
func PRForBranch(repoPath, branch string) (*PRStatus, error) {
	out, err := execGH("pr", "view", branch, "--json", "number,state", "-R", repoPath)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if bytes.Contains(exitErr.Stderr, []byte("no pull requests found")) ||
				bytes.Contains(exitErr.Stderr, []byte("Could not resolve to a")) {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("gh pr view %s: %w", branch, err)
	}
	var p struct {
		State  string `json:"state"`
		Number int    `json:"number"`
	}
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		return nil, err
	}
	return &PRStatus{State: p.State, Number: p.Number}, nil
}

// ListOpenPRs returns open PRs for the repo, used for auto-detect.
func ListOpenPRs(repoPath string) ([]OpenPR, error) {
	out, err := execGH("pr", "list",
		"--json", "number,headRefName,title,url",
		"--state", "open",
		"-R", repoPath,
	)
	if err != nil {
		return nil, err
	}
	var prs []OpenPR
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return nil, err
	}
	return prs, nil
}
