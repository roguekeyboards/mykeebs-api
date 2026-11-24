package lookup

import (
	"context"
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// ValidateSwitch returns every field on sw that isn't an approved value for
// its lookup category. An unset field is skipped, not treated as invalid.
func ValidateSwitch(ctx context.Context, sw repository.Switch) []FieldError {
	var checks []fieldCheck
	add := func(field string, value *string, category Category) {
		if value == nil {
			return
		}
		checks = append(checks, fieldCheck{Field: field, Value: *value, Category: category})
	}

	add("type", &sw.Type, CategorySwitchType)
	add("material.top_housing", sw.Material.TopHousing, CategorySwitchMaterial)
	add("material.bottom_housing", sw.Material.BottomHousing, CategorySwitchMaterial)
	add("material.stem", sw.Material.Stem, CategorySwitchMaterial)
	add("spring.material", sw.Spring.Material, CategorySwitchSpringMaterial)
	for i, pu := range sw.Purchases {
		add(fmt.Sprintf("purchases[%d].vendor", i), pu.Vendor, CategoryVendor)
		add(fmt.Sprintf("purchases[%d].order_status", i), pu.OrderStatus, CategoryOrderStatus)
	}

	return validateFields(ctx, checks)
}
