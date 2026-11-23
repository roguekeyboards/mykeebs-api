package buildrefs

import (
	"context"
	"errors"
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// FieldError reports that Field's Value doesn't match a real resource
// owned by the caller. Shaped like
// [github.com/roguekeyboards/mykeebs-api/internal/lookup.FieldError] (Field/Value,
// no Category) so REST/MCP callers can render it the same way they
// already render lookup validation failures.
type FieldError struct {
	Field  string
	Value  string
	Reason string
}

// ValidateReferences also checks that Plate and PCB are ids of the
// referenced keyboard's plates and PCBs, and that the build lists the part
// kind its build type takes: ec_parts for EC, switches otherwise. A list of
// the wrong kind is reported, not fetched. It doesn't report an unset/empty Keyboard or
// empty Switches/ECParts/KeycapKits entry - required/minLength constraints
// upstream already guard those.
//
// "Doesn't exist" and "exists but owned by someone else" are reported
// identically (every Get's ErrNotFound is already scoped to ownerID's
// collection), so this can't be used to probe whether an id exists under
// another account.
func ValidateReferences(
	ctx context.Context,
	ownerID string,
	b repository.Build,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	ecPartRepo repository.ECPartRepository,
	keycapSetRepo repository.KeycapSetRepository,
) ([]FieldError, error) {
	fieldErrs, switches, ecParts := partKindErrors(b)

	if b.Keyboard != "" {
		kb, err := keyboardRepo.Get(ctx, ownerID, b.Keyboard)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			fieldErrs = append(fieldErrs, FieldError{
				Field: "keyboard", Value: b.Keyboard,
				Reason: "does not reference a keyboard in your collection",
			})
		case err != nil:
			return nil, fmt.Errorf("checking keyboard %q: %w", b.Keyboard, err)
		default:
			if b.Plate != nil && kb.Plate(b.Plate) == nil {
				fieldErrs = append(fieldErrs, FieldError{
					Field: "plate", Value: *b.Plate,
					Reason: fmt.Sprintf("is not the id of one of keyboard %q's plates", b.Keyboard),
				})
			}
			if b.PCB != nil && kb.PCB(b.PCB) == nil {
				fieldErrs = append(fieldErrs, FieldError{
					Field: "pcb", Value: *b.PCB,
					Reason: fmt.Sprintf("is not the id of one of keyboard %q's pcbs", b.Keyboard),
				})
			}
		}
	}

	for i, entry := range switches {
		if entry.Switch == "" {
			continue
		}

		_, err := switchRepo.Get(ctx, ownerID, entry.Switch)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			fieldErrs = append(fieldErrs, FieldError{
				Field: fmt.Sprintf("switches[%d].switch", i), Value: entry.Switch,
				Reason: "does not reference a switch in your collection",
			})
		case err != nil:
			return nil, fmt.Errorf("checking switch %q: %w", entry.Switch, err)
		}
	}

	for i, entry := range ecParts {
		if entry.ECPart == "" {
			continue
		}

		_, err := ecPartRepo.Get(ctx, ownerID, entry.ECPart)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			fieldErrs = append(fieldErrs, FieldError{
				Field: fmt.Sprintf("ec_parts[%d].ec_part", i), Value: entry.ECPart,
				Reason: "does not reference an EC part in your collection",
			})
		case err != nil:
			return nil, fmt.Errorf("checking EC part %q: %w", entry.ECPart, err)
		}
	}

	keycapSets := make(map[string]*repository.KeycapSet)
	notFoundKeycapSets := make(map[string]bool)

	for i, entry := range b.KeycapKits {
		if entry.KeycapSet == "" {
			continue
		}

		ks, ok := keycapSets[entry.KeycapSet]
		if !ok {
			if notFoundKeycapSets[entry.KeycapSet] {
				fieldErrs = append(fieldErrs, FieldError{
					Field: fmt.Sprintf("keycap_kits[%d].keycap_set", i), Value: entry.KeycapSet,
					Reason: "does not reference a keycap set in your collection",
				})
				continue
			}

			var err error
			ks, err = keycapSetRepo.Get(ctx, ownerID, entry.KeycapSet)
			if errors.Is(err, repository.ErrNotFound) {
				notFoundKeycapSets[entry.KeycapSet] = true
				fieldErrs = append(fieldErrs, FieldError{
					Field: fmt.Sprintf("keycap_kits[%d].keycap_set", i), Value: entry.KeycapSet,
					Reason: "does not reference a keycap set in your collection",
				})
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("checking keycap set %q: %w", entry.KeycapSet, err)
			}
			keycapSets[entry.KeycapSet] = ks
		}

		if entry.Kit == "" {
			continue
		}

		if _, ok := ks.Kits[entry.Kit]; !ok {
			fieldErrs = append(fieldErrs, FieldError{
				Field: fmt.Sprintf("keycap_kits[%d].kit", i), Value: entry.Kit,
				Reason: fmt.Sprintf("does not reference a kit in keycap set %q", entry.KeycapSet),
			})
		}
	}

	return fieldErrs, nil
}

func partKindErrors(b repository.Build) ([]FieldError, []repository.BuildSwitchEntry, []repository.BuildECPartEntry) {
	buildType := b.EffectiveBuildType()
	if buildType == repository.BuildTypeEC {
		if len(b.Switches) > 0 {
			return []FieldError{{Field: "switches", Reason: "not allowed when build_type is EC; list ec_parts instead"}}, nil, b.ECParts
		}
		return nil, nil, b.ECParts
	}
	if len(b.ECParts) > 0 {
		return []FieldError{{Field: "ec_parts", Reason: fmt.Sprintf("only allowed when build_type is EC, not %s; list switches instead", buildType)}}, b.Switches, nil
	}
	return nil, b.Switches, nil
}
