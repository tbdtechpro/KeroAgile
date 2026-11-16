package artifacts_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// scratch writes files into a fresh scratch folder, dated a minute ago so the
// sweep does not take them for files still being written.
func scratch(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		writeOld(t, filepath.Join(dir, name), content)
	}
	return dir
}

func writeOld(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	old := time.Now().Add(-time.Minute)
	require.NoError(t, os.Chtimes(path, old, old))
}

func (f *fixture) sweep(t *testing.T, dir string) artifacts.SweepResult {
	t.Helper()
	p, err := f.svc.GetProject("KA")
	require.NoError(t, err)
	res, err := f.k.Sweep(context.Background(), artifacts.SweepInput{Dir: dir, SessionID: "sess-1", Project: p})
	require.NoError(t, err)
	return res
}

func (f *fixture) unfiled(t *testing.T) []*domain.Artifact {
	t.Helper()
	list, err := f.svc.ListArtifacts(domain.ArtifactQuery{ProjectID: "KA", Unfiled: true})
	require.NoError(t, err)
	return list
}

func TestSweep_KeepsNewFilesUnfiled(t *testing.T) {
	f := newFixture(t, nil)
	dir := scratch(t, map[string]string{"driver/tui_drive.py": "print('hi')", "shot.png": "png"})

	res := f.sweep(t, dir)
	assert.Equal(t, 2, res.Kept)
	list := f.unfiled(t)
	require.Len(t, list, 2)
	for _, a := range list {
		assert.Nil(t, a.TaskID)
		assert.Equal(t, domain.OriginSweep, a.Origin)
		assert.Equal(t, "sess-1", a.SessionID)
		assert.Equal(t, "sweep", a.CreatedBy)
	}

	res = f.sweep(t, dir)
	assert.Equal(t, 0, res.Kept+res.Updated, "sweeping again does nothing")
	assert.Len(t, f.unfiled(t), 2)
}

// Review Focus 1: a log that grows between turns is one unfiled artifact
// holding its latest contents, and the versions in between are not kept.
func TestSweep_SamePathChanging_OneArtifactLatestContents(t *testing.T) {
	f := newFixture(t, nil)
	dir := t.TempDir()
	log := filepath.Join(dir, "run.log")
	var shas []string
	content := ""
	for i := 0; i < 3; i++ {
		content += "line\n"
		writeOld(t, log, content)
		// a distinct mtime each time, as a real append would give
		at := time.Now().Add(time.Duration(i-10) * time.Second)
		require.NoError(t, os.Chtimes(log, at, at))
		f.sweep(t, dir)
		list := f.unfiled(t)
		require.Len(t, list, 1, "round %d", i)
		shas = append(shas, list[0].SHA256)
	}
	final := f.unfiled(t)[0]
	got, err := os.ReadFile(f.k.BlobPath(final.SHA256))
	require.NoError(t, err)
	assert.Equal(t, content, string(got))
	for _, old := range shas[:2] {
		assert.NoFileExists(t, f.k.BlobPath(old), "superseded version removed")
	}
	used, err := f.st.LocalBytes()
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), used)
}

func TestSweep_FiledThenChanged_NewUnfiled(t *testing.T) {
	f := newFixture(t, nil)
	dir := scratch(t, map[string]string{"run.log": "v1"})
	f.sweep(t, dir)
	first := f.unfiled(t)[0]
	_, err := f.svc.FileArtifact(first.ID, "KA-001")
	require.NoError(t, err)

	f.sweep(t, dir)
	assert.Empty(t, f.unfiled(t), "unchanged after filing: nothing new")

	writeOld(t, filepath.Join(dir, "run.log"), "v2 is longer")
	f.sweep(t, dir)
	list := f.unfiled(t)
	require.Len(t, list, 1)
	assert.NotEqual(t, first.ID, list[0].ID)
	filed, err := f.svc.GetArtifact(first.ID)
	require.NoError(t, err)
	assert.Equal(t, first.SHA256, filed.SHA256, "a filed artifact is never changed")
}

