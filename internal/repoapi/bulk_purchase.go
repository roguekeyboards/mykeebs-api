package repoapi

import (
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// Switches and EC parts share a purchase shape. These helpers work in
// api.SwitchPurchase/api.SwitchPurchaseInput, which ECPart converts to and
// from its identically-shaped api.ECPartPurchase/api.ECPartPurchaseInput.

func bulkPurchasesToAPI[T any](
	ps repository.BulkPurchases, isOwner bool, ownerPrefs repository.ProfilePreferences, conv func(api.SwitchPurchase) T,
) (*[]T, error) {
	if len(ps) == 0 {
		return nil, nil //nolint:nilnil // no purchases is a valid, expected result
	}

	out := make([]T, len(ps))
	for i, p := range ps {
		mapped, err := bulkPurchaseToAPI(p, isOwner, ownerPrefs)
		if err != nil {
			return nil, fmt.Errorf("purchases[%d]: %w", i, err)
		}
		out[i] = conv(mapped)
	}

	return &out, nil
}

func bulkPurchaseToAPI(p repository.BulkPurchase, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.SwitchPurchase, error) {
	out := api.SwitchPurchase{
		Vendor:      p.Vendor,
		OrderStatus: p.OrderStatus,
		Quantity:    p.Quantity,
	}
	if ownerPrefs.ShowPriceSingle(isOwner) {
		out.Price = p.Price
	}
	out.Currency = ownerPrefs.CurrencyFor(out.Price)
	if p.OrderDate != nil {
		d, err := parseAPIDate(*p.OrderDate)
		if err != nil {
			return api.SwitchPurchase{}, fmt.Errorf("parsing order_date: %w", err)
		}
		out.OrderDate = d
	}
	if p.DeliveryDate != nil {
		d, err := parseAPIDate(*p.DeliveryDate)
		if err != nil {
			return api.SwitchPurchase{}, fmt.Errorf("parsing delivery_date: %w", err)
		}
		out.DeliveryDate = d
	}

	return out, nil
}

func bulkPurchasesToRepo[T any](purchases *[]T, conv func(T) api.SwitchPurchaseInput) repository.BulkPurchases {
	if purchases == nil || len(*purchases) == 0 {
		return nil
	}
	out := make(repository.BulkPurchases, len(*purchases))
	for i, p := range *purchases {
		out[i] = bulkPurchaseToRepo(conv(p))
	}
	return out.WithoutEmpty()
}

func bulkPurchaseToRepo(p api.SwitchPurchaseInput) repository.BulkPurchase {
	out := repository.BulkPurchase{
		Vendor:      p.Vendor,
		Price:       p.Price,
		OrderStatus: p.OrderStatus,
		Quantity:    p.Quantity,
	}
	if p.OrderDate != nil {
		str := p.OrderDate.Format(dateLayout)
		out.OrderDate = &str
	}
	if p.DeliveryDate != nil {
		str := p.DeliveryDate.Format(dateLayout)
		out.DeliveryDate = &str
	}

	return out
}

// stripBulkPurchasePrices clears price and currency from every purchase,
// dropping any left empty.
func stripBulkPurchasePrices[T comparable](purchases *[]T, strip func(*T)) *[]T {
	if purchases == nil {
		return nil
	}

	var zero T
	kept := make([]T, 0, len(*purchases))
	for _, p := range *purchases {
		strip(&p)
		if p != zero {
			kept = append(kept, p)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return &kept
}
