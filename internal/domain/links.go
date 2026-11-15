package domain

import (
	"fmt"
	"strings"
	"time"
)

// A link ties a ticket to something in git or on the forge: a commit, a
// branch, a pull request or a tag. A ticket can have many. The older Branch,
// PRNumber and PRMerged fields keep their meaning; links are additional.

type LinkKind string

const (
	LinkCommit LinkKind = "commit"
	LinkBranch LinkKind = "branch"
	LinkPR     LinkKind = "pr"
	LinkTag    LinkKind = "tag"
)

// How a link was recorded.
const (
	SourceManual = "manual"
	SourceScan   = "scan"
)

// LinkSuperseded is the state of a commit link whose commit is no longer on
// any branch (rebased or squashed away).
const LinkSuperseded = "superseded"

type Link struct {
	ID        int64     `json:"id"`
	TaskID    string    `json:"task_id"`
	Kind      LinkKind  `json:"kind"`
	Repo      string    `json:"repo,omitempty"` // host/owner/repo
	Ref       string    `json:"ref"`            // SHA, branch, PR number or tag
	Title     string    `json:"title,omitempty"`
	URL       string    `json:"url,omitempty"`
	State     string    `json:"state,omitempty"` // a PR's open | merged | closed; a commit's superseded
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

// AddLink records a link on a ticket. Recording one that exists (same kind,
// repository and ref) changes nothing but its title, URL and state, and
// reports created false. The repository defaults to the ticket's project's;
// a URL is built for github.com repositories.
func (s *Service) AddLink(taskID string, l Link) (*Link, bool, error) {
	t, err := s.task(taskID)
	if err != nil {
		return nil, false, err
	}
	l.TaskID = t.ID
	l.Ref = strings.TrimSpace(l.Ref)
	switch l.Kind {
	case LinkCommit:
		l.Ref = strings.ToLower(l.Ref)
		if len(l.Ref) < 7 || len(l.Ref) > 64 || strings.Trim(l.Ref, "0123456789abcdef") != "" {
			return nil, false, fmt.Errorf("%w commit %q: give a hexadecimal SHA of at least 7 characters", ErrInvalid, l.Ref)
		}
	case LinkPR:
		l.Ref = strings.TrimPrefix(l.Ref, "#")
		if l.Ref == "" || strings.Trim(l.Ref, "0123456789") != "" {
			return nil, false, fmt.Errorf("%w pull request %q: give its number", ErrInvalid, l.Ref)
		}
	case LinkBranch:
		if l.Ref == "" {
			return nil, false, fmt.Errorf("%w branch: give its name", ErrInvalid)
		}
		if err := ValidBranchName(l.Ref); err != nil {
			return nil, false, err
		}
	case LinkTag:
		if l.Ref == "" || ValidBranchName(l.Ref) != nil {
			return nil, false, fmt.Errorf("%w tag %q: give its name", ErrInvalid, l.Ref)
		}
	default:
		return nil, false, fmt.Errorf("%w link kind %q: use commit, branch, pr or tag", ErrInvalid, l.Kind)
	}
	if l.Repo == "" {
		if p, err := s.store.GetProject(t.ProjectID); err == nil {
			l.Repo = p.RepoRemote
		}
	}
	l.Repo = NormalizeRemote(l.Repo)
	if l.URL == "" {
		l.URL = linkURL(l)
	}
	l.Title = OneLine(l.Title)
	l.State = strings.ToLower(strings.TrimSpace(l.State))
	if l.Source == "" {
		l.Source = SourceManual
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	}
	created, err := s.store.UpsertLink(&l)
	if err != nil {
		return nil, false, err
	}
	return &l, created, nil
}

func linkURL(l Link) string {
	if !strings.HasPrefix(l.Repo, "github.com/") {
		return ""
	}
	base := "https://" + l.Repo
	switch l.Kind {
	case LinkCommit:
		return base + "/commit/" + l.Ref
	case LinkPR:
		return base + "/pull/" + l.Ref
	case LinkBranch:
		return base + "/tree/" + l.Ref
	case LinkTag:
		return base + "/releases/tag/" + l.Ref
	}
	return ""
}

// ListLinks returns a ticket's links, oldest first.
func (s *Service) ListLinks(taskID string) ([]*Link, error) {
	t, err := s.task(taskID)
	if err != nil {
		return nil, err
	}
	return s.store.ListLinks(t.ID)
}
