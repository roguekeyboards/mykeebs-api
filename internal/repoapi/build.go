package repoapi

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// Build maps repository.Build to and from its wire representations,
// resolving the Keyboard/Switch/KeycapSet references it carries.
type Build struct {
	Images         repository.BuildImageStore
	KitImages      repository.KeycapKitImageStore
	KeyboardImages repository.KeyboardImageStore
	SwitchImages   repository.SwitchImageStore
	ECPartImages   repository.ECPartImageStore
	KeyboardRepo   repository.KeyboardRepository
	SwitchRepo     repository.SwitchRepository
	ECPartRepo     repository.ECPartRepository
	KeycapSetRepo  repository.KeycapSetRepository
}

// ToAPI maps a repository.Build to its wire representation, resolving the
// Keyboard/Switch/KeycapSet references it carries into denormalized
// objects so a client can render the build without follow-up requests. A
// reference that can't be resolved, any other repository error, b.BuildDate
// not matching dateLayout, or an image failing to sign fails it.
//
// The owner always sees their own Stabs.Price and TotalCost; a non-owner
// sees them only if ownerPrefs.ShowPriceToOthers. A build's Keyboard,
// Switches, and KeycapKits all belong to the same owner as the build
// itself, so ownerPrefs (looked up once for the build's owner) governs
// price visibility across the whole composite.
func (b Build) ToAPI(ctx context.Context, build repository.Build, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.Build, error) {
	showPrice := ownerPrefs.ShowPriceSingle(isOwner)
	buildDate, err := b.dateToAPI(build.BuildDate)
	if err != nil {
		return api.Build{}, err
	}

	imgs, err := b.imagesToAPI(ctx, repository.SortedBuildImages(build.Images), isOwner)
	if err != nil {
		return api.Build{}, err
	}

	keyboardRef, kb, err := b.keyboardRefToAPI(ctx, build.UserID, build.Keyboard, isOwner)
	if err != nil {
		return api.Build{}, err
	}
	plate := kb.Plate(build.Plate)
	pcb := kb.PCB(build.PCB)

	switches, switchesCost, err := b.switchEntriesResolvedToAPI(ctx, build.UserID, build.Switches, isOwner)
	if err != nil {
		return api.Build{}, err
	}

	ecParts, ecPartsCost, err := b.ecPartEntriesResolvedToAPI(ctx, build.UserID, build.ECParts, isOwner)
	if err != nil {
		return api.Build{}, err
	}

	keycapSets, keycapKitsCost, err := b.keycapSetRefsToAPI(ctx, build.UserID, build.KeycapKits, isOwner)
	if err != nil {
		return api.Build{}, err
	}

	out := api.Build{
		Id:            build.ID,
		Keyboard:      keyboardRef,
		BuildType:     build.EffectiveBuildType(),
		Plate:         b.plateRefToAPI(plate),
		Pcb:           b.pcbRefToAPI(pcb),
		CaseMountType: b.caseMountTypeToAPI(build.CaseMountType),
		Stabs:         b.stabsToAPI(build.Stabs, isOwner, ownerPrefs),
		Foam:          build.Foam,
		Switches:      switches,
		EcParts:       ecParts,
		KeycapSets:    keycapSets,
		BuildDate:     buildDate,
		Notes:         build.Notes,
		Visibility:    ownerVisibility(build.Visibility, isOwner),
		Images:        imgs,
	}

	if showPrice {
		var stabsPrice *float64
		if build.Stabs != nil {
			stabsPrice = build.Stabs.Price
		}
		out.TotalCost = roundCents(sumKnownCosts(
			kb.Purchase.Price, plate.Price(), pcb.Price(), switchesCost, ecPartsCost, keycapKitsCost, stabsPrice))
	}
	out.Currency = ownerPrefs.CurrencyFor(out.TotalCost)

	return out, nil
}

// ToRepo maps a generated BuildInput (already schema-validated by the
// OpenAPI request validator) to a repository.Build. It does not set UserID
// or ID - those come from the request's path/caller, not the body, and stay
// the handler's responsibility. Images is never set here - a build's images
// are managed entirely through the dedicated image endpoints, never carried
// in a build write.
func (b Build) ToRepo(in api.BuildInput) repository.Build {
	return repository.Build{
		Keyboard:      in.Keyboard,
		BuildType:     in.BuildType,
		Plate:         in.Plate,
		PCB:           in.Pcb,
		CaseMountType: b.caseMountTypeToRepo(in.CaseMountType),
		Stabs:         b.stabsToRepo(in.Stabs),
		Foam:          in.Foam,
		Switches:      b.switchEntriesToRepo(in.Switches),
		ECParts:       b.ecPartEntriesToRepo(in.EcParts),
		KeycapKits:    b.keycapKitEntriesToRepo(in.KeycapKits),
		BuildDate:     b.dateToRepo(in.BuildDate),
		Notes:         in.Notes,
		Visibility:    repository.Visibility(in.Visibility),
	}
}

