package lookup

import (
	"context"
	"fmt"
	"strings"

	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// ValidateECPart returns every field on p that isn't an approved value for
// its lookup category, plus the rules that hang off part_type: it must be
// a known type, only that type's sub-object may be set, and sizes and
// stems must be known. An unset field is skipped, not treated as invalid.
func ValidateECPart(ctx context.Context, p repository.ECPart) []FieldError {
	errs := validateECPartStructure(p)

	var checks []fieldCheck
	add := func(field string, value *string, category Category) {
		if value == nil {
			return
		}
		checks = append(checks, fieldCheck{Field: field, Value: *value, Category: category})
	}

	if p.Domes != nil {
		add("domes.material", p.Domes.Material, CategoryECDomeMaterial)
	}
	if p.Sliders != nil {
		add("sliders.material", p.Sliders.Material, CategorySwitchMaterial)
	}
	if p.Housings != nil {
		add("housings.material", p.Housings.Material, CategorySwitchMaterial)
	}
	if p.Springs != nil {
		add("springs.material", p.Springs.Material, CategorySwitchSpringMaterial)
	}
	if p.Rings != nil {
		add("rings.material", p.Rings.Material, CategoryECRingMaterial)
	}
	add("purchase.vendor", p.Purchase.Vendor, CategoryVendor)
	add("purchase.order_status", p.Purchase.OrderStatus, CategoryOrderStatus)

	return append(errs, validateFields(ctx, checks)...)
}

// ValidateECPartType returns a FieldError if t isn't one of
// repository.ECPartTypes.
func ValidateECPartType(t repository.ECPartType) []FieldError {
	if t.Valid() {
		return nil
	}
	return []FieldError{{Field: "part_type", Value: string(t), Reason: oneOf(repository.ECPartTypes)}}
}

func validateECPartStructure(p repository.ECPart) []FieldError {
	var errs []FieldError

	errs = append(errs, ValidateECPartType(p.PartType)...)

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

	if p.Sliders != nil {
		errs = append(errs, checkECPartSize("sliders.size", p.Sliders.Size)...)
		if p.Sliders.Stem != nil && !p.Sliders.Stem.Valid() {
			errs = append(errs, FieldError{Field: "sliders.stem", Value: string(*p.Sliders.Stem), Reason: oneOf(repository.ECSliderStems)})
		}
	}
	if p.Housings != nil {
		errs = append(errs, checkECPartSize("housings.size", p.Housings.Size)...)
	}

	return errs
}

func checkECPartSize(field string, size *repository.ECPartSize) []FieldError {
	if size == nil || size.Valid() {
		return nil
	}
	return []FieldError{{Field: field, Value: string(*size), Reason: oneOf(repository.ECPartSizes)}}
}

func oneOf[T ~string](values []T) string {
	names := make([]string, len(values))
	for i, v := range values {
		names[i] = string(v)
	}
	return "must be one of: " + strings.Join(names, ", ")
}
