package availability

import (
	"context"
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

const buildPageSize = 100

var allVisibilities = []repository.Visibility{
	repository.VisibilityPublic,
	repository.VisibilityAuthenticated,
	repository.VisibilityPrivate,
}

// Usage is what an owner's current builds use. Its zero value is an owner
// with no builds.
type Usage struct {
	switches map[string]int
}

// Load reads every build ownerID has and returns what their current builds
// use.
func Load(ctx context.Context, builds repository.BuildRepository, ownerID string) (Usage, error) {
	var all []repository.Build
	cursor := ""
	for {
		page, next, err := builds.List(ctx, ownerID, allVisibilities, "", buildPageSize, cursor)
		if err != nil {
			return Usage{}, fmt.Errorf("listing builds: %w", err)
		}
		all = append(all, page...)
		if next == "" {
			return FromBuilds(all), nil
		}
		cursor = next
	}
}

// FromBuilds returns what the current builds among builds use. A keyboard's
// current build is its newest by BuildDate; one without a date is older
// than any with one, and of builds with the same date the first in builds
// is current.
func FromBuilds(builds []repository.Build) Usage {
	current := map[string]repository.Build{}
	for _, b := range builds {
		if c, ok := current[b.Keyboard]; !ok || newer(b, c) {
			current[b.Keyboard] = b
		}
	}

	u := Usage{switches: map[string]int{}}
	for _, b := range current {
		for _, e := range b.Switches {
			u.switches[e.Switch] += e.Count
		}
	}
	return u
}

func newer(a, b repository.Build) bool {
	if a.BuildDate == nil {
		return false
	}
	return b.BuildDate == nil || *a.BuildDate > *b.BuildDate
}

// SwitchQuantity is how many of sw are free: its Delivered purchases'
// quantity less what current builds use, and never below zero.
func (u Usage) SwitchQuantity(sw repository.Switch) int {
	return max(sw.Purchases.DeliveredQuantity()-u.switches[sw.ID], 0)
}
