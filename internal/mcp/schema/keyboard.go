package schema

// ListKeyboardsInput is the list_keyboards tool input.
type ListKeyboardsInput struct {
	UserID string `json:"user_id,omitempty" jsonschema:"whose collection to list; omit for your own"`
	Limit  *int   `json:"limit,omitempty" jsonschema:"maximum number of keyboards to return (1-100, default 20)"`
	Cursor string `json:"cursor,omitempty" jsonschema:"resume from a previous call's next_cursor"`
}

// ListKeyboardsOutput is the list_keyboards tool output.
type ListKeyboardsOutput struct {
	Keyboards  []KeyboardSummary `json:"keyboards" jsonschema:"the keyboards in this page"`
	NextCursor string            `json:"next_cursor,omitempty" jsonschema:"pass as cursor to fetch the next page; empty when there are no more"`
}

// KeyboardSummary is the reduced keyboard shape list_keyboards returns.
type KeyboardSummary struct {
	ID          string   `json:"id" jsonschema:"the keyboard's unique id"`
	Brand       string   `json:"brand" jsonschema:"the keyboard's brand"`
	Name        string   `json:"name" jsonschema:"the keyboard's name"`
	Size        *string  `json:"size,omitempty" jsonschema:"the keyboard's size, e.g. 65% or TKL"`
	Layout      *string  `json:"layout,omitempty" jsonschema:"the keyboard's layout, e.g. ANSI or ISO"`
	OrderStatus *string  `json:"order_status,omitempty" jsonschema:"where the order stands, e.g. ordered or delivered"`
	HasImages   bool     `json:"has_images" jsonschema:"whether this keyboard has any images you can see (its owner sees every image, anyone else only approved ones); call list_keyboard_images for their ids"`
	TotalCost   *float64 `json:"total_cost,omitempty" jsonschema:"the base price plus every plate's and PCB's price, skipping unknown ones; present for the owner only if their show_price_to_me preference is set, and for any other caller only if the owner's show_price_to_others is"`
	Currency    *string  `json:"currency,omitempty" jsonschema:"the owner's display currency (an ISO 4217 code) for total_cost; present exactly when total_cost is"`
	Visibility  *string  `json:"visibility,omitempty" jsonschema:"who can read this keyboard; one of \"public\", \"authenticated\", \"private\"; only ever present for the keyboard's owner"`
}

// GetKeyboardInput is the get_keyboard tool input.
type GetKeyboardInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the keyboard's unique id"`
	UserID     string `json:"user_id,omitempty" jsonschema:"whose collection to read from; omit for your own"`
}

// GetKeyboardOutput is the get_keyboard tool output.
type GetKeyboardOutput struct {
	Keyboard Keyboard `json:"keyboard" jsonschema:"the requested keyboard"`
}

// CreateKeyboardInput is the create_keyboard tool input.
type CreateKeyboardInput struct {
	KeyboardInput
}

// CreateKeyboardOutput is the create_keyboard tool output.
type CreateKeyboardOutput struct {
	Keyboard Keyboard `json:"keyboard" jsonschema:"the created keyboard, including its server-generated id"`
}

// UpdateKeyboardInput is the update_keyboard tool input. Every field is
// replaced, so omitting an optional field clears it.
type UpdateKeyboardInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard to replace"`
	KeyboardInput
}

// UpdateKeyboardOutput is the update_keyboard tool output.
type UpdateKeyboardOutput struct {
	Keyboard Keyboard `json:"keyboard" jsonschema:"the updated keyboard"`
}

// DeleteKeyboardInput is the delete_keyboard tool input. OnDelete controls
// what happens if the keyboard is still referenced by a build: "block"
// (the default when omitted) fails the call; "cascade" deletes the
// keyboard and every referencing build.
type DeleteKeyboardInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard to delete"`
	OnDelete   string `json:"on_delete,omitempty" jsonschema:"how to handle a keyboard still referenced by a build: block (default) or cascade"`
}

// DeleteKeyboardOutput is the delete_keyboard tool output. DeletedBuildIDs
// is populated only when on_delete was "cascade" and at least one build
// referenced the keyboard.
type DeleteKeyboardOutput struct {
	DeletedBuildIDs []string `json:"deleted_build_ids,omitempty" jsonschema:"ids of builds also deleted, when on_delete was cascade"`
}

