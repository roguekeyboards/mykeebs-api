package schema

// BuildSwitchEntry is one entry in BuildInput.Switches.
type BuildSwitchEntry struct {
	Switch string `json:"switch" jsonschema:"the id of a Switch resource in the caller's collection"`
	Count  int    `json:"count" jsonschema:"how many of this switch are installed"`
}

// BuildECPartEntry is one entry in BuildInput.ECParts.
type BuildECPartEntry struct {
	ECPart string `json:"ec_part" jsonschema:"the id of an EC part in the caller's collection"`
	Count  int    `json:"count" jsonschema:"how many of this part are installed"`
}

// BuildKeycapKitEntry is one entry in BuildInput.KeycapKits.
type BuildKeycapKitEntry struct {
	KeycapSet string `json:"keycap_set" jsonschema:"the id of a KeycapSet resource in the caller's collection"`
	Kit       string `json:"kit" jsonschema:"the kit_id of one of that keycap set's kits"`
}

// BuildStabs is Build.Stabs.
type BuildStabs struct {
	Name      *string  `json:"name,omitempty" jsonschema:"the stabilizer's name; must be an approved build_stabilizer lookup value"`
	MountType *string  `json:"mount_type,omitempty" jsonschema:"how the stabilizer is mounted; must be an approved build_stabilizer_mount_type lookup value"`
	Price     *float64 `json:"price,omitempty" jsonschema:"price paid"`
	Currency  *string  `json:"currency,omitempty" jsonschema:"the owner's display currency (an ISO 4217 code) for price; present exactly when price is"`
}

// BuildStabsInput is the writable form of BuildStabs.
type BuildStabsInput struct {
	Name      *string  `json:"name,omitempty" jsonschema:"the stabilizer's name; must be an approved build_stabilizer lookup value"`
	MountType *string  `json:"mount_type,omitempty" jsonschema:"how the stabilizer is mounted; must be an approved build_stabilizer_mount_type lookup value"`
	Price     *float64 `json:"price,omitempty" jsonschema:"price paid"`
}

// BuildCaseMountType is BuildInput.CaseMountType.
type BuildCaseMountType struct {
	Type      *string `json:"type,omitempty" jsonschema:"the mount style; must be an approved build_case_mount_type lookup value"`
	Durometer *string `json:"durometer,omitempty" jsonschema:"gasket/o-ring hardness; must be an approved build_durometer lookup value, and only allowed with a case mount type whose supports_durometer is set"`
}

// BuildInput has no Images field - a build's images are managed entirely
// through their own tools, never carried in a build write.
type BuildInput struct {
	Keyboard      string                `json:"keyboard" jsonschema:"the id of a Keyboard resource in the caller's collection"`
	BuildType     string                `json:"build_type,omitempty" jsonschema:"the switch platform: an approved build_type lookup value, MX when omitted; an EC build lists ec_parts and no switches, any other lists switches and no ec_parts"`
	Plate         *string               `json:"plate,omitempty" jsonschema:"the id of the keyboard's plate that's installed; must be one of the keyboard's plates"`
	PCB           *string               `json:"pcb,omitempty" jsonschema:"the id of the keyboard's PCB that's installed; must be one of the keyboard's pcbs"`
	CaseMountType *BuildCaseMountType   `json:"case_mount_type,omitempty" jsonschema:"the case's mounting style"`
	Stabs         *BuildStabsInput      `json:"stabs,omitempty" jsonschema:"the stabilizers used"`
	Foam          *bool                 `json:"foam,omitempty" jsonschema:"whether the build has case foam"`
	Switches      []BuildSwitchEntry    `json:"switches,omitempty" jsonschema:"the switches used and how many of each"`
	ECParts       []BuildECPartEntry    `json:"ec_parts,omitempty" jsonschema:"the EC parts used and how many of each"`
	KeycapKits    []BuildKeycapKitEntry `json:"keycap_kits,omitempty" jsonschema:"the keycap kits used"`
	BuildDate     *string               `json:"build_date,omitempty" jsonschema:"when the build was assembled (YYYY-MM-DD)"`
	Notes         *string               `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility    string                `json:"visibility" jsonschema:"who can read this build; one of \"public\", \"authenticated\", \"private\""`
}

// Build reports HasImages rather than the image list itself - call
// list_build_images for the id of each image on file.
type Build struct {
	ID            string                `json:"id" jsonschema:"the build's unique id"`
	Keyboard      string                `json:"keyboard" jsonschema:"the id of the Keyboard resource this build is based on"`
	BuildType     string                `json:"build_type" jsonschema:"the switch platform, e.g. MX or EC"`
	Plate         *string               `json:"plate,omitempty" jsonschema:"the id of the keyboard's plate that's installed"`
	PCB           *string               `json:"pcb,omitempty" jsonschema:"the id of the keyboard's PCB that's installed"`
	CaseMountType *BuildCaseMountType   `json:"case_mount_type,omitempty" jsonschema:"the case's mounting style"`
	Stabs         *BuildStabs           `json:"stabs,omitempty" jsonschema:"the stabilizers used"`
	Foam          *bool                 `json:"foam,omitempty" jsonschema:"whether the build has case foam"`
	Switches      []BuildSwitchEntry    `json:"switches,omitempty" jsonschema:"the switches used and how many of each"`
	ECParts       []BuildECPartEntry    `json:"ec_parts,omitempty" jsonschema:"the EC parts used and how many of each"`
	KeycapKits    []BuildKeycapKitEntry `json:"keycap_kits,omitempty" jsonschema:"the keycap kits used"`
	BuildDate     *string               `json:"build_date,omitempty" jsonschema:"when the build was assembled (YYYY-MM-DD)"`
	Notes         *string               `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility    *string               `json:"visibility,omitempty" jsonschema:"who can read this build; one of \"public\", \"authenticated\", \"private\"; only ever present for the build's owner"`
	HasImages     bool                  `json:"has_images" jsonschema:"whether this build has any images you can see (its owner sees every image, anyone else only approved ones); call list_build_images for their ids"`
}