func TestSweep_DiscardRemembered(t *testing.T) {
	f := newFixture(t, nil)
	dir := scratch(t, map[string]string{"junk.txt": "junk"})
	f.sweep(t, dir)
	require.NoError(t, f.svc.DiscardArtifact(f.unfiled(t)[0].ID))

	writeOld(t, filepath.Join(dir, "junk.txt"), "junk, changed")
	f.sweep(t, dir)
	assert.Empty(t, f.unfiled(t))
}

func TestSweep_SkipsSecretsDepsLargeAndFresh(t *testing.T) {
	f := newFixture(t, func(s *artifacts.Settings) { s.SweepMaxFile = 10; s.SweepSkip = []string{"*.tmp"} })
	dir := scratch(t, map[string]string{
		"keep.sh":                   "ok",
		".env":                      "SECRET=1",
		".env.local":                "SECRET=1",
		"id_ed25519":                "key",
		"server.pem":                "key",
		"aws/credentials":           "key",
		"api_token.txt":             "key",
		"node_modules/x/index.js":   "dep",
		".git/config":               "git",
		"lib/pyte-0.8.dist-info/RE": "dep",
		"big.log":                   "more than ten bytes",
		"scratch.tmp":               "configured skip",
	})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fresh.log"), []byte("fresh"), 0o644))
	require.NoError(t, os.Symlink("/etc/hostname", filepath.Join(dir, "link")))

	res := f.sweep(t, dir)
	list := f.unfiled(t)
	require.Len(t, list, 1)
	assert.Equal(t, "keep.sh", list[0].Name)
	assert.Positive(t, res.Skipped)
	assert.Positive(t, res.Reasons["secret"])
	assert.Positive(t, res.Reasons["too large"])
	assert.Positive(t, res.Reasons["being written"])
}

func TestSweep_PolicyOff(t *testing.T) {
	f := newFixture(t, nil)
	_, err := f.svc.PatchProject("KA", domain.ProjectPatch{ArtifactPolicy: domain.To(domain.PolicyOff)})
	require.NoError(t, err)
	f.sweep(t, scratch(t, map[string]string{"a.log": "a"}))
	assert.Empty(t, f.unfiled(t))
}

func TestSweep_NoRoomLeavesFileForNextTime(t *testing.T) {
	f := newFixture(t, func(s *artifacts.Settings) { s.LocalMax = 5 })
	dir := scratch(t, map[string]string{"a.log": "twelve bytes"})
	res := f.sweep(t, dir)
	assert.Equal(t, 1, res.Reasons["no room"])
	assert.Empty(t, f.unfiled(t))
	assert.FileExists(t, filepath.Join(dir, "a.log"))
}

func TestSweep_MissingFolderIsNothing(t *testing.T) {
	f := newFixture(t, nil)
	res := f.sweep(t, filepath.Join(t.TempDir(), "gone"))
	assert.Zero(t, res.Kept)
}

func TestAttach_TakesOverSweptArtifact(t *testing.T) {
	f := newFixture(t, nil)
	dir := scratch(t, map[string]string{"run.log": "v1"})
	f.sweep(t, dir)
	swept := f.unfiled(t)[0]

	writeOld(t, filepath.Join(dir, "run.log"), "v2")
	a, err := f.k.Attach("KA-001", filepath.Join(dir, "run.log"), "", "", "claude")
	require.NoError(t, err)
	assert.Equal(t, swept.ID, a.ID, "the swept artifact is taken over")
	assert.Equal(t, "KA-001", *a.TaskID)
	assert.Equal(t, domain.OriginAttach, a.Origin)
	got, err := os.ReadFile(f.k.BlobPath(a.SHA256))
	require.NoError(t, err)
	assert.Equal(t, "v2", string(got))
	assert.Empty(t, f.unfiled(t))
}

func TestRecordSkip_ShownInStatus(t *testing.T) {
	f := newFixture(t, nil)
	require.NoError(t, f.k.RecordSkip("/home/u/elsewhere", "no project for this directory"))
	require.NoError(t, f.k.RecordSkip("/home/u/elsewhere", "no project for this directory"))
	st, err := f.k.Status()
	require.NoError(t, err)
	require.Len(t, st.SkippedSweeps, 1)
	assert.Equal(t, 2, st.SkippedSweeps[0].Count)
	assert.True(t, strings.Contains(st.SkippedSweeps[0].Reason, "no project"))
}