// KeyboardInput is the writable half of a keyboard, shared by
// create_keyboard and update_keyboard. Plates and PCBs have their own
// tools, like keycap kits.
type KeyboardInput struct {
	Brand      string                 `json:"brand" jsonschema:"the keyboard's brand"`
	Name       string                 `json:"name" jsonschema:"the keyboard's name"`
	Size       *string                `json:"size,omitempty" jsonschema:"the keyboard's size; must be an approved keyboard_size lookup value"`
	Layout     *string                `json:"layout,omitempty" jsonschema:"the keyboard's layout; must be an approved keyboard_layout value whose sizes include this keyboard's size"`
	Design     *KeyboardDesign        `json:"design,omitempty" jsonschema:"the case makeup"`
	Purchase   *KeyboardPurchaseInput `json:"purchase,omitempty" jsonschema:"where it was bought and the order's status; price is the base price, excluding plates and PCBs that have their own purchase"`
	Notes      *string                `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility string                 `json:"visibility" jsonschema:"who can read this keyboard; one of \"public\", \"authenticated\", \"private\""`
}

// Keyboard reports HasImages rather than the image list itself - call
// list_keyboard_images for the id of each image on file. Optional fields
// are pointers so a recorded zero stays distinguishable from an unset
// field, which is omitted entirely - matching what REST returns for the
// same stored keyboard.
type Keyboard struct {
	ID         string            `json:"id" jsonschema:"the keyboard's unique id"`
	Brand      string            `json:"brand" jsonschema:"the keyboard's brand"`
	Name       string            `json:"name" jsonschema:"the keyboard's name"`
	Size       *string           `json:"size,omitempty" jsonschema:"the keyboard's size, e.g. 65% or TKL"`
	Layout     *string           `json:"layout,omitempty" jsonschema:"the keyboard's layout, e.g. ANSI or ISO"`
	Design     *KeyboardDesign   `json:"design,omitempty" jsonschema:"the case makeup"`
	Plates     []KeyboardPlate   `json:"plates,omitempty" jsonschema:"every plate the keyboard has, in the order they were added; managed with create/update/delete_keyboard_plate"`
	PCBs       []KeyboardPCB     `json:"pcbs,omitempty" jsonschema:"every PCB the keyboard has, in the order they were added; managed with create/update/delete_keyboard_pcb"`
	Purchase   *KeyboardPurchase `json:"purchase,omitempty" jsonschema:"where it was bought and the order's status; price is the base price, excluding plates and PCBs that have their own purchase"`
	Notes      *string           `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility *string           `json:"visibility,omitempty" jsonschema:"who can read this keyboard; one of \"public\", \"authenticated\", \"private\"; only ever present for the keyboard's owner"`
	HasImages  bool              `json:"has_images" jsonschema:"whether this keyboard has any images you can see (its owner sees every image, anyone else only approved ones); call list_keyboard_images for their ids"`
	TotalCost  *float64          `json:"total_cost,omitempty" jsonschema:"the base price plus every plate's and PCB's price, skipping unknown ones"`
	Currency   *string           `json:"currency,omitempty" jsonschema:"the owner's display currency (an ISO 4217 code) for total_cost; present exactly when total_cost is"`
}

// KeyboardDesign is a keyboard's case makeup.
type KeyboardDesign struct {
	TopCase    *KeyboardMaterialColor `json:"top_case,omitempty" jsonschema:"the top case's material and color"`
	BottomCase *KeyboardMaterialColor `json:"bottom_case,omitempty" jsonschema:"the bottom case's material and color"`
	Weight     *KeyboardMaterialColor `json:"weight,omitempty" jsonschema:"the weight's material and color"`
}

// KeyboardMaterialColor is one physical part of a keyboard.
type KeyboardMaterialColor struct {
	Material *string `json:"material,omitempty" jsonschema:"what the part is made of"`
	Color    *string `json:"color,omitempty" jsonschema:"the part's color"`
}

// KeyboardPlate is one plate a keyboard has.
type KeyboardPlate struct {
	ID        string                `json:"id" jsonschema:"the plate's id, unique within the keyboard; builds reference the plate by it"`
	Material  string                `json:"material" jsonschema:"what the plate is made of"`
	Color     *string               `json:"color,omitempty" jsonschema:"the plate's color"`
	Thickness *float64              `json:"thickness,omitempty" jsonschema:"plate thickness in mm"`
	Purchase  *KeyboardPartPurchase `json:"purchase,omitempty" jsonschema:"where the plate was bought, if separately priced; absent for a plate that came with the keyboard"`
}

