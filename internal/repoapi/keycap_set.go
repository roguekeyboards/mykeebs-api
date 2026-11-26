package repoapi

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/roguekeyboards/mykeebs-api/internal/availability"
	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// KeycapSet maps repository.KeycapSet to and from its wire representations.
type KeycapSet struct {
	Images repository.KeycapKitImageStore
}

// ToAPI maps a repository.KeycapSet to its wire representation. The owner
// always sees each kit's own Purchase.Price; a non-owner sees it only if
// ownerPrefs.ShowPriceToOthers. Returns an error if a stored kit's Purchase
// date doesn't match dateLayout, or if a kit has an ImagePath and signing
// its URL fails. Kits are mapped concurrently, in repository.SortedKits
// order (seq, the order they were added, then kit_id). usage is what the
// owner's current builds use.
func (ks KeycapSet) ToAPI(ctx context.Context, set repository.KeycapSet, isOwner bool, ownerPrefs repository.ProfilePreferences, usage availability.Usage) (api.KeycapSet, error) {
	var kits *[]api.KeycapKit
	if len(set.Kits) > 0 {
		sorted := repository.SortedKits(set.Kits)
		mapped := make([]api.KeycapKit, len(sorted))
		errs := make([]error, len(sorted))

		var wg sync.WaitGroup
		for i, kit := range sorted {
			wg.Add(1)
			go func(i int, k repository.KeycapKit) {
				defer wg.Done()

				apiKit, err := ks.KitToAPI(ctx, k, isOwner, ownerPrefs)
				if err != nil {
					errs[i] = err
					return
				}
				mapped[i] = apiKit
			}(i, kit)
		}
		wg.Wait()

		if err := errors.Join(errs...); err != nil {
			return api.KeycapSet{}, err
		}

		kits = &mapped
	}

	out := api.KeycapSet{
		Id:           set.ID,
		Brand:        set.Brand,
		Name:         set.Name,
		Profile:      set.Profile,
		Material:     set.Material,
		Notes:        set.Notes,
		Visibility:   ownerVisibility(set.Visibility, isOwner),
		Kits:         kits,
		PrimaryKitId: validPrimaryKitID(set.PrimaryKitID, set.Kits),
		OrderStatus:  repository.AggregateOrderStatus(set.Kits),
		Available:    usage.KeycapSetAvailable(set),
	}
	if ownerPrefs.ShowPriceSingle(isOwner) {
		out.TotalCost = set.TotalCost()
	}
	out.Currency = ownerPrefs.CurrencyFor(out.TotalCost)

	return out, nil
}

// ToRepo maps a generated KeycapSetInput (already schema-validated by the
// OpenAPI request validator) to a repository.KeycapSet. It does not set
// UserID or ID - those come from the request's path/caller, not the body,
// and stay the handler's responsibility.
func (ks KeycapSet) ToRepo(in api.KeycapSetInput) repository.KeycapSet {
	return repository.KeycapSet{
		Brand:      in.Brand,
		Name:       in.Name,
		Profile:    in.Profile,
		Material:   in.Material,
		Notes:      in.Notes,
		Visibility: repository.Visibility(in.Visibility),
	}
}

// StripPrices clears the prices [KeycapSet.ToAPI] sets on out.
func (ks KeycapSet) StripPrices(out *api.KeycapSet) {
	out.TotalCost = nil
	out.Currency = nil
	if out.Kits == nil {
		return
	}
	for i := range *out.Kits {
		kit := &(*out.Kits)[i]
		if kit.Purchase == nil {
			continue
		}
		kit.Purchase.Price = nil
		kit.Purchase.Currency = nil
		if *kit.Purchase == (api.Purchase{}) {
			kit.Purchase = nil
		}
	}
}

