package schema

// ListSwitchesInput is the list_switches tool arguments.
type ListSwitchesInput struct {
	UserID string `json:"user_id,omitempty" jsonschema:"whose collection to list; omit for your own"`
	Limit  *int   `json:"limit,omitempty" jsonschema:"maximum number of switches to return (1-100, default 20)"`
	Cursor string `json:"cursor,omitempty" jsonschema:"resume from a previous call's next_cursor"`
}

// ListSwitchesOutput is the list_switches tool result.
type ListSwitchesOutput struct {
	Switches   []SwitchSummary `json:"switches" jsonschema:"the switches in this page"`
	NextCursor string          `json:"next_cursor,omitempty" jsonschema:"pass as cursor to fetch the next page; empty when there are no more"`
}

// SwitchSummary is the abbreviated switch shape returned by list_switches.
// Call get_switch for the remaining fields.
type SwitchSummary struct {
	ID          string   `json:"id" jsonschema:"the switch's unique id"`
	Brand       string   `json:"brand" jsonschema:"the switch's brand"`
	Name        string   `json:"name" jsonschema:"the switch's name"`
	Type        string   `json:"type" jsonschema:"the switch type, e.g. linear or tactile"`
	OrderStatus *string  `json:"order_status,omitempty" jsonschema:"where the order stands, e.g. ordered or delivered"`
	HasImage    bool     `json:"has_image" jsonschema:"whether this switch has an image you can see (its owner sees it whatever its moderation status, anyone else only once approved)"`
	Price       *float64 `json:"price,omitempty" jsonschema:"purchase price; present for the owner only if their show_price_to_me preference is set, and for any other caller only if the owner's show_price_to_others is"`
	Currency    *string  `json:"currency,omitempty" jsonschema:"the owner's display currency (an ISO 4217 code) for price; present exactly when price is"`
	Visibility  *string  `json:"visibility,omitempty" jsonschema:"who can read this switch; one of \"public\", \"authenticated\", \"private\"; only ever present for the switch's owner"`
}

// GetSwitchInput is the get_switch tool arguments.
type GetSwitchInput struct {
	SwitchID string `json:"switch_id" jsonschema:"the switch's unique id"`
	UserID   string `json:"user_id,omitempty" jsonschema:"whose collection to read from; omit for your own"`
}

// GetSwitchOutput is the get_switch tool result.
type GetSwitchOutput struct {
	Switch Switch `json:"switch" jsonschema:"the requested switch"`
}

// CreateSwitchInput is the create_switch tool arguments. Writes are always
// self-scoped, so unlike the read tools there is no user_id: a switch is
// created in the caller's own collection.
type CreateSwitchInput struct {
	SwitchInput
}

// CreateSwitchOutput is the create_switch tool result.
type CreateSwitchOutput struct {
	Switch Switch `json:"switch" jsonschema:"the created switch, including its server-generated id"`
}

// UpdateSwitchInput is the update_switch tool arguments. Every field is
// replaced, so omitting an optional field clears it rather than leaving the
// stored value in place - matching the REST PUT this mirrors.
type UpdateSwitchInput struct {
	SwitchID string `json:"switch_id" jsonschema:"the id of the switch to replace"`
	SwitchInput
}

// UpdateSwitchOutput is the update_switch tool result.
type UpdateSwitchOutput struct {
	Switch Switch `json:"switch" jsonschema:"the updated switch"`
}

// DeleteSwitchInput is the delete_switch tool input. OnDelete controls what
// happens if the switch is still referenced by a build: "block" (the
// default when omitted) fails the call; "cascade" deletes the switch and
// every referencing build.
type DeleteSwitchInput struct {
	SwitchID string `json:"switch_id" jsonschema:"the id of the switch to delete"`
	OnDelete string `json:"on_delete,omitempty" jsonschema:"how to handle a switch still referenced by a build: block (default) or cascade"`
}

// DeleteSwitchOutput is the delete_switch tool output. DeletedBuildIDs is
// populated only when on_delete was "cascade" and at least one build
// referenced the switch.
type DeleteSwitchOutput struct {
	DeletedBuildIDs []string `json:"deleted_build_ids,omitempty" jsonschema:"ids of builds also deleted, when on_delete was cascade"`
}

