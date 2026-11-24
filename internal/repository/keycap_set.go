package repository

import (
	"context"
	"fmt"
	"math"

	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
)

// KeycapKitPurchase tracks a kit's own purchase lifecycle, independent of
// the parent set's (a set can be assembled from kits bought separately).
type KeycapKitPurchase struct {
	Vendor       *string  `dynamodbav:"vendor,omitempty" json:"vendor,omitempty"`
	Price        *float64 `dynamodbav:"price,omitempty" json:"price,omitempty"`
	OrderDate    *string  `dynamodbav:"order_date,omitempty" json:"order_date,omitempty"`
	DeliveryDate *string  `dynamodbav:"delivery_date,omitempty" json:"delivery_date,omitempty"`
	OrderStatus  *string  `dynamodbav:"order_status,omitempty" json:"order_status,omitempty"`
}

// KeycapKit is one purchase within a KeycapSet (e.g. "Base", "Extension").
// KitID is server-generated and unique within its parent set, not globally.
// Kept as a field too (not just the Kits map's key), so a single KeycapKit
// value is still self-describing. Seq orders a set's kits, the same way
// KeyboardImageEntry.Seq orders images; kits added before it existed have
// 0 and sort by KitID among themselves.
type KeycapKit struct {
	KitID     string             `dynamodbav:"kit_id" json:"kit_id"`
	Name      string             `dynamodbav:"name" json:"name"`
	ImagePath *KeycapKitImageKey `dynamodbav:"image_path,omitempty" json:"image_path,omitempty"`
	Purchase  KeycapKitPurchase  `dynamodbav:"purchase" json:"purchase"`
	Seq       int                `dynamodbav:"seq" json:"-"`

	// ImageStatus is ImagePath's moderation status, empty on images stored
	// before moderation (see ImageStatus.Effective).
	// ImageRejectionReason is set only when rejected.
	ImageStatus          ImageStatus `dynamodbav:"image_status,omitempty" json:"-"`
	ImageRejectionReason *string     `dynamodbav:"image_rejection_reason,omitempty" json:"-"`
}

// KeycapSet is a keycap set in a user's collection, or shared with the
// caller. UserID is the DynamoDB partition key (the owner's IdP-issued
// user ID); ID is the sort key. Only Brand, Name, and Visibility are
// required, per api/openapi.yaml's KeycapSetInput schema.
type KeycapSet struct {
	UserID     string     `dynamodbav:"user_id" json:"-"`
	ID         string     `dynamodbav:"id" json:"id"`
	Brand      string     `dynamodbav:"brand" json:"brand"`
	Name       string     `dynamodbav:"name" json:"name"`
	Profile    *string    `dynamodbav:"profile,omitempty" json:"profile,omitempty"`
	Material   *string    `dynamodbav:"material,omitempty" json:"material,omitempty"`
	Notes      *string    `dynamodbav:"notes,omitempty" json:"notes,omitempty"`
	Visibility Visibility `dynamodbav:"visibility" json:"visibility"`
	// Keyed by KitID, so a single kit is addressable via UpdateItem
	// (kits.<kit_id>) without a whole-item read-modify-write.
	Kits map[string]KeycapKit `dynamodbav:"kits,omitempty" json:"kits,omitempty"`
	// PrimaryKitID names the kit whose image represents this set in a
	// list/grid view. Reads must treat a dangling reference as absent.
	PrimaryKitID *string `dynamodbav:"primary_kit_id,omitempty" json:"primary_kit_id,omitempty"`
}

// SortedKits flattens a Kits map into a slice ordered by Seq, then KitID.
func SortedKits(kits map[string]KeycapKit) []KeycapKit {
	return sortedBySeq(kits, func(k KeycapKit) (int, string) { return k.Seq, k.KitID })
}

// orderStatusProgression ranks every non-Cancelled order_status lookup
// value by how far along the purchase is - lower is less complete. Fixed
// by app semantics, not by the "order_status" lookup table's own row
// order, which isn't guaranteed to encode this meaning.
var orderStatusProgression = map[string]int{
	"Planned":   0,
	"Ordered":   1,
	"Shipped":   2,
	"Delivered": 3,
}

