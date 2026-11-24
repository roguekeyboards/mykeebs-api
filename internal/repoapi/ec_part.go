package repoapi

import (
	"context"
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// ECPart maps repository.ECPart to and from its wire representations.
type ECPart struct {
	Images repository.ECPartImageStore
}

// ToAPI maps a repository.ECPart to its wire representation. The owner
// always sees their own purchases' prices; a non-owner sees them only if
// ownerPrefs.ShowPriceToOthers. Returns an error if a stored purchase date
// doesn't match dateLayout, or the image fails to sign.
func (e ECPart) ToAPI(ctx context.Context, p repository.ECPart, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.ECPart, error) {
	purchases, purchase, err := bulkPurchasesToAPI(p.Purchases, isOwner, ownerPrefs, func(p api.SwitchPurchase) api.ECPartPurchase { return api.ECPartPurchase(p) })
	if err != nil {
		return api.ECPart{}, err
	}

	var image *api.ECPartImage
	if p.ImagePath != nil && p.ImageStatus.VisibleTo(isOwner) {
		image = &api.ECPartImage{}
		image.Status, image.RejectionReason = imageStatusToAPI(p.ImageStatus, p.ImageRejectionReason, isOwner)
		if p.ImageStatus.Downloadable() {
			url, err := e.Images.SignGet(ctx, *p.ImagePath)
			if err != nil {
				return api.ECPart{}, fmt.Errorf("signing EC part image: %w", err)
			}
			image.Url = &url
		}
	}

	return api.ECPart{
		Id:            p.ID,
		Brand:         p.Brand,
		Name:          p.Name,
		PartType:      api.ECPartType(p.PartType),
		Domes:         e.domesToAPI(p.Domes),
		Sliders:       e.slidersToAPI(p.Sliders),
		Housings:      e.housingsToAPI(p.Housings),
		Springs:       e.springsToAPI(p.Springs),
		Rings:         e.ringsToAPI(p.Rings),
		Purchase:      purchase,
		Purchases:     purchases,
		TotalQuantity: p.Purchases.TotalQuantity(),
		Notes:         p.Notes,
		Visibility:    ownerVisibility(p.Visibility, isOwner),
		Image:         image,
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
		Purchases:  bulkPurchasesToRepo(in.Purchases, in.Purchase, func(p api.ECPartPurchaseInput) api.SwitchPurchaseInput { return api.SwitchPurchaseInput(p) }),
		Notes:      in.Notes,
		Visibility: repository.Visibility(in.Visibility),
	}
}

// StripPrices clears the prices [ECPart.ToAPI] sets on out.
func (e ECPart) StripPrices(out *api.ECPart) {
	out.Purchases = stripBulkPurchasePrices(out.Purchases, func(p *api.ECPartPurchase) {
		p.Price = nil
		p.Currency = nil
	})
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
