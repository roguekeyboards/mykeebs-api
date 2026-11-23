package schema

import "github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"

// Validate implements [validate.Validatable].
func (in GetLookupInput) Validate(v *validate.V) {
	v.NotBlank("category", in.Category)
}
