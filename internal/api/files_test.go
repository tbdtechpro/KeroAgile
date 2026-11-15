package api_test

import (
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/api"
	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/store"
	"github.com/tbdtechpro/KeroAgile/internal/syncsrv"
)

type filesFixture struct {
	patchFixture
	k    *artifacts.Keeper
	work string
}

func newFilesFixture(t *testing.T) filesFixture {
	t.Helper()
	db, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	st := store.New(db)
	svc := domain.NewService(st)
	srv := api.New(svc, st, st, "test-secret", syncsrv.ModeStandalone, nil)
	require.NoError(t, svc.CreateProject("TST", "Test", ""))
	token := loginTestUser(t, srv, svc)
	_, err = svc.CreateTask("a ticket", "", "TST", domain.TaskCreateOpts{})
	require.NoError(t, err)
	k := artifacts.New(svc, st, artifacts.Settings{Dir: t.TempDir(), LocalMax: 1 << 30, MaxFile: 1 << 30,
		KeepUnreferenced: time.Hour, DefaultPolicy: domain.PolicyLocal,
		DiskFree: func(string) (uint64, error) { return 1 << 40, nil }})
	srv.SetFiles(k)
	return filesFixture{patchFixture: patchFixture{srv: srv, svc: svc, token: token}, k: k, work: t.TempDir()}
}

func (f filesFixture) attach(t *testing.T, name, content string) *domain.Artifact {
	t.Helper()
	p := filepath.Join(f.work, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	a, err := f.k.Attach("TST-001", p, "", "", "")
	require.NoError(t, err)
	return a
}

// get fetches a URL with no Authorization header, as a browser's <a> or
// <img> does.
func (f filesFixture) get(t *testing.T, url string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	f.srv.ServeHTTP(w, req)
	return w
}

func (f filesFixture) signedURL(t *testing.T, id int64) string {
	t.Helper()
	code, body := f.do(t, http.MethodPost, "/api/artifacts/"+itoa64(id)+"/url", "")
	require.Equal(t, http.StatusOK, code, string(body))
	var v map[string]string
	require.NoError(t, json.Unmarshal(body, &v))
	require.True(t, strings.HasPrefix(v["url"], "/api/artifacts/"))
	return v["url"]
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

func TestArtifactContent_SignedURL(t *testing.T) {
	f := newFilesFixture(t)
	a := f.attach(t, "run.log", "0123456789")
	url := f.signedURL(t, a.ID)

	w := f.get(t, url, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "0123456789", w.Body.String())
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "inline")

	w = f.get(t, url+"&download=1", nil)
	_, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment"))
	assert.Equal(t, "run.log", params["filename"])

	w = f.get(t, url, map[string]string{"Range": "bytes=2-4"})
	assert.Equal(t, http.StatusPartialContent, w.Code)
	assert.Equal(t, "234", w.Body.String())

	tampered := strings.Replace(url, "sig=", "sig=0", 1)
	assert.Equal(t, http.StatusForbidden, f.get(t, tampered, nil).Code)
	other := f.attach(t, "other.log", "secret")
	swapped := strings.Replace(url, "/"+itoa64(a.ID)+"/", "/"+itoa64(other.ID)+"/", 1)
	assert.Equal(t, http.StatusForbidden, f.get(t, swapped, nil).Code, "a link is for one file only")
	assert.Equal(t, http.StatusForbidden, f.get(t, "/api/artifacts/"+itoa64(a.ID)+"/content", nil).Code)
}

func TestArtifactContent_LinkExpires(t *testing.T) {
	f := newFilesFixture(t)
	a := f.attach(t, "run.log", "x")
	url := f.signedURL(t, a.ID)
	expired := strings.Replace(url, "exp=", "exp=1", 1) // a much smaller time, with the old signature
	assert.Equal(t, http.StatusForbidden, f.get(t, expired, nil).Code)
}

func TestArtifactURL_NeedsLogin(t *testing.T) {
	f := newFilesFixture(t)
	a := f.attach(t, "run.log", "x")
	w := f.get(t, "/api/artifacts/"+itoa64(a.ID)+"/url", nil)
	assert.NotEqual(t, http.StatusOK, w.Code)
	req := httptest.NewRequest(http.MethodPost, "/api/artifacts/"+itoa64(a.ID)+"/url", nil)
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListUnfiled(t *testing.T) {
	f := newFilesFixture(t)
	a := f.attach(t, "run.log", "x")
	_, err := f.svc.FileArtifact(a.ID, "TST-001")
	require.NoError(t, err)
	code, body := f.do(t, http.MethodGet, "/api/projects/tst/artifacts?unfiled=1", "")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "[]", strings.TrimSpace(string(body)))
	code, body = f.do(t, http.MethodGet, "/api/projects/TST/artifacts", "")
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(body), "run.log")
}