// StripPrices clears the prices [Build.ToAPI] sets on out.
func (b Build) StripPrices(out *api.Build) {
	out.TotalCost = nil
	out.Currency = nil
	if out.Stabs == nil {
		return
	}
	out.Stabs.Price = nil
	out.Stabs.Currency = nil
	if *out.Stabs == (api.BuildStabs{}) {
		out.Stabs = nil
	}
}

func (b Build) dateToAPI(s *string) (*openapi_types.Date, error) {
	if s == nil {
		return nil, nil //nolint:nilnil // no build date is a valid, expected result
	}

	d, err := parseAPIDate(*s)
	if err != nil {
		return nil, fmt.Errorf("parsing build_date: %w", err)
	}

	return d, nil
}

func (b Build) dateToRepo(d *openapi_types.Date) *string {
	if d == nil {
		return nil
	}

	s := d.Format(dateLayout)
	return &s
}

func (b Build) caseMountTypeToAPI(cmt *repository.BuildCaseMountType) *api.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &api.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func (b Build) caseMountTypeToRepo(cmt *api.BuildCaseMountType) *repository.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &repository.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func (b Build) stabsToAPI(s *repository.BuildStabs, isOwner bool, ownerPrefs repository.ProfilePreferences) *api.BuildStabs {
	if s == nil {
		return nil
	}

	out := &api.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
	}
	if ownerPrefs.ShowPriceSingle(isOwner) {
		out.Price = s.Price
	}
	out.Currency = ownerPrefs.CurrencyFor(out.Price)
	if *out == (api.BuildStabs{}) {
		return nil
	}

	return out
}

func (b Build) stabsToRepo(s *api.BuildStabsInput) *repository.BuildStabs {
	if s == nil {
		return nil
	}

	return &repository.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
		Price:     s.Price,
	}
}

func (b Build) switchEntriesToRepo(entries *[]api.BuildSwitchEntry) []repository.BuildSwitchEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildSwitchEntry, len(*entries))
	for i, e := range *entries {
		out[i] = repository.BuildSwitchEntry{Switch: e.Switch, Count: e.Count}
	}

	return out
}

func (b Build) ecPartEntriesToRepo(entries *[]api.BuildECPartEntry) []repository.BuildECPartEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildECPartEntry, len(*entries))
	for i, e := range *entries {
		out[i] = repository.BuildECPartEntry{ECPart: e.EcPart, Count: e.Count}
	}

	return out
}

func (b Build) keycapKitEntriesToRepo(entries *[]api.BuildKeycapKitEntry) []repository.BuildKeycapKitEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildKeycapKitEntry, len(*entries))
	for i, e := range *entries {
		out[i] = repository.BuildKeycapKitEntry{KeycapSet: e.KeycapSet, Kit: e.Kit}
	}

	return out
}

// keyboardRefToAPI resolves keyboardID into a denormalized reference plus
// the keyboard itself. The ref carries only the first of the keyboard's
// images the caller may see and download (see [servedInRef]).
func (b Build) keyboardRefToAPI(
	ctx context.Context, ownerID, keyboardID string, isOwner bool,
) (api.BuildKeyboardRef, *repository.Keyboard, error) {
	kb, err := b.KeyboardRepo.Get(ctx, ownerID, keyboardID)
	if err != nil {
		return api.BuildKeyboardRef{}, nil, fmt.Errorf("getting keyboard %q: %w", keyboardID, err)
	}

	ref := api.BuildKeyboardRef{
		Id:     kb.ID,
		Brand:  kb.Brand,
		Name:   kb.Name,
		Size:   kb.Size,
		Layout: kb.Layout,
	}

	imgs := repository.SortedKeyboardImages(kb.Images)
	served := func(img repository.KeyboardImage) bool { return servedInRef(img.Status, isOwner) }
	if i := slices.IndexFunc(imgs, served); i >= 0 {
		url, err := b.KeyboardImages.SignGetKeyboardImage(ctx, imgs[i].Path)
		if err != nil {
			return api.BuildKeyboardRef{}, nil, fmt.Errorf("signing keyboard image for keyboard %q: %w", keyboardID, err)
		}
		ref.ImageUrl = &url
	}

	return ref, kb, nil
}

func (b Build) plateRefToAPI(p *repository.KeyboardPlate) *api.BuildPlateRef {
	if p == nil {
		return nil
	}

	return &api.BuildPlateRef{
		Id:        p.ID,
		Material:  p.Material,
		Color:     p.Color,
		Thickness: p.Thickness,
	}
}

