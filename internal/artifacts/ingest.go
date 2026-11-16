package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// Attach keeps the file at path against a ticket. kind may be empty, to infer
// it from the file name. If an unfiled artifact was swept from the same path,
// it is taken over (filed, with the file's current contents) rather than
// left behind as a stale copy.
func (k *Keeper) Attach(taskID, path string, kind domain.ArtifactKind, note, author string) (*domain.Artifact, error) {
	task, err := k.svc.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	pol, err := k.policy(task.ProjectID)
	if err != nil {
		return nil, err
	}
	if pol == domain.PolicyOff {
		return nil, fmt.Errorf("%w file: project %s keeps no files (artifact_policy off)", domain.ErrInvalid, task.ProjectID)
	}
	if kind != "" && !kind.Valid() {
		return nil, fmt.Errorf("%w kind %q: use script, log, image, audio, video, document or other", domain.ErrInvalid, kind)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	in, err := k.ingest(abs)
	if err != nil {
		return nil, err
	}
	if kind == "" {
		kind = kindFor(abs)
	}

	if prev, err := k.st.LatestArtifactForPath(abs); err != nil {
		return nil, err
	} else if prev != nil && prev.TaskID == nil && prev.DiscardedAt == nil && prev.ProjectID == task.ProjectID {
		if err := k.st.UpdateArtifact(prev.ID, domain.ArtifactPatch{
			TaskID: domain.To(&task.ID), SHA256: domain.To(in.sha), Size: domain.To(in.size),
			SourceMTime: domain.To(in.mtime), Origin: domain.To(domain.OriginAttach),
		}); err != nil {
			return nil, err
		}
		if prev.SHA256 != in.sha {
			k.dropIfOrphan(prev.SHA256)
		}
		return k.svc.GetArtifact(prev.ID)
	}

	a := &domain.Artifact{
		ProjectID: task.ProjectID, TaskID: &task.ID, Kind: kind, Name: filepath.Base(abs),
		MediaType: mediaTypeFor(abs), SHA256: in.sha, Size: in.size, Note: domain.StripControl(strings.TrimSpace(note)),
		Origin: domain.OriginAttach, SourcePath: abs, SourceMTime: in.mtime, CreatedBy: author,
	}
	if err := k.svc.RecordArtifact(a); err != nil {
		return nil, err
	}
	return k.svc.GetArtifact(a.ID)
}

type ingested struct {
	sha   string
	size  int64
	mtime int64
}

// ingest copies the file at path into the holding area, named by its hash,
// and records the blob. Contents already held are not copied twice.
func (k *Keeper) ingest(path string) (ingested, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return ingested{}, err
	}
	if !fi.Mode().IsRegular() {
		return ingested{}, fmt.Errorf("%w file %s: not a regular file", domain.ErrInvalid, path)
	}
	if k.s.MaxFile > 0 && fi.Size() > k.s.MaxFile {
		return ingested{}, fmt.Errorf("%w file %s: %s is larger than max_file (%s)", domain.ErrInvalid,
			filepath.Base(path), HumanSize(fi.Size()), HumanSize(k.s.MaxFile))
	}
	if err := k.ensureRoom(fi.Size(), filepath.Base(path)); err != nil {
		return ingested{}, err
	}
	if err := os.MkdirAll(k.tmpDir(), 0o700); err != nil {
		return ingested{}, err
	}
	tmp, err := os.CreateTemp(k.tmpDir(), "in-*")
	if err != nil {
		return ingested{}, err
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed into place

	src, err := os.Open(path)
	if err != nil {
		tmp.Close()
		return ingested{}, err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), src)
	src.Close()
	if cerr := syncClose(tmp); err == nil {
		err = cerr
	}
	if err != nil {
		return ingested{}, err
	}
	sha := hex.EncodeToString(h.Sum(nil))

	dest := k.BlobPath(sha)
	if fi, err := os.Stat(dest); err != nil || fi.Size() != n {
		// Missing, or a damaged copy: put the fresh one in its place.
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			return ingested{}, err
		}
		if err := os.Rename(tmp.Name(), dest); err != nil {
			return ingested{}, err
		}
		// Kept files never change; fetch_artifact hands out this path.
		_ = os.Chmod(dest, 0o444)
	}
	if err := k.st.PutBlob(domain.Blob{SHA256: sha, Size: n, InLocal: true}); err != nil {
		return ingested{}, err
	}
	if err := k.st.SetBlobLocal(sha, true); err != nil {
		return ingested{}, err
	}
	return ingested{sha: sha, size: n, mtime: fi.ModTime().UnixNano()}, nil
}