// KeyboardPlateInput is the writable form of KeyboardPlate.
type KeyboardPlateInput struct {
	Material  string                     `json:"material" jsonschema:"what the plate is made of; must be an approved keyboard_plate_material lookup value"`
	Color     *string                    `json:"color,omitempty" jsonschema:"the plate's color"`
	Thickness *float64                   `json:"thickness,omitempty" jsonschema:"plate thickness in mm"`
	Purchase  *KeyboardPartPurchaseInput `json:"purchase,omitempty" jsonschema:"where the plate was bought; omit for a plate that came with the keyboard at no extra cost"`
}

// KeyboardPCB is one PCB a keyboard has.
type KeyboardPCB struct {
	ID           string                `json:"id" jsonschema:"the PCB's id, unique within the keyboard; builds reference the PCB by it"`
	Thickness    *float64              `json:"thickness,omitempty" jsonschema:"PCB thickness in mm"`
	Firmware     *string               `json:"firmware,omitempty" jsonschema:"the firmware the PCB runs, e.g. QMK/VIA"`
	Assembly     *string               `json:"assembly,omitempty" jsonschema:"how the PCB is assembled, e.g. hotswap or soldered"`
	Connectivity *string               `json:"connectivity,omitempty" jsonschema:"how the PCB connects, e.g. wired or wireless"`
	Purchase     *KeyboardPartPurchase `json:"purchase,omitempty" jsonschema:"where the PCB was bought, if separately priced; absent for a PCB that came with the keyboard"`
}

// KeyboardPCBInput is the writable form of KeyboardPCB.
type KeyboardPCBInput struct {
	Thickness    *float64                   `json:"thickness,omitempty" jsonschema:"PCB thickness in mm"`
	Firmware     *string                    `json:"firmware,omitempty" jsonschema:"the firmware the PCB runs; must be an approved keyboard_pcb_firmware lookup value"`
	Assembly     *string                    `json:"assembly,omitempty" jsonschema:"how the PCB is assembled; must be an approved keyboard_pcb_assembly_type lookup value"`
	Connectivity *string                    `json:"connectivity,omitempty" jsonschema:"how the PCB connects; must be an approved keyboard_pcb_connectivity_type lookup value"`
	Purchase     *KeyboardPartPurchaseInput `json:"purchase,omitempty" jsonschema:"where the PCB was bought; omit for a PCB that came with the keyboard at no extra cost"`
}

// KeyboardPartPurchase is a plate's or PCB's own purchase, independent of
// the keyboard's - extras are often bought from another vendor.
type KeyboardPartPurchase struct {
	Vendor                *string  `json:"vendor,omitempty" jsonschema:"where the part was bought"`
	Price                 *float64 `json:"price,omitempty" jsonschema:"price paid"`
	Currency              *string  `json:"currency,omitempty" jsonschema:"the owner's display currency (an ISO 4217 code) for price; present exactly when price is"`
	OrderDate             *string  `json:"order_date,omitempty" jsonschema:"when it was ordered (YYYY-MM-DD)"`
	DeliveryDate          *string  `json:"delivery_date,omitempty" jsonschema:"when it arrived (YYYY-MM-DD)"`
	EstimatedDeliveryDate *string  `json:"estimated_delivery_date,omitempty" jsonschema:"when it's expected to arrive (YYYY-MM-DD)"`
	OrderStatus           *string  `json:"order_status,omitempty" jsonschema:"where the order stands, e.g. ordered or delivered"`
}

// KeyboardPartPurchaseInput is the writable form of KeyboardPartPurchase.
type KeyboardPartPurchaseInput struct {
	Vendor                *string  `json:"vendor,omitempty" jsonschema:"where the part was bought"`
	Price                 *float64 `json:"price,omitempty" jsonschema:"price paid"`
	OrderDate             *string  `json:"order_date,omitempty" jsonschema:"when it was ordered (YYYY-MM-DD)"`
	DeliveryDate          *string  `json:"delivery_date,omitempty" jsonschema:"when it arrived (YYYY-MM-DD); can't be before order_date"`
	EstimatedDeliveryDate *string  `json:"estimated_delivery_date,omitempty" jsonschema:"when it's expected to arrive (YYYY-MM-DD), e.g. the last day of the quarter a group buy lists; can't be before order_date"`
	OrderStatus           *string  `json:"order_status,omitempty" jsonschema:"where the order stands, e.g. ordered or delivered"`
}