// AggregateOrderStatus derives a KeycapSet's overall order status from its
// kits' individual purchase.order_status values: the least-progressed
// status wins, so a set isn't shown as Delivered while a kit is still
// Ordered. "Cancelled" kits are excluded from that comparison - a
// cancelled kit doesn't drag the set backward - unless every kit is
// Cancelled, in which case the set is Cancelled too. Returns nil if kits
// is empty, or if every kit has no order_status set, or if a kit's
// order_status isn't a recognized value (unvalidated/legacy data - callers
// should not guess).
func AggregateOrderStatus(kits map[string]KeycapKit) *string {
	statuses := make([]*string, 0, len(kits))
	for _, k := range kits {
		statuses = append(statuses, k.Purchase.OrderStatus)
	}
	return leastProgressedOrderStatus(statuses)
}

func leastProgressedOrderStatus(statuses []*string) *string {
	var least string
	haveLeast := false
	sawCancelled := false

	for _, s := range statuses {
		if s == nil {
			continue
		}
		status := *s

		if status == "Cancelled" {
			sawCancelled = true
			continue
		}

		rank, ok := orderStatusProgression[status]
		if !ok {
			return nil
		}

		if !haveLeast || rank < orderStatusProgression[least] {
			least = status
			haveLeast = true
		}
	}

	if haveLeast {
		return &least
	}

	if sawCancelled {
		cancelled := "Cancelled"
		return &cancelled
	}

	return nil
}

// TotalCost sums the kits with a known price, or returns nil if none has
// one. It's rounded to cents so map iteration order can't change the
// float result.
func (s KeycapSet) TotalCost() *float64 {
	var total float64
	priced := false
	for _, k := range s.Kits {
		if k.Purchase.Price == nil {
			continue
		}
		total += *k.Purchase.Price
		priced = true
	}
	if !priced {
		return nil
	}

	total = math.Round(total*100) / 100
	return &total
}

