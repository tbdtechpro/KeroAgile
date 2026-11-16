package gitscan_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/git"
	"github.com/tbdtechpro/KeroAgile/internal/gitscan"
	"github.com/tbdtechpro/KeroAgile/internal/store"
)

type fixture struct {
	svc  *domain.Service
	st   *store.Store
	dir  string
	proj *domain.Project
	now  time.Time
}

func (f *fixture) git(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = f.dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

func (f *fixture) commit(t *testing.T, msg string) string {
	t.Helper()
	f.git(t, "commit", "-q", "--allow-empty", "-m", msg)
	return f.git(t, "rev-parse", "HEAD")
}

// newFixture is project KA (tickets KA-001..KA-003) and project OT (OT-001)
// with a git repository whose remote is github.com/o/ka.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	t.Setenv("PATH", noGH(t))
	db, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	st := store.New(db)
	svc := domain.NewService(st)
	f := &fixture{svc: svc, st: st, dir: t.TempDir(), now: time.Now().UTC()}
	require.NoError(t, svc.CreateProject("KA", "k", f.dir))
	require.NoError(t, svc.CreateProject("OT", "other", ""))
	for i := 0; i < 3; i++ {
		_, err := svc.CreateTask(fmt.Sprintf("t%d", i), "", "KA", domain.TaskCreateOpts{})
		require.NoError(t, err)
	}
	_, err = svc.CreateTask("theirs", "", "OT", domain.TaskCreateOpts{})
	require.NoError(t, err)
	f.git(t, "init", "-q", "-b", "main")
	f.git(t, "remote", "add", "origin", "git@github.com:o/ka.git")
	f.proj, err = svc.PatchProject("KA", domain.ProjectPatch{RepoRemote: domain.To("github.com/o/ka")})
	require.NoError(t, err)
	return f
}

// noGH is a PATH without a gh, so tests never reach the real one.
func noGH(t *testing.T) string {
	dir := t.TempDir()
	for _, tool := range []string{"git"} {
		p, err := exec.LookPath(tool)
		require.NoError(t, err)
		require.NoError(t, os.Symlink(p, filepath.Join(dir, tool)))
	}
	return dir
}

// fakeGH puts a gh on PATH that prints out and counts its calls.
func fakeGH(t *testing.T, out string) string {
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	script := fmt.Sprintf("#!/bin/sh\necho x >> '%s'\nprintf '%%s\\n' '%s'\n", calls, out)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return calls
}

func (f *fixture) run(t *testing.T) gitscan.Result {
	t.Helper()
	res, err := gitscan.Run(context.Background(), f.svc, f.st, git.Repo{Dir: f.dir, Remote: f.proj.RepoRemote}, f.proj, f.now)
	require.NoError(t, err)
	return res
}

func (f *fixture) links(t *testing.T, id string) []*domain.Link {
	t.Helper()
	ls, err := f.svc.ListLinks(id)
	require.NoError(t, err)
	return ls
}

func TestScan_LinksOwnProjectOnly(t *testing.T) {
	f := newFixture(t)
	sha := f.commit(t, "fix: the flicker (KA-002)\n\nAlso touches ka-3 and OT-001.")

	res := f.run(t)
	assert.Equal(t, 2, res.Commits)
	l := f.links(t, "KA-002")
	require.Len(t, l, 1)
	assert.Equal(t, sha, l[0].Ref)
	assert.Equal(t, "fix: the flicker (KA-002)", l[0].Title)
	assert.Equal(t, "https://github.com/o/ka/commit/"+sha, l[0].URL)
	assert.Equal(t, domain.SourceScan, l[0].Source)
	assert.Len(t, f.links(t, "KA-003"), 1, "IDs in the body and in lower case count")
	assert.Empty(t, f.links(t, "OT-001"), "another board's ticket is not linked by the scan")
}

func TestScan_Twice_NothingNew(t *testing.T) {
	f := newFixture(t)
	f.commit(t, "one (KA-001)")
	f.run(t)
	res := f.run(t)
	assert.Zero(t, res.Commits)
	f.commit(t, "two (KA-001)")
	res = f.run(t)
	assert.Equal(t, 1, res.Commits, "only what is new since the last scan")
	assert.Len(t, f.links(t, "KA-001"), 2)
}

