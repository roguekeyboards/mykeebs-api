// Package ecpartvalidate holds the EC part rule that neither
// api/openapi.yaml nor the lookup catalog expresses: only the sub-object
// matching part_type may be set. Shared by the REST handlers and the MCP
// tools so the rule can't drift.
package ecpartvalidate

import (
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// FieldError reports that Field failed for Reason.
type FieldError struct {
	Field  string
	Reason string
}

// Validate returns a FieldError for each sub-object set on p that doesn't
// match p.PartType.
func Validate(p repository.ECPart) []FieldError {
	var errs []FieldError
	for _, sub := range []struct {
		partType repository.ECPartType
		set      bool
	}{
		{repository.ECPartTypeDomes, p.Domes != nil},
		{repository.ECPartTypeSliders, p.Sliders != nil},
		{repository.ECPartTypeHousings, p.Housings != nil},
		{repository.ECPartTypeSprings, p.Springs != nil},
		{repository.ECPartTypeRings, p.Rings != nil},
	} {
		if sub.set && sub.partType != p.PartType {
			errs = append(errs, FieldError{
				Field:  string(sub.partType),
				Reason: fmt.Sprintf("only allowed when part_type is %s", sub.partType),
			})
		}
	}
	return errs
}
