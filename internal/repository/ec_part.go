package repository

import (
	"context"
	"fmt"
	"slices"

	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
)

// ECPartType is which electro-capacitive building block an ECPart is. It
// decides which of ECPart's per-type sub-objects may be set.
type ECPartType string

// The EC part types.
const (
	ECPartTypeDomes    ECPartType = "domes"
	ECPartTypeSliders  ECPartType = "sliders"
	ECPartTypeHousings ECPartType = "housings"
	ECPartTypeSprings  ECPartType = "springs"
	ECPartTypeRings    ECPartType = "rings"
)

// ECPartTypes lists every ECPartType, in display order.
var ECPartTypes = []ECPartType{ECPartTypeDomes, ECPartTypeSliders, ECPartTypeHousings, ECPartTypeSprings, ECPartTypeRings}

// Valid reports whether t is one of ECPartTypes.
func (t ECPartType) Valid() bool {
	return slices.Contains(ECPartTypes, t)
}

// ECPartSize is the key size a slider or housing fits.
type ECPartSize string

// The EC part sizes.
const (
	ECPartSize1U ECPartSize = "1U"
	ECPartSize2U ECPartSize = "2U"
)

// ECPartSizes lists every ECPartSize.
var ECPartSizes = []ECPartSize{ECPartSize1U, ECPartSize2U}

// Valid reports whether s is one of ECPartSizes.
func (s ECPartSize) Valid() bool {
	return slices.Contains(ECPartSizes, s)
}

// ECSliderStem is the keycap mount a slider takes: MX's cross stem, or the
// round stem OEM (Topre-style) keycaps fit.
type ECSliderStem string

// The slider stems.
const (
	ECSliderStemMX  ECSliderStem = "MX"
	ECSliderStemOEM ECSliderStem = "OEM"
)

// ECSliderStems lists every ECSliderStem.
var ECSliderStems = []ECSliderStem{ECSliderStemMX, ECSliderStemOEM}

// Valid reports whether s is one of ECSliderStems.
func (s ECSliderStem) Valid() bool {
	return slices.Contains(ECSliderStems, s)
}

// ECDomes is the domes-specific detail of an ECPart. Weight is in grams.
type ECDomes struct {
	Weight   *float64 `dynamodbav:"weight,omitempty" json:"weight,omitempty"`
	Material *string  `dynamodbav:"material,omitempty" json:"material,omitempty"`
	Color    *string  `dynamodbav:"color,omitempty" json:"color,omitempty"`
}

// ECSliders is the sliders-specific detail of an ECPart.
type ECSliders struct {
	Size     *ECPartSize   `dynamodbav:"size,omitempty" json:"size,omitempty"`
	Stem     *ECSliderStem `dynamodbav:"stem,omitempty" json:"stem,omitempty"`
	Material *string       `dynamodbav:"material,omitempty" json:"material,omitempty"`
}

// ECHousings is the housings-specific detail of an ECPart.
type ECHousings struct {
	Size     *ECPartSize `dynamodbav:"size,omitempty" json:"size,omitempty"`
	Material *string     `dynamodbav:"material,omitempty" json:"material,omitempty"`
}

// ECSprings is the springs-specific detail of an ECPart.
type ECSprings struct {
	Material *string `dynamodbav:"material,omitempty" json:"material,omitempty"`
}

// ECRings is the silencing-rings-specific detail of an ECPart. Thickness is
// in millimetres.
type ECRings struct {
	Material  *string  `dynamodbav:"material,omitempty" json:"material,omitempty"`
	Thickness *float64 `dynamodbav:"thickness,omitempty" json:"thickness,omitempty"`
}

// ECPartPurchase is where/how much/how many of a part were bought, plus its
// order lifecycle. Price is the total for Quantity units, as with
// SwitchPurchase.
type ECPartPurchase struct {
	Vendor       *string  `dynamodbav:"vendor,omitempty" json:"vendor,omitempty"`
	Price        *float64 `dynamodbav:"price,omitempty" json:"price,omitempty"`
	OrderDate    *string  `dynamodbav:"order_date,omitempty" json:"order_date,omitempty"`
	DeliveryDate *string  `dynamodbav:"delivery_date,omitempty" json:"delivery_date,omitempty"`
	OrderStatus  *string  `dynamodbav:"order_status,omitempty" json:"order_status,omitempty"`
	Quantity     *int     `dynamodbav:"quantity,omitempty" json:"quantity,omitempty"`
}