// CreateBuildInput is the create_build tool's input.
type CreateBuildInput struct {
	BuildInput
}

// CreateBuildOutput is the create_build tool's output.
type CreateBuildOutput struct {
	Build Build `json:"build" jsonschema:"the created build, including its server-generated id"`
}

// UpdateBuildInput is the update_build tool's input.
type UpdateBuildInput struct {
	BuildInput
	BuildID string `json:"build_id" jsonschema:"the build's unique id"`
}

// UpdateBuildOutput is the update_build tool's output.
type UpdateBuildOutput struct {
	Build Build `json:"build" jsonschema:"the updated build"`
}

// DeleteBuildInput is the delete_build tool's input.
type DeleteBuildInput struct {
	BuildID string `json:"build_id" jsonschema:"the id of the build to delete"`
}

// DeleteBuildOutput is the delete_build tool's output. Deleting is
// idempotent, so there is no payload.
type DeleteBuildOutput struct{}

// GetBuildInput is the get_build tool's input.
type GetBuildInput struct {
	BuildID string `json:"build_id" jsonschema:"the build's unique id"`
	UserID  string `json:"user_id,omitempty" jsonschema:"whose collection to read from; omit for your own"`
}

// GetBuildOutput is the get_build tool's output.
type GetBuildOutput struct {
	Build Build `json:"build" jsonschema:"the requested build"`
}

// ListBuildsInput is the list_builds tool's input.
type ListBuildsInput struct {
	UserID     string `json:"user_id,omitempty" jsonschema:"whose collection to list; omit for your own"`
	KeyboardID string `json:"keyboard_id,omitempty" jsonschema:"if set, only return builds whose keyboard field is this keyboard's id"`
	Limit      *int   `json:"limit,omitempty" jsonschema:"maximum number of builds to return (1-100, default 20)"`
	Cursor     string `json:"cursor,omitempty" jsonschema:"resume from a previous call's next_cursor"`
}

// ListBuildsOutput is the list_builds tool's output.
type ListBuildsOutput struct {
	Builds     []BuildSummary `json:"builds" jsonschema:"the builds in this page"`
	NextCursor string         `json:"next_cursor,omitempty" jsonschema:"pass as cursor to fetch the next page; empty when there are no more"`
}

