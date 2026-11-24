package schema

// ListECPartsInput is the list_ec_parts tool arguments.
type ListECPartsInput struct {
	UserID   string `json:"user_id,omitempty" jsonschema:"whose collection to list; omit for your own"`
	PartType string `json:"part_type,omitempty" jsonschema:"if set, only return parts of this type: domes, sliders, housings, springs or rings"`
	Limit    *int   `json:"limit,omitempty" jsonschema:"maximum number of EC parts to return (1-100, default 20)"`
	Cursor   string `json:"cursor,omitempty" jsonschema:"resume from a previous call's next_cursor, with the same part_type"`
}

// ListECPartsOutput is the list_ec_parts tool result.
type ListECPartsOutput struct {
	ECParts    []ECPartSummary `json:"ec_parts" jsonschema:"the EC parts in this page"`
	NextCursor string          `json:"next_cursor,omitempty" jsonschema:"pass as cursor to fetch the next page; empty when there are no more"`
}

// ECPartSummary is the abbreviated EC part shape returned by list_ec_parts.
// Call get_ec_part for the remaining fields.
type ECPartSummary struct {
	ID          string   `json:"id" jsonschema:"the EC part's unique id"`
	Brand       string   `json:"brand" jsonschema:"the part's brand"`
	Name        string   `json:"name" jsonschema:"the part's name"`
	PartType    string   `json:"part_type" jsonschema:"which building block this is: domes, sliders, housings, springs or rings"`
	OrderStatus *string  `json:"order_status,omitempty" jsonschema:"derived from every purchase's order_status: the least-progressed status wins (Planned < Ordered < Shipped < Delivered); a Cancelled purchase is ignored unless every purchase is Cancelled; omitted if no purchase has a status set"`
	Price       *float64 `json:"price,omitempty" jsonschema:"total paid across purchases with a known price; present for the owner only if their show_price_to_me preference is set, and for any other caller only if the owner's show_price_to_others is"`
	Currency    *string  `json:"currency,omitempty" jsonschema:"the owner's display currency (an ISO 4217 code) for price; present exactly when price is"`
	Visibility  *string  `json:"visibility,omitempty" jsonschema:"who can read this part; one of \"public\", \"authenticated\", \"private\"; only ever present for the part's owner"`
	HasImage    bool     `json:"has_image" jsonschema:"whether this part has an image you can see (its owner sees it whatever its moderation status, anyone else only once approved)"`
}

// GetECPartInput is the get_ec_part tool arguments.
type GetECPartInput struct {
	ECPartID string `json:"ec_part_id" jsonschema:"the EC part's unique id"`
	UserID   string `json:"user_id,omitempty" jsonschema:"whose collection to read from; omit for your own"`
}

// GetECPartOutput is the get_ec_part tool result.
type GetECPartOutput struct {
	ECPart ECPart `json:"ec_part" jsonschema:"the requested EC part"`
}

// CreateECPartInput is the create_ec_part tool arguments. Writes are
// always self-scoped, so there is no user_id.
type CreateECPartInput struct {
	ECPartInput
}

// CreateECPartOutput is the create_ec_part tool result.
type CreateECPartOutput struct {
	ECPart ECPart `json:"ec_part" jsonschema:"the created EC part, including its server-generated id"`
}

// UpdateECPartInput is the update_ec_part tool arguments. Every field is
// replaced, so omitting an optional field clears it, matching the REST PUT.
type UpdateECPartInput struct {
	ECPartID string `json:"ec_part_id" jsonschema:"the id of the EC part to replace"`
	ECPartInput
}

// UpdateECPartOutput is the update_ec_part tool result.
type UpdateECPartOutput struct {
	ECPart ECPart `json:"ec_part" jsonschema:"the updated EC part"`
}

