package repoapi

import (
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// ECPart maps repository.ECPart to and from its wire representations.
type ECPart struct{}

// ToAPI maps a repository.ECPart to its wire representation. The owner
// always sees their own purchase.price; a non-owner sees it only if
// ownerPrefs.ShowPriceToOthers. Returns an error if a stored Purchase date
// doesn't match dateLayout.
func (e ECPart) ToAPI(p repository.ECPart, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.ECPart, error) {
	purchase, err := e.purchaseToAPI(p.Purchase, isOwner, ownerPrefs)
	if err != nil {
		return api.ECPart{}, err
	}

	return api.ECPart{
		Id:         p.ID,
		Brand:      p.Brand,
		Name:       p.Name,
		PartType:   api.ECPartType(p.PartType),
		Domes:      e.domesToAPI(p.Domes),
		Sliders:    e.slidersToAPI(p.Sliders),
		Housings:   e.housingsToAPI(p.Housings),
		Springs:    e.springsToAPI(p.Springs),
		Rings:      e.ringsToAPI(p.Rings),
		Purchase:   purchase,
		Notes:      p.Notes,
		Visibility: ownerVisibility(p.Visibility, isOwner),
	}, nil
}

// ToRepo maps a generated ECPartInput (already schema-validated by the
// OpenAPI request validator) to a repository.ECPart. UserID and ID stay the
// handler's to set.
func (e ECPart) ToRepo(in api.ECPartInput) repository.ECPart {
	return repository.ECPart{
		Brand:      in.Brand,
		Name:       in.Name,
		PartType:   repository.ECPartType(in.PartType),
		Domes:      e.domesToRepo(in.Domes),
		Sliders:    e.slidersToRepo(in.Sliders),
		Housings:   e.housingsToRepo(in.Housings),
		Springs:    e.springsToRepo(in.Springs),
		Rings:      e.ringsToRepo(in.Rings),
		Purchase:   e.purchaseToRepo(in.Purchase),
		Notes:      in.Notes,
		Visibility: repository.Visibility(in.Visibility),
	}
}

// StripPrices clears the prices [ECPart.ToAPI] sets on out.
func (e ECPart) StripPrices(out *api.ECPart) {
	if out.Purchase == nil {
		return
	}
	out.Purchase.Price = nil
	out.Purchase.Currency = nil
	if *out.Purchase == (api.ECPartPurchase{}) {
		out.Purchase = nil
	}
}

func (e ECPart) domesToAPI(d *repository.ECDomes) *api.ECDomes {
	if d == nil {
		return nil
	}
	return &api.ECDomes{Weight: d.Weight, Material: d.Material, Color: d.Color}
}

func (e ECPart) domesToRepo(d *api.ECDomes) *repository.ECDomes {
	if d == nil {
		return nil
	}
	return &repository.ECDomes{Weight: d.Weight, Material: d.Material, Color: d.Color}
}

func (e ECPart) slidersToAPI(s *repository.ECSliders) *api.ECSliders {
	if s == nil {
		return nil
	}
	return &api.ECSliders{
		Size:     convertPtr[api.ECPartSize](s.Size),
		Stem:     convertPtr[api.ECSliderStem](s.Stem),
		Material: s.Material,
	}
}

func (e ECPart) slidersToRepo(s *api.ECSliders) *repository.ECSliders {
	if s == nil {
		return nil
	}
	return &repository.ECSliders{
		Size:     convertPtr[repository.ECPartSize](s.Size),
		Stem:     convertPtr[repository.ECSliderStem](s.Stem),
		Material: s.Material,
	}
}

func (e ECPart) housingsToAPI(h *repository.ECHousings) *api.ECHousings {
	if h == nil {
		return nil
	}
	return &api.ECHousings{Size: convertPtr[api.ECPartSize](h.Size), Material: h.Material}
}

func (e ECPart) housingsToRepo(h *api.ECHousings) *repository.ECHousings {
	if h == nil {
		return nil
	}
	return &repository.ECHousings{Size: convertPtr[repository.ECPartSize](h.Size), Material: h.Material}
}

func (e ECPart) springsToAPI(s *repository.ECSprings) *api.ECSprings {
	if s == nil {
		return nil
	}
	return &api.ECSprings{Material: s.Material}
}

func (e ECPart) springsToRepo(s *api.ECSprings) *repository.ECSprings {
	if s == nil {
		return nil
	}
	return &repository.ECSprings{Material: s.Material}
}

func (e ECPart) ringsToAPI(r *repository.ECRings) *api.ECRings {
	if r == nil {
		return nil
	}
	return &api.ECRings{Material: r.Material, Thickness: r.Thickness}
}

func (e ECPart) ringsToRepo(r *api.ECRings) *repository.ECRings {
	if r == nil {
		return nil
	}
	return &repository.ECRings{Material: r.Material, Thickness: r.Thickness}
}

func (e ECPart) purchaseToAPI(p repository.ECPartPurchase, isOwner bool, ownerPrefs repository.ProfilePreferences) (*api.ECPartPurchase, error) {
	if p == (repository.ECPartPurchase{}) {
		return nil, nil //nolint:nilnil // no purchase data is a valid, expected result
	}

	out := &api.ECPartPurchase{
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

func (e ECPart) purchaseToRepo(p *api.ECPartPurchaseInput) repository.ECPartPurchase {
	if p == nil {
		return repository.ECPartPurchase{}
	}

	out := repository.ECPartPurchase{
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
