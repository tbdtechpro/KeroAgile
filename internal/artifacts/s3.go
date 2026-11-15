package artifacts

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// S3Config names an S3-compatible bucket: Garage, MinIO, a cloud bucket or a
// NAS app.
type S3Config struct {
	Endpoint  string // e.g. http://192.0.2.10:3900
	Region    string // default us-east-1
	Bucket    string
	PathStyle bool // endpoint/bucket/key instead of bucket.endpoint/key
}

// S3 is a Remote over the S3 API: a small client that only puts, gets,
// checks and deletes single objects.
type S3 struct {
	cfg   S3Config
	creds Credentials
	http  *http.Client
	now   func() time.Time
}

func NewS3(cfg S3Config, creds Credentials) *S3 {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	cfg.Endpoint = strings.TrimRight(cfg.Endpoint, "/")
	tr := http.DefaultTransport.(*http.Transport).Clone()
	// Fail fast when the store cannot be reached (a dropped VPN): connect
	// within 3 seconds, and start answering within 10.
	tr.DialContext = (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	tr.ResponseHeaderTimeout = 10 * time.Second
	tr.TLSHandshakeTimeout = 5 * time.Second
	return &S3{cfg: cfg, creds: creds, http: &http.Client{Transport: tr}, now: time.Now}
}

func (s *S3) Name() string {
	return fmt.Sprintf("s3 %s/%s", s.cfg.Endpoint, s.cfg.Bucket)
}

func (s *S3) url(key string) (*url.URL, error) {
	u, err := url.Parse(s.cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	escaped := (&url.URL{Path: key}).EscapedPath()
	if s.cfg.PathStyle {
		u.Path = "/" + s.cfg.Bucket + "/" + key
		u.RawPath = "/" + s.cfg.Bucket + "/" + escaped
	} else {
		u.Host = s.cfg.Bucket + "." + u.Host
		u.Path = "/" + key
		u.RawPath = "/" + escaped
	}
	return u, nil
}

func (s *S3) do(ctx context.Context, method, key string, body io.Reader, size int64, payloadHash string) (*http.Response, error) {
	u, err := s.url(key)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.ContentLength = size
	}
	SignV4(req, s.creds, s.cfg.Region, payloadHash, s.now())
	return s.http.Do(req)
}

// errorFrom turns an unexpected answer into an error with the store's code.
func errorFrom(resp *http.Response, what string) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	code := ""
	if i := strings.Index(string(b), "<Code>"); i >= 0 {
		rest := string(b)[i+6:]
		if j := strings.Index(rest, "</Code>"); j >= 0 {
			code = rest[:j]
		}
	}
	if code == "" {
		code = strings.TrimSpace(string(b))
	}
	return fmt.Errorf("s3 %s: %s %s", what, resp.Status, code)
}

func (s *S3) Put(ctx context.Context, key string, r io.Reader, size int64, sha256hex string) error {
	resp, err := s.do(ctx, http.MethodPut, key, r, size, sha256hex)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return errorFrom(resp, "put "+key)
	}
	return nil
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := s.do(ctx, http.MethodGet, key, nil, 0, emptySHA256)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, errorFrom(resp, "get "+key)
	}
	return resp.Body, nil
}

func (s *S3) Head(ctx context.Context, key string) (int64, bool, error) {
	resp, err := s.do(ctx, http.MethodHead, key, nil, 0, emptySHA256)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return 0, false, nil
	case resp.StatusCode/100 == 2:
		return resp.ContentLength, true, nil
	}
	return 0, false, errorFrom(resp, "head "+key)
}

func (s *S3) Delete(ctx context.Context, key string) error {
	resp, err := s.do(ctx, http.MethodDelete, key, nil, 0, emptySHA256)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNotFound {
		return errorFrom(resp, "delete "+key)
	}
	return nil
}
