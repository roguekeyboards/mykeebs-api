package repomcp

import (
	"context"
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// Build maps repository.Build to and from its MCP tool shape.
type Build struct {
	KeyboardRepo  repository.KeyboardRepository
	SwitchRepo    repository.SwitchRepository
	ECPartRepo    repository.ECPartRepository
	KeycapSetRepo repository.KeycapSetRepository
}

// ToMCP never signs an image URL, unlike
// [github.com/roguekeyboards/mykeebs-api/internal/repoapi.Build.ToAPI] - it reports
// only HasImages, so this can't fail on a signing error. The owner always
// sees their own Stabs.Price; a non-owner sees it only if
// ownerPrefs.ShowPriceToOthers.
func (b Build) ToMCP(build repository.Build, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.Build {
	return schema.Build{
		ID:            build.ID,
		Keyboard:      build.Keyboard,
		BuildType:     build.EffectiveBuildType(),
		Plate:         build.Plate,
		PCB:           build.PCB,
		CaseMountType: b.caseMountTypeToMCP(build.CaseMountType),
		Stabs:         b.stabsToMCP(build.Stabs, isOwner, ownerPrefs),
		Foam:          build.Foam,
		Switches:      b.switchEntriesToMCP(build.Switches),
		ECParts:       b.ecPartEntriesToMCP(build.ECParts),
		KeycapKits:    b.keycapKitEntriesToMCP(build.KeycapKits),
		BuildDate:     build.BuildDate,
		Notes:         build.Notes,
		Visibility:    ownerVisibility(build.Visibility, isOwner),
		HasImages:     anyVisible(build.Images, func(e repository.BuildImageEntry) repository.ImageStatus { return e.Status }, isOwner),
	}
}

// ImagesToMCP lists the build's images the caller may see, in the order
// they were added. Only the owner sees each one's status.
func (b Build) ImagesToMCP(build repository.Build, isOwner bool) []schema.BuildImage {
	out := []schema.BuildImage{}
	for _, img := range repository.SortedBuildImages(build.Images) {
		if !img.Status.VisibleTo(isOwner) {
			continue
		}
		entry := schema.BuildImage{ImageID: img.ImageID}
		entry.Status, entry.RejectionReason = imageStatusToMCP(true, img.Status, img.RejectionReason, isOwner)
		out = append(out, entry)
	}

	return out
}

// ToMCPResolved is [Build.ToMCP] for a build read back from storage: a plate
// or PCB its keyboard no longer has is left out, as
// [github.com/roguekeyboards/mykeebs-api/internal/repoapi.Build.ToAPI] does, so the
// result can be sent back to update_build as is.
func (b Build) ToMCPResolved(
	ctx context.Context, build repository.Build, isOwner bool, ownerPrefs repository.ProfilePreferences,
) (schema.Build, error) {
	kb, err := b.KeyboardRepo.Get(ctx, build.UserID, build.Keyboard)
	if err != nil {
		return schema.Build{}, fmt.Errorf("getting keyboard %q for build %q: %w", build.Keyboard, build.ID, err)
	}
	if kb.Plate(build.Plate) == nil {
		build.Plate = nil
	}
	if kb.PCB(build.PCB) == nil {
		build.PCB = nil
	}

	return b.ToMCP(build, isOwner, ownerPrefs), nil
}

// FromMCP leaves ID and UserID unset: the caller sets ID, and UserID comes
// from ctx in the repository layer. Images are left unset too - never
// carried in a build write, managed one at a time via their own tools.
func (b Build) FromMCP(in schema.BuildInput) repository.Build {
	return repository.Build{
		Keyboard:      in.Keyboard,
		BuildType:     in.BuildType,
		Plate:         in.Plate,
		PCB:           in.PCB,
		CaseMountType: b.caseMountTypeFromMCP(in.CaseMountType),
		Stabs:         b.stabsFromMCP(in.Stabs),
		Foam:          in.Foam,
		Switches:      b.switchEntriesFromMCP(in.Switches),
		ECParts:       b.ecPartEntriesFromMCP(in.ECParts),
		KeycapKits:    b.keycapKitEntriesFromMCP(in.KeycapKits),
		BuildDate:     in.BuildDate,
		Notes:         in.Notes,
		Visibility:    repository.Visibility(in.Visibility),
	}
}

// ToMCPSummary denormalizes the keyboard via KeyboardRepo.Get and reports
// HasImage rather than a signed URL. Unlike [Build.ToMCP], the owner
// isn't unconditionally shown price here. Switches, EC parts and keycap kits are only
// fetched when TotalCost will be shown, since cost is all this uses them
// for.
func (b Build) ToMCPSummary(
	ctx context.Context, build repository.Build, isOwner bool, ownerPrefs repository.ProfilePreferences,
) (schema.BuildSummary, error) {
	summary := schema.BuildSummary{
		ID:         build.ID,
		KeyboardID: build.Keyboard,
		BuildType:  build.EffectiveBuildType(),
		BuildDate:  build.BuildDate,
		HasImage:   anyVisible(build.Images, func(e repository.BuildImageEntry) repository.ImageStatus { return e.Status }, isOwner),
	}
	if isOwner {
		v := string(build.Visibility)
		summary.Visibility = &v
	}

	kb, err := b.KeyboardRepo.Get(ctx, build.UserID, build.Keyboard)
	if err != nil {
		return schema.BuildSummary{}, fmt.Errorf("getting keyboard %q for build %q: %w", build.Keyboard, build.ID, err)
	}
	summary.Keyboard = &schema.BuildSummaryKeyboard{Brand: kb.Brand, Name: kb.Name}

	if ownerPrefs.ShowPriceSummary(isOwner) {
		switchesCost, err := b.switchesCost(ctx, build.UserID, build.Switches)
		if err != nil {
			return schema.BuildSummary{}, err
		}

		ecPartsCost, err := b.ecPartsCost(ctx, build.UserID, build.ECParts)
		if err != nil {
			return schema.BuildSummary{}, err
		}

		keycapKitsCost, err := b.keycapKitsCost(ctx, build.UserID, build.KeycapKits)
		if err != nil {
			return schema.BuildSummary{}, err
		}

		var stabsPrice *float64
		if build.Stabs != nil {
			stabsPrice = build.Stabs.Price
		}
		summary.TotalCost = roundCents(sumKnownCosts(
			kb.Purchase.Price, kb.Plate(build.Plate).Price(), kb.PCB(build.PCB).Price(),
			switchesCost, ecPartsCost, keycapKitsCost, stabsPrice))
	}
	summary.Currency = ownerPrefs.CurrencyFor(summary.TotalCost)

	return summary, nil
}

// switchesCost mirrors
// [github.com/roguekeyboards/mykeebs-api/internal/repoapi.Build.switchEntriesResolvedToAPI]'s
// calculation: switches are bought in bulk, so
// [github.com/roguekeyboards/mykeebs-api/internal/repository.SwitchPurchase.Price]
// is the total for Quantity units, not a per-unit price. An entry without
// a Quantity is excluded rather than guessed at.
func (b Build) switchesCost(ctx context.Context, ownerID string, entries []repository.BuildSwitchEntry) (*float64, error) {
	costs := make([]*float64, 0, len(entries))

	for _, e := range entries {
		sw, err := b.SwitchRepo.Get(ctx, ownerID, e.Switch)
		if err != nil {
			return nil, fmt.Errorf("getting switch %q: %w", e.Switch, err)
		}

		if sw.Purchase.Price != nil && sw.Purchase.Quantity != nil && *sw.Purchase.Quantity != 0 {
			entryCost := *sw.Purchase.Price / float64(*sw.Purchase.Quantity) * float64(e.Count)
			costs = append(costs, &entryCost)
		}
	}

	return sumKnownCosts(costs...), nil
}

// ecPartsCost prices each entry at (Price/Quantity)*Count, leaving out
// entries where either is unknown.
func (b Build) ecPartsCost(ctx context.Context, ownerID string, entries []repository.BuildECPartEntry) (*float64, error) {
	costs := make([]*float64, 0, len(entries))

	for _, e := range entries {
		p, err := b.ECPartRepo.Get(ctx, ownerID, e.ECPart)
		if err != nil {
			return nil, fmt.Errorf("getting EC part %q: %w", e.ECPart, err)
		}

		if p.Purchase.Price != nil && p.Purchase.Quantity != nil && *p.Purchase.Quantity != 0 {
			entryCost := *p.Purchase.Price / float64(*p.Purchase.Quantity) * float64(e.Count)
			costs = append(costs, &entryCost)
		}
	}

	return sumKnownCosts(costs...), nil
}

// keycapKitsCost mirrors
// [github.com/roguekeyboards/mykeebs-api/internal/repoapi.Build.keycapSetRefsToAPI]'s
// calculation: a kit listed more than once is priced once.
func (b Build) keycapKitsCost(
	ctx context.Context, ownerID string, entries []repository.BuildKeycapKitEntry,
) (*float64, error) {
	costs := make([]*float64, 0, len(entries))
	seen := map[repository.BuildKeycapKitEntry]bool{}

	for _, e := range entries {
		if seen[e] {
			continue
		}
		seen[e] = true

		ks, err := b.KeycapSetRepo.Get(ctx, ownerID, e.KeycapSet)
		if err != nil {
			return nil, fmt.Errorf("getting keycap set %q: %w", e.KeycapSet, err)
		}

		kit := findKit(&e.Kit, ks.Kits)
		if kit == nil {
			return nil, fmt.Errorf("getting kit %q of keycap set %q: %w", e.Kit, e.KeycapSet, repository.ErrNotFound)
		}
		costs = append(costs, kit.Purchase.Price)
	}

	return sumKnownCosts(costs...), nil
}

func (b Build) caseMountTypeToMCP(cmt *repository.BuildCaseMountType) *schema.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &schema.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func (b Build) caseMountTypeFromMCP(cmt *schema.BuildCaseMountType) *repository.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &repository.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func (b Build) stabsToMCP(s *repository.BuildStabs, isOwner bool, ownerPrefs repository.ProfilePreferences) *schema.BuildStabs {
	if s == nil {
		return nil
	}

	out := &schema.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
	}
	if ownerPrefs.ShowPriceSingle(isOwner) {
		out.Price = s.Price
	}
	out.Currency = ownerPrefs.CurrencyFor(out.Price)
	if *out == (schema.BuildStabs{}) {
		return nil
	}

	return out
}

func (b Build) stabsFromMCP(s *schema.BuildStabsInput) *repository.BuildStabs {
	if s == nil {
		return nil
	}

	return &repository.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
		Price:     s.Price,
	}
}

