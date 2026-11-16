package artifacts_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// Review I1: housekeeping removed every temp file, including ones another
// process was still writing.
func TestHousekeep_KeepsTempFilesInUse(t *testing.T) {
	f, _, c := remoteFixture(t, nil)
	tmp := filepath.Join(f.set.Dir, "tmp")
	require.NoError(t, os.MkdirAll(tmp, 0o700))
	fresh := filepath.Join(tmp, "in-fresh")
	stale := filepath.Join(tmp, "in-stale")
	require.NoError(t, os.WriteFile(fresh, []byte("being written"), 0o600))
	require.NoError(t, os.WriteFile(stale, []byte("left by a crash"), 0o600))
	old := c.t.Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(stale, old, old))

	require.NoError(t, f.k.Housekeep())
	assert.FileExists(t, fresh)
	assert.NoFileExists(t, stale)
}

// Review I2: a blob just taken in has no artifact yet, so it looks unused;
// another process short of room must not evict it in that moment.
func TestEviction_SparesContentsJustTakenIn(t *testing.T) {
	f := newFixture(t, nil)
	now := time.Now().UTC()
	require.NoError(t, f.st.PutBlob(domain.Blob{SHA256: "justin", Size: 10, InLocal: true}))
	require.NoError(t, f.st.MarkUnreferenced(now))
	cands, err := f.st.EvictionCandidates(0, now.Add(-10*time.Minute))
	require.NoError(t, err)
	assert.Empty(t, cands)
	cands, err = f.st.EvictionCandidates(0, now.Add(time.Minute))
	require.NoError(t, err)
	assert.Len(t, cands, 1, "once the grace period is over it may go")
}

// Review I2: if a swept file's contents were lost anyway, the sweep keeps it
// again instead of skipping it forever as unchanged.
func TestSweep_RecapturesWhatWasLost(t *testing.T) {
	f := newFixture(t, nil)
	dir := scratch(t, map[string]string{"run.log": "important"})
	f.sweep(t, dir)
	a := f.unfiled(t)[0]
	require.NoError(t, os.Remove(f.k.BlobPath(a.SHA256)))
	require.NoError(t, f.st.SetBlobLocal(a.SHA256, false))

	f.sweep(t, dir)
	assert.FileExists(t, f.k.BlobPath(a.SHA256))
	got, err := f.svc.GetArtifact(a.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WhereHere, got.Where)
}

// Review I3: one bad blob must not stop every upload behind it.
func TestSync_ContinuesPastADamagedFile(t *testing.T) {
	f, s3, _ := remoteFixture(t, nil)
	bad, err := f.k.Attach("KA-001", f.file(t, "a.log", "aaaaaaaaaaaaaaaaaaaa"), "", "", "")
	require.NoError(t, err)
	good, err := f.k.Attach("KA-001", f.file(t, "b.log", "bbbbbbbbbbbbbbbbbbbb"), "", "", "")
	require.NoError(t, err)
	p := f.k.BlobPath(bad.SHA256)
	require.NoError(t, os.Chmod(p, 0o600))
	require.NoError(t, os.WriteFile(p, []byte("trunc"), 0o600)) // a power cut mid-write

	res, err := f.k.SyncOnce(context.Background())
	require.Error(t, err, "the damage is reported")
	assert.Equal(t, 1, res.Uploaded)
	b, err := f.st.GetBlob(good.SHA256)
	require.NoError(t, err)
	assert.True(t, b.InRemote, "the good file went up regardless")
	assert.Len(t, s3.objects, 1)
}

// Review I3: a damaged local copy is not handed out as the file.
func TestFetch_DamagedLocalCopyIsNotServed(t *testing.T) {
	f := newFixture(t, nil)
	a, err := f.k.Attach("KA-001", f.file(t, "a.log", "the real contents"), "", "", "")
	require.NoError(t, err)
	p := f.k.BlobPath(a.SHA256)
	require.NoError(t, os.Chmod(p, 0o600))
	require.NoError(t, os.WriteFile(p, []byte("short"), 0o600))

	_, err = f.k.Fetch(context.Background(), a.ID)
	require.ErrorIs(t, err, artifacts.ErrUnavailable)
}

// Review I3: what fetch_artifact hands out is the store's own copy; it must
// not be writable by accident.
func TestKeptFilesAreReadOnly(t *testing.T) {
	f := newFixture(t, nil)
	a, err := f.k.Attach("KA-001", f.file(t, "a.log", "x"), "", "", "")
	require.NoError(t, err)
	fi, err := os.Stat(f.k.BlobPath(a.SHA256))
	require.NoError(t, err)
	assert.Zero(t, fi.Mode().Perm()&0o222)
}

// Review I4: a snapshot taken before the upload says the file is only here;
// once it has gone from here, the remote is asked anyway.
func TestFetch_AsksTheRemoteEvenWhenTheRecordDoesNotSayItIsThere(t *testing.T) {
	f, _, _ := remoteFixture(t, nil)
	a, err := f.k.Attach("KA-001", f.file(t, "big.log", "twenty bytes of text"), "", "", "")
	require.NoError(t, err)
	_, err = f.k.SyncOnce(context.Background())
	require.NoError(t, err)
	require.NoError(t, f.st.ClearBlobRemote(a.SHA256)) // the restored record
	require.NoError(t, os.Remove(f.k.BlobPath(a.SHA256)))

	path, err := f.k.Fetch(context.Background(), a.ID)
	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "twenty bytes of text", string(got))
}
