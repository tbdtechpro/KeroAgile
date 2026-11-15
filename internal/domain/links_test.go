package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func TestAddLink_Validates(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")
	bad := []domain.Link{
		{Kind: "issue", Ref: "1"},
		{Kind: domain.LinkCommit, Ref: "xyz"},
		{Kind: domain.LinkCommit, Ref: "abc12"},  // too short
		{Kind: domain.LinkPR, Ref: "#12a"},
		{Kind: domain.LinkBranch, Ref: "--evil"},
		{Kind: domain.LinkTag, Ref: ""},
	}
	for _, l := range bad {
		_, _, err := svc.AddLink("KA-001", l)
		require.ErrorIs(t, err, domain.ErrInvalid, "%+v", l)
	}
	_, _, err := svc.AddLink("KA-404", domain.Link{Kind: domain.LinkPR, Ref: "1"})
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAddLink_DefaultsRepoAndURL(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")
	_, err := svc.PatchProject("KA", domain.ProjectPatch{RepoRemote: domain.To("github.com/o/r")})
	require.NoError(t, err)

	l, created, err := svc.AddLink("ka-001", domain.Link{Kind: domain.LinkCommit, Ref: "ABCDEF1234567"})
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "github.com/o/r", l.Repo)
	assert.Equal(t, "abcdef1234567", l.Ref, "SHAs are stored in lower case")
	assert.Equal(t, "https://github.com/o/r/commit/abcdef1234567", l.URL)
	assert.Equal(t, domain.SourceManual, l.Source)

	pr, _, err := svc.AddLink("KA-001", domain.Link{Kind: domain.LinkPR, Ref: "#42"})
	require.NoError(t, err)
	assert.Equal(t, "42", pr.Ref)
	assert.Equal(t, "https://github.com/o/r/pull/42", pr.URL)

	br, _, err := svc.AddLink("KA-001", domain.Link{Kind: domain.LinkBranch, Ref: "feat/x"})
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/o/r/tree/feat/x", br.URL)

	tag, _, err := svc.AddLink("KA-001", domain.Link{Kind: domain.LinkTag, Ref: "v1.0.0", Repo: "gitlab.com/a/b"})
	require.NoError(t, err)
	assert.Equal(t, "gitlab.com/a/b", tag.Repo)
	assert.Equal(t, "", tag.URL, "URLs are only built for github.com")
}

func TestAddLink_TwiceIsHarmless(t *testing.T) {
	svc, _ := boardWith(t, "a ticket")
	_, created, err := svc.AddLink("KA-001", domain.Link{Kind: domain.LinkPR, Ref: "7", State: "open"})
	require.NoError(t, err)
	assert.True(t, created)
	l, created, err := svc.AddLink("KA-001", domain.Link{Kind: domain.LinkPR, Ref: "7", State: "merged", Title: "the fix"})
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, "merged", l.State, "a newer state is kept")
	assert.Equal(t, "the fix", l.Title)

	task, err := svc.GetTask("KA-001")
	require.NoError(t, err)
	require.Len(t, task.Links, 1)
}