// KitToAPI maps a repository.KeycapKit to its wire representation. Image
// is nil unless k.ImagePath is set and the caller may see it; its URL is a
// signed GET URL, absent once the image is rejected.
func (ks KeycapSet) KitToAPI(ctx context.Context, k repository.KeycapKit, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.KeycapKit, error) {
	purchase, err := ks.kitPurchaseToAPI(k.Purchase, isOwner, ownerPrefs)
	if err != nil {
		return api.KeycapKit{}, err
	}

	var image *api.KeycapKitImage
	if k.ImagePath != nil && k.ImageStatus.VisibleTo(isOwner) {
		image = &api.KeycapKitImage{}
		image.Status, image.RejectionReason = imageStatusToAPI(k.ImageStatus, k.ImageRejectionReason, isOwner)
		if k.ImageStatus.Downloadable() {
			url, err := ks.Images.SignGet(ctx, *k.ImagePath)
			if err != nil {
				return api.KeycapKit{}, fmt.Errorf("signing kit image: %w", err)
			}
			image.Url = &url
		}
	}

	return api.KeycapKit{
		KitId:    k.KitID,
		Name:     k.Name,
		Image:    image,
		Purchase: purchase,
	}, nil
}

// KitToRepo maps a generated KeycapKitInput (already schema-validated by
// the OpenAPI request validator) to a repository.KeycapKit. It does not
// set KitID - that's the handler's responsibility (server-generated on
// create).
func (ks KeycapSet) KitToRepo(in api.KeycapKitInput) repository.KeycapKit {
	return repository.KeycapKit{
		Name:     in.Name,
		Purchase: ks.kitPurchaseToRepo(in.Purchase),
	}
}

func (ks KeycapSet) kitPurchaseToAPI(p repository.KeycapKitPurchase, isOwner bool, ownerPrefs repository.ProfilePreferences) (*api.Purchase, error) {
	if p == (repository.KeycapKitPurchase{}) {
		return nil, nil //nolint:nilnil // no purchase data is a valid, expected result
	}

	out := &api.Purchase{
		Vendor:      p.Vendor,
		OrderStatus: p.OrderStatus,
	}
	if ownerPrefs.ShowPriceSingle(isOwner) {
		out.Price = p.Price
	}
	out.Currency = ownerPrefs.CurrencyFor(out.Price)
	if p.OrderDate != nil {
		d, err := parseAPIDate(*p.OrderDate)
		if err != nil {
			return nil, fmt.Errorf("parsing order_date: %w", err)
		}
		out.OrderDate = d
	}
	if p.DeliveryDate != nil {
		d, err := parseAPIDate(*p.DeliveryDate)
		if err != nil {
			return nil, fmt.Errorf("parsing delivery_date: %w", err)
		}
		out.DeliveryDate = d
	}
	if p.EstimatedDeliveryDate != nil {
		d, err := parseAPIDate(*p.EstimatedDeliveryDate)
		if err != nil {
			return nil, fmt.Errorf("parsing estimated_delivery_date: %w", err)
		}
		out.EstimatedDeliveryDate = d
	}

	return out, nil
}

func (ks KeycapSet) kitPurchaseToRepo(p *api.PurchaseInput) repository.KeycapKitPurchase {
	if p == nil {
		return repository.KeycapKitPurchase{}
	}

	out := repository.KeycapKitPurchase{
		Vendor:      p.Vendor,
		Price:       p.Price,
		OrderStatus: p.OrderStatus,
	}
	if p.OrderDate != nil {
		s := p.OrderDate.Format(dateLayout)
		out.OrderDate = &s
	}
	if p.DeliveryDate != nil {
		s := p.DeliveryDate.Format(dateLayout)
		out.DeliveryDate = &s
	}
	if p.EstimatedDeliveryDate != nil {
		s := p.EstimatedDeliveryDate.Format(dateLayout)
		out.EstimatedDeliveryDate = &s
	}

	return out
}

// validPrimaryKitID returns primaryKitID unchanged if it names a kit still
// present in kits, or nil otherwise (never set, or naming a since-deleted
// kit) - callers must not surface a dangling reference.
func validPrimaryKitID(primaryKitID *string, kits map[string]repository.KeycapKit) *string {
	if primaryKitID == nil {
		return nil
	}
	if _, ok := kits[*primaryKitID]; !ok {
		return nil
	}
	return primaryKitID
}
