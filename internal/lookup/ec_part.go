package lookup

import (
	"context"

	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// ValidateECPart returns every field on p that isn't an approved value for
// its lookup category. An unset field is skipped, not treated as invalid.
func ValidateECPart(ctx context.Context, p repository.ECPart) []FieldError {
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

	return validateFields(ctx, checks)
}