// KeycapSetRepository provides access to keycap sets.
type KeycapSetRepository interface {
	// List returns up to limit keycap sets owned by ownerID whose
	// Visibility is in visibilities, ordered by ID. cursor, if non-empty,
	// resumes from a previous call's returned cursor; the returned cursor
	// is empty when there are no more pages. Only the last page has fewer
	// than limit, and it can be empty.
	List(ctx context.Context, ownerID string, visibilities []Visibility, limit int, cursor string) (sets []KeycapSet, nextCursor string, err error)

	// Get returns the keycap set owned by ownerID with the given id, or
	// ErrNotFound if it doesn't exist. Get doesn't take a visibility
	// argument: unlike List, it fetches by exact key regardless of
	// visibility - the caller (a handler) checks the returned item's
	// Visibility via
	// [github.com/roguekeyboards/mykeebs-api/internal/authz.CanReadVisibility].
	Get(ctx context.Context, ownerID, id string) (*KeycapSet, error)

	// Create stores ks (UserID is set from ctx, ks.ID must already be set).
	// Returns ErrAlreadyExists on an ID collision.
	Create(ctx context.Context, ks KeycapSet) (*KeycapSet, error)

	// Update replaces the keycap set's own fields (Brand, Name, Profile,
	// Material, Notes, Visibility) with ks's (UserID is set from ctx, ks.ID
	// must already be set to the keycap set being updated); Kits and
	// PrimaryKitID are left untouched. Returns ErrNotFound if no keycap set
	// with that ID exists for the owner.
	Update(ctx context.Context, ks KeycapSet) (*KeycapSet, error)

	// Delete removes the caller's keycap set with the given id. Callers
	// clean up any kit images it had in a KeycapKitImageStore themselves,
	// before calling Delete. Idempotent: a nonexistent id is not an error.
	Delete(ctx context.Context, id string) error

	// DeleteAll removes every keycap set the caller owns. Callers clean
	// up images themselves, before calling DeleteAll. Idempotent.
	DeleteAll(ctx context.Context) error

	// AddKit adds kit to the set's Kits, keyed by kit.KitID (which must
	// already be set), with a server-assigned Seq that sorts it after every
	// existing kit (kit.Seq is ignored), and returns the stored kit, matching Create's shape
	// for every other entity. primary controls the set's PrimaryKitID,
	// applied atomically with the add: nil leaves it untouched, true makes
	// the new kit primary. Returns ErrNotFound if the parent set doesn't
	// exist, or an error wrapping a kit-id-collision sentinel if kit.KitID
	// is already in use (practically unreachable given a fresh UUID).
	AddKit(ctx context.Context, setID string, kit KeycapKit, primary *bool) (*KeycapKit, error)

	// UpdateKit returns ErrNotFound if setID or the kit doesn't exist.
	// primary controls the set's PrimaryKitID: nil leaves it untouched,
	// true makes this kit primary, false clears it but only if this kit is
	// the current primary - not atomic with the rest of the update, and
	// never fails the call on its own (PrimaryKitID's read path tolerates
	// a dangling reference as absent).
	UpdateKit(ctx context.Context, setID string, kit KeycapKit, primary *bool) (*KeycapKit, error)

	// DeleteKit removes the kit matching kitID from setID's Kits. Callers
	// clean up any image it had in a KeycapKitImageStore themselves, before
	// calling DeleteKit. If kitID was the set's PrimaryKitID, that's
	// cleared too, same caveats as UpdateKit's primary=false. Idempotent: a
	// kitID not present in the set is not an error. Returns ErrNotFound if
	// setID doesn't exist for the owner.
	DeleteKit(ctx context.Context, setID, kitID string) error

	// SetKitImagePath sets the kit matching kitID's ImagePath, with
	// ImageStatus pending. Returns ErrNotFound if setID or the kit doesn't
	// exist.
	SetKitImagePath(ctx context.Context, setID, kitID string, key KeycapKitImageKey) error

	// ClearKitImagePath clears the kit matching kitID's ImagePath (and its
	// moderation status) and returns the key that was cleared, or nil if it
	// was already unset. Idempotent: a kit with no ImagePath already set is
	// not an error. Returns ErrNotFound if setID or the kit doesn't exist.
	ClearKitImagePath(ctx context.Context, setID, kitID string) (*KeycapKitImageKey, error)

	// SetKitImageStatus sets the matching kit's image's moderation status to
	// `to` and its rejection reason to reason (cleared when nil), conditioned
	// on ImagePath still equalling forPath and the status still being from (a
	// missing status counts as approved). Returns ok=false (not an error) if
	// that condition fails or the item doesn't exist.
	SetKitImageStatus(ctx context.Context, ownerID, setID, kitID string, forPath KeycapKitImageKey, from, to ImageStatus, reason *string) (ok bool, err error)
}

// KeycapKitImageKey is the object key a kit's image is stored under in a
// KeycapKitImageStore.
type KeycapKitImageKey string

// NewKeycapKitImageKey builds the object key for one upload of kitID's
// image within setID. ownerID comes from ctx, not a parameter, so a caller
// can't build a key addressing anyone else's prefix. Each upload gets its
// own key because upload URLs are single-use: a replacement can't be
// written over the previous object.
func NewKeycapKitImageKey(ctx context.Context, setID, kitID, uploadID string) (KeycapKitImageKey, error) {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return "", ErrNoUserID
	}

	return KeycapKitImageKey(fmt.Sprintf("keycap-sets/%s/%s/kits/%s/image/%s", ownerID, setID, kitID, uploadID)), nil
}

// KeycapKitImageStore stores a kit's image object in a private object store,
// addressed by KeycapKitImageKey.
type KeycapKitImageStore interface {
	// SignGet returns a signed GET URL for key on the images
	// distribution, and when it stops working.
	SignGet(ctx context.Context, key KeycapKitImageKey) (url string, err error)

	// PresignPut returns a short-lived presigned PUT URL for key, locked
	// to contentType and size via the Content-Type and Content-Length the
	// upload must match.
	// Single-use: the upload must also send If-None-Match: *, so once an
	// object exists at key the URL can't write it again.
	PresignPut(ctx context.Context, key KeycapKitImageKey, contentType string, size int64) (url string, err error)

	// Delete removes the object at key. Idempotent: a nonexistent key is
	// not an error, matching S3's own DeleteObject semantics.
	Delete(ctx context.Context, key KeycapKitImageKey) error
}
