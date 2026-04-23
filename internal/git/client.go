package git

import (
	"os/exec"
	"strings"
)

type Client struct {
	RepoPath string
}

func New(repoPath string) *Client {
	return &Client{RepoPath: repoPath}
}

func (c *Client) execGit(args ...string) (string, error) {
	fullArgs := append([]string{"-C", c.RepoPath}, args...)
	out, err := exec.Command("git", fullArgs...).Output()
	return strings.TrimSpace(string(out)), err
}

func execGH(args ...string) (string, error) {
	out, err := exec.Command("gh", args...).Output()
	return strings.TrimSpace(string(out)), err
}

// Available returns true if the git binary is found.
func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// GHAvailable returns true if the gh CLI is found.
func GHAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}
