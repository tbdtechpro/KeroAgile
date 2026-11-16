package artifacts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// One uploader at a time, across every process sharing the database.
const (
	uploaderLease = "uploader"
	leaseTTL      = 2 * time.Minute
	// A swept file waits this long unchanged before upload, so the versions
	// of a file still being worked on are not sent.
	sweptSettle = 15 * time.Minute
	kvInstallID = "install_id"
	// evictGrace spares contents that only just stopped being used, or were
	// only just taken in and have no artifact yet.
	evictGrace = 10 * time.Minute
	// staleTemp is how old a temp file must be before housekeeping removes
	// it; younger ones may belong to an intake or download in progress.
	staleTemp = time.Hour
)

// SyncResult is what one pass of the uploader did.
type SyncResult struct {
	Uploaded int   `json:"uploaded"`
	Bytes    int64 `json:"bytes"`
	Evicted  int   `json:"evicted"`
	Busy     bool  `json:"busy,omitempty"` // another process holds the uploader
}

// installID names this database's objects on the remote, so machines with
// separate databases can share a bucket without touching each other's files.
func (k *Keeper) installID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return k.st.KVInit(kvInstallID, hex.EncodeToString(b))
}

func (k *Keeper) remoteKey(sha string) (string, error) {
	id, err := k.installID()
	if err != nil {
		return "", err
	}
	return id + "/" + sha, nil
}

// uploadCandidates are blobs held only here that may go to the remote: used
// by an artifact (not discarded) of a project whose policy is remote, and
// either filed or attached, or swept and unchanged for 15 minutes.
func (k *Keeper) uploadCandidates() ([]domain.Blob, error) {
	blobs, err := k.st.ListBlobs("in_local = 1 AND in_remote = 0")
	if err != nil {
		return nil, err
	}
	policies := map[string]domain.ArtifactPolicy{}
	now := k.s.Now()
	var out []domain.Blob
	for _, b := range blobs {
		users, err := k.st.ArtifactsForBlob(b.SHA256)
		if err != nil {
			return nil, err
		}
		for _, a := range users {
			pol, ok := policies[a.ProjectID]
			if !ok {
				if pol, err = k.policy(a.ProjectID); err != nil {
					return nil, err
				}
				policies[a.ProjectID] = pol
			}
			if pol != domain.PolicyRemote {
				continue
			}
			if a.TaskID != nil || a.Origin == domain.OriginAttach || now.Sub(a.UpdatedAt) >= sweptSettle {
				out = append(out, b)
				break
			}
		}
	}
	return out, nil
}

// SyncOnce uploads what is waiting, confirms each upload, and then removes
// from this machine what it may to get back under local_max. It does nothing
// when another process is already uploading.
func (k *Keeper) SyncOnce(ctx context.Context) (SyncResult, error) {
	var res SyncResult
	if k.s.Remote == nil {
		return res, errors.New("no remote store is configured ([artifacts] remote)")
	}
	holder := holderName()
	if ok, err := k.st.KVLease(uploaderLease, holder, leaseTTL, k.s.Now()); err != nil || !ok {
		res.Busy = err == nil
		return res, err
	}
	defer func() { _ = k.st.KVRelease(uploaderLease, holder) }()
	err := k.syncHeld(ctx, holder, &res)
	if err != nil {
		_ = k.st.KVSet(kvLastSyncError, domain.OneLine(fmt.Sprintf("%s: %v", k.s.Now().Format(time.RFC3339), err)))
	} else {
		_ = k.st.KVSet(kvLastSyncError, "")
	}
	_ = k.st.KVSet(kvLastSync, k.s.Now().Format(time.RFC3339))
	return res, err
}

func (k *Keeper) syncHeld(ctx context.Context, holder string, res *SyncResult) error {
	if err := k.Housekeep(); err != nil {
		return err
	}
	cands, err := k.uploadCandidates()
	if err != nil {
		return err
	}
	var failed []error
	for _, b := range cands {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if ok, err := k.st.KVLease(uploaderLease, holder, leaseTTL, k.s.Now()); err != nil || !ok {
			return err
		}
		if err := k.upload(ctx, b); err != nil {
			// One bad file must not hold up the rest; it is tried again
			// next pass and reported meanwhile.
			failed = append(failed, fmt.Errorf("upload %s: %w", b.SHA256[:12], err))
			if len(failed) >= 3 && res.Uploaded == 0 {
				break // the remote itself is likely down; stop knocking
			}
			continue
		}
		res.Uploaded++
		res.Bytes += b.Size
	}
	n, err := k.evictDownTo(k.s.LocalMax)
	res.Evicted = n
	return errors.Join(append(failed, err)...)
}