// ECPartInput is the writable half of an EC part, shared by create_ec_part
// and update_ec_part.
type ECPartInput struct {
	Brand      string                `json:"brand" jsonschema:"the part's brand"`
	Name       string                `json:"name" jsonschema:"the part's name"`
	PartType   string                `json:"part_type" jsonschema:"which building block this is: domes, sliders, housings, springs or rings; only the matching one of domes, sliders, housings, springs or rings may be set"`
	Domes      *ECDomes              `json:"domes,omitempty" jsonschema:"dome details; only with part_type domes"`
	Sliders    *ECSliders            `json:"sliders,omitempty" jsonschema:"slider details; only with part_type sliders"`
	Housings   *ECHousings           `json:"housings,omitempty" jsonschema:"housing details; only with part_type housings"`
	Springs    *ECSprings            `json:"springs,omitempty" jsonschema:"spring details; only with part_type springs"`
	Rings      *ECRings              `json:"rings,omitempty" jsonschema:"silencing ring details; only with part_type rings"`
	Purchases  []ECPartPurchaseInput `json:"purchases,omitempty" jsonschema:"every order of this part, each with where it was bought and the order's status"`
	Notes      *string               `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility string                `json:"visibility" jsonschema:"who can read this part; one of \"public\", \"authenticated\", \"private\""`
}

// ECPart is one EC part. It reports HasImage rather than the image itself;
// manage the image with set_ec_part_image/delete_ec_part_image. Optional
// fields are pointers so a recorded zero stays distinguishable from an
// unset field, which is omitted entirely.
type ECPart struct {
	ID            string           `json:"id" jsonschema:"the EC part's unique id"`
	Brand         string           `json:"brand" jsonschema:"the part's brand"`
	Name          string           `json:"name" jsonschema:"the part's name"`
	PartType      string           `json:"part_type" jsonschema:"which building block this is: domes, sliders, housings, springs or rings"`
	Domes         *ECDomes         `json:"domes,omitempty" jsonschema:"dome details"`
	Sliders       *ECSliders       `json:"sliders,omitempty" jsonschema:"slider details"`
	Housings      *ECHousings      `json:"housings,omitempty" jsonschema:"housing details"`
	Springs       *ECSprings       `json:"springs,omitempty" jsonschema:"spring details"`
	Rings         *ECRings         `json:"rings,omitempty" jsonschema:"silencing ring details"`
	Purchases     []ECPartPurchase `json:"purchases,omitempty" jsonschema:"every order of this part, each with where it was bought and the order's status"`
	TotalQuantity *int             `json:"total_quantity,omitempty" jsonschema:"the sum of purchases' quantities; omitted if no purchase has a quantity"`
	Notes         *string          `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility    *string          `json:"visibility,omitempty" jsonschema:"who can read this part; one of \"public\", \"authenticated\", \"private\"; only ever present for the part's owner"`
	HasImage      bool             `json:"has_image" jsonschema:"whether this part has an image you can see (its owner sees it whatever its moderation status, anyone else only once approved)"`

	ImageStatus          *string `json:"image_status,omitempty" jsonschema:"the image's moderation status: pending until it's checked, then approved or rejected; only ever present for the part's owner"`
	ImageRejectionReason *string `json:"image_rejection_reason,omitempty" jsonschema:"why a rejected image was rejected: the moderation label; not_webp when the file isn't a WebP; too_large when its longest side is over 2560px; or unprocessable when the file couldn't be checked; only ever present for the part's owner"`
}

// ECDomes is the domes-specific detail of an EC part.
type ECDomes struct {
	Weight   *float64 `json:"weight,omitempty" jsonschema:"nominal dome weight in grams"`
	Material *string  `json:"material,omitempty" jsonschema:"dome material; must be an approved ec_dome_material lookup value"`
	Color    *string  `json:"color,omitempty" jsonschema:"dome color"`
}

// ECSliders is the sliders-specific detail of an EC part.
type ECSliders struct {
	Size     *string `json:"size,omitempty" jsonschema:"the key size it fits: 1U or 2U"`
	Stem     *string `json:"stem,omitempty" jsonschema:"the keycap mount: MX (cross stem) or OEM (round, Topre-style)"`
	Material *string `json:"material,omitempty" jsonschema:"slider material; must be an approved switch_material lookup value"`
}

// ECHousings is the housings-specific detail of an EC part.
type ECHousings struct {
	Size     *string `json:"size,omitempty" jsonschema:"the key size it fits: 1U or 2U"`
	Material *string `json:"material,omitempty" jsonschema:"housing material; must be an approved switch_material lookup value"`
}

