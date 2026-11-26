package schema

// ListKeycapSetsInput is the list_keycap_sets tool input.
type ListKeycapSetsInput struct {
	UserID string `json:"user_id,omitempty" jsonschema:"whose collection to list; omit for your own"`
	Limit  *int   `json:"limit,omitempty" jsonschema:"maximum number of keycap sets to return (1-100, default 20)"`
	Cursor string `json:"cursor,omitempty" jsonschema:"resume from a previous call's next_cursor"`

	AvailableOnly bool `json:"available_only,omitempty" jsonschema:"only return sets that are available for a new build. Applied to each page after it's read, so a page can hold fewer than limit, even none, while next_cursor is still set"`
}

// ListKeycapSetsOutput is the list_keycap_sets tool output.
type ListKeycapSetsOutput struct {
	KeycapSets []KeycapSetSummary `json:"keycap_sets" jsonschema:"the keycap sets in this page"`
	NextCursor string             `json:"next_cursor,omitempty" jsonschema:"pass as cursor to fetch the next page; empty when there are no more"`
}

// KeycapSetSummary is the reduced keycap set shape list_keycap_sets returns.
type KeycapSetSummary struct {
	ID                 string   `json:"id" jsonschema:"the keycap set's unique id"`
	Brand              string   `json:"brand" jsonschema:"the keycap set's brand/designer, not where it was bought - see purchase.vendor on each kit for that"`
	Name               string   `json:"name" jsonschema:"the keycap set's name"`
	Profile            *string  `json:"profile,omitempty" jsonschema:"the keycap set's profile, e.g. Cherry or OEM"`
	PrimaryKitID       *string  `json:"primary_kit_id,omitempty" jsonschema:"the id of the kit whose image represents this set, if one is designated and it still exists"`
	PrimaryKitHasImage bool     `json:"primary_kit_has_image" jsonschema:"whether the primary kit has an image you can see (its owner sees it whatever its moderation status, anyone else only once approved)"`
	OrderStatus        *string  `json:"order_status,omitempty" jsonschema:"derived from every kit's purchase.order_status: the least-progressed status wins (Planned < Ordered < Shipped < Delivered), so the set isn't Delivered while a kit is still en route; a Cancelled kit is ignored unless every kit is Cancelled; omitted if no kit has a status set"`
	Available          bool     `json:"available" jsonschema:"whether the set is free for a new build: one of its base kits is Delivered and not on any keyboard's current build (its newest by build_date, one without a build_date counting as oldest). The base kits are the primary kit, if set; otherwise the only kit; otherwise every kit whose name contains \"base\", in any case"`
	TotalCost          *float64 `json:"total_cost,omitempty" jsonschema:"total cost of the set's kits with a known purchase price; kits with no known price are excluded rather than treated as zero, so this may undercount; present for the owner only if their show_price_to_me preference is set, and for any other caller only if the owner's show_price_to_others is"`
	Currency           *string  `json:"currency,omitempty" jsonschema:"the owner's display currency (an ISO 4217 code) for total_cost; present exactly when total_cost is"`
	Visibility         *string  `json:"visibility,omitempty" jsonschema:"who can read this keycap set, set on the keycap set itself rather than derived from its kits; one of \"public\", \"authenticated\", \"private\"; only ever present for the set's owner"`
}

// GetKeycapSetInput is the get_keycap_set tool input.
type GetKeycapSetInput struct {
	KeycapSetID string `json:"keycap_set_id" jsonschema:"the keycap set's unique id"`
	UserID      string `json:"user_id,omitempty" jsonschema:"whose collection to read from; omit for your own"`
}

// GetKeycapSetOutput is the get_keycap_set tool output.
type GetKeycapSetOutput struct {
	KeycapSet KeycapSet `json:"keycap_set" jsonschema:"the requested keycap set"`
}

