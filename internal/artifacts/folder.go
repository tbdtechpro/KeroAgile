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
)

// Folder is a Remote that is a directory the operating system already has
// mounted: a NAS share, a second disk, a synced folder. A key
// "<install>/<sha256>" is stored at <root>/<install>/<aa>/<sha256>.
//
// A network share that stalls (a dropped VPN) blocks the calls that touch
// it. Each call here gives up when its context ends, so the uploader and a
// fetch move on; the blocked system call is left behind until the share
// answers.
type Folder struct {
	root string
}

func NewFolder(root string) *Folder { return &Folder{root: filepath.Clean(root)} }

func (f *Folder) Name() string { return "folder " + f.root }

// path maps a key into the folder, refusing one that would leave it.
func (f *Folder) path(key string) (string, error) {
	dir, sha := filepath.Split(filepath.FromSlash(key))
	if sha == "" || strings.Contains(key, "..") || filepath.IsAbs(key) || len(sha) < 2 {
		return "", fmt.Errorf("folder store: bad key %q", key)
	}
	return filepath.Join(f.root, dir, sha[:2], sha), nil
}

// mounted fails when the folder itself is missing: an unmounted share must
// not look like an empty store.
func (f *Folder) mounted() error {
	fi, err := os.Stat(f.root)
	if err != nil {
		return fmt.Errorf("folder store %s: %w", f.root, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("folder store %s: not a directory", f.root)
	}
	return nil
}

// withCtx runs fn, giving up when ctx ends.
func withCtx[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	type result struct {
		v   T
		err error
	}
	ch := make(chan result, 1)
	go func() {
		v, err := fn()
		ch <- result{v, err}
	}()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-ctx.Done():
		var zero T
		return zero, fmt.Errorf("folder store %w: %v", ErrUnavailable, ctx.Err())
	}
}

func (f *Folder) Put(ctx context.Context, key string, r io.Reader, size int64, sha256hex string) error {
	dest, err := f.path(key)
	if err != nil {
		return err
	}
	_, err = withCtx(ctx, func() (struct{}, error) {
		if err := f.mounted(); err != nil {
			return struct{}{}, err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return struct{}{}, err
		}
		tmp, err := os.CreateTemp(filepath.Dir(dest), ".put-*")
		if err != nil {
			return struct{}{}, err
		}
		defer os.Remove(tmp.Name())
		h := sha256.New()
		n, err := io.Copy(io.MultiWriter(tmp, h), r)
		if cerr := tmp.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return struct{}{}, err
		}
		if n != size || hex.EncodeToString(h.Sum(nil)) != sha256hex {
			return struct{}{}, fmt.Errorf("folder store: %s arrived damaged", key)
		}
		return struct{}{}, os.Rename(tmp.Name(), dest)
	})
	return err
}

func (f *Folder) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	p, err := f.path(key)
	if err != nil {
		return nil, err
	}
	return withCtx(ctx, func() (io.ReadCloser, error) {
		if err := f.mounted(); err != nil {
			return nil, err
		}
		return os.Open(p)
	})
}

type headResult struct {
	size int64
	ok   bool
}

func (f *Folder) Head(ctx context.Context, key string) (int64, bool, error) {
	p, err := f.path(key)
	if err != nil {
		return 0, false, err
	}
	r, err := withCtx(ctx, func() (headResult, error) {
		if err := f.mounted(); err != nil {
			return headResult{}, err
		}
		fi, err := os.Stat(p)
		if errors.Is(err, os.ErrNotExist) {
			return headResult{}, nil
		}
		if err != nil {
			return headResult{}, err
		}
		return headResult{fi.Size(), true}, nil
	})
	return r.size, r.ok, err
}

func (f *Folder) Delete(ctx context.Context, key string) error {
	p, err := f.path(key)
	if err != nil {
		return err
	}
	_, err = withCtx(ctx, func() (struct{}, error) {
		if err := f.mounted(); err != nil {
			return struct{}{}, err
		}
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}
