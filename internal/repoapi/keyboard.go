package repoapi

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// Keyboard maps repository.Keyboard to and from its wire representations.
type Keyboard struct {
	Images repository.KeyboardImageStore
}

// ToAPI maps a repository.Keyboard to its wire representation. The owner
// always sees their own prices (purchase.price, each part's purchase.price,
// total_cost); a non-owner sees them only if ownerPrefs.ShowPriceToOthers.
// The rest of each purchase is unaffected. Returns an error if a stored
// purchase date doesn't match dateLayout, or an image fails to sign.
func (k Keyboard) ToAPI(ctx context.Context, kb repository.Keyboard, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.Keyboard, error) {
	purchase, err := k.purchaseToAPI(kb.Purchase, isOwner, ownerPrefs)
	if err != nil {
		return api.Keyboard{}, err
	}

	plates, err := k.platesToAPI(repository.SortedPlates(kb.Plates), isOwner, ownerPrefs)
	if err != nil {
		return api.Keyboard{}, err
	}

	pcbs, err := k.pcbsToAPI(repository.SortedPCBs(kb.PCBs), isOwner, ownerPrefs)
	if err != nil {
		return api.Keyboard{}, err
	}

	imgs, err := k.imagesToAPI(ctx, repository.SortedKeyboardImages(kb.Images), isOwner)
	if err != nil {
		return api.Keyboard{}, err
	}

	out := api.Keyboard{
		Id:         kb.ID,
		Brand:      kb.Brand,
		Name:       kb.Name,
		Size:       kb.Size,
		Layout:     kb.Layout,
		Design:     k.designToAPI(kb.Design),
		Plates:     plates,
		Pcbs:       pcbs,
		Purchase:   purchase,
		Notes:      kb.Notes,
		Visibility: ownerVisibility(kb.Visibility, isOwner),
		Images:     imgs,
	}
	if ownerPrefs.ShowPriceSingle(isOwner) {
		out.TotalCost = kb.TotalCost()
	}
	out.Currency = ownerPrefs.CurrencyFor(out.TotalCost)

	return out, nil
}