// KeycapSet is the full keycap set shape, including its kits.
// PrimaryKitID is nil if never set, or if the kit it named no longer
// exists - match it against Kits[].KitID to find that kit's image.
type KeycapSet struct {
	ID           string      `json:"id" jsonschema:"the keycap set's unique id"`
	Brand        string      `json:"brand" jsonschema:"the keycap set's brand/designer, not where it was bought - see purchase.vendor on each kit for that"`
	Name         string      `json:"name" jsonschema:"the keycap set's name"`
	Profile      *string     `json:"profile,omitempty" jsonschema:"the keycap set's profile, e.g. Cherry or OEM"`
	Material     *string     `json:"material,omitempty" jsonschema:"what the keycaps are made of"`
	Notes        *string     `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility   *string     `json:"visibility,omitempty" jsonschema:"who can read this keycap set; one of \"public\", \"authenticated\", \"private\"; only ever present for the set's owner"`
	Kits         []KeycapKit `json:"kits,omitempty" jsonschema:"the kits purchased as part of this set"`
	PrimaryKitID *string     `json:"primary_kit_id,omitempty" jsonschema:"the id of the kit, among kits, whose image represents this set; match against kits[].kit_id to find its image"`
	OrderStatus  *string     `json:"order_status,omitempty" jsonschema:"derived from every kit's purchase.order_status: the least-progressed status wins (Planned < Ordered < Shipped < Delivered), so the set isn't Delivered while a kit is still en route; a Cancelled kit is ignored unless every kit is Cancelled; omitted if no kit has a status set"`
	Available    bool        `json:"available" jsonschema:"whether the set is free for a new build: one of its base kits is Delivered and not on any keyboard's current build (its newest by build_date, one without a build_date counting as oldest). The base kits are the primary kit, if set; otherwise the only kit; otherwise every kit whose name contains \"base\", in any case"`
	TotalCost    *float64    `json:"total_cost,omitempty" jsonschema:"total cost of the set's kits with a known purchase price; kits with no known price are excluded rather than treated as zero, so this may undercount"`
	Currency     *string     `json:"currency,omitempty" jsonschema:"the owner's display currency (an ISO 4217 code) for total_cost; present exactly when total_cost is"`
}

// KeycapKit is one purchase within a keycap set. HasImage reports whether an
// image is on file - images are served to end users through REST, not this
// MCP surface; manage a kit's image with set_keycap_kit_image/delete_keycap_kit_image.
type KeycapKit struct {
	KitID    string             `json:"kit_id" jsonschema:"the kit's unique id, scoped to its parent keycap set"`
	Name     string             `json:"name" jsonschema:"the kit's name, e.g. Base or Novelties"`
	HasImage bool               `json:"has_image" jsonschema:"whether this kit has an image you can see (its owner sees it whatever its moderation status, anyone else only once approved)"`
	Purchase *KeycapKitPurchase `json:"purchase,omitempty" jsonschema:"where it was bought and the order's status"`

	ImageStatus          *string `json:"image_status,omitempty" jsonschema:"the image's moderation status: pending until it's checked, then approved or rejected; only ever present for the set's owner"`
	ImageRejectionReason *string `json:"image_rejection_reason,omitempty" jsonschema:"why a rejected image was rejected: the moderation label; not_webp when the file isn't a WebP; too_large when its longest side is over 2560px; or unprocessable when the file couldn't be checked; only ever present for the set's owner"`
}

// KeycapKitPurchase is a kit's purchase and order lifecycle, independent of
// the parent set's - a set can be assembled from kits bought separately.
// Dates are strings, not a date type - see
// [github.com/roguekeyboards/mykeebs-api/internal/repomcp.KeycapKitToMCP].
type KeycapKitPurchase struct {
	Vendor                *string  `json:"vendor,omitempty" jsonschema:"where the kit was bought"`
	Price                 *float64 `json:"price,omitempty" jsonschema:"price paid"`
	Currency              *string  `json:"currency,omitempty" jsonschema:"the owner's display currency (an ISO 4217 code) for price; present exactly when price is"`
	OrderDate             *string  `json:"order_date,omitempty" jsonschema:"when it was ordered (YYYY-MM-DD)"`
	DeliveryDate          *string  `json:"delivery_date,omitempty" jsonschema:"when it arrived (YYYY-MM-DD)"`
	EstimatedDeliveryDate *string  `json:"estimated_delivery_date,omitempty" jsonschema:"when it's expected to arrive (YYYY-MM-DD)"`
	OrderStatus           *string  `json:"order_status,omitempty" jsonschema:"where the order stands, e.g. ordered or delivered"`
}

// KeycapKitPurchaseInput is the writable form of KeycapKitPurchase.
type KeycapKitPurchaseInput struct {
	Vendor                *string  `json:"vendor,omitempty" jsonschema:"where the kit was bought"`
	Price                 *float64 `json:"price,omitempty" jsonschema:"price paid"`
	OrderDate             *string  `json:"order_date,omitempty" jsonschema:"when it was ordered (YYYY-MM-DD)"`
	DeliveryDate          *string  `json:"delivery_date,omitempty" jsonschema:"when it arrived (YYYY-MM-DD); can't be before order_date"`
	EstimatedDeliveryDate *string  `json:"estimated_delivery_date,omitempty" jsonschema:"when it's expected to arrive (YYYY-MM-DD), e.g. the last day of the quarter a group buy lists; can't be before order_date"`
	OrderStatus           *string  `json:"order_status,omitempty" jsonschema:"where the order stands, e.g. ordered or delivered"`
}

// KeycapSetInput is the writable half of a keycap set, shared by
// create_keycap_set and update_keycap_set. It has no Kits field - kits are
// managed one at a time via their own tools, never resent as part of a set
// edit.
type KeycapSetInput struct {
	Brand      string  `json:"brand" jsonschema:"the keycap set's brand/designer, not where it was bought - see purchase.vendor on each kit for that"`
	Name       string  `json:"name" jsonschema:"the keycap set's name"`
	Profile    *string `json:"profile,omitempty" jsonschema:"the keycap set's profile; must be an approved keycap_profile lookup value"`
	Material   *string `json:"material,omitempty" jsonschema:"what the keycaps are made of; must be an approved keycap_material lookup value"`
	Notes      *string `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility string  `json:"visibility" jsonschema:"who can read this keycap set; one of \"public\", \"authenticated\", \"private\""`
}