// ECSprings is the springs-specific detail of an EC part.
type ECSprings struct {
	Material *string `json:"material,omitempty" jsonschema:"spring material; must be an approved switch_spring_material lookup value"`
}

// ECRings is the silencing-rings-specific detail of an EC part.
type ECRings struct {
	Material  *string  `json:"material,omitempty" jsonschema:"ring material; must be an approved ec_ring_material lookup value"`
	Thickness *float64 `json:"thickness,omitempty" jsonschema:"ring thickness in mm"`
}

// ECPartPurchase is an EC part's purchase and order lifecycle, plus how
// many were bought.
type ECPartPurchase struct {
	Vendor       *string  `json:"vendor,omitempty" jsonschema:"where the parts were bought"`
	Price        *float64 `json:"price,omitempty" jsonschema:"the total paid for quantity parts"`
	Currency     *string  `json:"currency,omitempty" jsonschema:"the owner's display currency (an ISO 4217 code) for price; present exactly when price is"`
	OrderDate    *string  `json:"order_date,omitempty" jsonschema:"when it was ordered (YYYY-MM-DD)"`
	DeliveryDate *string  `json:"delivery_date,omitempty" jsonschema:"when it arrived (YYYY-MM-DD)"`
	OrderStatus  *string  `json:"order_status,omitempty" jsonschema:"where the order stands, e.g. ordered or delivered"`
	Quantity     *int     `json:"quantity,omitempty" jsonschema:"how many were bought"`
}

// ECPartPurchaseInput is the writable form of ECPartPurchase.
type ECPartPurchaseInput struct {
	Vendor       *string  `json:"vendor,omitempty" jsonschema:"where the parts were bought; must be an approved vendor lookup value"`
	Price        *float64 `json:"price,omitempty" jsonschema:"the total paid for quantity parts"`
	OrderDate    *string  `json:"order_date,omitempty" jsonschema:"when it was ordered (YYYY-MM-DD)"`
	DeliveryDate *string  `json:"delivery_date,omitempty" jsonschema:"when it arrived (YYYY-MM-DD)"`
	OrderStatus  *string  `json:"order_status,omitempty" jsonschema:"where the order stands; must be an approved order_status lookup value"`
	Quantity     *int     `json:"quantity,omitempty" jsonschema:"how many were bought"`
}

// DeleteECPartInput is the delete_ec_part tool's input.
type DeleteECPartInput struct {
	ECPartID string `json:"ec_part_id" jsonschema:"the id of the EC part to delete"`
	OnDelete string `json:"on_delete,omitempty" jsonschema:"how to handle an EC part still used by a build: block (default) or cascade"`
}

// DeleteECPartOutput is the delete_ec_part tool's output.
type DeleteECPartOutput struct {
	DeletedBuildIDs []string `json:"deleted_build_ids,omitempty" jsonschema:"ids of builds also deleted, when on_delete was cascade"`
}

// SetECPartImageInput is the set_ec_part_image tool's input. It doesn't
// carry the image bytes themselves - see UploadURL on the output.
type SetECPartImageInput struct {
	ECPartID    string `json:"ec_part_id" jsonschema:"the id of the EC part to set the image on"`
	ContentType string `json:"content_type" jsonschema:"the image's MIME type; must be image/webp, the only approved image_content_type lookup value"`
	SizeBytes   int64  `json:"size_bytes" jsonschema:"the image's exact size in bytes, at most 1572864 (1.5 MB). The upload must send exactly this many bytes"`
}

// SetECPartImageOutput is the set_ec_part_image tool's output.
type SetECPartImageOutput struct {
	UploadURL string `json:"upload_url" jsonschema:"a freshly-minted, short-lived presigned URL to PUT the image bytes to directly, using the requested content_type as the Content-Type header, an If-None-Match: * header, and a body of exactly size_bytes; single-use, so do not cache or persist it, and it expires within minutes"`
}

// DeleteECPartImageInput is the delete_ec_part_image tool's input.
type DeleteECPartImageInput struct {
	ECPartID string `json:"ec_part_id" jsonschema:"the id of the EC part to remove the image from"`
}

// DeleteECPartImageOutput is the delete_ec_part_image tool's output.
// Deleting is idempotent, so there is no payload.
type DeleteECPartImageOutput struct{}