// KeyboardPurchase is a keyboard's purchase and order lifecycle. Dates
// are strings, not a date type - see
// [github.com/roguekeyboards/mykeebs-api/internal/repomcp.KeyboardToMCP].
type KeyboardPurchase struct {
	Vendor                *string  `json:"vendor,omitempty" jsonschema:"where the keyboard was bought"`
	Price                 *float64 `json:"price,omitempty" jsonschema:"price paid"`
	Currency              *string  `json:"currency,omitempty" jsonschema:"the owner's display currency (an ISO 4217 code) for price; present exactly when price is"`
	OrderDate             *string  `json:"order_date,omitempty" jsonschema:"when it was ordered (YYYY-MM-DD)"`
	DeliveryDate          *string  `json:"delivery_date,omitempty" jsonschema:"when it arrived (YYYY-MM-DD)"`
	EstimatedDeliveryDate *string  `json:"estimated_delivery_date,omitempty" jsonschema:"when it's expected to arrive (YYYY-MM-DD)"`
	OrderStatus           *string  `json:"order_status,omitempty" jsonschema:"where the order stands, e.g. ordered or delivered"`
}

// KeyboardPurchaseInput is the writable form of KeyboardPurchase.
type KeyboardPurchaseInput struct {
	Vendor                *string  `json:"vendor,omitempty" jsonschema:"where the keyboard was bought"`
	Price                 *float64 `json:"price,omitempty" jsonschema:"price paid"`
	OrderDate             *string  `json:"order_date,omitempty" jsonschema:"when it was ordered (YYYY-MM-DD)"`
	DeliveryDate          *string  `json:"delivery_date,omitempty" jsonschema:"when it arrived (YYYY-MM-DD); can't be before order_date"`
	EstimatedDeliveryDate *string  `json:"estimated_delivery_date,omitempty" jsonschema:"when it's expected to arrive (YYYY-MM-DD), e.g. the last day of the quarter a group buy lists; can't be before order_date"`
	OrderStatus           *string  `json:"order_status,omitempty" jsonschema:"where the order stands, e.g. ordered or delivered"`
}

// ListKeyboardImagesInput is the list_keyboard_images tool's input.
type ListKeyboardImagesInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard to list images for"`
	UserID     string `json:"user_id,omitempty" jsonschema:"whose collection to read from; omit for your own"`
}

// ListKeyboardImagesOutput is the list_keyboard_images tool's output.
type ListKeyboardImagesOutput struct {
	Images []KeyboardImage `json:"images" jsonschema:"the keyboard's images"`
}

// KeyboardImage is one image on file for a keyboard. There's no URL - call
// add_keyboard_image/delete_keyboard_image to manage images by id; images
// are served to end users through REST, not this MCP surface.
type KeyboardImage struct {
	ImageID         string  `json:"image_id" jsonschema:"the image's id"`
	Status          *string `json:"status,omitempty" jsonschema:"the image's moderation status: pending until it's checked, then approved or rejected; only ever present for the keyboard's owner, since anyone else is only shown approved images"`
	RejectionReason *string `json:"rejection_reason,omitempty" jsonschema:"why a rejected image was rejected: the moderation label; not_webp when the file isn't a WebP; too_large when its longest side is over 2560px; or unprocessable when the file couldn't be checked; only ever present for the keyboard's owner"`
}

// AddKeyboardImageInput is the add_keyboard_image tool's input. It doesn't
// carry the image bytes themselves - see UploadURL on the output.
type AddKeyboardImageInput struct {
	KeyboardID  string `json:"keyboard_id" jsonschema:"the id of the keyboard to add an image to"`
	ContentType string `json:"content_type" jsonschema:"the image's MIME type; must be image/webp, the only approved image_content_type lookup value"`
	SizeBytes   int64  `json:"size_bytes" jsonschema:"the image's exact size in bytes, at most 1572864 (1.5 MB). The upload must send exactly this many bytes"`
}

