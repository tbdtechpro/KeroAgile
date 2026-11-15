package store_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func TestLinks_UpsertListAndCascade(t *testing.T) {
	s, now := artifactBoard(t)
	l := &domain.Link{TaskID: "KA-001", Kind: domain.LinkPR, Repo: "github.com/o/r", Ref: "7", State: "open", Source: "scan", CreatedAt: now}
	created, err := s.UpsertLink(l)
	require.NoError(t, err)
	assert.True(t, created)
	assert.NotZero(t, l.ID)

	again := &domain.Link{TaskID: "KA-001", Kind: domain.LinkPR, Repo: "github.com/o/r", Ref: "7", State: "merged", Title: "fix", CreatedAt: now.Add(time.Hour)}
	created, err = s.UpsertLink(again)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, l.ID, again.ID)
	assert.Equal(t, "merged", again.State)
	assert.Equal(t, "scan", again.Source, "the first recording's source stays")
	assert.True(t, again.CreatedAt.Equal(now), "and its time")

	_, err = s.UpsertLink(&domain.Link{TaskID: "KA-001", Kind: domain.LinkCommit, Repo: "github.com/o/r", Ref: "abcdef1", CreatedAt: now})
	require.NoError(t, err)
	links, err := s.ListLinks("KA-001")
	require.NoError(t, err)
	require.Len(t, links, 2)

	byRepo, err := s.LinksByRepo("github.com/o/r", domain.LinkCommit, "")
	require.NoError(t, err)
	require.Len(t, byRepo, 1)
	require.NoError(t, s.SetLinkState(byRepo[0].ID, domain.LinkSuperseded))
	byRepo, err = s.LinksByRepo("github.com/o/r", domain.LinkCommit, "")
	require.NoError(t, err)
	assert.Empty(t, byRepo)

	require.NoError(t, s.DeleteTask("KA-001"))
	links, err = s.ListLinks("KA-001")
	require.NoError(t, err)
	assert.Empty(t, links)
}
