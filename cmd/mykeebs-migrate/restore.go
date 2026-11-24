package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
)

// Run executes the restore. A re-run resumes: it skips whatever id-map.json
// records, re-uploads a single-slot image (switch, EC part, kit) the restored
// item is missing, and writes the profile again. A crash between a create and
// its id-map save leaves one duplicate item.
func (c *RestoreCmd) Run(ctx context.Context) error {
	token, err := resolveToken(c.Token, c.Issuer)
	if err != nil {
		return err
	}
	client, err := newAPIClient(c.BaseURL, token)
	if err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(c.In, "manifest.json")); err != nil {
		return fmt.Errorf("%s does not look like a dump directory (no manifest.json): %w", c.In, err)
	}

	if err := checkImageLimits(c.In); err != nil {
		return err
	}

	m, err := loadOrNewIDMap(c.In, client.subject)
	if err != nil {
		return err
	}
	m.RestoredAt = time.Now()

	fmt.Printf("restoring into %s (subject %s)\n\n", client.baseURL, client.subject)

	if err := restoreProfile(ctx, client, c.In); err != nil {
		return err
	}
	if err := restoreKeyboards(ctx, client, c.In, m); err != nil {
		return err
	}
	if err := restoreSwitches(ctx, client, c.In, m); err != nil {
		return err
	}
	if err := restoreECParts(ctx, client, c.In, m); err != nil {
		return err
	}
	if err := restoreKeycapSets(ctx, client, c.In, m); err != nil {
		return err
	}
	if err := restoreBuilds(ctx, client, c.In, m); err != nil {
		return err
	}
	// Saved even when nothing above created an item (a dump with only a
	// profile), since verify needs it.
	if err := m.save(); err != nil {
		return err
	}

	fmt.Printf("\nrestore complete; id map written to %s\n", m.path)
	return nil
}

// progress prints a "keyboards 3/24 <id> -> <newid>" style line.
func progress(entity string, i, total int, oldID, newID string) {
	fmt.Printf("  %s %d/%d %s -> %s\n", entity, i, total, oldID, newID)
}

// skipResume prints a "keyboards 3/24 <id> (already restored, skipped)" line.
func skipResume(entity string, i, total int, oldID string) {
	fmt.Printf("  %s %d/%d %s (already restored, skipped)\n", entity, i, total, oldID)
}

