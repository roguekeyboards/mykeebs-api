package repoapi

import (
	"context"
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/availability"
	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// Switch maps repository.Switch to and from its wire representations.
type Switch struct {
	Images repository.SwitchImageStore
}

// ToAPI maps a repository.Switch to its wire representation. The owner
// always sees their own purchases' prices; a non-owner sees them only if
// ownerPrefs.ShowPriceToOthers. usage is what the owner's current builds
// use. Returns an error if a stored purchase date doesn't match dateLayout,
// or an image fails to sign.
func (s Switch) ToAPI(ctx context.Context, sw repository.Switch, isOwner bool, ownerPrefs repository.ProfilePreferences, usage availability.Usage) (api.Switch, error) {
	purchases, err := bulkPurchasesToAPI(sw.Purchases, isOwner, ownerPrefs, func(p api.SwitchPurchase) api.SwitchPurchase { return p })
	if err != nil {
		return api.Switch{}, err
	}

	var image *api.SwitchImage
	if sw.ImagePath != nil && sw.ImageStatus.VisibleTo(isOwner) {
		image = &api.SwitchImage{}
		image.Status, image.RejectionReason = imageStatusToAPI(sw.ImageStatus, sw.ImageRejectionReason, isOwner)
		if sw.ImageStatus.Downloadable() {
			url, err := s.Images.SignGet(ctx, *sw.ImagePath)
			if err != nil {
				return api.Switch{}, fmt.Errorf("signing switch image: %w", err)
			}
			image.Url = &url
		}
	}

	out := api.Switch{
		Id:                sw.ID,
		Brand:             sw.Brand,
		Manufacturer:      sw.Manufacturer,
		Name:              sw.Name,
		Type:              sw.Type,
		Pins:              sw.Pins,
		FactoryLubed:      sw.FactoryLubed,
		Material:          s.materialToAPI(sw.Material),
		Force:             s.forceToAPI(sw.Force),
		Spring:            s.springToAPI(sw.Spring),
		Purchases:         purchases,
		TotalQuantity:     sw.Purchases.TotalQuantity(),
		AvailableQuantity: usage.SwitchQuantity(sw),
		OrderStatus:       sw.Purchases.OrderStatus(),
		Notes:             sw.Notes,
		Visibility:        ownerVisibility(sw.Visibility, isOwner),
		Image:             image,
	}
	if ownerPrefs.ShowPriceSingle(isOwner) {
		out.TotalCost = sw.Purchases.TotalPrice()
	}
	out.Currency = ownerPrefs.CurrencyFor(out.TotalCost)

	return out, nil
}

// ToRepo maps a generated SwitchInput (already schema-validated by the
// OpenAPI request validator) to a repository.Switch. It does not set UserID
// or ID - those come from the request's path/caller, not the body, and stay
// the handler's responsibility.
func (s Switch) ToRepo(in api.SwitchInput) repository.Switch {
	return repository.Switch{
		Brand:        in.Brand,
		Manufacturer: in.Manufacturer,
		Name:         in.Name,
		Type:         in.Type,
		Pins:         in.Pins,
		FactoryLubed: in.FactoryLubed,
		Material:     s.materialToRepo(in.Material),
		Force:        s.forceToRepo(in.Force),
		Spring:       s.springToRepo(in.Spring),
		Purchases:    bulkPurchasesToRepo(in.Purchases, func(p api.SwitchPurchaseInput) api.SwitchPurchaseInput { return p }),
		Notes:        in.Notes,
		Visibility:   repository.Visibility(in.Visibility),
	}
}

// StripPrices clears the prices [Switch.ToAPI] sets on out.
func (s Switch) StripPrices(out *api.Switch) {
	out.TotalCost = nil
	out.Currency = nil
	out.Purchases = stripBulkPurchasePrices(out.Purchases, func(p *api.SwitchPurchase) {
		p.Price = nil
		p.Currency = nil
	})
}

func (s Switch) materialToAPI(m repository.SwitchMaterial) *api.SwitchMaterial {
	if m.TopHousing == nil && m.BottomHousing == nil && m.Stem == nil {
		return nil
	}

	return &api.SwitchMaterial{
		TopHousing:    m.TopHousing,
		BottomHousing: m.BottomHousing,
		Stem:          m.Stem,
	}
}

func (s Switch) materialToRepo(m *api.SwitchMaterial) repository.SwitchMaterial {
	if m == nil {
		return repository.SwitchMaterial{}
	}

	return repository.SwitchMaterial{
		TopHousing:    m.TopHousing,
		BottomHousing: m.BottomHousing,
		Stem:          m.Stem,
	}
}

func (s Switch) forceToAPI(f repository.SwitchForce) *api.SwitchForce {
	if f.Actuation == nil && f.BottomOut == nil {
		return nil
	}

	return &api.SwitchForce{
		Actuation: f.Actuation,
		BottomOut: f.BottomOut,
	}
}

func (s Switch) forceToRepo(f *api.SwitchForce) repository.SwitchForce {
	if f == nil {
		return repository.SwitchForce{}
	}

	return repository.SwitchForce{
		Actuation: f.Actuation,
		BottomOut: f.BottomOut,
	}
}

func (s Switch) springToAPI(sp repository.SwitchSpring) *api.SwitchSpring {
	if sp.Material == nil && sp.PreTravel == nil && sp.TotalTravel == nil && sp.Length == nil {
		return nil
	}

	return &api.SwitchSpring{
		Material:    sp.Material,
		PreTravel:   sp.PreTravel,
		TotalTravel: sp.TotalTravel,
		Length:      sp.Length,
	}
}

func (s Switch) springToRepo(sp *api.SwitchSpring) repository.SwitchSpring {
	if sp == nil {
		return repository.SwitchSpring{}
	}

	return repository.SwitchSpring{
		Material:    sp.Material,
		PreTravel:   sp.PreTravel,
		TotalTravel: sp.TotalTravel,
		Length:      sp.Length,
	}
}