func (b Build) switchEntriesToMCP(entries []repository.BuildSwitchEntry) []schema.BuildSwitchEntry {
	if entries == nil {
		return nil
	}

	out := make([]schema.BuildSwitchEntry, len(entries))
	for i, e := range entries {
		out[i] = schema.BuildSwitchEntry{Switch: e.Switch, Count: e.Count}
	}

	return out
}

func (b Build) switchEntriesFromMCP(entries []schema.BuildSwitchEntry) []repository.BuildSwitchEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildSwitchEntry, len(entries))
	for i, e := range entries {
		out[i] = repository.BuildSwitchEntry{Switch: e.Switch, Count: e.Count}
	}

	return out
}

func (b Build) keycapKitEntriesToMCP(entries []repository.BuildKeycapKitEntry) []schema.BuildKeycapKitEntry {
	if entries == nil {
		return nil
	}

	out := make([]schema.BuildKeycapKitEntry, len(entries))
	for i, e := range entries {
		out[i] = schema.BuildKeycapKitEntry{KeycapSet: e.KeycapSet, Kit: e.Kit}
	}

	return out
}

func (b Build) keycapKitEntriesFromMCP(entries []schema.BuildKeycapKitEntry) []repository.BuildKeycapKitEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildKeycapKitEntry, len(entries))
	for i, e := range entries {
		out[i] = repository.BuildKeycapKitEntry{KeycapSet: e.KeycapSet, Kit: e.Kit}
	}

	return out
}

func (b Build) ecPartEntriesToMCP(entries []repository.BuildECPartEntry) []schema.BuildECPartEntry {
	if entries == nil {
		return nil
	}

	out := make([]schema.BuildECPartEntry, len(entries))
	for i, e := range entries {
		out[i] = schema.BuildECPartEntry{ECPart: e.ECPart, Count: e.Count}
	}

	return out
}

func (b Build) ecPartEntriesFromMCP(entries []schema.BuildECPartEntry) []repository.BuildECPartEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildECPartEntry, len(entries))
	for i, e := range entries {
		out[i] = repository.BuildECPartEntry{ECPart: e.ECPart, Count: e.Count}
	}

	return out
}
