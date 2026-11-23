package schema

// ProfileLink is one entry in a profile's links list.
type ProfileLink struct {
	Name string `json:"name" jsonschema:"display label for the link, e.g. \"Twitch\""`
	URL  string `json:"url" jsonschema:"an https URL; http and other schemes are rejected"`
}

// GetProfileInput is the get_profile tool arguments.
type GetProfileInput struct {
	Identifier string `json:"identifier" jsonschema:"a profile owner's user id or their username"`
}

// ProfilePreferences holds a user's display/visibility preferences.
type ProfilePreferences struct {
	Currency          string `json:"currency" jsonschema:"an ISO 4217 currency code; display-only hint for which currency a client should label or convert prices in - mykeebs-api stores no per-item currency and does no conversion"`
	ShowPriceToMe     bool   `json:"show_price_to_me" jsonschema:"whether the owner is shown price fields on their own items in list_* tool results; does not affect get_* tools, which always show the owner their own item's price"`
	ShowPriceToOthers bool   `json:"show_price_to_others" jsonschema:"whether other callers (subject to the item's own visibility rules) are shown this owner's item prices, in both get_* and list_* tool results"`
}

// ProfileInput is the writable half of a profile, shared by create_profile
// and update_profile. A write is a full replace, so an omitted optional
// field clears it. No user_id: a write always targets the caller's profile.
type ProfileInput struct {
	Username        string              `json:"username" jsonschema:"3-32 chars of lowercase letters, digits, hyphen, period, or underscore; no leading/trailing period, hyphen, or underscore; no consecutive periods; a Discord-compatible superset; unique across all profiles"`
	Discoverable    bool                `json:"discoverable" jsonschema:"whether the profile is listed in the public directory and readable by anyone; when false only you can read it"`
	DiscordUsername *string             `json:"discord_username,omitempty" jsonschema:"2-32 chars, lowercase letters, digits, period, or underscore, no leading/trailing period or underscore, no consecutive periods; not verified against Discord"`
	Bio             *string             `json:"bio,omitempty" jsonschema:"free-form bio; at most 500 characters"`
	Links           []ProfileLink       `json:"links,omitempty" jsonschema:"at most 5 name/url pairs; each url must be an https URL"`
	Preferences     *ProfilePreferences `json:"preferences,omitempty" jsonschema:"display/visibility preferences; omitted means every preference reverts to its default"`
}

// CreateProfileInput is the create_profile tool arguments.
type CreateProfileInput struct {
	ProfileInput
}

// CreateProfileOutput is the create_profile tool result.
type CreateProfileOutput struct {
	Profile Profile `json:"profile" jsonschema:"the created profile"`
}

// UpdateProfileInput is the update_profile tool arguments.
type UpdateProfileInput struct {
	ProfileInput
}

// UpdateProfileOutput is the update_profile tool result.
type UpdateProfileOutput struct {
	Profile Profile `json:"profile" jsonschema:"the updated profile"`
}

// GetProfileOutput is the get_profile tool result.
type GetProfileOutput struct {
	Profile Profile `json:"profile" jsonschema:"the requested profile"`
}

// DeleteProfileInput is the delete_profile tool arguments. It takes none: a
// delete always targets the caller's own profile.
type DeleteProfileInput struct{}

// DeleteProfileOutput is the delete_profile tool result.
type DeleteProfileOutput struct{}

// ListProfilesInput is the list_profiles tool arguments. Username and
// DiscordUsername are mutually-exclusive begins-with filters.
type ListProfilesInput struct {
	Limit           *int   `json:"limit,omitempty" jsonschema:"maximum number of profiles to return (1-100, default 20)"`
	Cursor          string `json:"cursor,omitempty" jsonschema:"resume from a previous call's next_cursor"`
	Username        string `json:"username,omitempty" jsonschema:"case-sensitive begins-with filter on username; mutually exclusive with discord_username"`
	DiscordUsername string `json:"discord_username,omitempty" jsonschema:"case-insensitive begins-with filter on discord_username; mutually exclusive with username"`
}

