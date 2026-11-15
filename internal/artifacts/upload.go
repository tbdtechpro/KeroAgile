package artifacts

import (
	"context"
	"fmt"

	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// uploadCandidates are blobs held only here that a project lets upload.
// The uploader (KA-078) fills this in; with no remote nothing waits.
func (k *Keeper) uploadCandidates() ([]domain.Blob, error) { return nil, nil }

// download pulls a blob from the remote store into the holding area.
func (k *Keeper) download(ctx context.Context, b *domain.Blob) error {
	return fmt.Errorf("%w: downloading is not built yet", ErrUnavailable)
}
