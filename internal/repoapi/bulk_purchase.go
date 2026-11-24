package repoapi

import (
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// Switches and EC parts share a purchase shape. These helpers work in
// api.SwitchPurchase/api.SwitchPurchaseInput, which ECPart converts to and
// from its identically-shaped api.ECPartPurchase/api.ECPartPurchaseInput.

// bulkPurchasesToAPI maps every purchase, and returns the earliest
// separately as the deprecated single purchase.
func bulkPurchasesToAPI[T any](
	ps repository.BulkPurchases, isOwner bool, ownerPrefs repository.ProfilePreferences, conv func(api.SwitchPurchase) T,
) (*[]T, *T, error) {
	if len(ps) == 0 {
		return nil, nil, nil
	}

	out := make([]T, len(ps))
	var earliest *T
	earliestIdx := earliestPurchase(ps)
	for i, p := range ps {
		mapped, err := bulkPurchaseToAPI(p, isOwner, ownerPrefs)
		if err != nil {
			return nil, nil, fmt.Errorf("purchases[%d]: %w", i, err)
		}
		out[i] = conv(mapped)
		if i == earliestIdx && p != (repository.BulkPurchase{}) {
			e := out[i]
			earliest = &e
		}
	}

	return &out, earliest, nil
}

// earliestPurchase is the index of the purchase with the oldest
// order_date, or 0 when none has one.
func earliestPurchase(ps repository.BulkPurchases) int {
	earliest := 0
	dated := false
	for i, p := range ps {
		if p.OrderDate == nil {
			continue
		}
		if !dated || *p.OrderDate < *ps[earliest].OrderDate {
			earliest = i
			dated = true
		}
	}
	return earliest
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

// bulkPurchasesToRepo maps purchases when sent, otherwise the deprecated
// single purchase as the only one.
func bulkPurchasesToRepo[T any](purchases *[]T, legacy *T, conv func(T) api.SwitchPurchaseInput) repository.BulkPurchases {
	if purchases != nil {
		if len(*purchases) == 0 {
			return nil
		}
		out := make(repository.BulkPurchases, len(*purchases))
		for i, p := range *purchases {
			out[i] = bulkPurchaseToRepo(conv(p))
		}
		return out
	}

	if legacy == nil || conv(*legacy) == (api.SwitchPurchaseInput{}) {
		return nil
	}
	return repository.BulkPurchases{bulkPurchaseToRepo(conv(*legacy))}
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