// CreateKeycapSetInput is the create_keycap_set tool input.
type CreateKeycapSetInput struct {
	KeycapSetInput
}

// CreateKeycapSetOutput is the create_keycap_set tool output.
type CreateKeycapSetOutput struct {
	KeycapSet KeycapSet `json:"keycap_set" jsonschema:"the created keycap set, including its server-generated id"`
}

// UpdateKeycapSetInput is the update_keycap_set tool input. Every field is
// replaced, so omitting an optional field clears it. Kits are preserved
// untouched - this can't be used to add, remove, or edit a kit.
type UpdateKeycapSetInput struct {
	KeycapSetID string `json:"keycap_set_id" jsonschema:"the id of the keycap set to replace"`
	KeycapSetInput
}

// UpdateKeycapSetOutput is the update_keycap_set tool output.
type UpdateKeycapSetOutput struct {
	KeycapSet KeycapSet `json:"keycap_set" jsonschema:"the updated keycap set"`
}

// DeleteKeycapSetInput is the delete_keycap_set tool input. OnDelete
// controls what happens if any kit in the set is still referenced by a
// build: "block" (the default when omitted) fails the call; "cascade"
// deletes the set and every referencing build.
type DeleteKeycapSetInput struct {
	KeycapSetID string `json:"keycap_set_id" jsonschema:"the id of the keycap set to delete"`
	OnDelete    string `json:"on_delete,omitempty" jsonschema:"how to handle a set still referenced by a build: block (default) or cascade"`
}

// DeleteKeycapSetOutput is the delete_keycap_set tool output.
// DeletedBuildIDs is populated only when on_delete was "cascade" and at
// least one build referenced a kit in the set.
type DeleteKeycapSetOutput struct {
	DeletedBuildIDs []string `json:"deleted_build_ids,omitempty" jsonschema:"ids of builds also deleted, when on_delete was cascade"`
}

// KeycapKitInput is the writable half of a kit, shared by create_keycap_kit
// and update_keycap_kit. It has no HasImage/image field - a kit's image is
// managed entirely through set_keycap_kit_image/delete_keycap_kit_image.
// Primary is a pointer, not a plain bool, so omitted can stay
// distinguishable from explicit false - see the field's own doc.
type KeycapKitInput struct {
	Name     string                  `json:"name" jsonschema:"the kit's name, e.g. Base or Novelties"`
	Purchase *KeycapKitPurchaseInput `json:"purchase,omitempty" jsonschema:"where it was bought and the order's status"`
	Primary  *bool                   `json:"primary,omitempty" jsonschema:"omit to leave the set's primary kit designation untouched; true makes this kit primary, replacing whichever kit held it before; false clears the designation, but only if this kit is the current primary"`
}

