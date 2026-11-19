package moderation

import (
	"context"
	"errors"
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/imagekey"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// Images reads and writes an image's moderation status by its object key,
// routing each key to the repository and slot it belongs to.
type Images struct {
	Keyboards  repository.KeyboardRepository
	Builds     repository.BuildRepository
	Switches   repository.SwitchRepository
	ECParts    repository.ECPartRepository
	KeycapSets repository.KeycapSetRepository
	Profiles   repository.ProfileRepository
}

// Status returns the moderation status of the image at key. current is
// false when the slot no longer holds key (the image was replaced or
// deleted, or its item is gone).
func (i *Images) Status(ctx context.Context, ref imagekey.Ref, key string) (status repository.ImageStatus, current bool, err error) {
	switch ref.Entity {
	case imagekey.Keyboard:
		kb, err := i.Keyboards.Get(ctx, ref.OwnerID, ref.ItemID)
		if err != nil {
			return absent(err)
		}
		e, ok := kb.Images[ref.ImageID]
		return e.Status.Effective(), ok && string(e.Path) == key, nil
	case imagekey.Build:
		b, err := i.Builds.Get(ctx, ref.OwnerID, ref.ItemID)
		if err != nil {
			return absent(err)
		}
		e, ok := b.Images[ref.ImageID]
		return e.Status.Effective(), ok && string(e.Path) == key, nil
	case imagekey.Switch:
		sw, err := i.Switches.Get(ctx, ref.OwnerID, ref.ItemID)
		if err != nil {
			return absent(err)
		}
		return sw.ImageStatus.Effective(), sw.ImagePath != nil && string(*sw.ImagePath) == key, nil
	case imagekey.ECPart:
		p, err := i.ECParts.Get(ctx, ref.OwnerID, ref.ItemID)
		if err != nil {
			return absent(err)
		}
		return p.ImageStatus.Effective(), p.ImagePath != nil && string(*p.ImagePath) == key, nil
	case imagekey.KeycapKit:
		set, err := i.KeycapSets.Get(ctx, ref.OwnerID, ref.ItemID)
		if err != nil {
			return absent(err)
		}
		kit, ok := set.Kits[ref.KitID]
		return kit.ImageStatus.Effective(), ok && kit.ImagePath != nil && string(*kit.ImagePath) == key, nil
	case imagekey.Profile:
		p, err := i.Profiles.Get(ctx, ref.OwnerID)
		if err != nil {
			return absent(err)
		}
		return p.AvatarStatus.Effective(), p.AvatarPath != nil && string(*p.AvatarPath) == key, nil
	}
	return "", false, fmt.Errorf("unhandled entity %q", ref.Entity)
}

// absent maps a missing item to not current, and passes other errors on.
func absent(err error) (repository.ImageStatus, bool, error) {
	if errors.Is(err, repository.ErrNotFound) {
		return "", false, nil
	}
	return "", false, err
}

// SetStatus moves the image at key from one status to another, with the
// conditional write each repository provides. ok is false when the slot
// no longer holds key or its status is no longer from.
func (i *Images) SetStatus(ctx context.Context, ref imagekey.Ref, key string, from, to repository.ImageStatus, reason *string) (ok bool, err error) {
	switch ref.Entity {
	case imagekey.Keyboard:
		return i.Keyboards.SetImageStatus(ctx, ref.OwnerID, ref.ItemID, ref.ImageID, repository.KeyboardImageKey(key), from, to, reason)
	case imagekey.Build:
		return i.Builds.SetImageStatus(ctx, ref.OwnerID, ref.ItemID, ref.ImageID, repository.BuildImageKey(key), from, to, reason)
	case imagekey.Switch:
		return i.Switches.SetImageStatus(ctx, ref.OwnerID, ref.ItemID, repository.SwitchImageKey(key), from, to, reason)
	case imagekey.ECPart:
		return i.ECParts.SetImageStatus(ctx, ref.OwnerID, ref.ItemID, repository.ECPartImageKey(key), from, to, reason)
	case imagekey.KeycapKit:
		return i.KeycapSets.SetKitImageStatus(ctx, ref.OwnerID, ref.ItemID, ref.KitID, repository.KeycapKitImageKey(key), from, to, reason)
	case imagekey.Profile:
		return i.Profiles.SetAvatarStatus(ctx, ref.OwnerID, repository.ProfileImageKey(key), from, to, reason)
	}
	return false, fmt.Errorf("unhandled entity %q", ref.Entity)
}
