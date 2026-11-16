package store_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/store"
)

func artifactBoard(t *testing.T) (*store.Store, time.Time) {
	t.Helper()
	s := testStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{ID: "KA", Name: "k"}))
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	for _, id := range []string{"KA-001", "KA-002"} {
		require.NoError(t, s.CreateTask(&domain.Task{ID: id, ProjectID: "KA", Title: id,
			Status: domain.StatusBacklog, Priority: domain.PriorityMedium, CreatedAt: now, UpdatedAt: now}))
	}
	return s, now
}

func ptr[T any](v T) *T { return &v }

func addArtifact(t *testing.T, s *store.Store, taskID *string, sha string, size int64, path string, at time.Time) *domain.Artifact {
	t.Helper()
	require.NoError(t, s.PutBlob(domain.Blob{SHA256: sha, Size: size, InLocal: true}))
	a := &domain.Artifact{ProjectID: "KA", TaskID: taskID, Kind: domain.KindLog, Name: "x.log", SHA256: sha, Size: size,
		Origin: domain.OriginSweep, SourcePath: path, CreatedAt: at, UpdatedAt: at}
	require.NoError(t, s.AddArtifact(a))
	require.NotZero(t, a.ID)
	return a
}

func TestArtifacts_ListAndWhere(t *testing.T) {
	s, now := artifactBoard(t)
	addArtifact(t, s, ptr("KA-001"), "aaa", 10, "/s/a", now)
	b := addArtifact(t, s, nil, "bbb", 20, "/s/b", now)
	require.NoError(t, s.SetBlobRemote("bbb", now))

	onTicket, err := s.ListArtifacts(domain.ArtifactQuery{TaskID: "KA-001"})
	require.NoError(t, err)
	require.Len(t, onTicket, 1)
	assert.Equal(t, "here", onTicket[0].Where)

	unfiled, err := s.ListArtifacts(domain.ArtifactQuery{ProjectID: "KA", Unfiled: true})
	require.NoError(t, err)
	require.Len(t, unfiled, 1)
	assert.Equal(t, b.ID, unfiled[0].ID)
	assert.Equal(t, "both", unfiled[0].Where)

	require.NoError(t, s.SetBlobLocal("bbb", false))
	got, err := s.GetArtifact(b.ID)
	require.NoError(t, err)
	assert.Equal(t, "remote", got.Where)

	require.NoError(t, s.SetBlobLocal("aaa", false))
	onTicket, err = s.ListArtifacts(domain.ArtifactQuery{TaskID: "KA-001"})
	require.NoError(t, err)
	assert.Equal(t, "unavailable", onTicket[0].Where)

	_, err = s.GetArtifact(9999)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestArtifacts_RemovedWithTicket_BlobMarkedUnreferenced(t *testing.T) {
	s, now := artifactBoard(t)
	addArtifact(t, s, ptr("KA-001"), "aaa", 10, "/s/a", now)
	addArtifact(t, s, ptr("KA-002"), "shared", 5, "/s/s1", now)
	addArtifact(t, s, ptr("KA-001"), "shared", 5, "/s/s2", now)

	require.NoError(t, s.DeleteTask("KA-001"))
	require.NoError(t, s.MarkUnreferenced(now.Add(time.Hour)))

	a, err := s.GetBlob("aaa")
	require.NoError(t, err)
	require.NotNil(t, a.UnreferencedAt, "nothing uses aaa now")
	assert.True(t, a.UnreferencedAt.Equal(now.Add(time.Hour)))
	sh, err := s.GetBlob("shared")
	require.NoError(t, err)
	assert.Nil(t, sh.UnreferencedAt, "KA-002 still uses it")

	// Used again: the mark is cleared.
	addArtifact(t, s, ptr("KA-002"), "aaa", 10, "/s/a2", now)
	require.NoError(t, s.MarkUnreferenced(now.Add(2*time.Hour)))
	a, err = s.GetBlob("aaa")
	require.NoError(t, err)
	assert.Nil(t, a.UnreferencedAt)
}

func TestArtifacts_DiscardIsNotAReference(t *testing.T) {
	s, now := artifactBoard(t)
	a := addArtifact(t, s, nil, "aaa", 10, "/s/a", now)
	require.NoError(t, s.UpdateArtifact(a.ID, domain.ArtifactPatch{DiscardedAt: domain.To(ptr(now))}))
	require.NoError(t, s.MarkUnreferenced(now))
	b, err := s.GetBlob("aaa")
	require.NoError(t, err)
	assert.NotNil(t, b.UnreferencedAt)

	discarded, err := s.PathDiscarded("/s/a")
	require.NoError(t, err)
	assert.True(t, discarded)
	list, err := s.ListArtifacts(domain.ArtifactQuery{ProjectID: "KA", Unfiled: true})
	require.NoError(t, err)
	assert.Empty(t, list, "discarded artifacts are not listed")
}

func TestArtifacts_LatestForPathAndUpdate(t *testing.T) {
	s, now := artifactBoard(t)
	addArtifact(t, s, ptr("KA-001"), "v1", 1, "/s/log", now)
	second := addArtifact(t, s, nil, "v2", 2, "/s/log", now)

	latest, err := s.LatestArtifactForPath("/s/log")
	require.NoError(t, err)
	assert.Equal(t, second.ID, latest.ID)

	require.NoError(t, s.PutBlob(domain.Blob{SHA256: "v3", Size: 3, InLocal: true}))
	require.NoError(t, s.UpdateArtifact(second.ID, domain.ArtifactPatch{
		SHA256: domain.To("v3"), Size: domain.To(int64(3)), SourceMTime: domain.To(int64(42)),
		TaskID: domain.To(ptr("KA-002")), Origin: domain.To(domain.OriginAttach),
	}))
	got, err := s.GetArtifact(second.ID)
	require.NoError(t, err)
	assert.Equal(t, "v3", got.SHA256)
	assert.Equal(t, int64(42), got.SourceMTime)
	assert.Equal(t, "KA-002", *got.TaskID)
	assert.Equal(t, domain.OriginAttach, got.Origin)

	none, err := s.LatestArtifactForPath("/nowhere")
	require.NoError(t, err)
	assert.Nil(t, none)
}

func TestBlobs_LocalBytesAndEvictionOrder(t *testing.T) {
	s, now := artifactBoard(t)
	addArtifact(t, s, ptr("KA-001"), "small", 100, "/s/1", now) // small: never evicted
	addArtifact(t, s, ptr("KA-001"), "big-old", 5000, "/s/2", now)
	addArtifact(t, s, ptr("KA-001"), "big-new", 6000, "/s/3", now)
	addArtifact(t, s, ptr("KA-001"), "big-local", 7000, "/s/4", now) // not uploaded: never evicted
	gone := addArtifact(t, s, nil, "unused", 50, "/s/5", now)
	for _, sha := range []string{"small", "big-old", "big-new"} {
		require.NoError(t, s.SetBlobRemote(sha, now))
	}
	require.NoError(t, s.TouchBlob("big-old", now.Add(time.Minute)))
	require.NoError(t, s.TouchBlob("big-new", now.Add(time.Hour)))
	require.NoError(t, s.UpdateArtifact(gone.ID, domain.ArtifactPatch{DiscardedAt: domain.To(ptr(now))}))
	require.NoError(t, s.MarkUnreferenced(now))

	total, err := s.LocalBytes()
	require.NoError(t, err)
	assert.Equal(t, int64(100+5000+6000+7000+50), total)

	cands, err := s.EvictionCandidates(1000, now.Add(time.Minute))
	require.NoError(t, err)
	var order []string
	for _, b := range cands {
		order = append(order, b.SHA256)
	}
	assert.Equal(t, []string{"unused", "big-old", "big-new"}, order)
}

func TestKV(t *testing.T) {
	s := testStore(t)
	_, ok, err := s.KVGet("k")
	require.NoError(t, err)
	assert.False(t, ok)
	require.NoError(t, s.KVSet("k", "v1"))
	require.NoError(t, s.KVSet("k", "v2"))
	v, ok, err := s.KVGet("k")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "v2", v)
}

func TestKVLease_OneHolder(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC()
	got, err := s.KVLease("uploader", "A", time.Minute, now)
	require.NoError(t, err)
	assert.True(t, got)
	got, err = s.KVLease("uploader", "B", time.Minute, now.Add(30*time.Second))
	require.NoError(t, err)
	assert.False(t, got, "A holds it")
	got, err = s.KVLease("uploader", "A", time.Minute, now.Add(30*time.Second))
	require.NoError(t, err)
	assert.True(t, got, "the holder renews it")
	got, err = s.KVLease("uploader", "B", time.Minute, now.Add(2*time.Minute))
	require.NoError(t, err)
	assert.True(t, got, "an expired lease can be taken")
}
