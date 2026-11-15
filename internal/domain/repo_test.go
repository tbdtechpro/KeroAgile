package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func TestNormalizeRemote(t *testing.T) {
	for _, in := range []string{
		"https://github.com/tbdtechpro/KeroAgile.git",
		"git@github.com:tbdtechpro/KeroAgile.git",
		"ssh://git@github.com/tbdtechpro/KeroAgile",
		"github.com/tbdtechpro/KeroAgile",
		"https://GitHub.com/tbdtechpro/KeroAgile/",
		"tbdtechpro/KeroAgile",
	} {
		assert.Equal(t, "github.com/tbdtechpro/KeroAgile", domain.NormalizeRemote(in), in)
	}
	assert.Equal(t, "", domain.NormalizeRemote("  "))
}

func TestClassifyRepo(t *testing.T) {
	dir, remote := domain.ClassifyRepo("/home/u/x")
	assert.Equal(t, "/home/u/x", dir)
	assert.Equal(t, "", remote)

	dir, remote = domain.ClassifyRepo("~/github/x")
	assert.Equal(t, "~/github/x", dir)
	assert.Equal(t, "", remote)

	dir, remote = domain.ClassifyRepo(`C:\work\x`)
	assert.Equal(t, `C:\work\x`, dir)
	assert.Equal(t, "", remote)

	dir, remote = domain.ClassifyRepo("https://github.com/o/r.git")
	assert.Equal(t, "", dir)
	assert.Equal(t, "github.com/o/r", remote)

	dir, remote = domain.ClassifyRepo("")
	assert.Equal(t, "", dir)
	assert.Equal(t, "", remote)
}

func projects(ps ...*domain.Project) []*domain.Project { return ps }

func TestMatchProject_LongestPrefix(t *testing.T) {
	ps := projects(
		&domain.Project{ID: "A", RepoDir: "/a"},
		&domain.Project{ID: "AB", RepoDir: "/a/b"},
	)
	p, err := domain.MatchProject(ps, "/a/b/c", "", "/home/u")
	require.NoError(t, err)
	assert.Equal(t, "AB", p.ID)

	p, err = domain.MatchProject(ps, "/a/x", "", "/home/u")
	require.NoError(t, err)
	assert.Equal(t, "A", p.ID)
}

func TestMatchProject_NotAPrefixAtBoundary(t *testing.T) {
	ps := projects(&domain.Project{ID: "AB", RepoDir: "/a/b"})
	_, err := domain.MatchProject(ps, "/a/bc", "", "/home/u")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestMatchProject_ExpandsHome(t *testing.T) {
	ps := projects(&domain.Project{ID: "X", RepoDir: "~/github/x"})
	p, err := domain.MatchProject(ps, "/home/u/github/x/sub", "", "/home/u")
	require.NoError(t, err)
	assert.Equal(t, "X", p.ID)
}

func TestMatchProject_TieIsInvalid(t *testing.T) {
	ps := projects(
		&domain.Project{ID: "ONE", RepoDir: "/a"},
		&domain.Project{ID: "TWO", RepoDir: "/a/"},
	)
	_, err := domain.MatchProject(ps, "/a", "", "/home/u")
	require.ErrorIs(t, err, domain.ErrInvalid)
	assert.Contains(t, err.Error(), "ONE")
	assert.Contains(t, err.Error(), "TWO")
}

func TestMatchProject_FallsBackToRemote(t *testing.T) {
	ps := projects(
		&domain.Project{ID: "D", RepoDir: "/elsewhere"},
		&domain.Project{ID: "R", RepoRemote: "github.com/o/r"},
	)
	p, err := domain.MatchProject(ps, "/work/r", "git@github.com:O/R.git", "/home/u")
	require.NoError(t, err)
	assert.Equal(t, "R", p.ID)
}

func TestMatchProject_TwoOnOneRemoteIsInvalid(t *testing.T) {
	ps := projects(
		&domain.Project{ID: "R1", RepoRemote: "github.com/o/r"},
		&domain.Project{ID: "R2", RepoRemote: "github.com/o/r"},
	)
	_, err := domain.MatchProject(ps, "/work/r", "https://github.com/o/r", "/home/u")
	require.ErrorIs(t, err, domain.ErrInvalid)
}

func TestMatchProject_NoneIsNotFound(t *testing.T) {
	ps := projects(&domain.Project{ID: "D", RepoDir: "/elsewhere"})
	_, err := domain.MatchProject(ps, "/work", "", "/home/u")
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = domain.MatchProject(ps, "", "", "/home/u")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestValidBranchName(t *testing.T) {
	for _, bad := range []string{"-x", "--upload-pack=x", "a b", "a..b", "a\tb", "a:b", "a~1", "a^", "a?", "a*", "a[", `a\b`, "a\x01"} {
		require.ErrorIs(t, domain.ValidBranchName(bad), domain.ErrInvalid, "%q", bad)
	}
	for _, good := range []string{"feat/ka-060", "fix/field-level-updates", "main", "KA-001-foo"} {
		require.NoError(t, domain.ValidBranchName(good), good)
	}
}

func TestPatchTask_RefusesBadBranch(t *testing.T) {
	svc, m := boardWith(t, "a ticket")
	_, err := svc.PatchTask("KA-001", domain.TaskPatch{Branch: domain.To("--exec=evil")})
	require.ErrorIs(t, err, domain.ErrInvalid)
	assert.Equal(t, "", m.tasks["KA-001"].Branch)

	_, err = svc.PatchTask("KA-001", domain.TaskPatch{Branch: domain.To("")})
	require.NoError(t, err, "clearing the branch is allowed")
}

func TestPatchProject_ValidatesPolicy(t *testing.T) {
	svc, m := boardWith(t)
	_, err := svc.PatchProject("ka", domain.ProjectPatch{ArtifactPolicy: domain.To(domain.ArtifactPolicy("upload"))})
	require.ErrorIs(t, err, domain.ErrInvalid)

	p, err := svc.PatchProject("ka", domain.ProjectPatch{
		ArtifactPolicy: domain.To(domain.PolicyLocal),
		RepoDir:        domain.To(" /work/ka "),
		RepoRemote:     domain.To("https://github.com/o/ka.git"),
	})
	require.NoError(t, err)
	assert.Equal(t, domain.PolicyLocal, p.ArtifactPolicy)
	assert.Equal(t, "/work/ka", p.RepoDir)
	assert.Equal(t, "github.com/o/ka", p.RepoRemote)
	assert.Equal(t, domain.PolicyLocal, m.projects["KA"].ArtifactPolicy)
}

func TestCreateProject_ClassifiesRepoPath(t *testing.T) {
	svc, _ := boardWith(t)
	require.NoError(t, svc.CreateProject("RD", "dir", "/work/rd"))
	require.NoError(t, svc.CreateProject("RR", "remote", "git@github.com:o/rr.git"))
	rd, err := svc.GetProject("rd")
	require.NoError(t, err)
	assert.Equal(t, "/work/rd", rd.RepoDir)
	assert.Equal(t, "/work/rd", rd.RepoPath, "repo_path keeps what was given")
	rr, err := svc.GetProject("rr")
	require.NoError(t, err)
	assert.Equal(t, "github.com/o/rr", rr.RepoRemote)
}
