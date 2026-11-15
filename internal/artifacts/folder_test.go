package artifacts_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

func hexsum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestFolderRemote_RoundTrip(t *testing.T) {
	root := t.TempDir()
	r := artifacts.NewFolder(root)
	ctx := context.Background()
	body := []byte("contents")
	sha := hexsum(body)
	key := "inst/" + sha

	require.NoError(t, r.Put(ctx, key, bytes.NewReader(body), int64(len(body)), sha))
	assert.FileExists(t, filepath.Join(root, "inst", sha[:2], sha))
	size, ok, err := r.Head(ctx, key)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, int64(len(body)), size)

	rc, err := r.Get(ctx, key)
	require.NoError(t, err)
	got, _ := io.ReadAll(rc)
	rc.Close()
	assert.Equal(t, body, got)

	err = r.Put(ctx, "inst/"+strings.Repeat("0", 64), bytes.NewReader(body), int64(len(body)), strings.Repeat("0", 64))
	require.Error(t, err, "contents that do not match their hash are refused")
	_, ok, err = r.Head(ctx, "inst/"+strings.Repeat("0", 64))
	require.NoError(t, err)
	assert.False(t, ok, "and leave nothing behind")

	require.NoError(t, r.Delete(ctx, key))
	_, ok, err = r.Head(ctx, key)
	require.NoError(t, err)
	assert.False(t, ok)
	require.NoError(t, r.Delete(ctx, key), "deleting what is gone is fine")
	assert.Contains(t, r.Name(), root)

	_, err = r.Get(ctx, "../../etc/passwd")
	require.Error(t, err, "keys cannot leave the folder")
}

func TestFolderRemote_SyncThroughKeeper(t *testing.T) {
	root := t.TempDir()
	f := newFixture(t, func(s *artifacts.Settings) {
		s.Remote = artifacts.NewFolder(root)
		s.DefaultPolicy = domain.PolicyRemote
		s.KeepLocalUnder = 4
	})
	a, err := f.k.Attach("KA-001", f.file(t, "big.log", "more than four bytes"), "", "", "")
	require.NoError(t, err)
	res, err := f.k.SyncOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Uploaded)

	require.NoError(t, os.Remove(f.k.BlobPath(a.SHA256)))
	require.NoError(t, f.st.SetBlobLocal(a.SHA256, false))
	path, err := f.k.Fetch(context.Background(), a.ID)
	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "more than four bytes", string(got))
}

func TestFolderRemote_MissingMountIsAnError(t *testing.T) {
	r := artifacts.NewFolder(filepath.Join(t.TempDir(), "not-mounted"))
	_, _, err := r.Head(context.Background(), "inst/abc")
	require.Error(t, err, "a folder that is not there is not an empty store")
}