// upload sends one blob and marks it remote only once the remote confirms it
// holds that key at that size.
func (k *Keeper) upload(ctx context.Context, b domain.Blob) error {
	key, err := k.remoteKey(b.SHA256)
	if err != nil {
		return err
	}
	if !k.localIntact(&b) {
		return fmt.Errorf("the copy here is missing or damaged")
	}
	f, err := os.Open(k.BlobPath(b.SHA256))
	if err != nil {
		return err
	}
	defer f.Close()
	if err := k.s.Remote.Put(ctx, key, f, b.Size, b.SHA256); err != nil {
		return err
	}
	size, ok, err := k.s.Remote.Head(ctx, key)
	if err != nil {
		return err
	}
	if !ok || size != b.Size {
		return fmt.Errorf("the remote does not hold %s at %d bytes after upload", key, b.Size)
	}
	return k.st.SetBlobRemote(b.SHA256, k.s.Now())
}

// evictDownTo removes what may be removed until the holding area is within
// limit. It reports how many blobs it removed.
func (k *Keeper) evictDownTo(limit int64) (int, error) {
	used, err := k.st.LocalBytes()
	if err != nil || limit <= 0 || used <= limit {
		return 0, err
	}
	cands, err := k.st.EvictionCandidates(k.s.KeepLocalUnder, k.s.Now().Add(-evictGrace))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, b := range cands {
		if used <= limit {
			break
		}
		if err := os.Remove(k.BlobPath(b.SHA256)); err != nil && !errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := k.st.SetBlobLocal(b.SHA256, false); err != nil {
			return n, err
		}
		used -= b.Size
		n++
	}
	return n, nil
}

// Housekeep dates contents nothing uses, and removes from this machine those
// unused for longer than keep_unreferenced. It never touches the remote.
func (k *Keeper) Housekeep() error {
	now := k.s.Now()
	if err := k.st.MarkUnreferenced(now); err != nil {
		return err
	}
	old, err := k.st.ListBlobs("in_local = 1 AND unreferenced_at IS NOT NULL AND unreferenced_at < ?",
		now.Add(-k.s.KeepUnreferenced).UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	for _, b := range old {
		if err := os.Remove(k.BlobPath(b.SHA256)); err != nil && !errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := k.st.SetBlobLocal(b.SHA256, false); err != nil {
			return err
		}
	}
	k.removeStaleTemp()
	return nil
}

// removeStaleTemp removes temp files left by an intake or download that was
// interrupted. Recent ones may belong to one still running in another
// process, so only those older than staleTemp go.
func (k *Keeper) removeStaleTemp() {
	entries, err := os.ReadDir(k.tmpDir())
	if err != nil {
		return
	}
	cutoff := k.s.Now().Add(-staleTemp)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(k.tmpDir(), e.Name()))
		}
	}
}

// download pulls a blob from the remote into the holding area, checking its
// hash on the way in.
func (k *Keeper) download(ctx context.Context, b *domain.Blob) error {
	if err := k.ensureRoom(b.Size, b.SHA256[:12]); err != nil {
		return err
	}
	key, err := k.remoteKey(b.SHA256)
	if err != nil {
		return err
	}
	rc, err := k.s.Remote.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("%w: %s is stored on the remote, not reachable now: %v", ErrUnavailable, b.SHA256[:12], err)
	}
	defer rc.Close()
	if err := os.MkdirAll(k.tmpDir(), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(k.tmpDir(), "dl-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), rc); err != nil {
		tmp.Close()
		return fmt.Errorf("%w: %s is stored on the remote, not reachable now: %v", ErrUnavailable, b.SHA256[:12], err)
	}
	if err := syncClose(tmp); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != b.SHA256 {
		return fmt.Errorf("the remote's copy of %s is damaged (hash %s)", b.SHA256[:12], got[:12])
	}
	dest := k.BlobPath(b.SHA256)
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return err
	}
	_ = os.Chmod(dest, 0o444)
	return k.st.SetBlobLocal(b.SHA256, true)
}

// RunUploader runs SyncOnce every interval until ctx ends. Errors are kept
// for artifacts status.
func (k *Keeper) RunUploader(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		pass, cancel := context.WithTimeout(ctx, 30*time.Minute)
		_, _ = k.SyncOnce(pass)
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func holderName() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("%s:%d", host, os.Getpid())
}