func TestScan_UnknownTicketIgnored(t *testing.T) {
	f := newFixture(t)
	f.commit(t, "mentions KA-999 and KA-")
	res := f.run(t)
	assert.Zero(t, res.Commits)
}

func TestScan_RewrittenCommitMarkedSuperseded(t *testing.T) {
	f := newFixture(t)
	f.commit(t, "base")
	f.git(t, "checkout", "-q", "-b", "feat/x")
	wip := f.commit(t, "wip (KA-001)")
	f.run(t)
	f.git(t, "checkout", "-q", "main")
	f.git(t, "branch", "-q", "-D", "feat/x")
	f.commit(t, "squashed (KA-001)")

	f.now = f.now.Add(2 * time.Hour)
	res := f.run(t)
	assert.Equal(t, 1, res.Superseded)
	var states = map[string]string{}
	for _, l := range f.links(t, "KA-001") {
		states[l.Ref] = l.State
	}
	assert.Equal(t, domain.LinkSuperseded, states[wip], "the link is kept, marked")
	assert.Len(t, states, 2)
}

func TestScan_DetachedHeadSkipsCommits(t *testing.T) {
	f := newFixture(t)
	sha := f.commit(t, "one (KA-001)")
	f.git(t, "checkout", "-q", sha)
	res := f.run(t)
	assert.Zero(t, res.Commits)
}

func TestScan_PRsAndTheirStates(t *testing.T) {
	f := newFixture(t)
	calls := fakeGH(t, `[{"number":5,"title":"Fix the flicker (KA-001)","headRefName":"fix/x","state":"MERGED","url":"https://github.com/o/ka/pull/5"},
{"number":6,"title":"Something","headRefName":"feat/KA-002-widget","state":"OPEN","url":"https://github.com/o/ka/pull/6"},
{"number":7,"title":"Unrelated","headRefName":"chore","state":"CLOSED","url":"u"}]`)

	res := f.run(t)
	assert.Equal(t, 2, res.PRs)
	l := f.links(t, "KA-001")
	require.Len(t, l, 1)
	assert.Equal(t, domain.LinkPR, l[0].Kind)
	assert.Equal(t, "5", l[0].Ref)
	assert.Equal(t, "merged", l[0].State)
	assert.Equal(t, "open", f.links(t, "KA-002")[0].State)

	f.now = f.now.Add(5 * time.Minute)
	f.run(t)
	b, err := os.ReadFile(calls)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(b), "x"), "gh is asked at most every ten minutes")

	f.now = f.now.Add(6 * time.Minute)
	f.run(t)
	b, _ = os.ReadFile(calls)
	assert.Equal(t, 2, strings.Count(string(b), "x"))
}

func TestScan_EmptyRepositoryIsNothing(t *testing.T) {
	f := newFixture(t)
	res := f.run(t)
	assert.Zero(t, res.Commits)
}

// gh offline or not logged in must not stop commits being linked.
func TestScan_GHFailureIsAWarning(t *testing.T) {
	f := newFixture(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\necho 'not logged in' >&2\nexit 4\n"), 0o755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	f.commit(t, "fix (KA-001)")

	res := f.run(t)
	assert.Equal(t, 1, res.Commits)
	require.Len(t, res.Warnings, 1)
	assert.Contains(t, res.Warnings[0], "not logged in")
}

// Review I7: the check for rewritten commits looked at the same 50 oldest
// links every time, so later ones were never checked.
func TestScan_SupersededCheckReachesEveryLink(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 55; i++ {
		f.commit(t, fmt.Sprintf("c%d (KA-001)", i))
	}
	f.git(t, "checkout", "-q", "-b", "feat/late")
	late := f.commit(t, "late (KA-001)")
	f.run(t)
	f.git(t, "checkout", "-q", "main")
	f.git(t, "branch", "-q", "-D", "feat/late")

	for i := 0; i < 3; i++ {
		f.now = f.now.Add(2 * time.Hour)
		f.run(t)
	}
	for _, l := range f.links(t, "KA-001") {
		if l.Ref == late {
			assert.Equal(t, domain.LinkSuperseded, l.State)
			return
		}
	}
	t.Fatal("no link for the late commit")
}
