package schema

import "github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"

// Validate implements [validate.Validatable].
func (in GetProfileInput) Validate(v *validate.V) {
	v.NotBlank("identifier", in.Identifier)
}

// Validate implements [validate.Validatable].
func (in ListProfilesInput) Validate(v *validate.V) {
	checkListLimit(v, in.Limit)
	if in.Username != "" && in.DiscordUsername != "" {
		v.Fail("discord_username", "mutually exclusive with username")
	}
}

// Validate implements [validate.Validatable].
func (in SetProfileImageInput) Validate(v *validate.V) {
	checkImageSize(v, in.SizeBytes)
}