// ECPart is one electro-capacitive building block in a user's collection -
// domes, sliders, housings, springs or silencing rings - bought on its own
// so a build can mix parts from different purchases. UserID is the
// DynamoDB partition key; ID is the sort key. At most the sub-object
// matching PartType is set, as checked by
// [github.com/roguekeyboards/mykeebs-api/internal/lookup.ValidateECPart].
type ECPart struct {
	UserID     string          `dynamodbav:"user_id" json:"-"`
	ID         string          `dynamodbav:"id" json:"id"`
	Brand      string          `dynamodbav:"brand" json:"brand"`
	Name       string          `dynamodbav:"name" json:"name"`
	PartType   ECPartType      `dynamodbav:"part_type" json:"part_type"`
	Domes      *ECDomes        `dynamodbav:"domes,omitempty" json:"domes,omitempty"`
	Sliders    *ECSliders      `dynamodbav:"sliders,omitempty" json:"sliders,omitempty"`
	Housings   *ECHousings     `dynamodbav:"housings,omitempty" json:"housings,omitempty"`
	Springs    *ECSprings      `dynamodbav:"springs,omitempty" json:"springs,omitempty"`
	Rings      *ECRings        `dynamodbav:"rings,omitempty" json:"rings,omitempty"`
	Purchase   ECPartPurchase  `dynamodbav:"purchase" json:"purchase"`
	Notes      *string         `dynamodbav:"notes,omitempty" json:"notes,omitempty"`
	Visibility Visibility      `dynamodbav:"visibility" json:"visibility"`
	ImagePath  *ECPartImageKey `dynamodbav:"image_path,omitempty" json:"-"`
	// ImageStatus is ImagePath's moderation status.
	// ImageRejectionReason is set only when rejected.
	ImageStatus          ImageStatus `dynamodbav:"image_status,omitempty" json:"-"`
	ImageRejectionReason *string     `dynamodbav:"image_rejection_reason,omitempty" json:"-"`
}

// ECPartRepository provides access to EC parts. List/Get take an explicit
// ownerID since reads can target another user's shared items;
// Create/Update/Delete read the caller from ctx (internal/ctx.UserID),
// since writes are always self-scoped.
type ECPartRepository interface {
	// List returns up to limit EC parts owned by ownerID whose Visibility
	// is in visibilities, ordered by ID. A non-empty partType returns only
	// parts of that type; a cursor issued under a different partType is
	// ErrInvalidCursor. The returned cursor is empty when there are no
	// more pages. Only the last page has fewer than limit, and it can be
	// empty.
	List(ctx context.Context, ownerID string, visibilities []Visibility, partType ECPartType, limit int, cursor string) (parts []ECPart, nextCursor string, err error)
	// Get returns the EC part owned by ownerID with the given id, or
	// ErrNotFound, regardless of visibility; the caller checks it via
	// [github.com/roguekeyboards/mykeebs-api/internal/authz.CanReadVisibility].
	Get(ctx context.Context, ownerID, id string) (*ECPart, error)
	// Create stores p (UserID is set from ctx, p.ID must already be set).
	// Returns ErrAlreadyExists on an ID collision.
	Create(ctx context.Context, p ECPart) (*ECPart, error)
	// Update replaces the caller's EC part's body fields, keeping its
	// image. Returns ErrNotFound if no part with p.ID exists for the
	// caller.
	Update(ctx context.Context, p ECPart) (*ECPart, error)
	// Delete removes the caller's EC part with the given id. Callers clean
	// up its image first. Idempotent.
	Delete(ctx context.Context, id string) error
	// DeleteAll removes every EC part the caller owns. Callers clean up
	// images themselves. Idempotent.
	DeleteAll(ctx context.Context) error
	// SetImagePath sets the caller's EC part's ImagePath, with ImageStatus
	// pending. Returns ErrNotFound if id doesn't exist.
	SetImagePath(ctx context.Context, id string, key ECPartImageKey) error
	// ClearImagePath clears the caller's EC part's ImagePath and its
	// moderation status, returning the cleared key, or nil if none was
	// set. Returns ErrNotFound if id doesn't exist.
	ClearImagePath(ctx context.Context, id string) (*ECPartImageKey, error)
	// SetImageStatus sets the part's image's moderation status to `to` and
	// its rejection reason to reason (cleared when nil), conditioned on
	// ImagePath still equalling forPath and the status still being from.
	// Returns ok=false (not an error) if that condition fails or the item
	// doesn't exist.
	SetImageStatus(ctx context.Context, ownerID, id string, forPath ECPartImageKey, from, to ImageStatus, reason *string) (ok bool, err error)
}

// ECPartImageKey is the object key an EC part's image is stored under in an
// ECPartImageStore.
type ECPartImageKey string

// NewECPartImageKey builds the object key for one upload of partID's image,
// with ownerID from ctx so a caller can't address anyone else's prefix.
// Each upload gets its own key because upload URLs are single-use.
func NewECPartImageKey(ctx context.Context, partID, uploadID string) (ECPartImageKey, error) {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return "", ErrNoUserID
	}
	return ECPartImageKey(fmt.Sprintf("ec-parts/%s/%s/image/%s", ownerID, partID, uploadID)), nil
}

// ECPartImageStore stores an EC part's image object in a private object
// store, addressed by ECPartImageKey.
type ECPartImageStore interface {
	// SignGet returns a signed GET URL for key on the images distribution.
	SignGet(ctx context.Context, key ECPartImageKey) (url string, err error)
	// PresignPut returns a short-lived, single-use presigned PUT URL for
	// key, locked to contentType and size.
	PresignPut(ctx context.Context, key ECPartImageKey, contentType string, size int64) (url string, err error)
	// Delete removes the object at key. Idempotent.
	Delete(ctx context.Context, key ECPartImageKey) error
}