func (b Build) pcbRefToAPI(p *repository.KeyboardPCB) *api.BuildPCBRef {
	if p == nil {
		return nil
	}

	return &api.BuildPCBRef{
		Id:           p.ID,
		Thickness:    p.Thickness,
		Firmware:     p.Firmware,
		Assembly:     p.Assembly,
		Connectivity: p.Connectivity,
	}
}

// switchEntriesResolvedToAPI resolves each entry's Switch id into a
// denormalized reference via a per-entry SwitchRepo.Get call, run
// concurrently across entries since each only touches its own out[i]. Also
// returns
// the summed cost across entries with a known per-unit price: switches are
// bought in bulk (SwitchPurchase.Price is the total for Quantity units, not
// a per-unit price - see SwitchPurchase's doc), so an entry contributes
// (Price/Quantity)*Count only when Quantity is set and non-zero; otherwise
// its cost is unknown and excluded rather than guessed at.
func (b Build) switchEntriesResolvedToAPI(
	ctx context.Context, ownerID string, entries []repository.BuildSwitchEntry, isOwner bool,
) (*[]api.BuildSwitchEntryResolved, *float64, error) {
	if entries == nil {
		return nil, nil, nil //nolint:nilnil // no switches is a valid, expected result
	}

	out := make([]api.BuildSwitchEntryResolved, len(entries))
	costs := make([]*float64, len(entries))
	errs := make([]error, len(entries))

	var wg sync.WaitGroup
	for i, e := range entries {
		wg.Add(1)
		go func(i int, e repository.BuildSwitchEntry) {
			defer wg.Done()

			sw, err := b.SwitchRepo.Get(ctx, ownerID, e.Switch)
			if err != nil {
				errs[i] = fmt.Errorf("getting switch %q: %w", e.Switch, err)
				return
			}

			out[i] = api.BuildSwitchEntryResolved{
				Count: e.Count,
				Switch: api.BuildSwitchRef{
					Id:           sw.ID,
					Brand:        sw.Brand,
					Manufacturer: sw.Manufacturer,
					Name:         sw.Name,
					Type:         sw.Type,
				},
			}

			if sw.ImagePath != nil && servedInRef(sw.ImageStatus, isOwner) {
				url, err := b.SwitchImages.SignGet(ctx, *sw.ImagePath)
				if err != nil {
					errs[i] = fmt.Errorf("signing switch image for switch %q: %w", e.Switch, err)
					return
				}
				out[i].Switch.ImageUrl = &url
			}

			if sw.Purchase.Price != nil && sw.Purchase.Quantity != nil && *sw.Purchase.Quantity != 0 {
				unitPrice := *sw.Purchase.Price / float64(*sw.Purchase.Quantity)
				entryCost := unitPrice * float64(e.Count)
				costs[i] = &entryCost
			}
		}(i, e)
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, nil, err
	}

	return &out, sumKnownCosts(costs...), nil
}

// keycapSetRefsToAPI groups entries by keycap set, in the order each set
// first appears, and resolves each set and its kits into a denormalized
// reference via one KeycapSetRepo.Get per set, run concurrently across
// sets. A kit listed more than once appears, and is priced, once. Also
// returns the summed price across kits with a known price.
func (b Build) keycapSetRefsToAPI(
	ctx context.Context, ownerID string, entries []repository.BuildKeycapKitEntry, isOwner bool,
) (*[]api.BuildKeycapSetRef, *float64, error) {
	if entries == nil {
		return nil, nil, nil //nolint:nilnil // no keycap kits is a valid, expected result
	}

	var setIDs []string
	kitIDs := map[string][]string{}
	for _, e := range entries {
		if _, ok := kitIDs[e.KeycapSet]; !ok {
			setIDs = append(setIDs, e.KeycapSet)
		}
		if !slices.Contains(kitIDs[e.KeycapSet], e.Kit) {
			kitIDs[e.KeycapSet] = append(kitIDs[e.KeycapSet], e.Kit)
		}
	}

	out := make([]api.BuildKeycapSetRef, len(setIDs))
	costs := make([][]*float64, len(setIDs))
	errs := make([]error, len(setIDs))

	var wg sync.WaitGroup
	for i, setID := range setIDs {
		wg.Add(1)
		go func(i int, setID string) {
			defer wg.Done()
			out[i], costs[i], errs[i] = b.keycapSetRefToAPI(ctx, ownerID, setID, kitIDs[setID], isOwner)
		}(i, setID)
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, nil, err
	}

	return &out, sumKnownCosts(slices.Concat(costs...)...), nil
}

