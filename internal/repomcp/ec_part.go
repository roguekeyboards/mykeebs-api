package repomcp

import (
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// ECPart maps repository.ECPart to and from its MCP tool shape.
type ECPart struct{}

// ToMCP maps a repository.ECPart to its MCP tool shape. The owner always
// sees their own purchase.price; a non-owner sees it only if
// ownerPrefs.ShowPriceToOthers.
func (e ECPart) ToMCP(p repository.ECPart, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.ECPart {
	return schema.ECPart{
		ID:         p.ID,
		Brand:      p.Brand,
		Name:       p.Name,
		PartType:   string(p.PartType),
		Domes:      e.domesToMCP(p.Domes),
		Sliders:    e.slidersToMCP(p.Sliders),
		Housings:   e.housingsToMCP(p.Housings),
		Springs:    e.springsToMCP(p.Springs),
		Rings:      e.ringsToMCP(p.Rings),
		Purchase:   e.purchaseToMCP(p.Purchase, isOwner, ownerPrefs),
		Notes:      p.Notes,
		Visibility: ownerVisibility(p.Visibility, isOwner),
	}
}

// ToMCPSummary maps a repository.ECPart to the abbreviated shape
// list_ec_parts returns. Price follows ownerPrefs.ShowPriceSummary, so
// unlike [ECPart.ToMCP] the owner isn't unconditionally shown it.
func (e ECPart) ToMCPSummary(p repository.ECPart, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.ECPartSummary {
	summary := schema.ECPartSummary{
		ID:          p.ID,
		Brand:       p.Brand,
		Name:        p.Name,
		PartType:    string(p.PartType),
		OrderStatus: p.Purchase.OrderStatus,
		Visibility:  ownerVisibility(p.Visibility, isOwner),
	}
	if ownerPrefs.ShowPriceSummary(isOwner) {
		summary.Price = p.Purchase.Price
	}
	summary.Currency = ownerPrefs.CurrencyFor(summary.Price)

	return summary
}

// FromMCP maps a create_ec_part/update_ec_part argument to its repository
// shape. ID and UserID are left for the caller and repository to set.
func (e ECPart) FromMCP(in schema.ECPartInput) repository.ECPart {
	return repository.ECPart{
		Brand:      in.Brand,
		Name:       in.Name,
		PartType:   repository.ECPartType(in.PartType),
		Domes:      e.domesFromMCP(in.Domes),
		Sliders:    e.slidersFromMCP(in.Sliders),
		Housings:   e.housingsFromMCP(in.Housings),
		Springs:    e.springsFromMCP(in.Springs),
		Rings:      e.ringsFromMCP(in.Rings),
		Purchase:   e.purchaseFromMCP(in.Purchase),
		Notes:      in.Notes,
		Visibility: repository.Visibility(in.Visibility),
	}
}

func (e ECPart) domesToMCP(d *repository.ECDomes) *schema.ECDomes {
	if d == nil {
		return nil
	}
	return &schema.ECDomes{Weight: d.Weight, Material: d.Material, Color: d.Color}
}

func (e ECPart) domesFromMCP(d *schema.ECDomes) *repository.ECDomes {
	if d == nil {
		return nil
	}
	return &repository.ECDomes{Weight: d.Weight, Material: d.Material, Color: d.Color}
}

func (e ECPart) slidersToMCP(s *repository.ECSliders) *schema.ECSliders {
	if s == nil {
		return nil
	}
	return &schema.ECSliders{Size: convertPtr[string](s.Size), Stem: convertPtr[string](s.Stem), Material: s.Material}
}

func (e ECPart) slidersFromMCP(s *schema.ECSliders) *repository.ECSliders {
	if s == nil {
		return nil
	}
	return &repository.ECSliders{
		Size:     convertPtr[repository.ECPartSize](s.Size),
		Stem:     convertPtr[repository.ECSliderStem](s.Stem),
		Material: s.Material,
	}
}

func (e ECPart) housingsToMCP(h *repository.ECHousings) *schema.ECHousings {
	if h == nil {
		return nil
	}
	return &schema.ECHousings{Size: convertPtr[string](h.Size), Material: h.Material}
}

func (e ECPart) housingsFromMCP(h *schema.ECHousings) *repository.ECHousings {
	if h == nil {
		return nil
	}
	return &repository.ECHousings{Size: convertPtr[repository.ECPartSize](h.Size), Material: h.Material}
}

func (e ECPart) springsToMCP(s *repository.ECSprings) *schema.ECSprings {
	if s == nil {
		return nil
	}
	return &schema.ECSprings{Material: s.Material}
}

func (e ECPart) springsFromMCP(s *schema.ECSprings) *repository.ECSprings {
	if s == nil {
		return nil
	}
	return &repository.ECSprings{Material: s.Material}
}

func (e ECPart) ringsToMCP(r *repository.ECRings) *schema.ECRings {
	if r == nil {
		return nil
	}
	return &schema.ECRings{Material: r.Material, Thickness: r.Thickness}
}

func (e ECPart) ringsFromMCP(r *schema.ECRings) *repository.ECRings {
	if r == nil {
		return nil
	}
	return &repository.ECRings{Material: r.Material, Thickness: r.Thickness}
}

func (e ECPart) purchaseToMCP(p repository.ECPartPurchase, isOwner bool, ownerPrefs repository.ProfilePreferences) *schema.ECPartPurchase {
	if p == (repository.ECPartPurchase{}) {
		return nil
	}

	out := &schema.ECPartPurchase{
		Vendor:       p.Vendor,
		OrderDate:    p.OrderDate,
		DeliveryDate: p.DeliveryDate,
		OrderStatus:  p.OrderStatus,
		Quantity:     p.Quantity,
	}
	if ownerPrefs.ShowPriceSingle(isOwner) {
		out.Price = p.Price
	}
	out.Currency = ownerPrefs.CurrencyFor(out.Price)

	return out
}

func (e ECPart) purchaseFromMCP(p *schema.ECPartPurchaseInput) repository.ECPartPurchase {
	if p == nil {
		return repository.ECPartPurchase{}
	}

	return repository.ECPartPurchase{
		Vendor:       p.Vendor,
		Price:        p.Price,
		OrderDate:    p.OrderDate,
		DeliveryDate: p.DeliveryDate,
		OrderStatus:  p.OrderStatus,
		Quantity:     p.Quantity,
	}
}