// dropIfOrphan removes contents from this machine at once when nothing uses
// them and the remote never received them: a superseded version of a file
// still being written. Anything else waits for housekeeping.
func (k *Keeper) dropIfOrphan(sha string) {
	refs, err := k.st.BlobRefs(sha)
	if err != nil || refs > 0 {
		return
	}
	b, err := k.st.GetBlob(sha)
	if err != nil || b.InRemote {
		return
	}
	if err := os.Remove(k.BlobPath(sha)); err == nil || errors.Is(err, os.ErrNotExist) {
		_ = k.st.SetBlobLocal(sha, false)
	}
}

// ensureRoom makes room for need more bytes, removing what may be removed,
// or returns ErrNoRoom.
func (k *Keeper) ensureRoom(need int64, name string) error {
	used, err := k.st.LocalBytes()
	if err != nil {
		return err
	}
	free, ferr := k.s.DiskFree(k.s.Dir)
	freeKnown := ferr == nil
	over := func() string {
		if k.s.LocalMax > 0 && used+need > k.s.LocalMax {
			return fmt.Sprintf("the holding area would exceed local_max (%s, %s in use)", HumanSize(k.s.LocalMax), HumanSize(used))
		}
		if freeKnown && int64(free)-need < k.s.MinFree {
			return fmt.Sprintf("the disk would have less than min_free (%s) free", HumanSize(k.s.MinFree))
		}
		return ""
	}
	if over() == "" {
		return nil
	}
	if err := k.st.MarkUnreferenced(k.s.Now()); err != nil {
		return err
	}
	cands, err := k.st.EvictionCandidates(k.s.KeepLocalUnder, k.s.Now().Add(-evictGrace))
	if err != nil {
		return err
	}
	for _, b := range cands {
		if over() == "" {
			break
		}
		if err := os.Remove(k.BlobPath(b.SHA256)); err != nil && !errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := k.st.SetBlobLocal(b.SHA256, false); err != nil {
			return err
		}
		used -= b.Size
		free += uint64(b.Size)
	}
	if why := over(); why != "" {
		return fmt.Errorf("%w to keep %s (%s): %s, and what is held cannot be removed until it is uploaded (or is kept local by policy). "+
			"Raise local_max, bring the remote store back, or discard files", ErrNoRoom, name, HumanSize(need), why)
	}
	return nil
}

// Fetch returns a local path holding an artifact's contents, pulling them
// from the remote store when they are not here.
func (k *Keeper) Fetch(ctx context.Context, id int64) (string, error) {
	a, err := k.svc.GetArtifact(id)
	if err != nil {
		return "", err
	}
	b, err := k.st.GetBlob(a.SHA256)
	if err != nil {
		return "", err
	}
	path := k.BlobPath(b.SHA256)
	if k.localIntact(b) {
		if !b.InLocal {
			_ = k.st.SetBlobLocal(b.SHA256, true)
		}
		_ = k.st.TouchBlob(b.SHA256, k.s.Now())
		return path, nil
	}
	if !b.InRemote && k.s.Remote != nil {
		// The record may predate the upload (a restored snapshot): ask.
		if key, err := k.remoteKey(b.SHA256); err == nil {
			if size, ok, err := k.s.Remote.Head(ctx, key); err == nil && ok && size == b.Size {
				_ = k.st.SetBlobRemote(b.SHA256, k.s.Now())
				b.InRemote = true
			}
		}
	}
	if k.s.Remote == nil || !b.InRemote {
		return "", fmt.Errorf("%w: %s is not on this machine and not on a remote store (sha256 %s)", ErrUnavailable, a.Name, a.SHA256)
	}
	if err := k.download(ctx, b); err != nil {
		return "", err
	}
	_ = k.st.TouchBlob(b.SHA256, k.s.Now())
	return path, nil
}

// localIntact reports whether this machine holds b's contents at their full
// size. A missing or damaged copy (cut short by a power loss, say) is moved
// aside and the record corrected, so it is neither served nor uploaded.
func (k *Keeper) localIntact(b *domain.Blob) bool {
	path := k.BlobPath(b.SHA256)
	fi, err := os.Stat(path)
	if err == nil && fi.Mode().IsRegular() && fi.Size() == b.Size {
		return true
	}
	if err == nil {
		_ = os.MkdirAll(k.tmpDir(), 0o700)
		_ = os.Rename(path, filepath.Join(k.tmpDir(), "damaged-"+b.SHA256))
	}
	if b.InLocal {
		_ = k.st.SetBlobLocal(b.SHA256, false)
		b.InLocal = false
	}
	return false
}

// syncClose flushes f to disk and closes it, so a file renamed into the
// holding area is whole even after a power loss.
func syncClose(f *os.File) error {
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