// imagesToAPI keeps the images the caller may see and resolves a signed GET
// URL for each one not rejected. images must already be ordered by Seq.
func (k Keyboard) imagesToAPI(ctx context.Context, images []repository.KeyboardImage, isOwner bool) (*[]api.KeyboardImage, error) {
	images = visibleImages(images, func(img repository.KeyboardImage) repository.ImageStatus { return img.Status }, isOwner)
	if len(images) == 0 {
		return nil, nil //nolint:nilnil // no images is a valid, expected result
	}

	out := make([]api.KeyboardImage, len(images))
	errs := make([]error, len(images))

	var wg sync.WaitGroup
	for i, img := range images {
		wg.Add(1)
		go func(i int, img repository.KeyboardImage) {
			defer wg.Done()

			out[i] = api.KeyboardImage{ImageId: img.ImageID}
			out[i].Status, out[i].RejectionReason = imageStatusToAPI(img.Status, img.RejectionReason, isOwner)
			if !img.Status.Downloadable() {
				return
			}
			url, err := k.Images.SignGetKeyboardImage(ctx, img.Path)
			if err != nil {
				errs[i] = fmt.Errorf("signing keyboard image %q: %w", img.ImageID, err)
				return
			}
			out[i].Url = &url
		}(i, img)
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return &out, nil
}

// ToRepo maps a generated KeyboardInput (already schema-validated by the
// OpenAPI request validator) to a repository.Keyboard. It does not set
// UserID or ID - those come from the request's path/caller, not the body,
// and stay the handler's responsibility. Plates and PCBs have their own
// routes, so the input carries neither.
func (k Keyboard) ToRepo(in api.KeyboardInput) repository.Keyboard {
	return repository.Keyboard{
		Brand:      in.Brand,
		Name:       in.Name,
		Size:       in.Size,
		Layout:     in.Layout,
		Design:     k.designToRepo(in.Design),
		Purchase:   k.purchaseToRepo(in.Purchase),
		Notes:      in.Notes,
		Visibility: repository.Visibility(in.Visibility),
	}
}

// StripPrices clears the prices [Keyboard.ToAPI] sets on out.
func (k Keyboard) StripPrices(out *api.Keyboard) {
	out.TotalCost = nil
	out.Currency = nil
	out.Purchase = k.stripPurchasePrice(out.Purchase)
	if out.Plates != nil {
		for i := range *out.Plates {
			(*out.Plates)[i].Purchase = k.stripPurchasePrice((*out.Plates)[i].Purchase)
		}
	}
	if out.Pcbs != nil {
		for i := range *out.Pcbs {
			(*out.Pcbs)[i].Purchase = k.stripPurchasePrice((*out.Pcbs)[i].Purchase)
		}
	}
}

// stripPurchasePrice returns p without its price, or nil if nothing else
// is left.
func (k Keyboard) stripPurchasePrice(p *api.Purchase) *api.Purchase {
	if p == nil {
		return nil
	}
	p.Price = nil
	p.Currency = nil
	if *p == (api.Purchase{}) {
		return nil
	}
	return p
}

func (k Keyboard) materialColorToAPI(m repository.KeyboardMaterialColor) *api.MaterialColor {
	if m.Material == nil && m.Color == nil {
		return nil
	}

	return &api.MaterialColor{
		Material: m.Material,
		Color:    m.Color,
	}
}

func (k Keyboard) materialColorToRepo(m *api.MaterialColor) repository.KeyboardMaterialColor {
	if m == nil {
		return repository.KeyboardMaterialColor{}
	}

	return repository.KeyboardMaterialColor{
		Material: m.Material,
		Color:    m.Color,
	}
}

func (k Keyboard) designToAPI(d repository.KeyboardDesign) *api.KeyboardDesign {
	topCase := k.materialColorToAPI(d.TopCase)
	bottomCase := k.materialColorToAPI(d.BottomCase)
	weight := k.materialColorToAPI(d.Weight)
	if topCase == nil && bottomCase == nil && weight == nil {
		return nil
	}

	return &api.KeyboardDesign{
		TopCase:    topCase,
		BottomCase: bottomCase,
		Weight:     weight,
	}
}

func (k Keyboard) designToRepo(d *api.KeyboardDesign) repository.KeyboardDesign {
	if d == nil {
		return repository.KeyboardDesign{}
	}

	return repository.KeyboardDesign{
		TopCase:    k.materialColorToRepo(d.TopCase),
		BottomCase: k.materialColorToRepo(d.BottomCase),
		Weight:     k.materialColorToRepo(d.Weight),
	}
}

func (k Keyboard) platesToAPI(plates []repository.KeyboardPlate, isOwner bool, ownerPrefs repository.ProfilePreferences) (*[]api.KeyboardPlate, error) {
	if len(plates) == 0 {
		return nil, nil //nolint:nilnil // no plates is a valid, expected result
	}

	out := make([]api.KeyboardPlate, len(plates))
	for i, p := range plates {
		plate, err := k.PlateToAPI(p, isOwner, ownerPrefs)
		if err != nil {
			return nil, err
		}
		out[i] = plate
	}

	return &out, nil
}

// PlateToAPI maps one plate, with the same price rules as [Keyboard.ToAPI].
func (k Keyboard) PlateToAPI(p repository.KeyboardPlate, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.KeyboardPlate, error) {
	purchase, err := k.purchaseToAPI(p.Purchase, isOwner, ownerPrefs)
	if err != nil {
		return api.KeyboardPlate{}, fmt.Errorf("plate %q: %w", p.ID, err)
	}

	return api.KeyboardPlate{
		Id:        p.ID,
		Material:  p.Material,
		Color:     p.Color,
		Thickness: p.Thickness,
		Purchase:  purchase,
	}, nil
}

// PlateToRepo maps a plate request body. ID is left for the handler to set.
func (k Keyboard) PlateToRepo(in api.KeyboardPlateInput) repository.KeyboardPlate {
	return repository.KeyboardPlate{
		Material:  in.Material,
		Color:     in.Color,
		Thickness: in.Thickness,
		Purchase:  k.purchaseToRepo(in.Purchase),
	}
}

func (k Keyboard) pcbsToAPI(pcbs []repository.KeyboardPCB, isOwner bool, ownerPrefs repository.ProfilePreferences) (*[]api.KeyboardPCB, error) {
	if len(pcbs) == 0 {
		return nil, nil //nolint:nilnil // no PCBs is a valid, expected result
	}

	out := make([]api.KeyboardPCB, len(pcbs))
	for i, p := range pcbs {
		pcb, err := k.PCBToAPI(p, isOwner, ownerPrefs)
		if err != nil {
			return nil, err
		}
		out[i] = pcb
	}

	return &out, nil
}

// PCBToAPI maps one PCB, with the same price rules as [Keyboard.ToAPI].
func (k Keyboard) PCBToAPI(p repository.KeyboardPCB, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.KeyboardPCB, error) {
	purchase, err := k.purchaseToAPI(p.Purchase, isOwner, ownerPrefs)
	if err != nil {
		return api.KeyboardPCB{}, fmt.Errorf("PCB %q: %w", p.ID, err)
	}

	return api.KeyboardPCB{
		Id:           p.ID,
		Thickness:    p.Thickness,
		Firmware:     p.Firmware,
		Assembly:     p.Assembly,
		Connectivity: p.Connectivity,
		Purchase:     purchase,
	}, nil
}

// PCBToRepo maps a PCB request body. ID is left for the handler to set.
func (k Keyboard) PCBToRepo(in api.KeyboardPCBInput) repository.KeyboardPCB {
	return repository.KeyboardPCB{
		Thickness:    in.Thickness,
		Firmware:     in.Firmware,
		Assembly:     in.Assembly,
		Connectivity: in.Connectivity,
		Purchase:     k.purchaseToRepo(in.Purchase),
	}
}

func (k Keyboard) purchaseToAPI(p repository.KeyboardPurchase, isOwner bool, ownerPrefs repository.ProfilePreferences) (*api.Purchase, error) {
	if p == (repository.KeyboardPurchase{}) {
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

func (k Keyboard) purchaseToRepo(p *api.PurchaseInput) repository.KeyboardPurchase {
	if p == nil {
		return repository.KeyboardPurchase{}
	}

	out := repository.KeyboardPurchase{
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