// itemDirs returns the sorted immediate subdirectories of dumpDir/sub (one per
// dumped item). Missing parent means "nothing dumped for this entity".
func itemDirs(dumpDir, sub string) ([]string, error) {
	parent := filepath.Join(dumpDir, sub)
	entries, err := os.ReadDir(parent)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", parent, err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// ---- keyboards ----

func restoreKeyboards(ctx context.Context, client *apiClient, dumpDir string, m *idMap) error {
	oldIDs, err := itemDirs(dumpDir, "keyboards")
	if err != nil {
		return err
	}
	total := len(oldIDs)
	if total > 0 {
		fmt.Printf("keyboards (%d)...\n", total)
	}
	for i, oldID := range oldIDs {
		itemDir := filepath.Join(dumpDir, "keyboards", oldID)

		var full api.Keyboard
		if err := readUpgradedJSON(filepath.Join(itemDir, "item.json"), upgradeKeyboardJSON, &full); err != nil {
			return err
		}

		// Resume-aware, like keycap sets: create the keyboard only if it
		// isn't mapped yet, then (re)walk its parts and images, skipping any
		// already mapped.
		mapped := m.Keyboards[oldID]
		if mapped == nil {
			visibility, err := dumpedVisibility(full.Visibility)
			if err != nil {
				return fmt.Errorf("keyboard %s: %w", oldID, err)
			}
			input := api.KeyboardInput{
				Brand:      full.Brand,
				Design:     full.Design,
				Layout:     full.Layout,
				Name:       full.Name,
				Notes:      full.Notes,
				Purchase:   purchaseInput(full.Purchase),
				Size:       full.Size,
				Visibility: visibility,
			}
			var created api.Keyboard
			if err := client.doJSON(ctx, http.MethodPost, client.userPath("keyboards"), input, &created); err != nil {
				return fmt.Errorf("creating keyboard (was %s): %w", oldID, err)
			}
			mapped = &mappedEntity{NewID: created.Id, Images: map[string]string{}}
			m.Keyboards[oldID] = mapped
			if err := m.save(); err != nil {
				return err
			}
			progress("keyboards", i+1, total, oldID, created.Id)
		} else {
			skipResume("keyboards", i+1, total, oldID)
		}
		if mapped.Plates == nil {
			mapped.Plates = map[string]string{}
		}
		if mapped.PCBs == nil {
			mapped.PCBs = map[string]string{}
		}

		if err := restoreKeyboardParts(ctx, client, m, mapped, full); err != nil {
			return err
		}

		if err := restoreArrayImages(ctx, client, itemDir,
			client.userPath("keyboards/"+mapped.NewID+"/images"),
			client.userPath("keyboards/"+mapped.NewID),
			mapped); err != nil {
			return fmt.Errorf("keyboard %s images: %w", mapped.NewID, err)
		}
		if err := m.save(); err != nil {
			return err
		}
	}
	return nil
}

// restoreKeyboardParts adds full's plates and PCBs to the restored keyboard
// in their dumped order, so they keep it, recording each new part id.
func restoreKeyboardParts(ctx context.Context, client *apiClient, m *idMap, mapped *mappedEntity, full api.Keyboard) error {
	if full.Plates != nil {
		for _, p := range *full.Plates {
			if _, done := mapped.Plates[p.Id]; done {
				continue
			}
			in := api.KeyboardPlateInput{
				Material:  p.Material,
				Color:     p.Color,
				Thickness: p.Thickness,
				Purchase:  purchaseInput(p.Purchase),
			}
			var created api.KeyboardPlate
			if err := client.doJSON(ctx, http.MethodPost, client.userPath("keyboards/"+mapped.NewID+"/plates"), in, &created); err != nil {
				return fmt.Errorf("creating plate %q on keyboard %s: %w", p.Id, mapped.NewID, err)
			}
			mapped.Plates[p.Id] = created.Id
			if err := m.save(); err != nil {
				return err
			}
		}
	}
	if full.Pcbs != nil {
		for _, p := range *full.Pcbs {
			if _, done := mapped.PCBs[p.Id]; done {
				continue
			}
			in := api.KeyboardPCBInput{
				Thickness:    p.Thickness,
				Firmware:     p.Firmware,
				Assembly:     p.Assembly,
				Connectivity: p.Connectivity,
				Purchase:     purchaseInput(p.Purchase),
			}
			var created api.KeyboardPCB
			if err := client.doJSON(ctx, http.MethodPost, client.userPath("keyboards/"+mapped.NewID+"/pcbs"), in, &created); err != nil {
				return fmt.Errorf("creating PCB %q on keyboard %s: %w", p.Id, mapped.NewID, err)
			}
			mapped.PCBs[p.Id] = created.Id
			if err := m.save(); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---- switches ----

func restoreSwitches(ctx context.Context, client *apiClient, dumpDir string, m *idMap) error {
	oldIDs, err := itemDirs(dumpDir, "switches")
	if err != nil {
		return err
	}
	total := len(oldIDs)
	if total > 0 {
		fmt.Printf("switches (%d)...\n", total)
	}
	for i, oldID := range oldIDs {
		itemDir := filepath.Join(dumpDir, "switches", oldID)
		if mapped, done := m.Switches[oldID]; done {
			skipResume("switches", i+1, total, oldID)
			if err := finishItemImage(ctx, client, itemDir, client.userPath("switches/"+mapped.NewID)); err != nil {
				return fmt.Errorf("switch %s image: %w", mapped.NewID, err)
			}
			continue
		}

		var full api.Switch
		if err := readJSONFile(filepath.Join(itemDir, "item.json"), &full); err != nil {
			return err
		}
		visibility, err := dumpedVisibility(full.Visibility)
		if err != nil {
			return fmt.Errorf("switch %s: %w", oldID, err)
		}
		input := api.SwitchInput{
			Brand:        full.Brand,
			FactoryLubed: full.FactoryLubed,
			Force:        full.Force,
			Manufacturer: full.Manufacturer,
			Material:     full.Material,
			Name:         full.Name,
			Notes:        full.Notes,
			Pins:         full.Pins,
			Purchases:    switchPurchasesInput(full.Purchases),
			Spring:       full.Spring,
			Type:         full.Type,
			Visibility:   visibility,
		}
		var created api.Switch
		if err := client.doJSON(ctx, http.MethodPost, client.userPath("switches"), input, &created); err != nil {
			return fmt.Errorf("creating switch (was %s): %w", oldID, err)
		}
		m.Switches[oldID] = &mappedEntity{NewID: created.Id}
		if err := m.save(); err != nil {
			return err
		}
		progress("switches", i+1, total, oldID, created.Id)

		if err := restoreItemImage(ctx, client, itemDir, client.userPath("switches/"+created.Id)); err != nil {
			return fmt.Errorf("switch %s image: %w", created.Id, err)
		}
	}
	return nil
}

// ---- EC parts ----

func restoreECParts(ctx context.Context, client *apiClient, dumpDir string, m *idMap) error {
	oldIDs, err := itemDirs(dumpDir, "ec-parts")
	if err != nil {
		return err
	}
	total := len(oldIDs)
	if total > 0 {
		fmt.Printf("ec-parts (%d)...\n", total)
	}
	for i, oldID := range oldIDs {
		itemDir := filepath.Join(dumpDir, "ec-parts", oldID)
		if mapped, done := m.ECParts[oldID]; done {
			skipResume("ec-parts", i+1, total, oldID)
			if err := finishItemImage(ctx, client, itemDir, client.userPath("ec-parts/"+mapped.NewID)); err != nil {
				return fmt.Errorf("EC part %s image: %w", mapped.NewID, err)
			}
			continue
		}

		var full api.ECPart
		if err := readJSONFile(filepath.Join(itemDir, "item.json"), &full); err != nil {
			return err
		}
		visibility, err := dumpedVisibility(full.Visibility)
		if err != nil {
			return fmt.Errorf("EC part %s: %w", oldID, err)
		}
		input := api.ECPartInput{
			Brand:      full.Brand,
			Domes:      full.Domes,
			Housings:   full.Housings,
			Name:       full.Name,
			Notes:      full.Notes,
			PartType:   full.PartType,
			Purchases:  ecPartPurchasesInput(full.Purchases),
			Rings:      full.Rings,
			Sliders:    full.Sliders,
			Springs:    full.Springs,
			Visibility: visibility,
		}
		var created api.ECPart
		if err := client.doJSON(ctx, http.MethodPost, client.userPath("ec-parts"), input, &created); err != nil {
			return fmt.Errorf("creating EC part (was %s): %w", oldID, err)
		}
		m.ECParts[oldID] = &mappedEntity{NewID: created.Id}
		if err := m.save(); err != nil {
			return err
		}
		progress("ec-parts", i+1, total, oldID, created.Id)

		if err := restoreItemImage(ctx, client, itemDir, client.userPath("ec-parts/"+created.Id)); err != nil {
			return fmt.Errorf("EC part %s image: %w", created.Id, err)
		}
	}
	return nil
}

// ---- keycap sets ----

func restoreKeycapSets(ctx context.Context, client *apiClient, dumpDir string, m *idMap) error {
	oldIDs, err := itemDirs(dumpDir, "keycap-sets")
	if err != nil {
		return err
	}
	total := len(oldIDs)
	if total > 0 {
		fmt.Printf("keycap-sets (%d)...\n", total)
	}
	for i, oldSetID := range oldIDs {
		setDir := filepath.Join(dumpDir, "keycap-sets", oldSetID)

		var full api.KeycapSet
		if err := readJSONFile(filepath.Join(setDir, "item.json"), &full); err != nil {
			return err
		}

		// Resume-aware: create the set only if it isn't mapped yet, then
		// (re)walk kits, skipping any already mapped. A crash between "set
		// created" and "all kits created" must still finish the kits.
		mapped := m.KeycapSets[oldSetID]
		if mapped == nil {
			visibility, err := dumpedVisibility(full.Visibility)
			if err != nil {
				return fmt.Errorf("keycap set %s: %w", oldSetID, err)
			}
			input := api.KeycapSetInput{
				Brand:      full.Brand,
				Material:   full.Material,
				Name:       full.Name,
				Notes:      full.Notes,
				Profile:    full.Profile,
				Visibility: visibility,
			}
			var createdSet api.KeycapSet
			if err := client.doJSON(ctx, http.MethodPost, client.userPath("keycap-sets"), input, &createdSet); err != nil {
				return fmt.Errorf("creating keycap set (was %s): %w", oldSetID, err)
			}
			mapped = &mappedKeycaps{NewID: createdSet.Id, Kits: map[string]string{}}
			m.KeycapSets[oldSetID] = mapped
			if err := m.save(); err != nil {
				return err
			}
			progress("keycap-sets", i+1, total, oldSetID, createdSet.Id)
		} else {
			skipResume("keycap-sets", i+1, total, oldSetID)
		}

		var primaryKitID string
		if full.PrimaryKitId != nil {
			primaryKitID = *full.PrimaryKitId
		}
		var kits []api.KeycapKit
		if full.Kits != nil {
			kits = *full.Kits
		}
		var restoredSet []byte
		for _, kit := range kits {
			kitDir := filepath.Join(setDir, "kits", kit.KitId)
			if newKitID, done := mapped.Kits[kit.KitId]; done {
				if !hasSingleImage(kitDir, "image") {
					continue
				}
				if restoredSet == nil {
					body, err := client.getRaw(ctx, client.userPath("keycap-sets/"+mapped.NewID))
					if err != nil {
						return fmt.Errorf("re-fetching keycap set %s to check kit images: %w", mapped.NewID, err)
					}
					restoredSet = body
				}
				url, err := kitImageURL(restoredSet, newKitID)
				if err != nil {
					return err
				}
				if url != "" {
					continue
				}
				if err := restoreKitImage(ctx, client, kitDir, mapped.NewID, newKitID); err != nil {
					return fmt.Errorf("keycap set %s kit %s image: %w", mapped.NewID, newKitID, err)
				}
				continue
			}
			kitInput := api.KeycapKitInput{
				Name:     kit.Name,
				Purchase: purchaseInput(kit.Purchase),
			}
			if kit.KitId == primaryKitID && primaryKitID != "" {
				t := true
				kitInput.Primary = &t
			}
			var createdKit api.KeycapKit
			if err := client.doJSON(ctx, http.MethodPost,
				client.userPath("keycap-sets/"+mapped.NewID+"/kits"), kitInput, &createdKit); err != nil {
				return fmt.Errorf("creating kit %q in keycap set %s (was %s): %w", kit.Name, mapped.NewID, oldSetID, err)
			}
			mapped.Kits[kit.KitId] = createdKit.KitId
			if err := m.save(); err != nil {
				return err
			}
			fmt.Printf("    kit %q -> %s\n", kit.Name, createdKit.KitId)

			if err := restoreKitImage(ctx, client, kitDir, mapped.NewID, createdKit.KitId); err != nil {
				return fmt.Errorf("keycap set %s kit %s image: %w", mapped.NewID, createdKit.KitId, err)
			}
		}
	}
	return nil
}

// ---- builds ----

func restoreBuilds(ctx context.Context, client *apiClient, dumpDir string, m *idMap) error {
	oldIDs, err := itemDirs(dumpDir, "builds")
	if err != nil {
		return err
	}
	total := len(oldIDs)
	if total > 0 {
		fmt.Printf("builds (%d)...\n", total)
	}
	for i, oldID := range oldIDs {
		if _, done := m.Builds[oldID]; done {
			skipResume("builds", i+1, total, oldID)
			continue
		}
		itemDir := filepath.Join(dumpDir, "builds", oldID)

		var full api.Build
		if err := readUpgradedJSON(filepath.Join(itemDir, "item.json"), upgradeBuildJSON, &full); err != nil {
			return err
		}
		input, err := buildInputFromResolved(full, m)
		if err != nil {
			return fmt.Errorf("build %s: %w", oldID, err)
		}
		var created api.Build
		if err := client.doJSON(ctx, http.MethodPost, client.userPath("builds"), input, &created); err != nil {
			return fmt.Errorf("creating build (was %s): %w", oldID, err)
		}
		mapped := &mappedEntity{NewID: created.Id, Images: map[string]string{}}
		m.Builds[oldID] = mapped
		if err := m.save(); err != nil {
			return err
		}
		progress("builds", i+1, total, oldID, created.Id)

		if err := restoreArrayImages(ctx, client, itemDir,
			client.userPath("builds/"+created.Id+"/images"),
			client.userPath("builds/"+created.Id),
			mapped); err != nil {
			return fmt.Errorf("build %s images: %w", created.Id, err)
		}
		if err := m.save(); err != nil {
			return err
		}
	}
	return nil
}

func purchaseInput(p *api.Purchase) *api.PurchaseInput {
	if p == nil {
		return nil
	}
	return &api.PurchaseInput{
		Vendor:       p.Vendor,
		Price:        p.Price,
		OrderDate:    p.OrderDate,
		DeliveryDate: p.DeliveryDate,
		OrderStatus:  p.OrderStatus,
	}
}

func switchPurchasesInput(ps *[]api.SwitchPurchase) *[]api.SwitchPurchaseInput {
	if ps == nil {
		return nil
	}
	out := make([]api.SwitchPurchaseInput, len(*ps))
	for i, p := range *ps {
		out[i] = api.SwitchPurchaseInput{
			Vendor:       p.Vendor,
			Price:        p.Price,
			OrderDate:    p.OrderDate,
			DeliveryDate: p.DeliveryDate,
			OrderStatus:  p.OrderStatus,
			Quantity:     p.Quantity,
		}
	}
	return &out
}

func ecPartPurchasesInput(ps *[]api.ECPartPurchase) *[]api.ECPartPurchaseInput {
	if ps == nil {
		return nil
	}
	out := make([]api.ECPartPurchaseInput, len(*ps))
	for i, p := range *ps {
		out[i] = api.ECPartPurchaseInput{
			Vendor:       p.Vendor,
			Price:        p.Price,
			OrderDate:    p.OrderDate,
			DeliveryDate: p.DeliveryDate,
			OrderStatus:  p.OrderStatus,
			Quantity:     p.Quantity,
		}
	}
	return &out
}

func stabsInput(s *api.BuildStabs) *api.BuildStabsInput {
	if s == nil {
		return nil
	}
	return &api.BuildStabsInput{
		Name:      s.Name,
		MountType: s.MountType,
		Price:     s.Price,
	}
}

// dumpedVisibility unwraps a dumped item's visibility. The API returns it
// only to the item's owner, and a dump reads the caller's own collection, so
// it's missing only if the dump was taken as a different user.
func dumpedVisibility(v *api.Visibility) (api.Visibility, error) {
	if v == nil {
		return "", errors.New("dumped item has no visibility (was the dump taken as a different user?)")
	}
	return *v, nil
}

// buildInputFromResolved collapses a resolved Build GET body to a BuildInput,
// remapping every cross-entity reference through m. A reference with no
// mapping is a hard error — we never POST a build with a dangling ref.
func buildInputFromResolved(full api.Build, m *idMap) (api.BuildInput, error) {
	keyboardID := full.Keyboard.Id
	if keyboardID == "" {
		return api.BuildInput{}, errors.New("resolved build has no keyboard")
	}
	kb, ok := m.Keyboards[keyboardID]
	if !ok {
		return api.BuildInput{}, fmt.Errorf("keyboard %s is not in the id map (deleted before the dump, or not restored yet)", keyboardID)
	}
	visibility, err := dumpedVisibility(full.Visibility)
	if err != nil {
		return api.BuildInput{}, err
	}

	input := api.BuildInput{
		BuildDate:     full.BuildDate,
		BuildType:     full.BuildType,
		CaseMountType: full.CaseMountType,
		Foam:          full.Foam,
		Keyboard:      kb.NewID,
		Notes:         full.Notes,
		Stabs:         stabsInput(full.Stabs),
		Visibility:    visibility,
	}

	if full.Plate != nil {
		plate, ok := kb.Plates[full.Plate.Id]
		if !ok {
			return api.BuildInput{}, fmt.Errorf("plate %s of keyboard %s is not in the id map", full.Plate.Id, keyboardID)
		}
		input.Plate = &plate
	}
	if full.Pcb != nil {
		pcb, ok := kb.PCBs[full.Pcb.Id]
		if !ok {
			return api.BuildInput{}, fmt.Errorf("pcb %s of keyboard %s is not in the id map", full.Pcb.Id, keyboardID)
		}
		input.Pcb = &pcb
	}

	if full.Switches != nil {
		entries := make([]api.BuildSwitchEntry, 0, len(*full.Switches))
		for _, e := range *full.Switches {
			sw, ok := m.Switches[e.Switch.Id]
			if !ok {
				return api.BuildInput{}, fmt.Errorf("switch %s is not in the id map; restore switches first", e.Switch.Id)
			}
			entries = append(entries, api.BuildSwitchEntry{Switch: sw.NewID, Count: e.Count})
		}
		input.Switches = &entries
	}

	if full.EcParts != nil {
		entries := make([]api.BuildECPartEntry, 0, len(*full.EcParts))
		for _, e := range *full.EcParts {
			part, ok := m.ECParts[e.EcPart.Id]
			if !ok {
				return api.BuildInput{}, fmt.Errorf("EC part %s is not in the id map; restore EC parts first", e.EcPart.Id)
			}
			entries = append(entries, api.BuildECPartEntry{EcPart: part.NewID, Count: e.Count})
		}
		input.EcParts = &entries
	}

	if full.KeycapSets != nil {
		var entries []api.BuildKeycapKitEntry
		for _, ks := range *full.KeycapSets {
			set, ok := m.KeycapSets[ks.Id]
			if !ok {
				return api.BuildInput{}, fmt.Errorf("keycap set %s is not in the id map; restore keycap sets first", ks.Id)
			}
			for _, kit := range ks.Kits {
				newKit, ok := set.Kits[kit.KitId]
				if !ok {
					return api.BuildInput{}, fmt.Errorf("kit %s of keycap set %s is not in the id map", kit.KitId, ks.Id)
				}
				entries = append(entries, api.BuildKeycapKitEntry{KeycapSet: set.NewID, Kit: newKit})
			}
		}
		input.KeycapKits = &entries
	}

	return input, nil
}

// ---- shared image restore ----

// restoreArrayImages walks itemDir/images/images.manifest.json, re-uploading
// each image via the two-step POST-then-PUT, then re-fetching the parent to
// verify the stored bytes hash-match the dump. mapped.Images gains
// old_image_id -> new_image_id for each.
func restoreArrayImages(ctx context.Context, client *apiClient, itemDir, imagesPath, parentPath string, mapped *mappedEntity) error {
	manifestPath := filepath.Join(itemDir, "images", "images.manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return nil
	}
	var manifest []imageManifestEntry
	if err := readJSONFile(manifestPath, &manifest); err != nil {
		return err
	}

	for j, entry := range manifest {
		if _, done := mapped.Images[entry.ImageID]; done {
			continue
		}
		data, err := os.ReadFile(filepath.Join(itemDir, "images", entry.Filename)) //nolint:gosec // inside the dump dir.
		if err != nil {
			return fmt.Errorf("reading %s: %w", entry.Filename, err)
		}

		req := api.ImageUploadRequest{ContentType: entry.ContentType, SizeBytes: int64(len(data))}
		var upload api.KeyboardImageUpload // {image_id, upload_url} — same shape for builds.
		if err := client.doJSON(ctx, http.MethodPost, imagesPath, req, &upload); err != nil {
			return fmt.Errorf("allocating image slot: %w", err)
		}
		if err := uploadImage(ctx, upload.UploadUrl, entry.ContentType, data); err != nil {
			return err
		}
		mapped.Images[entry.ImageID] = upload.ImageId

		if err := verifyParentImageHash(ctx, client, parentPath, upload.ImageId, entry.SHA256); err != nil {
			return err
		}
		fmt.Printf("    image %d/%d uploaded and verified\n", j+1, len(manifest))
	}
	return nil
}

// loadSingleImageManifest reads dir/<name>.manifest.json + the image file it
// names. found is false when there is no manifest (the entity had no image).
func loadSingleImageManifest(dir, name string) (entry imageManifestEntry, data []byte, found bool, err error) {
	manifestPath := filepath.Join(dir, name+".manifest.json")
	if _, statErr := os.Stat(manifestPath); os.IsNotExist(statErr) {
		return imageManifestEntry{}, nil, false, nil
	}
	if err := readJSONFile(manifestPath, &entry); err != nil {
		return imageManifestEntry{}, nil, false, err
	}
	data, err = os.ReadFile(filepath.Join(dir, entry.Filename)) //nolint:gosec // inside the dump dir.
	if err != nil {
		return imageManifestEntry{}, nil, false, fmt.Errorf("reading %s: %w", entry.Filename, err)
	}
	return entry, data, true, nil
}

// uploadSingleSlot POSTs an ImageUploadRequest to imagePath, expecting a
// {upload_url} response (the shape switch/EC part/kit/avatar all share), then
// PUTs the bytes.
func uploadSingleSlot(ctx context.Context, client *apiClient, imagePath string, entry imageManifestEntry, data []byte) error {
	req := api.ImageUploadRequest{ContentType: entry.ContentType, SizeBytes: int64(len(data))}
	var upload api.SwitchImageUpload
	if err := client.doJSON(ctx, http.MethodPost, imagePath, req, &upload); err != nil {
		return fmt.Errorf("allocating image slot: %w", err)
	}
	return uploadImage(ctx, upload.UploadUrl, entry.ContentType, data)
}

// restoreItemImage restores itemDir/image.<ext> onto the single-slot item at
// itemPath.
func restoreItemImage(ctx context.Context, client *apiClient, itemDir, itemPath string) error {
	entry, data, found, err := loadSingleImageManifest(itemDir, "image")
	if err != nil || !found {
		return err
	}
	if err := uploadSingleSlot(ctx, client, itemPath+"/image", entry, data); err != nil {
		return err
	}
	body, err := client.getRaw(ctx, itemPath)
	if err != nil {
		return fmt.Errorf("re-fetching %s to verify image: %w", itemPath, err)
	}
	return verifyURLHash(ctx, topLevelImageURL(body), entry.SHA256)
}

// finishItemImage uploads an already-restored single-slot item's image if the
// dump has one and the item doesn't: a failure between the create and the
// upload leaves the item mapped but imageless.
func finishItemImage(ctx context.Context, client *apiClient, itemDir, itemPath string) error {
	if !hasSingleImage(itemDir, "image") {
		return nil
	}
	body, err := client.getRaw(ctx, itemPath)
	if err != nil {
		return fmt.Errorf("re-fetching %s to check its image: %w", itemPath, err)
	}
	if topLevelImageURL(body) != "" {
		return nil
	}
	return restoreItemImage(ctx, client, itemDir, itemPath)
}

// hasSingleImage reports whether dir holds a dumped single-slot image.
func hasSingleImage(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name+".manifest.json"))
	return err == nil
}

// restoreKitImage restores kitDir/image.<ext> onto one kit of a keycap set,
// verifying by locating that kit in the re-fetched set.
func restoreKitImage(ctx context.Context, client *apiClient, kitDir, setID, kitID string) error {
	entry, data, found, err := loadSingleImageManifest(kitDir, "image")
	if err != nil || !found {
		return err
	}
	if err := uploadSingleSlot(ctx, client,
		client.userPath("keycap-sets/"+setID+"/kits/"+kitID+"/image"), entry, data); err != nil {
		return err
	}
	body, err := client.getRaw(ctx, client.userPath("keycap-sets/"+setID))
	if err != nil {
		return fmt.Errorf("re-fetching keycap set %s to verify kit image: %w", setID, err)
	}
	url, err := kitImageURL(body, kitID)
	if err != nil {
		return err
	}
	return verifyURLHash(ctx, url, entry.SHA256)
}

// verifyURLHash downloads url and checks its bytes against wantSHA. An empty
// url means the image is missing after upload, which is an error.
func verifyURLHash(ctx context.Context, url, wantSHA string) error {
	if url == "" {
		return errors.New("image is not present after upload")
	}
	data, _, err := downloadImage(ctx, url)
	if err != nil {
		return fmt.Errorf("downloading restored image: %w", err)
	}
	if got := sha256Hex(data); got != wantSHA {
		return fmt.Errorf("restored image hash mismatch: want %s, got %s", wantSHA, got)
	}
	return nil
}

// topLevelImageURL reads {"image":{"url"}} from a body, "" if absent.
func topLevelImageURL(body []byte) string {
	var v struct {
		Image *struct {
			URL string `json:"url"`
		} `json:"image"`
	}
	if err := json.Unmarshal(body, &v); err != nil || v.Image == nil {
		return ""
	}
	return v.Image.URL
}

// kitImageURL finds kits[kit_id==kitID].image.url in a keycap set body.
func kitImageURL(body []byte, kitID string) (string, error) {
	var v struct {
		Kits []struct {
			KitID string `json:"kit_id"`
			Image *struct {
				URL string `json:"url"`
			} `json:"image"`
		} `json:"kits"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return "", fmt.Errorf("reading kits: %w", err)
	}
	for _, k := range v.Kits {
		if k.KitID == kitID {
			if k.Image == nil {
				return "", nil
			}
			return k.Image.URL, nil
		}
	}
	return "", fmt.Errorf("kit %s not found in re-fetched set", kitID)
}

// verifyParentImageHash re-fetches parentPath, finds imageID in its images[],
// downloads it, and checks the bytes against wantSHA.
func verifyParentImageHash(ctx context.Context, client *apiClient, parentPath, imageID, wantSHA string) error {
	body, err := client.getRaw(ctx, parentPath)
	if err != nil {
		return fmt.Errorf("re-fetching %s to verify image: %w", parentPath, err)
	}
	refs, err := arrayImageRefs(body)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if ref.imageID != imageID {
			continue
		}
		data, _, err := downloadImage(ctx, ref.url)
		if err != nil {
			return fmt.Errorf("downloading restored image %s: %w", imageID, err)
		}
		if got := sha256Hex(data); got != wantSHA {
			return fmt.Errorf("restored image %s hash mismatch: want %s, got %s", imageID, wantSHA, got)
		}
		return nil
	}
	return fmt.Errorf("restored image %s not found on %s after upload", imageID, parentPath)
}
