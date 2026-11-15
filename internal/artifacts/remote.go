package artifacts

import (
	"context"
	"io"
)

// Remote is a store that holds contents away from this machine: an
// S3-compatible bucket or a folder. Keys are "<install-id>/<sha256>".
type Remote interface {
	// Put stores size bytes from r under key; sha256hex is their hash, which
	// a store may use to check what it received.
	Put(ctx context.Context, key string, r io.Reader, size int64, sha256hex string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Head reports whether key exists and its size.
	Head(ctx context.Context, key string) (size int64, ok bool, err error)
	Delete(ctx context.Context, key string) error
	// Name describes the store for status output, without secrets.
	Name() string
}
