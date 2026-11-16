package domain

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// A project names where its code lives in two separate ways:
//
//   - RepoDir, the local directory of its checkout, which git commands run in
//     and which a working directory is matched against;
//   - RepoRemote, the forge repository in normalised form ("github.com/o/r"),
//     which gh is asked about and which a clone's origin URL is matched
//     against.
//
// The older RepoPath held either one, which no single use could rely on.

// ClassifyRepo sorts an old-style repo_path value into a directory or a
// remote. A path starting with "/", "~", "." or a drive letter is a
// directory; anything else is a remote.
func ClassifyRepo(s string) (dir, remote string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if looksLikeDir(s) {
		return s, ""
	}
	return "", NormalizeRemote(s)
}

func looksLikeDir(s string) bool {
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~") || strings.HasPrefix(s, ".") {
		return true
	}
	// C:\ or C:/
	return len(s) >= 3 && s[1] == ':' && (s[2] == '\\' || s[2] == '/') &&
		((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z'))
}

// NormalizeRemote turns any spelling of a git remote into "host/owner/repo":
// https and ssh URLs, scp-style "git@host:owner/repo", and "host/owner/repo"
// itself. A bare "owner/repo" is taken to be on github.com. The host is
// lower-cased; owner and repository keep their case.
func NormalizeRemote(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
		if at := strings.Index(s, "@"); at >= 0 && at < strings.Index(s+"/", "/") {
			s = s[at+1:] // user info
		}
	} else if at := strings.Index(s, "@"); at >= 0 {
		// scp-style: git@host:owner/repo
		s = strings.Replace(s[at+1:], ":", "/", 1)
	}
	s = strings.TrimRight(s, "/")
	s = strings.TrimSuffix(s, ".git")
	parts := strings.Split(s, "/")
	if len(parts) == 2 {
		parts = append([]string{"github.com"}, parts...)
	}
	host := parts[0]
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i] // a port
	}
	parts[0] = strings.ToLower(host)
	return strings.Join(parts, "/")
}

// ExpandHome replaces a leading "~" with home.
func ExpandHome(dir, home string) string {
	if dir == "~" {
		return home
	}
	if strings.HasPrefix(dir, "~/") && home != "" {
		return home + dir[1:]
	}
	return dir
}

// cleanDir makes a directory comparable: home expanded, forward slashes,
// no trailing separator.
func cleanDir(dir, home string) string {
	dir = strings.ReplaceAll(ExpandHome(strings.TrimSpace(dir), home), `\`, "/")
	if dir == "" {
		return ""
	}
	return path.Clean(dir)
}

// MatchProject finds the one project a working directory belongs to.
//
// A project whose RepoDir is cwd or contains it wins, the deepest first, so a
// project in ~/x/y beats one in ~/x. Only when no directory matches is the
// clone's remote compared with each project's RepoRemote. Two projects that
// match equally well are an ErrInvalid naming both: the caller must not
// guess, because the answer decides where files are kept and uploaded. No
// match is ErrNotFound.
func MatchProject(projects []*Project, cwd, remote, home string) (*Project, error) {
	cwd = cleanDir(cwd, home)
	if cwd != "" {
		var best []*Project
		bestLen := -1
		for _, p := range projects {
			dir := cleanDir(p.RepoDir, home)
			if dir == "" || !(cwd == dir || strings.HasPrefix(cwd, strings.TrimSuffix(dir, "/")+"/")) {
				continue
			}
			switch {
			case len(dir) > bestLen:
				best, bestLen = []*Project{p}, len(dir)
			case len(dir) == bestLen:
				best = append(best, p)
			}
		}
		if p, err := onlyOne(best, "directory "+cwd); p != nil || err != nil {
			return p, err
		}
	}
	if want := NormalizeRemote(remote); want != "" {
		var hits []*Project
		for _, p := range projects {
			if p.RepoRemote != "" && strings.EqualFold(NormalizeRemote(p.RepoRemote), want) {
				hits = append(hits, p)
			}
		}
		if p, err := onlyOne(hits, "remote "+want); p != nil || err != nil {
			return p, err
		}
	}
	return nil, fmt.Errorf("no project for directory %q: %w", cwd, ErrNotFound)
}

func onlyOne(ps []*Project, what string) (*Project, error) {
	switch len(ps) {
	case 0:
		return nil, nil
	case 1:
		return ps[0], nil
	}
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	sort.Strings(ids)
	return nil, fmt.Errorf("%w project: %s matches %s equally; give each project its own directory with `KeroAgile project set`",
		ErrInvalid, what, strings.Join(ids, " and "))
}

// ValidBranchName refuses a branch name that git or gh could read as an
// option, or that git does not allow. The empty name is accepted: callers
// use it to clear a branch.
func ValidBranchName(b string) error {
	if b == "" {
		return nil
	}
	bad := func(why string) error {
		return fmt.Errorf("%w branch %q: %s", ErrInvalid, b, why)
	}
	if strings.HasPrefix(b, "-") {
		return bad("must not start with -")
	}
	if strings.Contains(b, "..") {
		return bad("must not contain ..")
	}
	for _, r := range b {
		if r <= ' ' || r == 0x7f {
			return bad("must not contain spaces or control characters")
		}
		if strings.ContainsRune(`~^:?*[\`, r) {
			return bad("must not contain any of ~ ^ : ? * [ \\")
		}
	}
	return nil
}
