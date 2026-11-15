package artifacts_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func remoteFixture(t *testing.T, tune func(*artifacts.Settings)) (*fixture, *fakeS3, *clock) {
	t.Helper()
	s3 := newFakeS3(t)
	c := &clock{t: time.Now().UTC()}
	f := newFixture(t, func(s *artifacts.Settings) {
		s.Remote = s3.remote()
		s.DefaultPolicy = domain.PolicyRemote
		s.KeepLocalUnder = 10
		s.Now = c.now
		if tune != nil {
			tune(s)
		}
	})
	return f, s3, c
}

func TestSync_UploadsConfirmsAndEvictsLarge(t *testing.T) {
	f, s3, _ := remoteFixture(t, func(s *artifacts.Settings) { s.LocalMax = 30 })
	small, err := f.k.Attach("KA-001", f.file(t, "small.log", "tiny"), "", "", "")
	require.NoError(t, err)
	big, err := f.k.Attach("KA-001", f.file(t, "big.log", "twenty bytes of text"), "", "", "")
	require.NoError(t, err)
	_, err = f.k.Attach("KA-002", f.file(t, "other.log", "another twenty bytes"), "", "", "")
	require.ErrorIs(t, err, artifacts.ErrNoRoom, "before uploading nothing can be evicted")

	res, err := f.k.SyncOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, res.Uploaded)
	assert.Len(t, s3.objects, 2)

	b, err := f.st.GetBlob(big.SHA256)
	require.NoError(t, err)
	assert.True(t, b.InRemote)
	for key := range s3.objects {
		assert.True(t, strings.Contains(key, "/"), "keys carry the install ID: %s", key)
	}

	// Now there is something to evict, so the third file fits.
	other, err := f.k.Attach("KA-002", f.file(t, "other.log", "another twenty bytes"), "", "", "")
	require.NoError(t, err)
	assert.NoFileExists(t, f.k.BlobPath(big.SHA256), "the large uploaded file left this machine")
	assert.FileExists(t, f.k.BlobPath(small.SHA256), "small files always stay")
	assert.FileExists(t, f.k.BlobPath(other.SHA256))

	got, err := f.svc.GetArtifact(big.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WhereRemote, got.Where)
}

func TestFetch_RemoteOnly_PullsBack(t *testing.T) {
	f, _, _ := remoteFixture(t, nil)
	a, err := f.k.Attach("KA-001", f.file(t, "big.log", "twenty bytes of text"), "", "", "")
	require.NoError(t, err)
	_, err = f.k.SyncOnce(context.Background())
	require.NoError(t, err)
	require.NoError(t, os.Remove(f.k.BlobPath(a.SHA256)))
	require.NoError(t, f.st.SetBlobLocal(a.SHA256, false))

	path, err := f.k.Fetch(context.Background(), a.ID)
	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "twenty bytes of text", string(got))
	again, err := f.svc.GetArtifact(a.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WhereBoth, again.Where)
}

// Review Focus 5: the remote cannot be reached (Teleport down).
func TestFetch_RemoteUnreachable_FailsFast(t *testing.T) {
	f, s3, _ := remoteFixture(t, nil)
	a, err := f.k.Attach("KA-001", f.file(t, "big.log", "twenty bytes of text"), "", "", "")
	require.NoError(t, err)
	_, err = f.k.SyncOnce(context.Background())
	require.NoError(t, err)
	require.NoError(t, os.Remove(f.k.BlobPath(a.SHA256)))
	s3.srv.Close()

	start := time.Now()
	_, err = f.k.Fetch(context.Background(), a.ID)
	require.ErrorIs(t, err, artifacts.ErrUnavailable)
	assert.Contains(t, err.Error(), "stored on the remote, not reachable now")
	assert.Less(t, time.Since(start), 5*time.Second)

	// Keeping new files does not need the remote.
	_, err = f.k.Attach("KA-001", f.file(t, "new.log", "offline"), "", "", "")
	require.NoError(t, err)
	res, err := f.k.SyncOnce(context.Background())
	require.Error(t, err)
	assert.Zero(t, res.Uploaded)
	st, err := f.k.Status()
	require.NoError(t, err)
	assert.NotEmpty(t, st.LastSyncError)
	assert.Equal(t, 1, st.WaitingFiles)
}

