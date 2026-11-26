package repomcp

import (
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// Switches and EC parts share a purchase shape. These helpers work in
// schema.SwitchPurchase/schema.SwitchPurchaseInput, which ECPart converts
// to and from its identically-shaped schema.ECPartPurchase/
// schema.ECPartPurchaseInput.

// Dates pass through as strings, unlike repoapi, so this can't fail on a
// malformed one.
func bulkPurchasesToMCP[T any](
	ps repository.BulkPurchases, isOwner bool, ownerPrefs repository.ProfilePreferences, conv func(schema.SwitchPurchase) T,
) []T {
	if len(ps) == 0 {
		return nil
	}

	out := make([]T, len(ps))
	for i, p := range ps {
		mapped := schema.SwitchPurchase{
			Vendor:                p.Vendor,
			OrderDate:             p.OrderDate,
			DeliveryDate:          p.DeliveryDate,
			EstimatedDeliveryDate: p.EstimatedDeliveryDate,
			OrderStatus:           p.OrderStatus,
			Quantity:              p.Quantity,
		}
		if ownerPrefs.ShowPriceSingle(isOwner) {
			mapped.Price = p.Price
		}
		mapped.Currency = ownerPrefs.CurrencyFor(mapped.Price)
		out[i] = conv(mapped)
	}

	return out
}

func bulkPurchasesFromMCP[T any](in []T, conv func(T) schema.SwitchPurchaseInput) repository.BulkPurchases {
	if len(in) == 0 {
		return nil
	}

	out := make(repository.BulkPurchases, len(in))
	for i, p := range in {
		c := conv(p)
		out[i] = repository.BulkPurchase{
			Vendor:                c.Vendor,
			Price:                 c.Price,
			OrderDate:             c.OrderDate,
			DeliveryDate:          c.DeliveryDate,
			EstimatedDeliveryDate: c.EstimatedDeliveryDate,
			OrderStatus:           c.OrderStatus,
			Quantity:              c.Quantity,
		}
	}

	return out.WithoutEmpty()
}