// keycapSetRefToAPI resolves setID and the given kits within it, returning
// each kit's price alongside.
func (b Build) keycapSetRefToAPI(
	ctx context.Context, ownerID, setID string, kitIDs []string, isOwner bool,
) (api.BuildKeycapSetRef, []*float64, error) {
	ks, err := b.KeycapSetRepo.Get(ctx, ownerID, setID)
	if err != nil {
		return api.BuildKeycapSetRef{}, nil, fmt.Errorf("getting keycap set %q: %w", setID, err)
	}

	ref := api.BuildKeycapSetRef{
		Id:      ks.ID,
		Brand:   ks.Brand,
		Name:    ks.Name,
		Profile: ks.Profile,
		Kits:    make([]api.BuildKeycapKitRef, len(kitIDs)),
	}
	costs := make([]*float64, len(kitIDs))

	for i, kitID := range kitIDs {
		kit, ok := ks.Kits[kitID]
		if !ok {
			return api.BuildKeycapSetRef{}, nil, fmt.Errorf("getting kit %q of keycap set %q: %w", kitID, setID, repository.ErrNotFound)
		}

		ref.Kits[i] = api.BuildKeycapKitRef{KitId: kit.KitID, Name: kit.Name}
		costs[i] = kit.Purchase.Price

		if kit.ImagePath != nil && servedInRef(kit.ImageStatus, isOwner) {
			url, err := b.KitImages.SignGet(ctx, *kit.ImagePath)
			if err != nil {
				return api.BuildKeycapSetRef{}, nil, fmt.Errorf("signing kit image for kit %q: %w", kitID, err)
			}
			ref.Kits[i].ImageUrl = &url
		}
	}

	return ref, costs, nil
}

// imagesToAPI keeps the images the caller may see and resolves a signed GET
// URL for each one not rejected. images must already be ordered by Seq.
func (b Build) imagesToAPI(ctx context.Context, images []repository.BuildImage, isOwner bool) (*[]api.BuildImage, error) {
	images = visibleImages(images, func(img repository.BuildImage) repository.ImageStatus { return img.Status }, isOwner)
	if len(images) == 0 {
		return nil, nil //nolint:nilnil // no images is a valid, expected result
	}

	out := make([]api.BuildImage, len(images))
	errs := make([]error, len(images))

	var wg sync.WaitGroup
	for i, img := range images {
		wg.Add(1)
		go func(i int, img repository.BuildImage) {
			defer wg.Done()

			out[i] = api.BuildImage{ImageId: img.ImageID}
			out[i].Status, out[i].RejectionReason = imageStatusToAPI(img.Status, img.RejectionReason, isOwner)
			if !img.Status.Downloadable() {
				return
			}
			url, err := b.Images.SignGetBuildImage(ctx, img.Path)
			if err != nil {
				errs[i] = fmt.Errorf("signing build image %q: %w", img.ImageID, err)
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

// servedInRef reports whether a referenced item's image is shown in a
// build's reference to it. A reference carries only a URL, with no
// status, so an image there must be one the caller may both see and
// download.
func servedInRef(status repository.ImageStatus, isOwner bool) bool {
	return status.VisibleTo(isOwner) && status.Downloadable()
}

// ecPartEntriesResolvedToAPI prices each entry at (Price/Quantity)*Count,
// leaving out entries where either is unknown.
func (b Build) ecPartEntriesResolvedToAPI(
	ctx context.Context, ownerID string, entries []repository.BuildECPartEntry, isOwner bool,
) (*[]api.BuildECPartEntryResolved, *float64, error) {
	if entries == nil {
		return nil, nil, nil //nolint:nilnil // no EC parts is a valid, expected result
	}

	out := make([]api.BuildECPartEntryResolved, len(entries))
	costs := make([]*float64, len(entries))
	errs := make([]error, len(entries))

	var wg sync.WaitGroup
	for i, e := range entries {
		wg.Go(func() {
			p, err := b.ECPartRepo.Get(ctx, ownerID, e.ECPart)
			if err != nil {
				errs[i] = fmt.Errorf("getting EC part %q: %w", e.ECPart, err)
				return
			}

			out[i] = api.BuildECPartEntryResolved{
				Count: e.Count,
				EcPart: api.BuildECPartRef{
					Id:       p.ID,
					Brand:    p.Brand,
					Name:     p.Name,
					PartType: api.ECPartType(p.PartType),
				},
			}

			if p.ImagePath != nil && servedInRef(p.ImageStatus, isOwner) {
				url, err := b.ECPartImages.SignGet(ctx, *p.ImagePath)
				if err != nil {
					errs[i] = fmt.Errorf("signing EC part image for EC part %q: %w", e.ECPart, err)
					return
				}
				out[i].EcPart.ImageUrl = &url
			}

			if p.Purchase.Price != nil && p.Purchase.Quantity != nil && *p.Purchase.Quantity != 0 {
				entryCost := *p.Purchase.Price / float64(*p.Purchase.Quantity) * float64(e.Count)
				costs[i] = &entryCost
			}
		})
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, nil, err
	}

	return &out, sumKnownCosts(costs...), nil
}