func TestSync_SmallFilesStayLocalAfterUpload(t *testing.T) {
	f, _, _ := remoteFixture(t, nil)
	a, err := f.k.Attach("KA-001", f.file(t, "s.log", "tiny"), "", "", "")
	require.NoError(t, err)
	_, err = f.k.SyncOnce(context.Background())
	require.NoError(t, err)
	got, err := f.svc.GetArtifact(a.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WhereBoth, got.Where)
}

func TestSync_PolicyLocalNeverUploads(t *testing.T) {
	f, s3, _ := remoteFixture(t, nil)
	_, err := f.svc.PatchProject("KA", domain.ProjectPatch{ArtifactPolicy: domain.To(domain.PolicyLocal)})
	require.NoError(t, err)
	_, err = f.k.Attach("KA-001", f.file(t, "work.log", "stays here"), "", "", "")
	require.NoError(t, err)
	res, err := f.k.SyncOnce(context.Background())
	require.NoError(t, err)
	assert.Zero(t, res.Uploaded)
	assert.Empty(t, s3.objects)
}

func TestSync_SweptWaits15Minutes(t *testing.T) {
	f, s3, c := remoteFixture(t, nil)
	f.sweep(t, scratch(t, map[string]string{"draft.log": "still changing"}))

	res, err := f.k.SyncOnce(context.Background())
	require.NoError(t, err)
	assert.Zero(t, res.Uploaded, "a swept file may still be changing")

	c.t = c.t.Add(16 * time.Minute)
	res, err = f.k.SyncOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Uploaded)
	assert.Len(t, s3.objects, 1)
}

func TestSync_OneUploaderAtATime(t *testing.T) {
	f, _, c := remoteFixture(t, nil)
	ok, err := f.st.KVLease("uploader", "someone-else", 2*time.Minute, c.t)
	require.NoError(t, err)
	require.True(t, ok)
	_, err = f.k.Attach("KA-001", f.file(t, "x.log", "waiting for the other uploader"), "", "", "")
	require.NoError(t, err)

	res, err := f.k.SyncOnce(context.Background())
	require.NoError(t, err)
	assert.True(t, res.Busy)
	assert.Zero(t, res.Uploaded)
}

func TestPrune_ListsThenAppliesOnlyWithFlag(t *testing.T) {
	f, s3, c := remoteFixture(t, nil)
	a, err := f.k.Attach("KA-001", f.file(t, "gone.log", "twenty bytes of text"), "", "", "")
	require.NoError(t, err)
	_, err = f.k.SyncOnce(context.Background())
	require.NoError(t, err)
	require.NoError(t, f.svc.DiscardArtifact(a.ID))

	_, err = f.k.SyncOnce(context.Background()) // dates it unused
	require.NoError(t, err)
	listed, err := f.k.Prune(context.Background(), false)
	require.NoError(t, err)
	assert.Empty(t, listed, "kept for keep_unreferenced first")

	c.t = c.t.Add(15 * 24 * time.Hour)
	listed, err = f.k.Prune(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Len(t, s3.objects, 1, "listing removes nothing")

	_, err = f.k.Prune(context.Background(), true)
	require.NoError(t, err)
	assert.Empty(t, s3.objects)
	b, err := f.st.GetBlob(a.SHA256)
	require.NoError(t, err)
	assert.False(t, b.InRemote)
}

func TestSync_DropsLocalCopiesUnusedTooLong(t *testing.T) {
	f, _, c := remoteFixture(t, func(s *artifacts.Settings) { s.Remote = nil; s.DefaultPolicy = domain.PolicyLocal })
	a, err := f.k.Attach("KA-001", f.file(t, "x.log", "x"), "", "", "")
	require.NoError(t, err)
	require.NoError(t, f.svc.DiscardArtifact(a.ID))
	require.NoError(t, f.k.Housekeep())
	assert.FileExists(t, f.k.BlobPath(a.SHA256))
	c.t = c.t.Add(15 * 24 * time.Hour)
	require.NoError(t, f.k.Housekeep())
	assert.NoFileExists(t, f.k.BlobPath(a.SHA256))
}

// Found against a real Garage: a finished `artifacts sync` kept the lease
// for its whole two minutes, so the next sync reported "busy".
func TestSync_ReleasesTheLeaseWhenDone(t *testing.T) {
	f, _, c := remoteFixture(t, nil)
	_, err := f.k.SyncOnce(context.Background())
	require.NoError(t, err)
	ok, err := f.st.KVLease("uploader", "another-process", 2*time.Minute, c.t)
	require.NoError(t, err)
	assert.True(t, ok, "the lease is free once the pass is over")
}
