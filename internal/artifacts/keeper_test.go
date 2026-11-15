package artifacts_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/store"
)

type fixture struct {
	svc  *domain.Service
	st   *store.Store
	k    *artifacts.Keeper
	set  artifacts.Settings
	work string // where test files are written
}

// newFixture is project KA with tickets KA-001 and KA-002, user claude, and a
// keeper holding files in a temporary directory, with generous limits unless
// tune changes them.
func newFixture(t *testing.T, tune func(*artifacts.Settings)) *fixture {
	t.Helper()
	db, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	st := store.New(db)
	svc := domain.NewService(st)
	require.NoError(t, svc.CreateProject("KA", "k", ""))
	_, err = svc.CreateUser("claude", "Claude", true)
	require.NoError(t, err)
	for _, title := range []string{"one", "two"} {
		_, err := svc.CreateTask(title, "", "KA", domain.TaskCreateOpts{})
		require.NoError(t, err)
	}
	set := artifacts.Settings{
		Dir:              filepath.Join(t.TempDir(), "artifacts"),
		LocalMax:         1 << 30,
		MinFree:          0,
		KeepLocalUnder:   256 << 10,
		MaxFile:          1 << 30,
		SweepMaxFile:     100 << 20,
		KeepUnreferenced: 14 * 24 * time.Hour,
		DefaultPolicy:    domain.PolicyLocal,
		DiskFree:         func(string) (uint64, error) { return 1 << 40, nil },
	}
	if tune != nil {
		tune(&set)
	}
	return &fixture{svc: svc, st: st, k: artifacts.New(svc, st, set), set: set, work: t.TempDir()}
}

