package artifacts_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbdtechpro/KeroAgile/internal/artifacts"
)

// AWS's worked example "GET Object" from the Signature Version 4 docs.
func TestSigV4_AWSExampleGetObject(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	require.NoError(t, err)
	req.Header.Set("Range", "bytes=0-9")
	creds := artifacts.Credentials{AccessKey: "AKIAIOSFODNN7EXAMPLE", SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"}
	at := time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)

	artifacts.SignV4(req, creds, "us-east-1", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", at)

	assert.Equal(t, "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,"+
		"SignedHeaders=host;range;x-amz-content-sha256;x-amz-date,"+
		"Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41", req.Header.Get("Authorization"))
}

func TestCreds_ProfileFromFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "credentials")
	require.NoError(t, os.WriteFile(file, []byte(`
[default]
aws_access_key_id = DEFAULTKEY
aws_secret_access_key = defaultsecret

[garage]
aws_access_key_id=GKabc
aws_secret_access_key = s3cr3t
`), 0o600))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", file)
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_PROFILE", "")

	c, err := artifacts.LoadCredentials("garage")
	require.NoError(t, err)
	assert.Equal(t, "GKabc", c.AccessKey)
	assert.Equal(t, "s3cr3t", c.SecretKey)

	c, err = artifacts.LoadCredentials("")
	require.NoError(t, err)
	assert.Equal(t, "DEFAULTKEY", c.AccessKey)

	t.Setenv("AWS_ACCESS_KEY_ID", "ENVKEY")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "envsecret")
	c, err = artifacts.LoadCredentials("")
	require.NoError(t, err)
	assert.Equal(t, "ENVKEY", c.AccessKey, "with no profile named, the environment comes first")

	_, err = artifacts.LoadCredentials("missing")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "s3cr3t")
}

// fakeS3 is a bucket in memory behind path-style URLs. It refuses requests
// that are not signed, and uploads whose body does not match the hash they
// were signed with.
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	puts    int
	srv     *httptest.Server
}

func newFakeS3(t *testing.T) *fakeS3 {
	f := &fakeS3{objects: map[string][]byte{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=AK/") {
			http.Error(w, "unsigned", http.StatusForbidden)
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/bucket/")
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			sum := sha256.Sum256(body)
			if hex.EncodeToString(sum[:]) != r.Header.Get("X-Amz-Content-Sha256") {
				http.Error(w, "XAmzContentSHA256Mismatch", http.StatusBadRequest)
				return
			}
			f.objects[key] = body
			f.puts++
		case http.MethodGet, http.MethodHead:
			b, ok := f.objects[key]
			if !ok {
				http.Error(w, "NoSuchKey", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(b)))
			if r.Method == http.MethodGet {
				_, _ = w.Write(b)
			}
		case http.MethodDelete:
			delete(f.objects, key)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeS3) remote() *artifacts.S3 {
	return artifacts.NewS3(artifacts.S3Config{Endpoint: f.srv.URL, Region: "garage", Bucket: "bucket", PathStyle: true},
		artifacts.Credentials{AccessKey: "AK", SecretKey: "SK"})
}

func TestS3_PutHeadGetDelete(t *testing.T) {
	f := newFakeS3(t)
	s := f.remote()
	ctx := context.Background()
	body := []byte("contents")
	sum := sha256.Sum256(body)

	require.NoError(t, s.Put(ctx, "inst/abc", bytes.NewReader(body), int64(len(body)), hex.EncodeToString(sum[:])))
	size, ok, err := s.Head(ctx, "inst/abc")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, int64(len(body)), size)

	rc, err := s.Get(ctx, "inst/abc")
	require.NoError(t, err)
	got, _ := io.ReadAll(rc)
	rc.Close()
	assert.Equal(t, body, got)

	require.NoError(t, s.Delete(ctx, "inst/abc"))
	_, ok, err = s.Head(ctx, "inst/abc")
	require.NoError(t, err)
	assert.False(t, ok)

	err = s.Put(ctx, "inst/bad", bytes.NewReader(body), int64(len(body)), strings.Repeat("0", 64))
	require.Error(t, err, "the store refuses contents that do not match their hash")
	assert.Contains(t, s.Name(), "bucket")
}