// SwitchInput is the writable half of a switch, shared by create_switch and
// update_switch. Optional fields are pointers so an omitted field stays
// distinguishable from one explicitly set to zero.
type SwitchInput struct {
	Brand        string               `json:"brand" jsonschema:"the switch's brand"`
	Manufacturer *string              `json:"manufacturer,omitempty" jsonschema:"who physically manufactures the switch, if different from the brand"`
	Name         string               `json:"name" jsonschema:"the switch's name"`
	Type         string               `json:"type" jsonschema:"the switch type; must be an approved switch_type lookup value"`
	Pins         *int                 `json:"pins,omitempty" jsonschema:"pin count, typically 3 or 5"`
	FactoryLubed *bool                `json:"factory_lubed,omitempty" jsonschema:"whether the switch ships pre-lubed"`
	Material     *SwitchMaterial      `json:"material,omitempty" jsonschema:"housing and stem materials; each must be an approved switch_material lookup value"`
	Force        *SwitchForce         `json:"force,omitempty" jsonschema:"actuation and bottom-out force, in grams"`
	Spring       *SwitchSpring        `json:"spring,omitempty" jsonschema:"spring material, plus its length and travel distances in mm"`
	Purchase     *SwitchPurchaseInput `json:"purchase,omitempty" jsonschema:"where it was bought and the order's status"`
	Notes        *string              `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility   string               `json:"visibility" jsonschema:"who can read this switch; one of \"public\", \"authenticated\", \"private\""`
}

// Switch reports HasImage rather than the image bytes themselves - images
// are served to end users through REST, not this MCP surface; manage a
// switch's image by id with set_switch_image/delete_switch_image. Optional
// fields are pointers so a recorded zero (a free purchase, a 0g force)
// stays distinguishable from an unset field, which is omitted entirely -
// matching what REST returns for the same stored switch.
type Switch struct {
	ID           string          `json:"id" jsonschema:"the switch's unique id"`
	Brand        string          `json:"brand" jsonschema:"the switch's brand"`
	Manufacturer *string         `json:"manufacturer,omitempty" jsonschema:"who physically manufactures the switch, if different from the brand"`
	Name         string          `json:"name" jsonschema:"the switch's name"`
	Type         string          `json:"type" jsonschema:"the switch type, e.g. linear or tactile"`
	Pins         *int            `json:"pins,omitempty" jsonschema:"pin count, typically 3 or 5"`
	FactoryLubed *bool           `json:"factory_lubed,omitempty" jsonschema:"whether the switch ships pre-lubed"`
	Material     *SwitchMaterial `json:"material,omitempty" jsonschema:"housing and stem materials"`
	Force        *SwitchForce    `json:"force,omitempty" jsonschema:"actuation and bottom-out force, in grams"`
	Spring       *SwitchSpring   `json:"spring,omitempty" jsonschema:"spring material, plus its length and travel distances in mm"`
	Purchase     *SwitchPurchase `json:"purchase,omitempty" jsonschema:"where it was bought and the order's status"`
	Notes        *string         `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility   *string         `json:"visibility,omitempty" jsonschema:"who can read this switch; one of \"public\", \"authenticated\", \"private\"; only ever present for the switch's owner"`
	HasImage     bool            `json:"has_image" jsonschema:"whether this switch has an image you can see (its owner sees it whatever its moderation status, anyone else only once approved)"`

	ImageStatus          *string `json:"image_status,omitempty" jsonschema:"the image's moderation status: pending until it's checked, then approved or rejected; only ever present for the switch's owner"`
	ImageRejectionReason *string `json:"image_rejection_reason,omitempty" jsonschema:"why a rejected image was rejected: the moderation label; not_webp when the file isn't a WebP; too_large when its longest side is over 2560px; or unprocessable when the file couldn't be checked; only ever present for the switch's owner"`
}

// SwitchMaterial is a switch's housing and stem materials.
type SwitchMaterial struct {
	TopHousing    *string `json:"top_housing,omitempty" jsonschema:"top housing material"`
	BottomHousing *string `json:"bottom_housing,omitempty" jsonschema:"bottom housing material"`
	Stem          *string `json:"stem,omitempty" jsonschema:"stem material"`
}

