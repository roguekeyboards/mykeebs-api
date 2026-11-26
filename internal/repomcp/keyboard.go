package repomcp

import (
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// Keyboard maps repository.Keyboard to and from its MCP tool shape. It has
// no dependencies - unlike repoapi.Keyboard, this never signs image
// URLs; ToMCP reports only HasImages.
type Keyboard struct{}

// ToMCP maps a repository.Keyboard to its MCP tool shape. Pointers pass
// through undereferenced so a recorded zero survives, as
// [repoapi.Keyboard.ToAPI] does. The owner always sees their own prices
// (purchase.price, each part's purchase.price, total_cost); a non-owner
// sees them only if ownerPrefs.ShowPriceToOthers.
func (k Keyboard) ToMCP(kb repository.Keyboard, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.Keyboard {
	out := schema.Keyboard{
		ID:         kb.ID,
		Brand:      kb.Brand,
		Name:       kb.Name,
		Size:       kb.Size,
		Layout:     kb.Layout,
		Design:     k.designToMCP(kb.Design),
		Plates:     k.platesToMCP(repository.SortedPlates(kb.Plates), isOwner, ownerPrefs),
		PCBs:       k.pcbsToMCP(repository.SortedPCBs(kb.PCBs), isOwner, ownerPrefs),
		Purchase:   k.purchaseToMCP(kb.Purchase, isOwner, ownerPrefs),
		Notes:      kb.Notes,
		Visibility: ownerVisibility(kb.Visibility, isOwner),
		HasImages:  anyVisible(kb.Images, func(e repository.KeyboardImageEntry) repository.ImageStatus { return e.Status }, isOwner),
	}
	if ownerPrefs.ShowPriceSingle(isOwner) {
		out.TotalCost = kb.TotalCost()
	}
	out.Currency = ownerPrefs.CurrencyFor(out.TotalCost)

	return out
}

// ToMCPSummary lifts order_status out of purchase, so a keyboard still on
// order is visible while browsing a list. TotalCost rather than the base
// price, so a keyboard with priced parts doesn't list low. It's shown per
// ownerPrefs.ShowPriceToMe (owner) or ownerPrefs.ShowPriceToOthers
// (non-owner) - unlike [Keyboard.ToMCP], the owner isn't unconditionally
// shown it here.
func (k Keyboard) ToMCPSummary(kb repository.Keyboard, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.KeyboardSummary {
	summary := schema.KeyboardSummary{
		ID:          kb.ID,
		Brand:       kb.Brand,
		Name:        kb.Name,
		Size:        kb.Size,
		Layout:      kb.Layout,
		OrderStatus: kb.Purchase.OrderStatus,
		HasImages:   anyVisible(kb.Images, func(e repository.KeyboardImageEntry) repository.ImageStatus { return e.Status }, isOwner),
	}
	if ownerPrefs.ShowPriceSummary(isOwner) {
		summary.TotalCost = kb.TotalCost()
	}
	summary.Currency = ownerPrefs.CurrencyFor(summary.TotalCost)
	if isOwner {
		v := string(kb.Visibility)
		summary.Visibility = &v
	}

	return summary
}

// ImagesToMCP lists the keyboard's images the caller may see, in the
// order they were added. Only the owner sees each one's status.
func (k Keyboard) ImagesToMCP(kb repository.Keyboard, isOwner bool) []schema.KeyboardImage {
	out := []schema.KeyboardImage{}
	for _, img := range repository.SortedKeyboardImages(kb.Images) {
		if !img.Status.VisibleTo(isOwner) {
			continue
		}
		entry := schema.KeyboardImage{ImageID: img.ImageID}
		entry.Status, entry.RejectionReason = imageStatusToMCP(true, img.Status, img.RejectionReason, isOwner)
		out = append(out, entry)
	}

	return out
}

func (k Keyboard) designToMCP(d repository.KeyboardDesign) *schema.KeyboardDesign {
	topCase := k.materialColorToMCP(d.TopCase)
	bottomCase := k.materialColorToMCP(d.BottomCase)
	weight := k.materialColorToMCP(d.Weight)

	if topCase == nil && bottomCase == nil && weight == nil {
		return nil
	}

	return &schema.KeyboardDesign{
		TopCase:    topCase,
		BottomCase: bottomCase,
		Weight:     weight,
	}
}

func (k Keyboard) materialColorToMCP(mc repository.KeyboardMaterialColor) *schema.KeyboardMaterialColor {
	if mc.Material == nil && mc.Color == nil {
		return nil
	}

	return &schema.KeyboardMaterialColor{
		Material: mc.Material,
		Color:    mc.Color,
	}
}

func (k Keyboard) platesToMCP(plates []repository.KeyboardPlate, isOwner bool, ownerPrefs repository.ProfilePreferences) []schema.KeyboardPlate {
	if len(plates) == 0 {
		return nil
	}

	out := make([]schema.KeyboardPlate, len(plates))
	for i, p := range plates {
		out[i] = k.PlateToMCP(p, isOwner, ownerPrefs)
	}

	return out
}

// PlateToMCP maps one plate, with the same price rules as [Keyboard.ToMCP].
func (k Keyboard) PlateToMCP(p repository.KeyboardPlate, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.KeyboardPlate {
	return schema.KeyboardPlate{
		ID:        p.ID,
		Material:  p.Material,
		Color:     p.Color,
		Thickness: p.Thickness,
		Purchase:  k.partPurchaseToMCP(p.Purchase, isOwner, ownerPrefs),
	}
}

func (k Keyboard) pcbsToMCP(pcbs []repository.KeyboardPCB, isOwner bool, ownerPrefs repository.ProfilePreferences) []schema.KeyboardPCB {
	if len(pcbs) == 0 {
		return nil
	}

	out := make([]schema.KeyboardPCB, len(pcbs))
	for i, p := range pcbs {
		out[i] = k.PCBToMCP(p, isOwner, ownerPrefs)
	}

	return out
}

// PCBToMCP maps one PCB, with the same price rules as [Keyboard.ToMCP].
func (k Keyboard) PCBToMCP(p repository.KeyboardPCB, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.KeyboardPCB {
	return schema.KeyboardPCB{
		ID:           p.ID,
		Thickness:    p.Thickness,
		Firmware:     p.Firmware,
		Assembly:     p.Assembly,
		Connectivity: p.Connectivity,
		Purchase:     k.partPurchaseToMCP(p.Purchase, isOwner, ownerPrefs),
	}
}

func (k Keyboard) partPurchaseToMCP(p repository.KeyboardPurchase, isOwner bool, ownerPrefs repository.ProfilePreferences) *schema.KeyboardPartPurchase {
	kp := k.purchaseToMCP(p, isOwner, ownerPrefs)
	if kp == nil {
		return nil
	}

	out := schema.KeyboardPartPurchase(*kp)
	return &out
}

// Dates pass through as strings, unlike [repoapi.Keyboard.ToAPI], so this
// can't fail on a malformed one.
func (k Keyboard) purchaseToMCP(p repository.KeyboardPurchase, isOwner bool, ownerPrefs repository.ProfilePreferences) *schema.KeyboardPurchase {
	if p == (repository.KeyboardPurchase{}) {
		return nil
	}

	out := &schema.KeyboardPurchase{
		Vendor:                p.Vendor,
		OrderDate:             p.OrderDate,
		DeliveryDate:          p.DeliveryDate,
		EstimatedDeliveryDate: p.EstimatedDeliveryDate,
		OrderStatus:           p.OrderStatus,
	}
	if ownerPrefs.ShowPriceSingle(isOwner) {
		out.Price = p.Price
	}
	out.Currency = ownerPrefs.CurrencyFor(out.Price)

	return out
}

// FromMCP maps a create_keyboard/update_keyboard tool argument to its
// repository shape. ID and UserID are left unset: the caller sets ID, and
// UserID comes from ctx in the repository layer. Plates and PCBs have
// their own tools, so the input carries neither.
func (k Keyboard) FromMCP(in schema.KeyboardInput) repository.Keyboard {
	return repository.Keyboard{
		Brand:      in.Brand,
		Name:       in.Name,
		Size:       in.Size,
		Layout:     in.Layout,
		Design:     k.designFromMCP(in.Design),
		Purchase:   k.purchaseFromMCP(in.Purchase),
		Notes:      in.Notes,
		Visibility: repository.Visibility(in.Visibility),
	}
}

func (k Keyboard) designFromMCP(d *schema.KeyboardDesign) repository.KeyboardDesign {
	if d == nil {
		return repository.KeyboardDesign{}
	}

	return repository.KeyboardDesign{
		TopCase:    k.materialColorFromMCP(d.TopCase),
		BottomCase: k.materialColorFromMCP(d.BottomCase),
		Weight:     k.materialColorFromMCP(d.Weight),
	}
}

func (k Keyboard) materialColorFromMCP(mc *schema.KeyboardMaterialColor) repository.KeyboardMaterialColor {
	if mc == nil {
		return repository.KeyboardMaterialColor{}
	}

	return repository.KeyboardMaterialColor{
		Material: mc.Material,
		Color:    mc.Color,
	}
}

// PlateFromMCP maps a plate tool argument. ID is left for the caller to set.
func (k Keyboard) PlateFromMCP(in schema.KeyboardPlateInput) repository.KeyboardPlate {
	return repository.KeyboardPlate{
		Material:  in.Material,
		Color:     in.Color,
		Thickness: in.Thickness,
		Purchase:  k.partPurchaseFromMCP(in.Purchase),
	}
}

// PCBFromMCP maps a PCB tool argument. ID is left for the caller to set.
func (k Keyboard) PCBFromMCP(in schema.KeyboardPCBInput) repository.KeyboardPCB {
	return repository.KeyboardPCB{
		Thickness:    in.Thickness,
		Firmware:     in.Firmware,
		Assembly:     in.Assembly,
		Connectivity: in.Connectivity,
		Purchase:     k.partPurchaseFromMCP(in.Purchase),
	}
}

func (k Keyboard) partPurchaseFromMCP(p *schema.KeyboardPartPurchaseInput) repository.KeyboardPurchase {
	if p == nil {
		return repository.KeyboardPurchase{}
	}

	return k.purchaseFromMCP((*schema.KeyboardPurchaseInput)(p))
}

func (k Keyboard) purchaseFromMCP(p *schema.KeyboardPurchaseInput) repository.KeyboardPurchase {
	if p == nil {
		return repository.KeyboardPurchase{}
	}

	return repository.KeyboardPurchase{
		Vendor:                p.Vendor,
		Price:                 p.Price,
		OrderDate:             p.OrderDate,
		DeliveryDate:          p.DeliveryDate,
		EstimatedDeliveryDate: p.EstimatedDeliveryDate,
		OrderStatus:           p.OrderStatus,
	}
}