func (f *fixture) file(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(f.work, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

func TestAttach_StoresContentsOnceByHash(t *testing.T) {
	f := newFixture(t, nil)
	p1 := f.file(t, "a/run.sh", "echo hi\n")
	p2 := f.file(t, "b/copy.sh", "echo hi\n")

	a1, err := f.k.Attach("ka-001", p1, "", "first", "claude")
	require.NoError(t, err)
	a2, err := f.k.Attach("KA-002", p2, "", "", "claude")
	require.NoError(t, err)

	assert.Equal(t, a1.SHA256, a2.SHA256)
	assert.NotEqual(t, a1.ID, a2.ID)
	assert.Equal(t, "KA-001", *a1.TaskID)
	assert.Equal(t, domain.KindScript, a1.Kind)
	assert.Equal(t, "run.sh", a1.Name)
	assert.Equal(t, "first", a1.Note)
	assert.Equal(t, p1, a1.SourcePath)
	assert.Equal(t, domain.OriginAttach, a1.Origin)
	assert.Equal(t, int64(8), a1.Size)

	used, err := f.st.LocalBytes()
	require.NoError(t, err)
	assert.Equal(t, int64(8), used, "one copy")
	got, err := os.ReadFile(f.k.BlobPath(a1.SHA256))
	require.NoError(t, err)
	assert.Equal(t, "echo hi\n", string(got))

	task, err := f.svc.GetTask("KA-001")
	require.NoError(t, err)
	require.Len(t, task.Artifacts, 1)
	assert.Equal(t, domain.WhereHere, task.Artifacts[0].Where)
}

func TestAttach_KindFromExtension(t *testing.T) {
	f := newFixture(t, nil)
	cases := map[string]domain.ArtifactKind{
		"x.sh": domain.KindScript, "x.py": domain.KindScript, "x.go": domain.KindScript,
		"x.log": domain.KindLog, "x.out": domain.KindLog,
		"x.png": domain.KindImage, "x.jpg": domain.KindImage, "x.gif": domain.KindImage,
		"x.wav": domain.KindAudio, "x.mp3": domain.KindAudio,
		"x.mp4": domain.KindVideo, "x.webm": domain.KindVideo,
		"x.pdf": domain.KindDocument, "x.md": domain.KindDocument,
		"x.bin": domain.KindOther,
	}
	for name, want := range cases {
		a, err := f.k.Attach("KA-001", f.file(t, name, name), "", "", "")
		require.NoError(t, err, name)
		assert.Equal(t, want, a.Kind, name)
	}
	a, err := f.k.Attach("KA-001", f.file(t, "y.txt", "y"), domain.KindLog, "", "")
	require.NoError(t, err)
	assert.Equal(t, domain.KindLog, a.Kind, "a given kind wins")
	_, err = f.k.Attach("KA-001", f.file(t, "z.txt", "z"), "movie", "", "")
	require.ErrorIs(t, err, domain.ErrInvalid)
}

func TestAttach_RefusesWhatIsNotAFile(t *testing.T) {
	f := newFixture(t, nil)
	_, err := f.k.Attach("KA-001", f.work, "", "", "")
	require.ErrorIs(t, err, domain.ErrInvalid)
	_, err = f.k.Attach("KA-001", filepath.Join(f.work, "missing"), "", "", "")
	require.Error(t, err)
	_, err = f.k.Attach("KA-404", f.file(t, "x.log", "x"), "", "", "")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAttach_TooLarge(t *testing.T) {
	f := newFixture(t, func(s *artifacts.Settings) { s.MaxFile = 10 })
	_, err := f.k.Attach("KA-001", f.file(t, "big.log", "more than ten bytes"), "", "", "")
	require.ErrorIs(t, err, domain.ErrInvalid)
	assert.Contains(t, err.Error(), "max_file")
}

// Review Focus 3: the holding area is full of things that cannot be removed.
func TestAttach_NoRoom_RefusesAndKeepsWhatIsKept(t *testing.T) {
	f := newFixture(t, func(s *artifacts.Settings) { s.LocalMax = 100 })
	kept, err := f.k.Attach("KA-001", f.file(t, "kept.log", string(make([]byte, 90))), "", "", "")
	require.NoError(t, err)

	_, err = f.k.Attach("KA-001", f.file(t, "new.log", string(make([]byte, 20))+"x"), "", "", "")
	require.ErrorIs(t, err, artifacts.ErrNoRoom)
	assert.Contains(t, err.Error(), "local_max")

	path, err := f.k.Fetch(context.Background(), kept.ID)
	require.NoError(t, err, "nothing already kept was dropped")
	assert.FileExists(t, path)
	entries, err := os.ReadDir(filepath.Join(f.set.Dir, "tmp"))
	if err == nil {
		assert.Empty(t, entries, "no partial copy left behind")
	}
}

func TestAttach_MakesRoomFromContentsNothingUses(t *testing.T) {
	f := newFixture(t, func(s *artifacts.Settings) { s.LocalMax = 100 })
	old, err := f.k.Attach("KA-001", f.file(t, "old.log", string(make([]byte, 90))), "", "", "")
	require.NoError(t, err)
	require.NoError(t, f.svc.DiscardArtifact(old.ID))

	a, err := f.k.Attach("KA-001", f.file(t, "new.log", "fits once the old one goes"), "", "", "")
	require.NoError(t, err)
	assert.FileExists(t, f.k.BlobPath(a.SHA256))
	assert.NoFileExists(t, f.k.BlobPath(old.SHA256))
	b, err := f.st.GetBlob(old.SHA256)
	require.NoError(t, err)
	assert.False(t, b.InLocal)
}

func TestAttach_KeepsTheDiskAboveMinFree(t *testing.T) {
	f := newFixture(t, func(s *artifacts.Settings) {
		s.MinFree = 1000
		s.DiskFree = func(string) (uint64, error) { return 1010, nil }
	})
	_, err := f.k.Attach("KA-001", f.file(t, "x.log", "twenty bytes of text"), "", "", "")
	require.ErrorIs(t, err, artifacts.ErrNoRoom)
	assert.Contains(t, err.Error(), "min_free")
}

func TestAttach_PolicyOffRefused(t *testing.T) {
	f := newFixture(t, nil)
	_, err := f.svc.PatchProject("KA", domain.ProjectPatch{ArtifactPolicy: domain.To(domain.PolicyOff)})
	require.NoError(t, err)
	_, err = f.k.Attach("KA-001", f.file(t, "x.log", "x"), "", "", "")
	require.ErrorIs(t, err, domain.ErrInvalid)
	assert.Contains(t, err.Error(), "off")
}

// Review Focus 4: a database restored from a snapshot says a file is here,
// but the holding area no longer has it.
func TestFetch_MissingLocalFileIsUnavailable(t *testing.T) {
	f := newFixture(t, nil)
	a, err := f.k.Attach("KA-001", f.file(t, "x.log", "x"), "", "", "")
	require.NoError(t, err)
	require.NoError(t, os.Remove(f.k.BlobPath(a.SHA256)))

	_, err = f.k.Fetch(context.Background(), a.ID)
	require.ErrorIs(t, err, artifacts.ErrUnavailable)
	assert.Contains(t, err.Error(), a.SHA256)

	got, err := f.svc.GetArtifact(a.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.WhereUnavailable, got.Where, "the record is corrected")
}

func TestFileAndDiscard(t *testing.T) {
	f := newFixture(t, nil)
	a, err := f.k.Attach("KA-001", f.file(t, "x.log", "x"), "", "", "")
	require.NoError(t, err)

	moved, err := f.svc.FileArtifact(a.ID, "ka-002")
	require.NoError(t, err)
	assert.Equal(t, "KA-002", *moved.TaskID)

	require.NoError(t, f.svc.DiscardArtifact(a.ID))
	_, err = f.svc.FileArtifact(a.ID, "KA-001")
	require.ErrorIs(t, err, domain.ErrInvalid)
	list, err := f.svc.ListArtifacts(domain.ArtifactQuery{TaskID: "KA-002"})
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestFileArtifact_OtherProjectRefused(t *testing.T) {
	f := newFixture(t, nil)
	require.NoError(t, f.svc.CreateProject("OT", "other", ""))
	_, err := f.svc.CreateTask("theirs", "", "OT", domain.TaskCreateOpts{})
	require.NoError(t, err)
	a, err := f.k.Attach("KA-001", f.file(t, "x.log", "x"), "", "", "")
	require.NoError(t, err)
	_, err = f.svc.FileArtifact(a.ID, "OT-001")
	require.ErrorIs(t, err, domain.ErrInvalid)
}

func TestStatus(t *testing.T) {
	f := newFixture(t, func(s *artifacts.Settings) { s.LocalMax = 1000 })
	_, err := f.k.Attach("KA-001", f.file(t, "x.log", "0123456789"), "", "", "")
	require.NoError(t, err)
	st, err := f.k.Status()
	require.NoError(t, err)
	assert.Equal(t, int64(10), st.LocalBytes)
	assert.Equal(t, int64(1000), st.LocalMax)
	assert.Equal(t, "none", st.Remote)
}

var _ = errors.Is

// File names and directories are shown in terminals; escape sequences in
// them would be obeyed.
func TestNamesAndSkipsLoseControlCharacters(t *testing.T) {
	f := newFixture(t, nil)
	a, err := f.k.Attach("KA-001", f.file(t, "evil\x1b[2Jname\n.log", "x"), "", "", "")
	require.NoError(t, err)
	assert.Equal(t, "evil[2Jname.log", a.Name)
	assert.Contains(t, a.SourcePath, "\x1b", "the source path stays exact, to find the file again")

	require.NoError(t, f.k.RecordSkip("/home/u/\x1b]0;title\x07dir", "no project\x1b[31m"))
	st, err := f.k.Status()
	require.NoError(t, err)
	assert.Equal(t, "/home/u/]0;titledir", st.SkippedSweeps[0].Dir)
	assert.Equal(t, "no project[31m", st.SkippedSweeps[0].Reason)
}
