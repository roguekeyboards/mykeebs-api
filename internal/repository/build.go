package repository

import (
	"context"
	"fmt"
	"sort"

	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
)

// BuildSwitchEntry is one entry in Build.Switches.
type BuildSwitchEntry struct {
	Switch string `dynamodbav:"switch" json:"switch"`
	Count  int    `dynamodbav:"count" json:"count"`
}

// BuildECPartEntry is one entry in Build.ECParts.
type BuildECPartEntry struct {
	ECPart string `dynamodbav:"ec_part" json:"ec_part"`
	Count  int    `dynamodbav:"count" json:"count"`
}

// Build types with special handling: an EC build lists ECParts, any other
// lists Switches.
const (
	BuildTypeMX = "MX"
	BuildTypeEC = "EC"
)

// BuildKeycapKitEntry references one kit by KeycapKit.KitID.
type BuildKeycapKitEntry struct {
	KeycapSet string `dynamodbav:"keycap_set" json:"keycap_set"`
	Kit       string `dynamodbav:"kit" json:"kit"`
}

// BuildStabs has open-vocabulary Name and MountType, validated via
// [github.com/roguekeyboards/mykeebs-api/internal/lookup.ValidateBuild], not
// closed Go enums.
type BuildStabs struct {
	Name      *string  `dynamodbav:"name,omitempty" json:"name,omitempty"`
	MountType *string  `dynamodbav:"mount_type,omitempty" json:"mount_type,omitempty"`
	Price     *float64 `dynamodbav:"price,omitempty" json:"price,omitempty"`
}

// BuildCaseMountType has open-vocabulary Type and Durometer, validated
// via [github.com/roguekeyboards/mykeebs-api/internal/lookup.ValidateBuild], not
// closed Go enums.
type BuildCaseMountType struct {
	Type      *string `dynamodbav:"type,omitempty" json:"type,omitempty"`
	Durometer *string `dynamodbav:"durometer,omitempty" json:"durometer,omitempty"`
}

// BuildImageEntry is one value in the Build.Images map (keyed by image id).
// Seq is a repository-internal ordering key: on add it's set to
// time.Now().UnixNano(), so a new image sorts after existing ones without
// reading the current max. Wall-clock, not a monotonic source - a backward
// clock step between two adds could misorder one image (cosmetic on a
// single-user list, self-heals on any reorder). The API/MCP layers sort on
// Seq to present images in add order; it's not in JSON.
type BuildImageEntry struct {
	Path BuildImageKey `dynamodbav:"path" json:"-"`
	Seq  int           `dynamodbav:"seq" json:"-"`

	// Status is empty on images stored before moderation; read it through
	// ImageStatus.Effective. RejectionReason is set only when rejected.
	Status          ImageStatus `dynamodbav:"status,omitempty" json:"-"`
	RejectionReason *string     `dynamodbav:"rejection_reason,omitempty" json:"-"`
}

// BuildImage is an image id paired with its stored entry, the ordered
// element type the API/MCP layers work with. SortedBuildImages builds a
// []BuildImage from a Build.Images map.
type BuildImage struct {
	ImageID string
	Path    BuildImageKey
	Seq     int

	// Status is the entry's effective status, never empty.
	Status          ImageStatus
	RejectionReason *string
}

