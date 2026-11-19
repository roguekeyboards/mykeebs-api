// Package imagekey parses an image's object key back into the item and
// image slot it belongs to. The moderation Lambda and the operator CLI
// both start from a bare key, so they share this parsing. The formats are
// the ones repository.New*ImageKey build.
package imagekey

import (
	"errors"
	"fmt"
	"strings"
)

// Entity is the kind of item an image belongs to.
type Entity string

// The entities that carry images.
const (
	Keyboard  Entity = "keyboard"
	Build     Entity = "build"
	Switch    Entity = "switch"
	ECPart    Entity = "ec_part"
	KeycapKit Entity = "keycap_kit"
	Profile   Entity = "profile"
)

// ErrUnknownKey is returned for a key that isn't a current image key format.
var ErrUnknownKey = errors.New("unknown image key format")

// Ref is the item and slot a key addresses. ItemID is the keyboard,
// build, switch, EC part or keycap set id (empty for a profile); KitID is set only
// for a keycap kit. ImageID is the images map key for keyboards and
// builds, and the upload id for the single-image slots.
type Ref struct {
	Entity  Entity
	OwnerID string
	ItemID  string
	KitID   string
	ImageID string
}

// Parse returns the Ref key addresses, or ErrUnknownKey. key is the
// object key as stored, not URL-encoded: an S3 event notification's key
// must be decoded first.
func Parse(key string) (Ref, error) {
	p := strings.Split(key, "/")
	switch {
	case match(p, "keyboards", "", "", "images", ""):
		return Ref{Entity: Keyboard, OwnerID: p[1], ItemID: p[2], ImageID: p[4]}, nil
	case match(p, "builds", "", "", "images", ""):
		return Ref{Entity: Build, OwnerID: p[1], ItemID: p[2], ImageID: p[4]}, nil
	case match(p, "switches", "", "", "image", ""):
		return Ref{Entity: Switch, OwnerID: p[1], ItemID: p[2], ImageID: p[4]}, nil
	case match(p, "ec-parts", "", "", "image", ""):
		return Ref{Entity: ECPart, OwnerID: p[1], ItemID: p[2], ImageID: p[4]}, nil
	case match(p, "keycap-sets", "", "", "kits", "", "image", ""):
		return Ref{Entity: KeycapKit, OwnerID: p[1], ItemID: p[2], KitID: p[4], ImageID: p[6]}, nil
	case match(p, "profiles", "", "avatar", ""):
		return Ref{Entity: Profile, OwnerID: p[1], ImageID: p[3]}, nil
	}
	return Ref{}, fmt.Errorf("%w: %q", ErrUnknownKey, key)
}

// match reports whether parts has pattern's shape: the same length, each
// literal in pattern matched exactly, and every "" in pattern a non-empty
// id segment.
func match(parts []string, pattern ...string) bool {
	if len(parts) != len(pattern) {
		return false
	}
	for i, want := range pattern {
		if parts[i] == "" || (want != "" && parts[i] != want) {
			return false
		}
	}
	return true
}
