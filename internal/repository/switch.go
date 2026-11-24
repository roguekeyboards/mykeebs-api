package repository

import (
	"context"
	"fmt"

	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
)

// SwitchMaterial is the housing/stem material makeup of a switch.
type SwitchMaterial struct {
	TopHousing    *string `dynamodbav:"top_housing,omitempty" json:"top_housing,omitempty"`
	BottomHousing *string `dynamodbav:"bottom_housing,omitempty" json:"bottom_housing,omitempty"`
	Stem          *string `dynamodbav:"stem,omitempty" json:"stem,omitempty"`
}

// SwitchForce is a switch's nominal actuation/bottom-out force, in grams.
type SwitchForce struct {
	Actuation *float64 `dynamodbav:"actuation,omitempty" json:"actuation,omitempty"`
	BottomOut *float64 `dynamodbav:"bottom_out,omitempty" json:"bottom_out,omitempty"`
}

// SwitchSpring is a switch's spring material, plus its length and travel
// distances in mm.
type SwitchSpring struct {
	Material    *string  `dynamodbav:"material,omitempty" json:"material,omitempty"`
	PreTravel   *float64 `dynamodbav:"pre_travel,omitempty" json:"pre_travel,omitempty"`
	TotalTravel *float64 `dynamodbav:"total_travel,omitempty" json:"total_travel,omitempty"`
	Length      *float64 `dynamodbav:"length,omitempty" json:"length,omitempty"`
}

// Switch is a mechanical keyboard switch in a user's collection, or shared
// with the caller. UserID is the DynamoDB partition key (the owner's
// IdP-issued user ID); ID is the sort key. Only Brand, Name, Type, and
// Visibility are required, per api/openapi.yaml's SwitchInput schema; every
// other field (here and in SwitchMaterial/SwitchForce/SwitchSpring/
// BulkPurchase) is a pointer so nil ("not provided") round-trips
// distinctly from an explicit zero value.
type Switch struct {
	UserID       string          `dynamodbav:"user_id" json:"-"`
	ID           string          `dynamodbav:"id" json:"id"`
	Brand        string          `dynamodbav:"brand" json:"brand"`
	Manufacturer *string         `dynamodbav:"manufacturer,omitempty" json:"manufacturer,omitempty"`
	Name         string          `dynamodbav:"name" json:"name"`
	Type         string          `dynamodbav:"type" json:"type"`
	Pins         *int            `dynamodbav:"pins,omitempty" json:"pins,omitempty"`
	FactoryLubed *bool           `dynamodbav:"factory_lubed,omitempty" json:"factory_lubed,omitempty"`
	Material     SwitchMaterial  `dynamodbav:"material" json:"material"`
	Force        SwitchForce     `dynamodbav:"force" json:"force"`
	Spring       SwitchSpring    `dynamodbav:"spring" json:"spring"`
	Purchases    BulkPurchases   `dynamodbav:"purchases,omitempty" json:"purchases,omitempty"`
	Notes        *string         `dynamodbav:"notes,omitempty" json:"notes,omitempty"`
	Visibility   Visibility      `dynamodbav:"visibility" json:"visibility"`
	ImagePath    *SwitchImageKey `dynamodbav:"image_path,omitempty" json:"-"`

	// ImageStatus is ImagePath's moderation status, empty on images stored
	// before moderation (see ImageStatus.Effective).
	// ImageRejectionReason is set only when rejected.
	ImageStatus          ImageStatus `dynamodbav:"image_status,omitempty" json:"-"`
	ImageRejectionReason *string     `dynamodbav:"image_rejection_reason,omitempty" json:"-"`
}

