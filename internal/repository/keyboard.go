package repository

import (
	"context"
	"fmt"
	"math"
	"sort"

	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
)

// KeyboardMaterialColor is a physical part described by material and color
// (top case, bottom case, or weight).
type KeyboardMaterialColor struct {
	Material *string `dynamodbav:"material,omitempty" json:"material,omitempty"`
	Color    *string `dynamodbav:"color,omitempty" json:"color,omitempty"`
}

// KeyboardDesign is a keyboard's physical case makeup.
type KeyboardDesign struct {
	TopCase    KeyboardMaterialColor `dynamodbav:"top_case" json:"top_case"`
	BottomCase KeyboardMaterialColor `dynamodbav:"bottom_case" json:"bottom_case"`
	Weight     KeyboardMaterialColor `dynamodbav:"weight" json:"weight"`
}

// KeyboardPlate is one plate a keyboard has. ID is server-generated and
// unique within the keyboard; builds reference a plate by it. A plate
// with no purchase came with the keyboard at no extra cost. Seq orders a
// keyboard's plates, the same way KeyboardImageEntry.Seq orders images.
type KeyboardPlate struct {
	ID        string           `dynamodbav:"id" json:"id"`
	Material  string           `dynamodbav:"material" json:"material"`
	Color     *string          `dynamodbav:"color,omitempty" json:"color,omitempty"`
	Thickness *float64         `dynamodbav:"thickness,omitempty" json:"thickness,omitempty"`
	Purchase  KeyboardPurchase `dynamodbav:"purchase" json:"purchase"`
	Seq       int              `dynamodbav:"seq" json:"-"`
}

// KeyboardPCB is one PCB a keyboard has. ID is server-generated and unique
// within the keyboard; builds reference a PCB by it. A PCB with no
// purchase came with the keyboard at no extra cost. Seq orders a
// keyboard's PCBs, the same way KeyboardImageEntry.Seq orders images.
type KeyboardPCB struct {
	ID           string           `dynamodbav:"id" json:"id"`
	Thickness    *float64         `dynamodbav:"thickness,omitempty" json:"thickness,omitempty"`
	Firmware     *string          `dynamodbav:"firmware,omitempty" json:"firmware,omitempty"`
	Assembly     *string          `dynamodbav:"assembly,omitempty" json:"assembly,omitempty"`
	Connectivity *string          `dynamodbav:"connectivity,omitempty" json:"connectivity,omitempty"`
	Purchase     KeyboardPurchase `dynamodbav:"purchase" json:"purchase"`
	Seq          int              `dynamodbav:"seq" json:"-"`
}

// SortedPlates flattens a Plates map into a slice ordered by Seq, then ID.
func SortedPlates(plates map[string]KeyboardPlate) []KeyboardPlate {
	return sortedBySeq(plates, func(p KeyboardPlate) (int, string) { return p.Seq, p.ID })
}

// SortedPCBs flattens a PCBs map into a slice ordered by Seq, then ID.
func SortedPCBs(pcbs map[string]KeyboardPCB) []KeyboardPCB {
	return sortedBySeq(pcbs, func(p KeyboardPCB) (int, string) { return p.Seq, p.ID })
}

// KeyboardPlatesMap builds a Plates map from an ordered slice, assigning
// Seq by position. The inverse of SortedPlates, for callers holding plates
// as a list (e.g. tests).
func KeyboardPlatesMap(plates []KeyboardPlate) map[string]KeyboardPlate {
	if len(plates) == 0 {
		return nil
	}
	out := make(map[string]KeyboardPlate, len(plates))
	for i, p := range plates {
		p.Seq = i
		out[p.ID] = p
	}
	return out
}

// KeyboardPCBsMap is KeyboardPlatesMap for PCBs.
func KeyboardPCBsMap(pcbs []KeyboardPCB) map[string]KeyboardPCB {
	if len(pcbs) == 0 {
		return nil
	}
	out := make(map[string]KeyboardPCB, len(pcbs))
	for i, p := range pcbs {
		p.Seq = i
		out[p.ID] = p
	}
	return out
}

// sortedBySeq orders by seq, then id, so entries stored before they had a
// seq (all 0) keep a stable order.
func sortedBySeq[T any](m map[string]T, key func(T) (int, string)) []T {
	out := make([]T, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		si, idi := key(out[i])
		sj, idj := key(out[j])
		if si != sj {
			return si < sj
		}
		return idi < idj
	})
	return out
}

