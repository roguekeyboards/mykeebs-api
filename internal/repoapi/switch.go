package repoapi

import (
	"context"
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// Switch maps repository.Switch to and from its wire representations.
type Switch struct {
	Images repository.SwitchImageStore
}

// ToAPI maps a repository.Switch to its wire representation. The owner
// always sees their own purchase.price; a non-owner sees it only if
// ownerPrefs.ShowPriceToOthers. Returns an error if a stored Purchase date
// doesn't match dateLayout, or an image fails to sign.
func (s Switch) ToAPI(ctx context.Context, sw repository.Switch, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.Switch, error) {
	purchase, err := s.purchaseToAPI(sw.Purchase, isOwner, ownerPrefs)
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

	return api.Switch{
		Id:           sw.ID,
		Brand:        sw.Brand,
		Manufacturer: sw.Manufacturer,
		Name:         sw.Name,
		Type:         sw.Type,
		Pins:         sw.Pins,
		FactoryLubed: sw.FactoryLubed,
		Material:     s.materialToAPI(sw.Material),
		Force:        s.forceToAPI(sw.Force),
		Spring:       s.springToAPI(sw.Spring),
		Purchase:     purchase,
		Notes:        sw.Notes,
		Visibility:   ownerVisibility(sw.Visibility, isOwner),
		Image:        image,
	}, nil
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
		Purchase:     s.purchaseToRepo(in.Purchase),
		Notes:        in.Notes,
		Visibility:   repository.Visibility(in.Visibility),
	}
}

// StripPrices clears the prices [Switch.ToAPI] sets on out.
func (s Switch) StripPrices(out *api.Switch) {
	if out.Purchase == nil {
		return
	}
	out.Purchase.Price = nil
	out.Purchase.Currency = nil
	if *out.Purchase == (api.SwitchPurchase{}) {
		out.Purchase = nil
	}
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

func (s Switch) purchaseToAPI(p repository.SwitchPurchase, isOwner bool, ownerPrefs repository.ProfilePreferences) (*api.SwitchPurchase, error) {
	if p.Vendor == nil && p.Price == nil && p.OrderDate == nil && p.DeliveryDate == nil &&
		p.OrderStatus == nil && p.Quantity == nil {
		return nil, nil //nolint:nilnil // no purchase data is a valid, expected result
	}

	out := &api.SwitchPurchase{
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

	return out, nil
}

func (s Switch) purchaseToRepo(p *api.SwitchPurchaseInput) repository.SwitchPurchase {
	if p == nil {
		return repository.SwitchPurchase{}
	}

	out := repository.SwitchPurchase{
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