// SwitchRepository provides access to switches. List/Get take an explicit
// ownerID since reads can target another user's shared items;
// Create/Update/Delete read the caller from ctx (internal/ctx.UserID)
// instead, since writes are always self-scoped.
type SwitchRepository interface {
	// List returns up to limit switches owned by ownerID whose Visibility is
	// in visibilities, ordered by ID. cursor, if non-empty, resumes from a
	// previous call's returned cursor; the returned cursor is empty when
	// there are no more pages. Only the last page has fewer than limit,
	// and it can be empty.
	List(ctx context.Context, ownerID string, visibilities []Visibility, limit int, cursor string) (switches []Switch, nextCursor string, err error)

	// Get returns the switch owned by ownerID with the given id, or
	// ErrNotFound if it doesn't exist. Get doesn't take a visibility
	// argument: unlike List, it fetches by exact key regardless of
	// visibility - the caller (a handler) checks the returned item's
	// Visibility via
	// [github.com/roguekeyboards/mykeebs-api/internal/authz.CanReadVisibility].
	Get(ctx context.Context, ownerID, id string) (*Switch, error)

	// Create stores sw (UserID is set from ctx, sw.ID must already be set).
	// Returns ErrAlreadyExists on an ID collision.
	Create(ctx context.Context, sw Switch) (*Switch, error)

	// Update replaces the caller's switch (UserID is set from ctx, sw.ID
	// must already be set to the switch being updated). Returns ErrNotFound
	// if no switch with that id exists for the caller.
	Update(ctx context.Context, sw Switch) (*Switch, error)

	// Delete removes the caller's switch with the given id. Callers clean
	// up any image it had in a SwitchImageStore themselves, before calling
	// Delete. Idempotent: a nonexistent id is not an error.
	Delete(ctx context.Context, id string) error

	// DeleteAll removes every switch the caller owns. Callers clean
	// up images themselves, before calling DeleteAll. Idempotent.
	DeleteAll(ctx context.Context) error

	// SetImagePath sets the caller's switch's ImagePath, with ImageStatus
	// pending. Returns ErrNotFound if id doesn't exist.
	SetImagePath(ctx context.Context, id string, key SwitchImageKey) error

	// ClearImagePath clears the caller's switch's ImagePath (and its
	// moderation status) and returns the key that was cleared, or nil if it
	// was already unset. Idempotent: a switch with no ImagePath already set is
	// not an error. Returns ErrNotFound if id doesn't exist.
	ClearImagePath(ctx context.Context, id string) (*SwitchImageKey, error)

	// SetImageStatus sets the switch's image's moderation status to `to` and
	// its rejection reason to reason (cleared when nil), conditioned on
	// ImagePath still equalling forPath and the status still being from (a
	// missing status counts as approved). Returns ok=false (not an error) if
	// that condition fails or the item doesn't exist.
	SetImageStatus(ctx context.Context, ownerID, id string, forPath SwitchImageKey, from, to ImageStatus, reason *string) (ok bool, err error)
}

// SwitchImageKey is the object key a switch's image is stored under in a
// SwitchImageStore.
type SwitchImageKey string

// NewSwitchImageKey builds the object key for one upload of switchID's
// image. ownerID comes from ctx, not a parameter, so a caller can't build a
// key addressing anyone else's prefix. Each upload gets its own key because
// upload URLs are single-use: a replacement can't be written over the
// previous object.
func NewSwitchImageKey(ctx context.Context, switchID, uploadID string) (SwitchImageKey, error) {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return "", ErrNoUserID
	}

	return SwitchImageKey(fmt.Sprintf("switches/%s/%s/image/%s", ownerID, switchID, uploadID)), nil
}

// SwitchImageStore stores a switch's image object in a private object store,
// addressed by SwitchImageKey.
type SwitchImageStore interface {
	// SignGet returns a signed GET URL for key on the images
	// distribution, and when it stops working.
	SignGet(ctx context.Context, key SwitchImageKey) (url string, err error)

	// PresignPut returns a short-lived presigned PUT URL for key, locked to
	// contentType and size via the Content-Type and Content-Length the upload
	// must match.
	// Single-use: the upload must also send If-None-Match: *, so once an
	// object exists at key the URL can't write it again.
	PresignPut(ctx context.Context, key SwitchImageKey, contentType string, size int64) (url string, err error)

	// Delete removes the object at key. Idempotent: a nonexistent key is
	// not an error, matching S3's own DeleteObject semantics.
	Delete(ctx context.Context, key SwitchImageKey) error
}
