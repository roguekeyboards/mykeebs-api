package cascadedelete

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// ECPartResult is a DeleteECPart call's success return value.
// DeletedBuildIDs is empty except in cascade mode.
type ECPartResult struct {
	Result
}

// DeleteECPart deletes the EC part identified by (ownerID, ecPartID),
// applying onDelete's policy toward any build that still references it:
//
//   - OnDeleteBlock: if any build references ecPartID, deletes nothing and
//     returns a *BlockedError listing those build ids.
//   - OnDeleteCascade: deletes every build referencing ecPartID first, then
//     the part. Returns the deleted build ids.
//
// Each item's S3 image objects are deleted before its DynamoDB record, so
// a failed call can be retried to completion. A missing part is not an
// error. DeleteECPart does no authorization; ownerID must already be the
// caller's own subject.
func DeleteECPart(
	ctx context.Context,
	ecPartRepo repository.ECPartRepository,
	buildRepo repository.BuildRepository,
	buildImages repository.BuildImageStore,
	ecPartImages repository.ECPartImageStore,
	ownerID, ecPartID string,
	onDelete OnDelete,
) (ECPartResult, error) {
	switch onDelete {
	case OnDeleteBlock, OnDeleteCascade:
	default:
		return ECPartResult{}, fmt.Errorf("deleting EC part %q: unknown on_delete value %q", ecPartID, onDelete)
	}

	deletePart := func() error {
		p, err := ecPartRepo.Get(ctx, ownerID, ecPartID)
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("getting EC part %q for image cleanup: %w", ecPartID, err)
		}
		if p.ImagePath != nil {
			if err := ecPartImages.Delete(ctx, *p.ImagePath); err != nil {
				return fmt.Errorf("deleting image for EC part %q: %w", ecPartID, err)
			}
		}
		if err := ecPartRepo.Delete(ctx, ecPartID); err != nil {
			return fmt.Errorf("deleting EC part %q: %w", ecPartID, err)
		}
		return nil
	}

	buildIDs, err := buildRepo.FindBuildsReferencingECPart(ctx, ownerID, ecPartID)
	if err != nil {
		return ECPartResult{}, fmt.Errorf("finding builds referencing EC part %q: %w", ecPartID, err)
	}

	if onDelete == OnDeleteBlock {
		if len(buildIDs) > 0 {
			return ECPartResult{}, &BlockedError{BuildIDs: buildIDs}
		}
		return ECPartResult{}, deletePart()
	}

	errs := make([]error, len(buildIDs))
	var wg sync.WaitGroup
	for i, buildID := range buildIDs {
		wg.Go(func() {
			b, err := buildRepo.Get(ctx, ownerID, buildID)
			if errors.Is(err, repository.ErrNotFound) {
				return
			}
			if err != nil {
				errs[i] = fmt.Errorf("getting build %q for image cleanup: %w", buildID, err)
				return
			}
			if err := deleteBuildImages(ctx, buildImages, b); err != nil {
				errs[i] = fmt.Errorf("deleting images for build %q referencing EC part %q: %w", buildID, ecPartID, err)
				return
			}
			if err := buildRepo.Delete(ctx, buildID); err != nil {
				errs[i] = fmt.Errorf("cascade-deleting build %q referencing EC part %q: %w", buildID, ecPartID, err)
			}
		})
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return ECPartResult{}, err
	}

	if err := deletePart(); err != nil {
		return ECPartResult{}, fmt.Errorf("deleting EC part %q after cascading %d build(s): %w", ecPartID, len(buildIDs), err)
	}

	return ECPartResult{Result: Result{DeletedBuildIDs: buildIDs}}, nil
}
