// Package gitscan links tickets to the commits and pull requests that name
// them. It reads the repository a session works in, links only tickets of
// that repository's own project, and never removes a link.
package gitscan

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/git"
)

// Store is what the scan keeps between runs, and the links it revisits.
type Store interface {
	KVGet(key string) (string, bool, error)
	KVSet(key, value string) error
	LinksByRepo(repo string, kind domain.LinkKind, state string) ([]*domain.Link, error)
	SetLinkState(id int64, state string) error
}

// Result counts what one scan recorded.
type Result struct {
	Commits    int `json:"commits"`    // new commit links
	PRs        int `json:"prs"`        // pull request links, new or updated
	Superseded int `json:"superseded"` // commit links marked rewritten away
	// Warnings are steps that failed without stopping the scan: gh not
	// logged in or offline, for example.
	Warnings []string `json:"warnings,omitempty"`
}

const (
	maxLog         = 1000
	prEvery        = 10 * time.Minute
	supersedeEvery = time.Hour
	supersedeBatch = 50
	prLimit        = 100
)

// Run scans repo for project p. now decides the rate limits.
func Run(ctx context.Context, svc *domain.Service, st Store, repo git.Repo, p *domain.Project, now time.Time) (Result, error) {
	var res Result
	if p == nil || repo.Dir == "" {
		return res, errors.New("git scan: no project or no checkout")
	}
	ids := idPattern(p.ID)
	known := map[string]bool{}
	ticket := func(text string) []string {
		var out []string
		seen := map[string]bool{}
		for _, m := range ids.FindAllStringSubmatch(text, -1) {
			n, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			id := fmt.Sprintf("%s-%03d", p.ID, n)
			if seen[id] {
				continue
			}
			seen[id] = true
			ok, cached := known[id]
			if !cached {
				_, err := svc.GetTask(id)
				ok = err == nil
				known[id] = ok
			}
			if ok {
				out = append(out, id)
			}
		}
		return out
	}
	repoName := p.RepoRemote
	if repoName == "" {
		if url, err := git.RemoteURL(repo.Dir); err == nil {
			repoName = domain.NormalizeRemote(url)
		}
	}

	var err error
	c := git.New(repo.Dir)
	if branch, berr := c.HeadBranch(ctx); berr == nil && branch != "" {
		res.Commits, err = scanCommits(ctx, svc, st, c, repo.Dir, branch, repoName, ticket)
	}
	if due(st, "supersede:"+repo.Dir, supersedeEvery, now) && repoName != "" {
		n, serr := markSuperseded(ctx, st, c, repoName, "supersede-cursor:"+repoName)
		res.Superseded = n
		if serr != nil {
			res.Warnings = append(res.Warnings, "superseded check: "+domain.OneLine(serr.Error()))
		}
	}
	if due(st, "prscan:"+repoName+":"+repo.Dir, prEvery, now) {
		n, perr := scanPRs(ctx, svc, repo, repoName, ticket)
		res.PRs = n
		if perr != nil {
			res.Warnings = append(res.Warnings, "pull requests: "+domain.OneLine(perr.Error()))
		}
	}
	return res, err
}

// idPattern matches "<PROJECT>-<n>" as a word, in any letter case.
func idPattern(project string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9])` + regexp.QuoteMeta(project) + `-(\d+)\b`)
}

// due reports whether the step named key last ran longer than every ago, and
// if so records now as its last run.
func due(st Store, key string, every time.Duration, now time.Time) bool {
	if v, ok, err := st.KVGet(key); err == nil && ok {
		if last, err := time.Parse(time.RFC3339Nano, v); err == nil && now.Sub(last) < every {
			return false
		}
	}
	_ = st.KVSet(key, now.UTC().Format(time.RFC3339Nano))
	return true
}

func scanCommits(ctx context.Context, svc *domain.Service, st Store, c *git.Client, dir, branch, repo string,
	ticket func(string) []string) (int, error) {
	key := "scan:" + dir + ":" + branch
	since, _, _ := st.KVGet(key)
	entries, err := c.LogSince(ctx, since, maxLog)
	if err != nil {
		return 0, err
	}
	n := 0
	// Oldest first, so links appear in the order the work happened.
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		for _, id := range ticket(e.Subject + "\n" + e.Body) {
			_, created, err := svc.AddLink(id, domain.Link{Kind: domain.LinkCommit, Ref: e.SHA, Repo: repo,
				Title: e.Subject, Source: domain.SourceScan})
			if err != nil {
				return n, err
			}
			if created {
				n++
			}
		}
	}
	if len(entries) > 0 {
		return n, st.KVSet(key, entries[0].SHA)
	}
	return n, nil
}

// markSuperseded checks up to supersedeBatch commit links per run, carrying
// on from where the last run stopped and wrapping round, so every link is
// checked in turn however many there are.
func markSuperseded(ctx context.Context, st Store, c *git.Client, repo, cursorKey string) (int, error) {
	links, err := st.LinksByRepo(repo, domain.LinkCommit, "")
	if err != nil {
		return 0, err
	}
	var after int64
	if v, ok, _ := st.KVGet(cursorKey); ok {
		after, _ = strconv.ParseInt(v, 10, 64)
	}
	start := 0
	for start < len(links) && links[start].ID <= after {
		start++
	}
	// Carry on after the cursor and wrap round to the start, so a batch
	// covers every link when there are fewer than supersedeBatch.
	rotated := append(append([]*domain.Link{}, links[start:]...), links[:start]...)
	batch := rotated
	if len(batch) > supersedeBatch {
		batch = batch[:supersedeBatch]
	}
	if len(batch) > 0 {
		_ = st.KVSet(cursorKey, strconv.FormatInt(batch[len(batch)-1].ID, 10))
	}
	n := 0
	for _, l := range batch {
		gone, err := c.Orphaned(ctx, l.Ref)
		if err != nil || !gone {
			continue
		}
		if err := st.SetLinkState(l.ID, domain.LinkSuperseded); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func scanPRs(ctx context.Context, svc *domain.Service, repo git.Repo, repoName string, ticket func(string) []string) (int, error) {
	prs, err := repo.ListPRs(ctx, prLimit)
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			err = fmt.Errorf("gh: %s", strings.TrimSpace(string(exit.Stderr)))
		}
		if errors.Is(err, exec.ErrNotFound) {
			return 0, nil // no gh here: nothing to ask
		}
		return 0, err
	}
	n := 0
	for _, pr := range prs {
		for _, id := range ticket(pr.Title + "\n" + pr.HeadRefName) {
			if _, _, err := svc.AddLink(id, domain.Link{Kind: domain.LinkPR, Ref: strconv.Itoa(pr.Number), Repo: repoName,
				Title: pr.Title, URL: pr.URL, State: strings.ToLower(pr.State), Source: domain.SourceScan}); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}