// SortedBuildImages flattens an Images map into a slice ordered by Seq
// (ascending), the order images were added in.
func SortedBuildImages(images map[string]BuildImageEntry) []BuildImage {
	out := make([]BuildImage, 0, len(images))
	for id, entry := range images {
		out = append(out, BuildImage{
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

// BuildImagesMap builds an Images map from an ordered slice, assigning Seq
// by position. The inverse of SortedBuildImages, for callers holding images
// as a list (reload tooling, tests).
func BuildImagesMap(images []BuildImage) map[string]BuildImageEntry {
	if len(images) == 0 {
		return nil
	}
	out := make(map[string]BuildImageEntry, len(images))
	for i, img := range images {
		out[img.ImageID] = BuildImageEntry{Path: img.Path, Seq: i}
	}
	return out
}

// Build has UserID as the DynamoDB partition key and ID as the sort key.
// Plate and PCB are ids of Keyboard's plates and PCBs; reads treat one the
// keyboard no longer has as absent.
type Build struct {
	UserID   string `dynamodbav:"user_id" json:"-"`
	ID       string `dynamodbav:"id" json:"id"`
	Keyboard string `dynamodbav:"keyboard" json:"keyboard"`
	// BuildType is empty on builds stored before it existed; see
	// EffectiveBuildType.
	BuildType     string                `dynamodbav:"build_type,omitempty" json:"build_type,omitempty"`
	Plate         *string               `dynamodbav:"plate,omitempty" json:"plate,omitempty"`
	PCB           *string               `dynamodbav:"pcb,omitempty" json:"pcb,omitempty"`
	CaseMountType *BuildCaseMountType   `dynamodbav:"case_mount_type,omitempty" json:"case_mount_type,omitempty"`
	Stabs         *BuildStabs           `dynamodbav:"stabs,omitempty" json:"stabs,omitempty"`
	Foam          *bool                 `dynamodbav:"foam,omitempty" json:"foam,omitempty"`
	Switches      []BuildSwitchEntry    `dynamodbav:"switches,omitempty" json:"switches,omitempty"`
	ECParts       []BuildECPartEntry    `dynamodbav:"ec_parts,omitempty" json:"ec_parts,omitempty"`
	KeycapKits    []BuildKeycapKitEntry `dynamodbav:"keycap_kits,omitempty" json:"keycap_kits,omitempty"`
	BuildDate     *string               `dynamodbav:"build_date,omitempty" json:"build_date,omitempty"`
	Notes         *string               `dynamodbav:"notes,omitempty" json:"notes,omitempty"`
	Visibility    Visibility            `dynamodbav:"visibility" json:"visibility"`
	// Images is keyed by image id. AddImage/DeleteImage address a single
	// entry in place (images.<id>); the API/MCP layers project it to an
	// ordered list via SortedBuildImages, sorting on each entry's Seq.
	Images map[string]BuildImageEntry `dynamodbav:"images,omitempty" json:"-"`
}

// EffectiveBuildType is BuildType, or BuildTypeMX for a build stored
// before build types existed.
func (b Build) EffectiveBuildType() string {
	if b.BuildType == "" {
		return BuildTypeMX
	}
	return b.BuildType
}

// BuildRepository provides access to builds.
type BuildRepository interface {
	// List returns ownerID's builds. If keyboardID is non-empty, only
	// builds whose Keyboard field equals keyboardID are returned. Only the
	// last page has fewer than limit, and it can be empty. A cursor issued
	// under a different keyboardID is ErrInvalidCursor.
	List(ctx context.Context, ownerID string, visibilities []Visibility, keyboardID string, limit int, cursor string) (builds []Build, nextCursor string, err error)

	// Get fetches by exact key regardless of visibility; the caller checks
	// the result via
	// [github.com/roguekeyboards/mykeebs-api/internal/authz.CanReadVisibility].
	Get(ctx context.Context, ownerID, id string) (*Build, error)

	// Create returns ErrAlreadyExists on an ID collision.
	Create(ctx context.Context, b Build) (*Build, error)

	// Update replaces the body-settable fields and re-diffs the build's
	// ref-markers in one TransactWriteItems. Returns ErrNotFound if no build
	// with that ID exists for the owner, or ErrMutationConflict if the
	// transaction keeps losing to concurrent writers.
	Update(ctx context.Context, b Build) (*Build, error)

	// Delete removes the caller's build and its ref-markers, atomically.
	// Callers clean up any images it had in a BuildImageStore themselves,
	// before calling Delete. Idempotent: a nonexistent id is not an error.
	Delete(ctx context.Context, id string) error

	// DeleteAll removes every build the caller owns, ref-markers included.
	// Callers clean up images themselves, before calling DeleteAll.
	// Idempotent.
	DeleteAll(ctx context.Context) error

	// AddImage adds image (image.ImageID must be set) to the build's Images
	// map with a server-assigned Seq that sorts it after every existing image,
	// with Status pending; image.Seq and image.Status are ignored. Returns
	// ErrNotFound if the parent build doesn't exist, ErrImageLimitReached if
	// it already has MaxImagesPerItem images, or a wrapped duplicate-id error
	// if image.ImageID is already present.
	AddImage(ctx context.Context, buildID string, image BuildImage) error

	// DeleteImage removes imageID from buildID's Images map and returns the
	// key that was cleared. Idempotent: an imageID not present returns
	// (nil, nil). Returns ErrNotFound if buildID doesn't exist.
	DeleteImage(ctx context.Context, buildID, imageID string) (*BuildImageKey, error)

	// SetImageStatus sets the matching image's moderation status to `to` and
	// its rejection reason to reason (cleared when nil), conditioned on Path
	// still equalling forPath and the status still being from (a missing
	// status counts as approved).
	// Returns ok=false (not an error) if that condition fails or the item
	// doesn't exist.
	SetImageStatus(ctx context.Context, ownerID, buildID, imageID string, forPath BuildImageKey, from, to ImageStatus, reason *string) (ok bool, err error)

	// FindBuildsReferencingKeyboard returns the ids of every build owned by
	// ownerID whose keyboard field is keyboardID. Used by KeyboardRepository
	// Delete to block/cascade a delete that's still referenced.
	FindBuildsReferencingKeyboard(ctx context.Context, ownerID, keyboardID string) ([]string, error)

	// FindBuildsReferencingSwitch returns the ids of every build owned by
	// ownerID with a switches[] entry referencing switchID. Used by
	// SwitchRepository Delete to block/cascade a delete that's still
	// referenced.
	FindBuildsReferencingSwitch(ctx context.Context, ownerID, switchID string) ([]string, error)

	// FindBuildsReferencingECPart returns the ids of every build owned by
	// ownerID with an ec_parts[] entry referencing ecPartID.
	FindBuildsReferencingECPart(ctx context.Context, ownerID, ecPartID string) ([]string, error)

	// FindBuildsReferencingKeycapKit returns the ids of every build owned
	// by ownerID with a keycap_kits[] entry referencing the (keycapSetID,
	// kitID) pair. Used by KeycapSetRepository DeleteKit to block/cascade a
	// delete of a single kit that's still referenced.
	FindBuildsReferencingKeycapKit(ctx context.Context, ownerID, keycapSetID, kitID string) ([]string, error)

	// FindBuildsReferencingKeycapSet returns the ids of every build owned
	// by ownerID with a keycap_kits[] entry referencing any kit in
	// keycapSetID. Used by KeycapSetRepository Delete to block/cascade a
	// whole-set delete that's still referenced by any of its kits.
	FindBuildsReferencingKeycapSet(ctx context.Context, ownerID, keycapSetID string) ([]string, error)
}

// BuildImageKey is the object key an image is stored under in a
// BuildImageStore.
type BuildImageKey string

// NewBuildImageKey takes ownerID from ctx, not a parameter, so a caller
// can't build a key addressing anyone else's prefix.
func NewBuildImageKey(ctx context.Context, buildID, imageID string) (BuildImageKey, error) {
	ownerID, ok := apictx.UserID(ctx)
	if !ok {
		return "", ErrNoUserID
	}

	return BuildImageKey(fmt.Sprintf("builds/%s/%s/images/%s", ownerID, buildID, imageID)), nil
}

// BuildImageStore is a parallel interface to [KeycapKitImageStore], not
// shared, since a build's images are a growable array of server-generated
// ids rather than a single optional slot.
type BuildImageStore interface {
	// SignGetBuildImage returns a signed GET URL for key on the images
	// distribution, and when it stops working.
	SignGetBuildImage(ctx context.Context, key BuildImageKey) (url string, err error)

	// PresignPutBuildImage returns a short-lived presigned PUT URL for key,
	// locked to contentType and size via the Content-Type and Content-Length
	// the upload must match.
	// Single-use: the upload must also send If-None-Match: *, so once an
	// object exists at key the URL can't write it again.
	PresignPutBuildImage(ctx context.Context, key BuildImageKey, contentType string, size int64) (url string, err error)

	// DeleteBuildImage is idempotent, matching S3's own DeleteObject.
	DeleteBuildImage(ctx context.Context, key BuildImageKey) error
}
