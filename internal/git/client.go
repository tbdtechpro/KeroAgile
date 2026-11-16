package git

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
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

// Repo is where a project's code lives: a local checkout, a forge
// repository ("host/owner/repo"), or both.
type Repo struct {
	Dir    string
	Remote string
}

// gh runs the GitHub CLI about r: with -R when the remote is known, otherwise
// inside the checkout so gh works the repository out itself.
func (r Repo) gh(args ...string) (string, error) {
	if r.Remote == "" && r.Dir == "" {
		return "", fmt.Errorf("gh: the project has neither a directory nor a remote")
	}
	if r.Remote != "" {
		// Flags must come before "--", after which gh reads only arguments.
		at := slices.Index(args, "--")
		if at < 0 {
			at = len(args)
		}
		args = slices.Concat(args[:at], []string{"-R", r.Remote}, args[at:])
	}
	cmd := exec.Command("gh", args...)
	if r.Remote == "" {
		cmd.Dir = r.Dir
	}
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// RemoteURL returns the origin URL of the checkout in dir.
func RemoteURL(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
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

// ForProject returns where p's code lives, with "~" in its directory
// expanded.
func ForProject(p *domain.Project) Repo {
	home, _ := os.UserHomeDir()
	return Repo{Dir: domain.ExpandHome(p.RepoDir, home), Remote: p.RepoRemote}
}
