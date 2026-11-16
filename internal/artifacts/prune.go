package artifacts

import (
	"context"
	"errors"
	"time"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// Prune lists the remote contents that no artifact has used for longer than
// keep_unreferenced, and deletes them from the remote only when apply is
// set. It works from this database's records and never lists the bucket, so
// it cannot delete what another machine stored there.
func (k *Keeper) Prune(ctx context.Context, apply bool) ([]domain.Blob, error) {
	if k.s.Remote == nil {
		return nil, errors.New("no remote store is configured ([artifacts] remote)")
	}
	now := k.s.Now()
	if err := k.st.MarkUnreferenced(now); err != nil {
		return nil, err
	}
	old, err := k.st.ListBlobs("in_remote = 1 AND unreferenced_at IS NOT NULL AND unreferenced_at < ?",
		now.Add(-k.s.KeepUnreferenced).UTC().Format(time.RFC3339Nano))
	if err != nil || !apply {
		return old, err
	}
	for _, b := range old {
		key, err := k.remoteKey(b.SHA256)
		if err != nil {
			return old, err
		}
		if err := k.s.Remote.Delete(ctx, key); err != nil {
			return old, err
		}
		if err := k.st.ClearBlobRemote(b.SHA256); err != nil {
			return old, err
		}
	}
	return old, nil
}