// ListProfilesOutput is the list_profiles tool result.
type ListProfilesOutput struct {
	Profiles   []ProfileSummary `json:"profiles" jsonschema:"the discoverable profiles in this page"`
	NextCursor string           `json:"next_cursor,omitempty" jsonschema:"pass as cursor to fetch the next page; empty when there are no more"`
}

// ProfileSummary is a list_profiles row - no bio or links; use UserID to
// chain to get_profile or the collection tools.
type ProfileSummary struct {
	Username        string  `json:"username" jsonschema:"the profile's unique username"`
	UserID          string  `json:"user_id" jsonschema:"the profile owner's user id; pass to get_profile or as the user_id argument to list_keyboards, list_builds, and the other collection tools"`
	DiscordUsername *string `json:"discord_username,omitempty" jsonschema:"the owner's Discord username, if given"`
	HasAvatar       bool    `json:"has_avatar" jsonschema:"whether this profile has an avatar you can see (its owner sees it whatever its moderation status, anyone else only once approved)"`
}

// SetProfileImageInput is the set_profile_image tool's input. It takes no
// user id - a write always targets the caller's own profile - and doesn't
// carry the image bytes themselves (see UploadURL on the output).
type SetProfileImageInput struct {
	ContentType string `json:"content_type" jsonschema:"the image's MIME type; must be image/webp, the only approved image_content_type lookup value"`
	SizeBytes   int64  `json:"size_bytes" jsonschema:"the image's exact size in bytes, at most 1572864 (1.5 MB). The upload must send exactly this many bytes"`
}

// SetProfileImageOutput is the set_profile_image tool's output. UploadURL
// is a presigned S3 PUT URL - the caller uploads the image bytes directly
// to it; the tool call itself never carries image bytes.
type SetProfileImageOutput struct {
	UploadURL string `json:"upload_url" jsonschema:"a freshly-minted, short-lived presigned URL to PUT the avatar bytes to directly, using the requested content_type as the Content-Type header, an If-None-Match: * header, and a body of exactly size_bytes; single-use, so do not cache or persist it, and it expires within minutes"`
}

// DeleteProfileImageInput is the delete_profile_image tool's input. It
// takes none: a delete always targets the caller's own profile.
type DeleteProfileImageInput struct{}

// DeleteProfileImageOutput is the delete_profile_image tool's output.
// Deleting is idempotent, so there is no payload.
type DeleteProfileImageOutput struct{}

// Profile is a user's public identity. HasAvatar reports whether an avatar
// is on file; MCP never serves the image itself.
type Profile struct {
	Username        string              `json:"username" jsonschema:"the profile's unique username"`
	UserID          string              `json:"user_id" jsonschema:"the profile owner's user id; pass it as the user_id argument to list_keyboards, list_builds, and the other collection tools"`
	Discoverable    bool                `json:"discoverable" jsonschema:"whether the profile is listed in the public directory"`
	DiscordUsername *string             `json:"discord_username,omitempty" jsonschema:"the owner's Discord username, if given"`
	Bio             *string             `json:"bio,omitempty" jsonschema:"free-form bio, if given"`
	Links           []ProfileLink       `json:"links,omitempty" jsonschema:"the profile's links (name/url pairs)"`
	HasAvatar       bool                `json:"has_avatar" jsonschema:"whether this profile has an avatar you can see (its owner sees it whatever its moderation status, anyone else only once approved)"`
	Preferences     *ProfilePreferences `json:"preferences,omitempty" jsonschema:"display/visibility preferences; only ever present for the profile's owner"`

	AvatarStatus          *string `json:"avatar_status,omitempty" jsonschema:"the avatar's moderation status: pending until it's checked, then approved or rejected; only ever present for the profile's owner"`
	AvatarRejectionReason *string `json:"avatar_rejection_reason,omitempty" jsonschema:"why a rejected avatar was rejected: the moderation label; not_webp when the file isn't a WebP; too_large when its longest side is over 2560px; or unprocessable when the file couldn't be checked; only ever present for the profile's owner"`
}
