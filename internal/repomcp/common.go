package repomcp

import "github.com/roguekeyboards/mykeebs-api/internal/repository"

// ownerVisibility returns v for the item's owner and nil for anyone else.
func ownerVisibility(v repository.Visibility, isOwner bool) *string {
	if !isOwner {
		return nil
	}
	out := string(v)

	return &out
}

// anyVisible reports whether the caller may see any of images - see
// [repository.ImageStatus.VisibleTo].
func anyVisible[T any](images map[string]T, statusOf func(T) repository.ImageStatus, isOwner bool) bool {
	for _, img := range images {
		if statusOf(img).VisibleTo(isOwner) {
			return true
		}
	}

	return false
}

// imageStatusToMCP is an image's moderation status and rejection reason
// as the caller sees them: both are the owner's alone, and absent when
// there's no image.
func imageStatusToMCP(hasImage bool, status repository.ImageStatus, reason *string, isOwner bool) (*string, *string) {
	if !isOwner || !hasImage {
		return nil, nil
	}
	out := string(status.Effective())

	return &out, reason
}

// convertPtr converts a pointer to one string type into a pointer to
// another, keeping nil.
func convertPtr[To, From ~string](v *From) *To {
	if v == nil {
		return nil
	}
	return new(To(*v))
}