// SwitchForce is a switch's nominal actuation and bottom-out force, in grams.
type SwitchForce struct {
	Actuation *float64 `json:"actuation,omitempty" jsonschema:"actuation force in grams"`
	BottomOut *float64 `json:"bottom_out,omitempty" jsonschema:"bottom-out force in grams"`
}

// SwitchSpring is a switch's spring material, plus its length and travel
// distances in mm.
type SwitchSpring struct {
	Material    *string  `json:"material,omitempty" jsonschema:"spring material"`
	PreTravel   *float64 `json:"pre_travel,omitempty" jsonschema:"pre-travel distance in mm"`
	TotalTravel *float64 `json:"total_travel,omitempty" jsonschema:"total travel distance in mm"`
	Length      *float64 `json:"length,omitempty" jsonschema:"length of the stock spring in mm"`
}

// SwitchPurchase is a switch's purchase and order lifecycle, plus how many
// were bought. Dates are strings, not a date type - see
// [github.com/roguekeyboards/mykeebs-api/internal/repomcp.SwitchToMCP].
type SwitchPurchase struct {
	Vendor       *string  `json:"vendor,omitempty" jsonschema:"where the switches were bought"`
	Price        *float64 `json:"price,omitempty" jsonschema:"price paid"`
	Currency     *string  `json:"currency,omitempty" jsonschema:"the owner's display currency (an ISO 4217 code) for price; present exactly when price is"`
	OrderDate    *string  `json:"order_date,omitempty" jsonschema:"when it was ordered (YYYY-MM-DD)"`
	DeliveryDate *string  `json:"delivery_date,omitempty" jsonschema:"when it arrived (YYYY-MM-DD)"`
	OrderStatus  *string  `json:"order_status,omitempty" jsonschema:"where the order stands, e.g. ordered or delivered"`
	Quantity     *int     `json:"quantity,omitempty" jsonschema:"how many were bought"`
}

// SwitchPurchaseInput is the writable form of SwitchPurchase.
type SwitchPurchaseInput struct {
	Vendor       *string  `json:"vendor,omitempty" jsonschema:"where the switches were bought"`
	Price        *float64 `json:"price,omitempty" jsonschema:"price paid"`
	OrderDate    *string  `json:"order_date,omitempty" jsonschema:"when it was ordered (YYYY-MM-DD)"`
	DeliveryDate *string  `json:"delivery_date,omitempty" jsonschema:"when it arrived (YYYY-MM-DD)"`
	OrderStatus  *string  `json:"order_status,omitempty" jsonschema:"where the order stands, e.g. ordered or delivered"`
	Quantity     *int     `json:"quantity,omitempty" jsonschema:"how many were bought"`
}

// SetSwitchImageInput is the set_switch_image tool's input. It doesn't
// carry the image bytes themselves - see UploadURL on the output.
type SetSwitchImageInput struct {
	SwitchID    string `json:"switch_id" jsonschema:"the id of the switch to set the image on"`
	ContentType string `json:"content_type" jsonschema:"the image's MIME type; must be image/webp, the only approved image_content_type lookup value"`
	SizeBytes   int64  `json:"size_bytes" jsonschema:"the image's exact size in bytes, at most 1572864 (1.5 MB). The upload must send exactly this many bytes"`
}

// SetSwitchImageOutput is the set_switch_image tool's output. UploadURL is
// a presigned S3 PUT URL - the caller uploads the image bytes directly to
// it, matching REST's SetSwitchImage; the tool call itself never carries
// image bytes.
type SetSwitchImageOutput struct {
	UploadURL string `json:"upload_url" jsonschema:"a freshly-minted, short-lived presigned URL to PUT the image bytes to directly, using the requested content_type as the Content-Type header, an If-None-Match: * header, and a body of exactly size_bytes; single-use, so do not cache or persist it, and it expires within minutes"`
}

// DeleteSwitchImageInput is the delete_switch_image tool's input.
type DeleteSwitchImageInput struct {
	SwitchID string `json:"switch_id" jsonschema:"the id of the switch to remove the image from"`
}

// DeleteSwitchImageOutput is the delete_switch_image tool's output.
// Deleting is idempotent, so there is no payload.
type DeleteSwitchImageOutput struct{}