// CreateKeycapKitInput is the create_keycap_kit tool input.
type CreateKeycapKitInput struct {
	KeycapSetID string `json:"keycap_set_id" jsonschema:"the id of the keycap set to add the kit to"`
	KeycapKitInput
}

// CreateKeycapKitOutput is the create_keycap_kit tool output.
type CreateKeycapKitOutput struct {
	KeycapKit KeycapKit `json:"keycap_kit" jsonschema:"the created kit, including its server-generated id"`
}

// UpdateKeycapKitInput is the update_keycap_kit tool input. Every field is
// replaced, so omitting an optional field clears it. The kit's image, if
// any, is preserved untouched - this can't be used to set or clear it.
type UpdateKeycapKitInput struct {
	KeycapSetID string `json:"keycap_set_id" jsonschema:"the id of the keycap set the kit belongs to"`
	KitID       string `json:"kit_id" jsonschema:"the id of the kit to replace"`
	KeycapKitInput
}

// UpdateKeycapKitOutput is the update_keycap_kit tool output.
type UpdateKeycapKitOutput struct {
	KeycapKit KeycapKit `json:"keycap_kit" jsonschema:"the updated kit"`
}

// DeleteKeycapKitInput is the delete_keycap_kit tool input. OnDelete
// controls what happens if the kit is still referenced by a build: "block"
// (the default when omitted) fails the call; "cascade" deletes the kit and
// every referencing build.
type DeleteKeycapKitInput struct {
	KeycapSetID string `json:"keycap_set_id" jsonschema:"the id of the keycap set the kit belongs to"`
	KitID       string `json:"kit_id" jsonschema:"the id of the kit to delete"`
	OnDelete    string `json:"on_delete,omitempty" jsonschema:"how to handle a kit still referenced by a build: block (default) or cascade"`
}

// DeleteKeycapKitOutput is the delete_keycap_kit tool output.
// DeletedBuildIDs is populated only when on_delete was "cascade" and at
// least one build referenced the kit.
type DeleteKeycapKitOutput struct {
	DeletedBuildIDs []string `json:"deleted_build_ids,omitempty" jsonschema:"ids of builds also deleted, when on_delete was cascade"`
}

// SetKeycapKitImageInput is the set_keycap_kit_image tool input. It doesn't
// carry the image bytes themselves - see UploadURL on the output.
type SetKeycapKitImageInput struct {
	KeycapSetID string `json:"keycap_set_id" jsonschema:"the id of the keycap set the kit belongs to"`
	KitID       string `json:"kit_id" jsonschema:"the id of the kit to set the image on"`
	ContentType string `json:"content_type" jsonschema:"the image's MIME type; must be image/webp, the only approved image_content_type lookup value"`
	SizeBytes   int64  `json:"size_bytes" jsonschema:"the image's exact size in bytes, at most 1572864 (1.5 MB). The upload must send exactly this many bytes"`
}

// SetKeycapKitImageOutput is the set_keycap_kit_image tool output.
// UploadURL is a presigned S3 PUT URL - the caller uploads the image bytes
// directly to it, matching REST's SetKeycapKitImage; the tool call itself
// never carries image bytes.
type SetKeycapKitImageOutput struct {
	UploadURL string `json:"upload_url" jsonschema:"a freshly-minted, short-lived presigned URL to PUT the image bytes to directly, using the requested content_type as the Content-Type header, an If-None-Match: * header, and a body of exactly size_bytes; single-use, so do not cache or persist it, and it expires within minutes"`
}

// DeleteKeycapKitImageInput is the delete_keycap_kit_image tool input.
type DeleteKeycapKitImageInput struct {
	KeycapSetID string `json:"keycap_set_id" jsonschema:"the id of the keycap set the kit belongs to"`
	KitID       string `json:"kit_id" jsonschema:"the id of the kit to remove the image from"`
}

// DeleteKeycapKitImageOutput is the delete_keycap_kit_image tool output.
// Deleting is idempotent, so there is no payload.
type DeleteKeycapKitImageOutput struct{}