// KeyboardPurchase is where/how much a keyboard was bought, plus its
// order lifecycle. Fuller shape than SwitchPurchase - keyboards are
// tracked with order/delivery dates and status.
type KeyboardPurchase struct {
	Vendor                *string  `dynamodbav:"vendor,omitempty" json:"vendor,omitempty"`
	Price                 *float64 `dynamodbav:"price,omitempty" json:"price,omitempty"`
	OrderDate             *string  `dynamodbav:"order_date,omitempty" json:"order_date,omitempty"`
	DeliveryDate          *string  `dynamodbav:"delivery_date,omitempty" json:"delivery_date,omitempty"`
	EstimatedDeliveryDate *string  `dynamodbav:"estimated_delivery_date,omitempty" json:"estimated_delivery_date,omitempty"`
	OrderStatus           *string  `dynamodbav:"order_status,omitempty" json:"order_status,omitempty"`
}

// KeyboardImageEntry is one value in the Keyboard.Images map (keyed by
// image id). Seq is a repository-internal ordering key: on add it's set to
// time.Now().UnixNano(), so a new image sorts after existing ones without
// reading the current max. Wall-clock, not a monotonic source - a backward
// clock step between two adds could misorder one image (cosmetic on a
// single-user list, and self-heals if the images are ever reordered). The
// API/MCP layers sort on Seq to present images in add order; it's not in
// JSON. A future reorder endpoint would renumber entries 0..n - a later
// add's nanosecond stamp still sorts last.
type KeyboardImageEntry struct {
	Path KeyboardImageKey `dynamodbav:"path" json:"-"`
	Seq  int              `dynamodbav:"seq" json:"-"`

	// Status is empty on images stored before moderation; read it through
	// ImageStatus.Effective. RejectionReason is set only when rejected.
	Status          ImageStatus `dynamodbav:"status,omitempty" json:"-"`
	RejectionReason *string     `dynamodbav:"rejection_reason,omitempty" json:"-"`
}

// KeyboardImage is an image id paired with its stored entry, the ordered
// element type the API/MCP layers work with. SortedKeyboardImages builds a
// []KeyboardImage from a Keyboard.Images map.
type KeyboardImage struct {
	ImageID string
	Path    KeyboardImageKey
	Seq     int

	// Status is the entry's effective status, never empty.
	Status          ImageStatus
	RejectionReason *string
}

