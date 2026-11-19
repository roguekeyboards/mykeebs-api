package repoapi

import (
	"fmt"
	"math"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// dateLayout matches how openapi_types.Date marshals/unmarshals.
const dateLayout = "2006-01-02"

func parseAPIDate(s string) (*openapi_types.Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return nil, fmt.Errorf("stored date %q does not match layout %q: %w", s, dateLayout, err)
	}

	return &openapi_types.Date{Time: t}, nil
}

// sumKnownCosts sums the non-nil components, treating a nil one as
// excluded rather than zero. Returns nil if none are set.
func sumKnownCosts(components ...*float64) *float64 {
	var total float64
	var haveAny bool
	for _, c := range components {
		if c == nil {
			continue
		}
		total += *c
		haveAny = true
	}
	if !haveAny {
		return nil //nolint:nilnil // no known-priced components is a valid, expected result
	}

	return &total
}

// roundCents rounds a cost to whole cents.
func roundCents(cost *float64) *float64 {
	if cost == nil {
		return nil
	}
	rounded := math.Round(*cost*100) / 100
	return &rounded
}

// ownerVisibility returns v for the item's owner and nil for anyone else.
func ownerVisibility(v repository.Visibility, isOwner bool) *api.Visibility {
	if !isOwner {
		return nil
	}
	out := api.Visibility(v)

	return &out
}

// imageStatusToAPI is the moderation status and rejection reason the
// caller sees: both are the owner's alone.
func imageStatusToAPI(status repository.ImageStatus, reason *string, isOwner bool) (*api.ImageStatus, *string) {
	if !isOwner {
		return nil, nil
	}
	out := api.ImageStatus(status.Effective())

	return &out, reason
}

// visibleImages keeps the images the caller may see - see
// [repository.ImageStatus.VisibleTo].
func visibleImages[T any](images []T, statusOf func(T) repository.ImageStatus, isOwner bool) []T {
	var out []T
	for _, img := range images {
		if statusOf(img).VisibleTo(isOwner) {
			out = append(out, img)
		}
	}

	return out
}

// convertPtr converts a pointer to one string type into a pointer to
// another, keeping nil.
func convertPtr[To, From ~string](v *From) *To {
	if v == nil {
		return nil
	}
	return new(To(*v))
}
