package availability

import (
	"context"
	"fmt"
	"strings"

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
	kits     map[kitRef]bool
}

type kitRef struct {
	setID, kitID string
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

	u := Usage{switches: map[string]int{}, kits: map[kitRef]bool{}}
	for _, b := range current {
		for _, e := range b.Switches {
			u.switches[e.Switch] += e.Count
		}
		for _, e := range b.KeycapKits {
			u.kits[kitRef{e.KeycapSet, e.Kit}] = true
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

// KeycapSetAvailable reports whether one of set's base kits is Delivered
// and not on a current build. The base kits are the primary kit, if set;
// otherwise the only kit, if there's one; otherwise every kit whose name
// contains "base", in any case.
func (u Usage) KeycapSetAvailable(set repository.KeycapSet) bool {
	for _, k := range baseKits(set) {
		delivered := k.Purchase.OrderStatus != nil && *k.Purchase.OrderStatus == "Delivered"
		if delivered && !u.kits[kitRef{set.ID, k.KitID}] {
			return true
		}
	}
	return false
}

func baseKits(set repository.KeycapSet) []repository.KeycapKit {
	if set.PrimaryKitID != nil {
		if k, ok := set.Kits[*set.PrimaryKitID]; ok {
			return []repository.KeycapKit{k}
		}
	}
	if len(set.Kits) == 1 {
		for _, k := range set.Kits {
			return []repository.KeycapKit{k}
		}
	}

	var bases []repository.KeycapKit
	for _, k := range set.Kits {
		if strings.Contains(strings.ToLower(k.Name), "base") {
			bases = append(bases, k)
		}
	}
	return bases
}
