package artifacts

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// The sweep is the safety net under attach_file: after every turn it keeps
// the files in a session's scratch folder that are not kept already, as
// unfiled artifacts of the session's project. It never guesses a ticket and
// never touches the network.

// SweepInput is one sweep: a scratch folder, the session it belongs to, and
// the project the session is working in.
type SweepInput struct {
	Dir       string
	SessionID string
	Project   *domain.Project
}

// SweepResult counts what a sweep did.
type SweepResult struct {
	Kept    int            `json:"kept"`    // new unfiled artifacts
	Updated int            `json:"updated"` // unfiled artifacts given newer contents
	Skipped int            `json:"skipped"`
	Reasons map[string]int `json:"reasons,omitempty"` // why files were skipped
}

// Limits on one sweep, so a scratch folder full of build output cannot fill
// the holding area in one turn.
const (
	sweepMaxFiles  = 500
	sweepMaxBytes  = 500 << 20
	sweepFreshness = 2 * time.Second // a file changed this recently may still be being written
)

// skipDirs are directory names the sweep does not enter: dependencies,
// caches and repositories.
var skipDirs = []string{"node_modules", ".git", "__pycache__", ".venv", "venv", ".cache", ".pytest_cache",
	".mypy_cache", ".tox", "target", "site-packages", "*.dist-info", "*.egg-info", ".gradle", ".next"}

// secretNames are file names that look like secrets. They are never kept.
var secretNames = []string{".env", ".env.*", "*.pem", "*.key", "id_rsa*", "id_ed25519*", "id_ecdsa*", "id_dsa*",
	"*.p12", "*.pfx", "credentials", "credentials.*", ".netrc", ".pgpass", "*.kdbx", "*.keystore", "*.jks",
	"*secret*", "*token*", "*password*", ".npmrc", ".pypirc", "*.ovpn"}

func matchAny(name string, patterns []string) bool {
	lower := strings.ToLower(name)
	for _, p := range patterns {
		if ok, _ := filepath.Match(strings.ToLower(p), lower); ok {
			return true
		}
	}
	return false
}

// Sweep keeps what is new or changed in in.Dir. A folder that does not exist
// is nothing to sweep.
func (k *Keeper) Sweep(ctx context.Context, in SweepInput) (SweepResult, error) {
	res := SweepResult{Reasons: map[string]int{}}
	if in.Project == nil {
		return res, errors.New("sweep: no project")
	}
	if domain.EffectivePolicy(in.Project, k.s.DefaultPolicy) == domain.PolicyOff {
		return res, nil
	}
	root, err := filepath.Abs(in.Dir)
	if err != nil {
		return res, err
	}
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return res, nil
	}
	skip := func(why string) {
		res.Skipped++
		res.Reasons[why]++
	}
	files, total := 0, int64(0)
	now := k.s.Now()
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return fs.SkipAll
		}
		if err != nil {
			return nil // unreadable: leave it
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (matchAny(name, skipDirs) || matchAny(name, k.s.SweepSkip)) {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() { // symlinks, sockets, devices
			return nil
		}
		if matchAny(name, secretNames) {
			skip("secret")
			return nil
		}
		if matchAny(name, k.s.SweepSkip) {
			skip("configured")
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if k.s.SweepMaxFile > 0 && info.Size() > k.s.SweepMaxFile {
			skip("too large")
			return nil
		}
		if now.Sub(info.ModTime()) < sweepFreshness {
			skip("being written")
			return nil
		}
		if files >= sweepMaxFiles || total+info.Size() > sweepMaxBytes {
			skip("sweep limit")
			return fs.SkipAll
		}
		did, err := k.sweepFile(path, info, in)
		switch {
		case errors.Is(err, ErrNoRoom):
			skip("no room")
		case err != nil:
			skip("error")
		case did == "kept":
			res.Kept++
			files++
			total += info.Size()
		case did == "updated":
			res.Updated++
			files++
			total += info.Size()
		}
		return nil
	})
	return res, err
}

// sweepFile keeps one file, unless it is kept already, unchanged, or was
// discarded. It reports "kept", "updated" or "".
func (k *Keeper) sweepFile(path string, info fs.FileInfo, in SweepInput) (string, error) {
	discarded, err := k.st.PathDiscarded(path)
	if err != nil || discarded {
		return "", err
	}
	latest, err := k.st.LatestArtifactForPath(path)
	if err != nil {
		return "", err
	}
	mtime := info.ModTime().UnixNano()
	if latest != nil && latest.Size == info.Size() && latest.SourceMTime == mtime {
		return "", nil
	}
	got, err := k.ingest(path)
	if err != nil {
		return "", err
	}
	if latest != nil && latest.SHA256 == got.sha {
		return "", k.st.UpdateArtifact(latest.ID, domain.ArtifactPatch{SourceMTime: domain.To(got.mtime)})
	}
	if latest != nil && latest.TaskID == nil && latest.ProjectID == in.Project.ID {
		if err := k.st.UpdateArtifact(latest.ID, domain.ArtifactPatch{
			SHA256: domain.To(got.sha), Size: domain.To(got.size), SourceMTime: domain.To(got.mtime),
		}); err != nil {
			return "", err
		}
		k.dropIfOrphan(latest.SHA256)
		return "updated", nil
	}
	a := &domain.Artifact{
		ProjectID: in.Project.ID, Kind: kindFor(path), Name: filepath.Base(path), MediaType: mediaTypeFor(path),
		SHA256: got.sha, Size: got.size, Origin: domain.OriginSweep, SourcePath: path, SourceMTime: got.mtime,
		SessionID: in.SessionID, CreatedBy: "sweep",
	}
	if err := k.svc.RecordArtifact(a); err != nil {
		return "", err
	}
	return "kept", nil
}

// RecordSkip notes that a sweep of dir captured nothing, and why, so status
// can show which directories are not covered.
func (k *Keeper) RecordSkip(dir, reason string) error {
	return k.st.RecordSkip(dir, reason, k.s.Now())
}

// RecordHookError keeps the last error of the Stop hook for status.
func (k *Keeper) RecordHookError(err error) {
	msg := ""
	if err != nil {
		msg = fmt.Sprintf("%s: %v", k.s.Now().Format(time.RFC3339), err)
	}
	_ = k.st.KVSet(kvLastHookError, msg)
}