// AddKeyboardImageOutput is the add_keyboard_image tool's output. UploadURL
// is a presigned S3 PUT URL - the caller uploads the image bytes directly
// to it, matching REST's AddKeyboardImage; the tool call itself never
// carries image bytes.
type AddKeyboardImageOutput struct {
	ImageID   string `json:"image_id" jsonschema:"the newly-created image's id"`
	UploadURL string `json:"upload_url" jsonschema:"a freshly-minted, short-lived presigned URL to PUT the image bytes to directly, using the requested content_type as the Content-Type header, an If-None-Match: * header, and a body of exactly size_bytes; single-use, so do not cache or persist it, and it expires within minutes"`
}

// DeleteKeyboardImageInput is the delete_keyboard_image tool's input.
type DeleteKeyboardImageInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard the image belongs to"`
	ImageID    string `json:"image_id" jsonschema:"the id of the image to remove, as returned by add_keyboard_image"`
}

// DeleteKeyboardImageOutput is the delete_keyboard_image tool's output.
// Deleting is idempotent, so there is no payload.
type DeleteKeyboardImageOutput struct{}

// CreateKeyboardPlateInput is the create_keyboard_plate tool input.
type CreateKeyboardPlateInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard to add the plate to"`
	KeyboardPlateInput
}

// CreateKeyboardPlateOutput is the create_keyboard_plate tool output.
type CreateKeyboardPlateOutput struct {
	Plate KeyboardPlate `json:"plate" jsonschema:"the created plate, including its server-generated id"`
}

// UpdateKeyboardPlateInput is the update_keyboard_plate tool input. Every
// field is replaced, so omitting an optional field clears it.
type UpdateKeyboardPlateInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard the plate belongs to"`
	PlateID    string `json:"plate_id" jsonschema:"the id of the plate to replace"`
	KeyboardPlateInput
}

// UpdateKeyboardPlateOutput is the update_keyboard_plate tool output.
type UpdateKeyboardPlateOutput struct {
	Plate KeyboardPlate `json:"plate" jsonschema:"the updated plate"`
}

// DeleteKeyboardPlateInput is the delete_keyboard_plate tool input.
// OnDelete controls what happens if a build uses the plate: "block" (the
// default when omitted) fails the call; "cascade" deletes the plate and
// every build using it.
type DeleteKeyboardPlateInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard the plate belongs to"`
	PlateID    string `json:"plate_id" jsonschema:"the id of the plate to delete"`
	OnDelete   string `json:"on_delete,omitempty" jsonschema:"how to handle a plate a build uses: block (default) or cascade"`
}

// DeleteKeyboardPlateOutput is the delete_keyboard_plate tool output.
type DeleteKeyboardPlateOutput struct {
	DeletedBuildIDs []string `json:"deleted_build_ids,omitempty" jsonschema:"ids of builds also deleted, when on_delete was cascade"`
}

// CreateKeyboardPCBInput is the create_keyboard_pcb tool input.
type CreateKeyboardPCBInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard to add the PCB to"`
	KeyboardPCBInput
}

// CreateKeyboardPCBOutput is the create_keyboard_pcb tool output.
type CreateKeyboardPCBOutput struct {
	PCB KeyboardPCB `json:"pcb" jsonschema:"the created PCB, including its server-generated id"`
}

// UpdateKeyboardPCBInput is the update_keyboard_pcb tool input. Every field
// is replaced, so omitting an optional field clears it.
type UpdateKeyboardPCBInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard the PCB belongs to"`
	PCBID      string `json:"pcb_id" jsonschema:"the id of the PCB to replace"`
	KeyboardPCBInput
}

// UpdateKeyboardPCBOutput is the update_keyboard_pcb tool output.
type UpdateKeyboardPCBOutput struct {
	PCB KeyboardPCB `json:"pcb" jsonschema:"the updated PCB"`
}

// DeleteKeyboardPCBInput is DeleteKeyboardPlateInput for PCBs.
type DeleteKeyboardPCBInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard the PCB belongs to"`
	PCBID      string `json:"pcb_id" jsonschema:"the id of the PCB to delete"`
	OnDelete   string `json:"on_delete,omitempty" jsonschema:"how to handle a PCB a build uses: block (default) or cascade"`
}

// DeleteKeyboardPCBOutput is the delete_keyboard_pcb tool output.
type DeleteKeyboardPCBOutput struct {
	DeletedBuildIDs []string `json:"deleted_build_ids,omitempty" jsonschema:"ids of builds also deleted, when on_delete was cascade"`
}