// BuildSummary is the reduced build shape list_builds returns.
type BuildSummary struct {
	ID         string                `json:"id" jsonschema:"the build's unique id"`
	KeyboardID string                `json:"keyboard_id" jsonschema:"the id of the referenced Keyboard resource; useful for grouping builds by keyboard"`
	BuildType  string                `json:"build_type" jsonschema:"the switch platform, e.g. MX or EC"`
	BuildDate  *string               `json:"build_date,omitempty" jsonschema:"when the build was assembled (YYYY-MM-DD)"`
	HasImage   bool                  `json:"has_image" jsonschema:"whether this build has any images you can see (its owner sees every image, anyone else only approved ones)"`
	Keyboard   *BuildSummaryKeyboard `json:"keyboard,omitempty" jsonschema:"the build's keyboard, denormalized for display"`
	TotalCost  *float64              `json:"total_cost,omitempty" jsonschema:"total cost of the build's priced components (keyboard, switches or EC parts, keycap kits, stabs); components with no known price are excluded rather than treated as zero, so this may undercount; present for the owner only if their show_price_to_me preference is set, and for any other caller only if the owner's show_price_to_others is"`
	Currency   *string               `json:"currency,omitempty" jsonschema:"the owner's display currency (an ISO 4217 code) for total_cost; present exactly when total_cost is"`
	Visibility *string               `json:"visibility,omitempty" jsonschema:"who can read this build; one of \"public\", \"authenticated\", \"private\"; only ever present for the build's owner"`
}

// BuildSummaryKeyboard is BuildSummary's denormalized keyboard reference.
type BuildSummaryKeyboard struct {
	Brand string `json:"brand" jsonschema:"the referenced keyboard's brand"`
	Name  string `json:"name" jsonschema:"the referenced keyboard's name"`
}

// AddBuildImageInput is the add_build_image tool's input. It doesn't carry
// the image bytes themselves - see UploadURL on the output.
type AddBuildImageInput struct {
	BuildID     string `json:"build_id" jsonschema:"the id of the build to add an image to"`
	ContentType string `json:"content_type" jsonschema:"the image's MIME type; must be image/webp, the only approved image_content_type lookup value"`
	SizeBytes   int64  `json:"size_bytes" jsonschema:"the image's exact size in bytes, at most 1572864 (1.5 MB). The upload must send exactly this many bytes"`
}

// AddBuildImageOutput is the add_build_image tool's output. UploadURL is a
// presigned S3 PUT URL - the caller uploads the image bytes directly to it,
// matching REST's AddBuildImage; the tool call itself never carries image
// bytes.
type AddBuildImageOutput struct {
	ImageID   string `json:"image_id" jsonschema:"the newly-created image's id"`
	UploadURL string `json:"upload_url" jsonschema:"a freshly-minted, short-lived presigned URL to PUT the image bytes to directly, using the requested content_type as the Content-Type header, an If-None-Match: * header, and a body of exactly size_bytes; single-use, so do not cache or persist it, and it expires within minutes"`
}

// DeleteBuildImageInput is the delete_build_image tool's input.
type DeleteBuildImageInput struct {
	BuildID string `json:"build_id" jsonschema:"the id of the build the image belongs to"`
	ImageID string `json:"image_id" jsonschema:"the id of the image to remove, as returned by add_build_image"`
}

// DeleteBuildImageOutput is the delete_build_image tool's output. Deleting
// is idempotent, so there is no payload.
type DeleteBuildImageOutput struct{}

// ListBuildImagesInput is the list_build_images tool's input.
type ListBuildImagesInput struct {
	BuildID string `json:"build_id" jsonschema:"the id of the build to list images for"`
	UserID  string `json:"user_id,omitempty" jsonschema:"whose collection to read from; omit for your own"`
}

// ListBuildImagesOutput is the list_build_images tool's output.
type ListBuildImagesOutput struct {
	Images []BuildImage `json:"images" jsonschema:"the build's images"`
}

// BuildImage is one image on file for a build. There's no URL - call
// add_build_image/delete_build_image to manage images by id; images are
// served to end users through REST, not this MCP surface.
type BuildImage struct {
	ImageID         string  `json:"image_id" jsonschema:"the image's id"`
	Status          *string `json:"status,omitempty" jsonschema:"the image's moderation status: pending until it's checked, then approved or rejected; only ever present for the build's owner, since anyone else is only shown approved images"`
	RejectionReason *string `json:"rejection_reason,omitempty" jsonschema:"why a rejected image was rejected: the moderation label; not_webp when the file isn't a WebP; too_large when its longest side is over 2560px; or unprocessable when the file couldn't be checked; only ever present for the build's owner"`
}
