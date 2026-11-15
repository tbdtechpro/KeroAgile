// Package artifacts keeps files against tickets: scripts, logs, screenshots,
// recordings. Records live in the database; contents live in a holding area
// on this machine, named by their SHA-256, and optionally on a remote store
// that larger files are moved to.
//
// Nothing here is called on the way to drawing the TUI or answering an
// ordinary MCP call; only attach, fetch, the sweep and the uploader touch
// files, and only the uploader and fetch touch the network.
package artifacts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// Settings are the [artifacts] configuration, with sizes in bytes.
type Settings struct {
	Dir              string        // the holding area
	LocalMax         int64         // most the holding area may hold
	MinFree          int64         // never take the disk below this much free space
	KeepLocalUnder   int64         // files this small stay here even once uploaded
	MaxFile          int64         // larger files are refused
	SweepMaxFile     int64         // the sweep skips larger files
	KeepUnreferenced time.Duration // how long contents nothing uses are kept
	DefaultPolicy    domain.ArtifactPolicy
	SweepSkip        []string // extra names (globs) the sweep leaves alone
	Remote           Remote   // nil: no remote store

	// Seams for tests; nil means the real thing.
	DiskFree func(dir string) (uint64, error)
	Now      func() time.Time
}

// Records is what the keeper needs from the database beyond domain.Service:
// bookkeeping for blobs and a small key-value table. store.Store provides it.
type Records interface {
	PutBlob(b domain.Blob) error
	GetBlob(sha string) (*domain.Blob, error)
	SetBlobLocal(sha string, local bool) error
	SetBlobRemote(sha string, at time.Time) error
	ClearBlobRemote(sha string) error
	TouchBlob(sha string, at time.Time) error
	LocalBytes() (int64, error)
	MarkUnreferenced(now time.Time) error
	EvictionCandidates(keepLocalUnder int64) ([]domain.Blob, error)
	ListBlobs(cond string, args ...any) ([]domain.Blob, error)
	BlobRefs(sha string) (int, error)
	UpdateArtifact(id int64, p domain.ArtifactPatch) error
	LatestArtifactForPath(path string) (*domain.Artifact, error)
	PathDiscarded(path string) (bool, error)
	ListArtifacts(q domain.ArtifactQuery) ([]*domain.Artifact, error)
	KVGet(key string) (string, bool, error)
	KVSet(key, value string) error
	KVLease(key, holder string, ttl time.Duration, now time.Time) (bool, error)
	KVInit(key, value string) (string, error)
	KVRelease(key, holder string) error
	ArtifactsForBlob(sha string) ([]*domain.Artifact, error)
	RecordSkip(dir, reason string, at time.Time) error
	ListSkips() ([]domain.SweepSkip, error)
}

// ErrNoRoom means a file was refused because keeping it would take the
// holding area over local_max or the disk below min_free, and nothing could
// be removed to make room.
var ErrNoRoom = errors.New("no room")

// ErrUnavailable means a file's contents are neither here nor on a reachable
// remote.
var ErrUnavailable = errors.New("unavailable")

// Keeper keeps files: it takes them in, finds room for them, and hands them
// back.
type Keeper struct {
	svc *domain.Service
	st  Records
	s   Settings
}

func New(svc *domain.Service, st Records, s Settings) *Keeper {
	if s.DiskFree == nil {
		s.DiskFree = diskFree
	}
	if s.Now == nil {
		s.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Keeper{svc: svc, st: st, s: s}
}

// Settings returns the keeper's settings.
func (k *Keeper) Settings() Settings { return k.s }

// BlobPath is where contents with this hash are held on this machine.
func (k *Keeper) BlobPath(sha string) string {
	if len(sha) < 4 {
		return filepath.Join(k.s.Dir, sha)
	}
	return filepath.Join(k.s.Dir, sha[:2], sha[2:4], sha)
}

func (k *Keeper) tmpDir() string { return filepath.Join(k.s.Dir, "tmp") }

// policy is what happens to files of project id.
func (k *Keeper) policy(projectID string) (domain.ArtifactPolicy, error) {
	p, err := k.svc.GetProject(projectID)
	if err != nil {
		return "", err
	}
	return domain.EffectivePolicy(p, k.s.DefaultPolicy), nil
}

// Status is the state of the holding area and the remote.
type Status struct {
	Dir           string             `json:"dir"`
	LocalBytes    int64              `json:"local_bytes"`
	LocalMax      int64              `json:"local_max"`
	DiskFree      int64              `json:"disk_free,omitempty"`
	MinFree       int64              `json:"min_free"`
	Remote        string             `json:"remote"`
	WaitingBytes  int64              `json:"waiting_bytes"` // held here only, for projects that upload
	WaitingFiles  int                `json:"waiting_files"`
	LastSync      string             `json:"last_sync,omitempty"`
	LastSyncError string             `json:"last_sync_error,omitempty"`
	LastHookError string             `json:"last_hook_error,omitempty"`
	SkippedSweeps []domain.SweepSkip `json:"skipped_sweeps,omitempty"`
	DefaultPolicy string             `json:"default_policy"`
}

func (k *Keeper) Status() (Status, error) {
	used, err := k.st.LocalBytes()
	if err != nil {
		return Status{}, err
	}
	st := Status{Dir: k.s.Dir, LocalBytes: used, LocalMax: k.s.LocalMax, MinFree: k.s.MinFree,
		Remote: "none", DefaultPolicy: string(domain.EffectivePolicy(nil, k.s.DefaultPolicy))}
	if free, err := k.s.DiskFree(k.s.Dir); err == nil {
		st.DiskFree = int64(free)
	}
	if k.s.Remote != nil {
		st.Remote = k.s.Remote.Name()
		waiting, err := k.uploadCandidates()
		if err != nil {
			return Status{}, err
		}
		for _, b := range waiting {
			st.WaitingBytes += b.Size
			st.WaitingFiles++
		}
	}
	st.LastSync, _, _ = k.st.KVGet(kvLastSync)
	st.LastSyncError, _, _ = k.st.KVGet(kvLastSyncError)
	st.LastHookError, _, _ = k.st.KVGet(kvLastHookError)
	st.SkippedSweeps, _ = k.st.ListSkips()
	return st, nil
}

const (
	kvLastSync      = "artifacts:last_sync"
	kvLastSyncError = "artifacts:last_sync_error"
	kvLastHookError = "hook:last_error"
)

// HumanSize formats n bytes as "1.5 MiB".
func HumanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// existingParent is dir, or its nearest parent that exists: the holding area
// may not have been created yet.
func existingParent(dir string) string {
	for {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}