// SortedKeyboardImages flattens an Images map into a slice ordered by Seq
// (ascending), the order images were added in.
func SortedKeyboardImages(images map[string]KeyboardImageEntry) []KeyboardImage {
	out := make([]KeyboardImage, 0, len(images))
	for id, entry := range images {
		out = append(out, KeyboardImage{
			ImageID:         id,
			Path:            entry.Path,
			Seq:             entry.Seq,
			Status:          entry.Status.Effective(),
			RejectionReason: entry.RejectionReason,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out
}

// KeyboardImagesMap builds an Images map from an ordered slice, assigning
// Seq by position. The inverse of SortedKeyboardImages, for callers holding
// images as a list (e.g. reload tooling, tests).
func KeyboardImagesMap(images []KeyboardImage) map[string]KeyboardImageEntry {
	if len(images) == 0 {
		return nil
	}
	out := make(map[string]KeyboardImageEntry, len(images))
	for i, img := range images {
		out[img.ImageID] = KeyboardImageEntry{Path: img.Path, Seq: i}
	}
	return out
}

// Keyboard is a mechanical keyboard in a user's collection, or shared with
// the caller. UserID is the DynamoDB partition key (the owner's IdP-issued
// user ID); ID is the sort key. Only Brand, Name, and Visibility are
// required, per api/openapi.yaml's KeyboardInput schema; every other
// optional field (here and in KeyboardMaterialColor/KeyboardDesign/
// KeyboardPlate/KeyboardPCB/KeyboardPurchase) is a pointer so nil ("not
// provided") round-trips distinctly from an explicit zero value.
// Purchase.Price is the base price, excluding plates and PCBs that carry
// their own purchase.
type Keyboard struct {
	UserID string         `dynamodbav:"user_id" json:"-"`
	ID     string         `dynamodbav:"id" json:"id"`
	Brand  string         `dynamodbav:"brand" json:"brand"`
	Name   string         `dynamodbav:"name" json:"name"`
	Size   *string        `dynamodbav:"size,omitempty" json:"size,omitempty"`
	Layout *string        `dynamodbav:"layout,omitempty" json:"layout,omitempty"`
	Design KeyboardDesign `dynamodbav:"design" json:"design"`
	// Plates and PCBs are keyed by part id, so a single part is addressable
	// via UpdateItem (plates.<id>) without a whole-item read-modify-write.
	// The API/MCP layers project them to ordered lists via SortedPlates and
	// SortedPCBs.
	Plates     map[string]KeyboardPlate `dynamodbav:"plates,omitempty" json:"-"`
	PCBs       map[string]KeyboardPCB   `dynamodbav:"pcbs,omitempty" json:"-"`
	Purchase   KeyboardPurchase         `dynamodbav:"purchase" json:"purchase"`
	Notes      *string                  `dynamodbav:"notes,omitempty" json:"notes,omitempty"`
	Visibility Visibility               `dynamodbav:"visibility" json:"visibility"`
	// Images is keyed by image id. AddImage/DeleteImage address a single
	// entry in place (images.<id>); the API/MCP layers project it to an
	// ordered list via SortedKeyboardImages, sorting on each entry's Seq.
	Images map[string]KeyboardImageEntry `dynamodbav:"images,omitempty" json:"-"`
}

// Plate returns the keyboard's plate with the given id, or nil if id is nil
// or the keyboard has no such plate.
func (kb Keyboard) Plate(id *string) *KeyboardPlate {
	if id == nil {
		return nil
	}
	p, ok := kb.Plates[*id]
	if !ok {
		return nil
	}
	return &p
}

// PCB returns the keyboard's PCB with the given id, or nil if id is nil or
// the keyboard has no such PCB.
func (kb Keyboard) PCB(id *string) *KeyboardPCB {
	if id == nil {
		return nil
	}
	p, ok := kb.PCBs[*id]
	if !ok {
		return nil
	}
	return &p
}

// Price is p's purchase price, or nil if p is nil.
func (p *KeyboardPlate) Price() *float64 {
	if p == nil {
		return nil
	}
	return p.Purchase.Price
}

// Price is p's purchase price, or nil if p is nil.
func (p *KeyboardPCB) Price() *float64 {
	if p == nil {
		return nil
	}
	return p.Purchase.Price
}

// TotalCost sums the base price and every plate's and PCB's price, skipping
// unknown ones, or returns nil if none is known. Rounded to cents, like
// [KeycapSet.TotalCost].
func (kb Keyboard) TotalCost() *float64 {
	prices := []*float64{kb.Purchase.Price}
	for _, p := range kb.Plates {
		prices = append(prices, p.Purchase.Price)
	}
	for _, p := range kb.PCBs {
		prices = append(prices, p.Purchase.Price)
	}

	var total float64
	priced := false
	for _, p := range prices {
		if p == nil {
			continue
		}
		total += *p
		priced = true
	}
	if !priced {
		return nil
	}

	total = math.Round(total*100) / 100
	return &total
}

// KeyboardRepository provides access to keyboards.
type KeyboardRepository interface {
	// List returns up to limit keyboards owned by ownerID whose Visibility
	// is in visibilities, ordered by ID. cursor, if non-empty, resumes from
	// a previous call's returned cursor; the returned cursor is empty when
	// there are no more pages. Only the last page has fewer than limit,
	// and it can be empty.
	List(ctx context.Context, ownerID string, visibilities []Visibility, limit int, cursor string) (keyboards []Keyboard, nextCursor string, err error)

	// Get returns the keyboard owned by ownerID with the given id, or
	// ErrNotFound if it doesn't exist. Get doesn't take a visibility
	// argument: unlike List, it fetches by exact key regardless of
	// visibility - the caller (a handler) checks the returned item's
	// Visibility via
	// [github.com/roguekeyboards/mykeebs-api/internal/authz.CanReadVisibility].
	Get(ctx context.Context, ownerID, id string) (*Keyboard, error)

	// Create stores kb (UserID is set from ctx, kb.ID must already be set).
	// Returns ErrAlreadyExists on an ID collision.
	Create(ctx context.Context, kb Keyboard) (*Keyboard, error)

	// Update replaces the caller's keyboard (UserID is set from ctx, kb.ID
	// must already be set to the keyboard being updated). Plates, PCBs and
	// Images are left untouched. Returns ErrNotFound if no keyboard with
	// that id exists for the caller.
	Update(ctx context.Context, kb Keyboard) (*Keyboard, error)

	// AddPlate adds plate (plate.ID must already be set) to keyboardID's
	// Plates with a server-assigned Seq that sorts it after every existing
	// plate; plate.Seq is ignored. Returns the stored plate, ErrNotFound if
	// the keyboard doesn't exist, or a wrapped duplicate-id error if
	// plate.ID is already in use (practically unreachable given a fresh
	// UUID).
	AddPlate(ctx context.Context, keyboardID string, plate KeyboardPlate) (*KeyboardPlate, error)

	// UpdatePlate replaces the plate matching plate.ID, keeping its Seq.
	// Returns ErrNotFound if keyboardID or the plate doesn't exist.
	UpdatePlate(ctx context.Context, keyboardID string, plate KeyboardPlate) (*KeyboardPlate, error)

	// DeletePlate removes plateID from keyboardID's Plates. Idempotent: a
	// plateID not present is not an error. Returns ErrNotFound if
	// keyboardID doesn't exist for the owner.
	DeletePlate(ctx context.Context, keyboardID, plateID string) error

	// AddPCB is AddPlate for PCBs.
	AddPCB(ctx context.Context, keyboardID string, pcb KeyboardPCB) (*KeyboardPCB, error)

	// UpdatePCB is UpdatePlate for PCBs.
	UpdatePCB(ctx context.Context, keyboardID string, pcb KeyboardPCB) (*KeyboardPCB, error)

	// DeletePCB is DeletePlate for PCBs.
	DeletePCB(ctx context.Context, keyboardID, pcbID string) error

	// Delete removes the caller's keyboard with the given id. Callers clean
	// up any images it had in a KeyboardImageStore themselves, before
	// calling Delete - see
	// [github.com/roguekeyboards/mykeebs-api/internal/cascadedelete.DeleteKeyboard].
	// Idempotent: a nonexistent id is not an error.
	Delete(ctx context.Context, id string) error

	// DeleteAll removes every keyboard the caller owns. Callers clean
	// up images themselves, before calling DeleteAll. Idempotent.
	DeleteAll(ctx context.Context) error

	// AddImage adds image (image.ImageID must be set) to the keyboard's Images
	// map with a server-assigned Seq that sorts it after every existing image
	// (see KeyboardImageEntry.Seq), with Status pending; image.Seq and
	// image.Status are ignored. Returns ErrNotFound if the keyboard doesn't
	// exist, ErrImageLimitReached if it already has MaxImagesPerItem images,
	// or a wrapped duplicate-id error if image.ImageID is already present.
	AddImage(ctx context.Context, keyboardID string, image KeyboardImage) error

	// DeleteImage removes imageID from keyboardID's Images map and returns
	// the key that was cleared, or nil if it wasn't there. Idempotent: an
	// imageID not present is not an error. Returns ErrNotFound if keyboardID
	// doesn't exist for the owner.
	DeleteImage(ctx context.Context, keyboardID, imageID string) (*KeyboardImageKey, error)

	// SetImageStatus sets the matching image's moderation status to `to` and
	// its rejection reason to reason (cleared when nil), conditioned on Path
	// still equalling forPath and the status still being from (a missing
	// status counts as approved).
	// Returns ok=false (not an error) if that condition fails or the item
	// doesn't exist.
	SetImageStatus(ctx context.Context, ownerID, keyboardID, imageID string, forPath KeyboardImageKey, from, to ImageStatus, reason *string) (ok bool, err error)
}

// KeyboardImageKey is the object key an image is stored under in a
// KeyboardImageStore.
type KeyboardImageKey string

// NewKeyboardImageKey builds the deterministic object key for imageID's
// image within keyboardID. ownerID comes from ctx, not a parameter, so a
// caller can't build a key addressing anyone else's prefix.
func NewKeyboardImageKey(ctx context.Context, keyboardID, imageID string) (KeyboardImageKey, error) {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return "", ErrNoUserID
	}

	return KeyboardImageKey(fmt.Sprintf("keyboards/%s/%s/images/%s", ownerID, keyboardID, imageID)), nil
}

// KeyboardImageStore is a parallel interface to [SwitchImageStore], not
// shared, since a keyboard's images are a growable array of
// server-generated ids rather than a single optional slot.
type KeyboardImageStore interface {
	// SignGetKeyboardImage returns a signed GET URL for key on the images
	// distribution, and when it stops working.
	SignGetKeyboardImage(ctx context.Context, key KeyboardImageKey) (url string, err error)

	// PresignPutKeyboardImage returns a short-lived presigned PUT URL for
	// key, locked to contentType and size via the Content-Type and
	// Content-Length the upload must match.
	// Single-use: the upload must also send If-None-Match: *, so once an
	// object exists at key the URL can't write it again.
	PresignPutKeyboardImage(ctx context.Context, key KeyboardImageKey, contentType string, size int64) (url string, err error)

	// DeleteKeyboardImage removes the object at key. Idempotent: a
	// nonexistent key is not an error, matching S3's own DeleteObject
	// semantics.
	DeleteKeyboardImage(ctx context.Context, key KeyboardImageKey) error
}
