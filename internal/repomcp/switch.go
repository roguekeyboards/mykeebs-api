package repomcp

import (
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// Switch maps repository.Switch to and from its MCP tool shape. It has no
// dependencies - unlike repoapi.Switch, this never signs an image URL;
// ToMCP reports only HasImage.
type Switch struct{}

// ToMCP maps a repository.Switch to its MCP tool shape. Optional fields
// pass through as pointers rather than being dereferenced, so a recorded
// zero survives the round trip instead of being indistinguishable from
// unset - the same reason [repoapi.Switch.ToAPI] keeps them. The nested
// material/force/spring/purchase groups still collapse to nil when every
// field in them is unset, so an all-empty group is omitted entirely. The
// owner always sees their own purchase.price; a non-owner sees it only if
// ownerPrefs.ShowPriceToOthers.
func (s Switch) ToMCP(sw repository.Switch, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.Switch {
	out := schema.Switch{
		ID:           sw.ID,
		Brand:        sw.Brand,
		Manufacturer: sw.Manufacturer,
		Name:         sw.Name,
		Type:         sw.Type,
		Pins:         sw.Pins,
		FactoryLubed: sw.FactoryLubed,
		Material:     s.materialToMCP(sw.Material),
		Force:        s.forceToMCP(sw.Force),
		Spring:       s.springToMCP(sw.Spring),
		Purchase:     s.purchaseToMCP(sw.Purchase, isOwner, ownerPrefs),
		Notes:        sw.Notes,
		Visibility:   ownerVisibility(sw.Visibility, isOwner),
		HasImage:     sw.ImagePath != nil && sw.ImageStatus.VisibleTo(isOwner),
	}
	out.ImageStatus, out.ImageRejectionReason = imageStatusToMCP(sw.ImagePath != nil, sw.ImageStatus, sw.ImageRejectionReason, isOwner)

	return out
}

// ToMCPSummary maps a repository.Switch to the abbreviated shape
// list_switches returns. Lifts order_status out of purchase, so a switch
// still on order is visible while browsing a list. Price is shown per
// ownerPrefs.ShowPriceToMe (owner) or ownerPrefs.ShowPriceToOthers
// (non-owner) - unlike [Switch.ToMCP], the owner isn't unconditionally
// shown price here.
func (s Switch) ToMCPSummary(sw repository.Switch, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.SwitchSummary {
	summary := schema.SwitchSummary{
		ID:          sw.ID,
		Brand:       sw.Brand,
		Name:        sw.Name,
		Type:        sw.Type,
		OrderStatus: sw.Purchase.OrderStatus,
		HasImage:    sw.ImagePath != nil && sw.ImageStatus.VisibleTo(isOwner),
	}
	if ownerPrefs.ShowPriceSummary(isOwner) {
		summary.Price = sw.Purchase.Price
	}
	summary.Currency = ownerPrefs.CurrencyFor(summary.Price)
	if isOwner {
		v := string(sw.Visibility)
		summary.Visibility = &v
	}

	return summary
}

func (s Switch) materialToMCP(m repository.SwitchMaterial) *schema.SwitchMaterial {
	if m.TopHousing == nil && m.BottomHousing == nil && m.Stem == nil {
		return nil
	}

	return &schema.SwitchMaterial{
		TopHousing:    m.TopHousing,
		BottomHousing: m.BottomHousing,
		Stem:          m.Stem,
	}
}

func (s Switch) forceToMCP(f repository.SwitchForce) *schema.SwitchForce {
	if f.Actuation == nil && f.BottomOut == nil {
		return nil
	}

	return &schema.SwitchForce{
		Actuation: f.Actuation,
		BottomOut: f.BottomOut,
	}
}

func (s Switch) springToMCP(sp repository.SwitchSpring) *schema.SwitchSpring {
	if sp.Material == nil && sp.PreTravel == nil && sp.TotalTravel == nil && sp.Length == nil {
		return nil
	}

	return &schema.SwitchSpring{
		Material:    sp.Material,
		PreTravel:   sp.PreTravel,
		TotalTravel: sp.TotalTravel,
		Length:      sp.Length,
	}
}

// Dates pass through as strings, unlike [repoapi.Switch.ToAPI], so this
// can't fail on a malformed one.
func (s Switch) purchaseToMCP(p repository.SwitchPurchase, isOwner bool, ownerPrefs repository.ProfilePreferences) *schema.SwitchPurchase {
	if p.Vendor == nil && p.Price == nil && p.OrderDate == nil &&
		p.DeliveryDate == nil && p.OrderStatus == nil && p.Quantity == nil {
		return nil
	}

	out := &schema.SwitchPurchase{
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

// FromMCP maps a create_switch/update_switch tool argument to its
// repository shape. ID and UserID are left unset: the caller sets ID
// (fresh for a create, the target's for an update), and UserID comes from
// ctx in the repository layer.
func (s Switch) FromMCP(in schema.SwitchInput) repository.Switch {
	return repository.Switch{
		Brand:        in.Brand,
		Manufacturer: in.Manufacturer,
		Name:         in.Name,
		Type:         in.Type,
		Pins:         in.Pins,
		FactoryLubed: in.FactoryLubed,
		Material:     s.materialFromMCP(in.Material),
		Force:        s.forceFromMCP(in.Force),
		Spring:       s.springFromMCP(in.Spring),
		Purchase:     s.purchaseFromMCP(in.Purchase),
		Notes:        in.Notes,
		Visibility:   repository.Visibility(in.Visibility),
	}
}

func (s Switch) materialFromMCP(m *schema.SwitchMaterial) repository.SwitchMaterial {
	if m == nil {
		return repository.SwitchMaterial{}
	}

	return repository.SwitchMaterial{
		TopHousing:    m.TopHousing,
		BottomHousing: m.BottomHousing,
		Stem:          m.Stem,
	}
}

func (s Switch) forceFromMCP(f *schema.SwitchForce) repository.SwitchForce {
	if f == nil {
		return repository.SwitchForce{}
	}

	return repository.SwitchForce{
		Actuation: f.Actuation,
		BottomOut: f.BottomOut,
	}
}

func (s Switch) springFromMCP(sp *schema.SwitchSpring) repository.SwitchSpring {
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

func (s Switch) purchaseFromMCP(p *schema.SwitchPurchaseInput) repository.SwitchPurchase {
	if p == nil {
		return repository.SwitchPurchase{}
	}

	return repository.SwitchPurchase{
		Vendor:       p.Vendor,
		Price:        p.Price,
		OrderDate:    p.OrderDate,
		DeliveryDate: p.DeliveryDate,
		OrderStatus:  p.OrderStatus,
		Quantity:     p.Quantity,
	}
}
