package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// verifyStatus is the per-item outcome.
type verifyStatus string

const (
	statusOK                 verifyStatus = "OK"
	statusMissing            verifyStatus = "MISSING"
	statusFieldMismatch      verifyStatus = "FIELD_MISMATCH"
	statusImageCountMismatch verifyStatus = "IMAGE_COUNT_MISMATCH"
	statusImageHashMismatch  verifyStatus = "IMAGE_HASH_MISMATCH"
)

// verifyResult is one line of the report.
type verifyResult struct {
	entity string
	oldID  string
	newID  string
	status verifyStatus
	detail string
}

// Run executes verify and prints a per-item report, exiting non-zero if
// anything is not OK.
func (c *VerifyCmd) Run(ctx context.Context) error {
	token, err := resolveToken(c.Token, c.Issuer)
	if err != nil {
		return err
	}
	client, err := newAPIClient(c.BaseURL, token)
	if err != nil {
		return err
	}

	var m idMap
	if err := readJSONFile(filepath.Join(c.In, "id-map.json"), &m); err != nil {
		return fmt.Errorf("verify needs the id-map.json a restore wrote: %w", err)
	}
	m.ensureMaps()

	fmt.Printf("verifying %s (subject %s) against %s\n\n", client.baseURL, client.subject, c.In)

	var results []verifyResult
	for _, step := range []struct {
		name string
		run  func() []verifyResult
	}{
		{"profile", func() []verifyResult { return verifyProfile(ctx, client, c.In) }},
		{"keyboards", func() []verifyResult { return verifyKeyboards(ctx, client, c.In, &m) }},
		{"switches", func() []verifyResult {
			return verifySingleImageItems(ctx, client, c.In, "switches", "switch", m.Switches)
		}},
		{"ec-parts", func() []verifyResult {
			return verifySingleImageItems(ctx, client, c.In, "ec-parts", "ec_part", m.ECParts)
		}},
		{"keycap-sets", func() []verifyResult { return verifyKeycapSets(ctx, client, c.In, &m) }},
		{"builds", func() []verifyResult { return verifyBuilds(ctx, client, c.In, &m) }},
	} {
		fmt.Printf("checking %s...\n", step.name)
		got := step.run()
		for _, r := range got {
			line := fmt.Sprintf("  %-20s %-10s %s -> %s", r.status, r.entity, r.oldID, r.newID)
			if r.detail != "" {
				line += "  (" + r.detail + ")"
			}
			fmt.Println(line)
		}
		results = append(results, got...)
	}

	bad := 0
	for _, r := range results {
		if r.status != statusOK {
			bad++
		}
	}
	fmt.Printf("\n%d items checked, %d not OK\n", len(results), bad)
	if bad > 0 {
		return fmt.Errorf("%d item(s) failed verification", bad)
	}
	return nil
}

// jsonScalars strips server-owned / derived keys and reduces the rest to a
// comparable map, so verify compares what a user actually supplied. Nested
// objects are kept; arrays of images and resolved refs are dropped.
func jsonScalars(raw []byte, dropKeys ...string) (map[string]any, error) {
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("parsing item for comparison: %w", err)
	}
	for _, k := range append([]string{"id", "user_id"}, dropKeys...) {
		delete(v, k)
	}
	dropCurrency(v)
	return v, nil
}

// dropCurrency removes every currency key: it comes from the owner's
// profile, which is checked on its own and which a dump may not have.
func dropCurrency(v any) {
	switch t := v.(type) {
	case map[string]any:
		delete(t, "currency")
		for _, child := range t {
			dropCurrency(child)
		}
	case []any:
		for _, child := range t {
			dropCurrency(child)
		}
	}
}

func compareScalars(dumpRaw, liveRaw []byte, dropKeys ...string) (bool, string) {
	a, err := jsonScalars(dumpRaw, dropKeys...)
	if err != nil {
		return false, err.Error()
	}
	b, err := jsonScalars(liveRaw, dropKeys...)
	if err != nil {
		return false, err.Error()
	}
	if reflect.DeepEqual(a, b) {
		return true, ""
	}
	return false, "scalar fields differ from the dump"
}

// compareKeyboards compares a keyboard's supplied fields after upgrading a
// legacy dump. Part ids are server-generated and total_cost is derived, so
// neither is compared; parts are compared in order.
func compareKeyboards(dumpRaw, liveRaw []byte) (bool, string) {
	dumpRaw, err := upgradeKeyboardJSON(dumpRaw)
	if err != nil {
		return false, "upgrading dump: " + err.Error()
	}
	norm := func(raw []byte) ([]byte, error) {
		var v map[string]any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		for _, key := range []string{"plates", "pcbs"} {
			parts, _ := v[key].([]any)
			for _, p := range parts {
				if part, ok := p.(map[string]any); ok {
					delete(part, "id")
				}
			}
		}
		return json.Marshal(v)
	}
	if dumpRaw, err = norm(dumpRaw); err != nil {
		return false, "parsing dump: " + err.Error()
	}
	if liveRaw, err = norm(liveRaw); err != nil {
		return false, "parsing live keyboard: " + err.Error()
	}
	return compareScalars(dumpRaw, liveRaw, "images", "total_cost")
}

// ---- per-entity verify ----

func verifyKeyboards(ctx context.Context, client *apiClient, dumpDir string, m *idMap) []verifyResult {
	var out []verifyResult
	for oldID, mapped := range m.Keyboards {
		r := verifyResult{entity: "keyboard", oldID: oldID, newID: mapped.NewID}
		itemDir := filepath.Join(dumpDir, "keyboards", oldID)
		dumpBody, err := os.ReadFile(filepath.Join(itemDir, "item.json")) //nolint:gosec // dump dir.
		if err != nil {
			r.status, r.detail = statusMissing, "dump item.json unreadable: "+err.Error()
			out = append(out, r)
			continue
		}
		liveBody, err := client.getRaw(ctx, client.userPath("keyboards/"+mapped.NewID))
		if err != nil {
			r.status, r.detail = statusMissing, err.Error()
			out = append(out, r)
			continue
		}
		if ok, detail := compareKeyboards(dumpBody, liveBody); !ok {
			r.status, r.detail = statusFieldMismatch, detail
			out = append(out, r)
			continue
		}
		if st, detail := verifyArrayImages(ctx, itemDir, liveBody, mapped); st != statusOK {
			r.status, r.detail = st, detail
			out = append(out, r)
			continue
		}
		r.status = statusOK
		out = append(out, r)
	}
	return out
}

// verifySingleImageItems verifies the collection dumped under sub, whose items
// have a single image slot.
func verifySingleImageItems(ctx context.Context, client *apiClient, dumpDir, sub, entity string, ids map[string]*mappedEntity) []verifyResult {
	var out []verifyResult
	for oldID, mapped := range ids {
		r := verifyResult{entity: entity, oldID: oldID, newID: mapped.NewID}
		itemDir := filepath.Join(dumpDir, sub, oldID)
		dumpBody, err := os.ReadFile(filepath.Join(itemDir, "item.json")) //nolint:gosec // dump dir.
		if err != nil {
			r.status, r.detail = statusMissing, "dump item.json unreadable: "+err.Error()
			out = append(out, r)
			continue
		}
		liveBody, err := client.getRaw(ctx, client.userPath(sub+"/"+mapped.NewID))
		if err != nil {
			r.status, r.detail = statusMissing, err.Error()
			out = append(out, r)
			continue
		}
		if ok, detail := compareScalars(dumpBody, liveBody, "image"); !ok {
			r.status, r.detail = statusFieldMismatch, detail
			out = append(out, r)
			continue
		}
		if st, detail := verifySingleImage(ctx, itemDir, "image", topLevelImageURL(liveBody)); st != statusOK {
			r.status, r.detail = st, detail
			out = append(out, r)
			continue
		}
		r.status = statusOK
		out = append(out, r)
	}
	return out
}

func verifyKeycapSets(ctx context.Context, client *apiClient, dumpDir string, m *idMap) []verifyResult {
	var out []verifyResult
	for oldID, mapped := range m.KeycapSets {
		r := verifyResult{entity: "keycap_set", oldID: oldID, newID: mapped.NewID}
		setDir := filepath.Join(dumpDir, "keycap-sets", oldID)
		dumpBody, err := os.ReadFile(filepath.Join(setDir, "item.json")) //nolint:gosec // dump dir.
		if err != nil {
			r.status, r.detail = statusMissing, "dump item.json unreadable: "+err.Error()
			out = append(out, r)
			continue
		}
		liveBody, err := client.getRaw(ctx, client.userPath("keycap-sets/"+mapped.NewID))
		if err != nil {
			r.status, r.detail = statusMissing, err.Error()
			out = append(out, r)
			continue
		}
		// kits[], primary_kit_id, order_status, and total_cost are all
		// server-managed/derived relative to KeycapSetInput.
		if ok, detail := compareScalars(dumpBody, liveBody, "kits", "primary_kit_id", "order_status", "total_cost"); !ok {
			r.status, r.detail = statusFieldMismatch, detail
			out = append(out, r)
			continue
		}
		if st, detail := verifyKitImages(ctx, setDir, liveBody, mapped); st != statusOK {
			r.status, r.detail = st, detail
			out = append(out, r)
			continue
		}
		r.status = statusOK
		out = append(out, r)
	}
	return out
}

func verifyBuilds(ctx context.Context, client *apiClient, dumpDir string, m *idMap) []verifyResult {
	var out []verifyResult
	for oldID, mapped := range m.Builds {
		r := verifyResult{entity: "build", oldID: oldID, newID: mapped.NewID}
		itemDir := filepath.Join(dumpDir, "builds", oldID)
		var dumpFull api.Build
		if err := readUpgradedJSON(filepath.Join(itemDir, "item.json"), upgradeBuildJSON, &dumpFull); err != nil {
			r.status, r.detail = statusMissing, "dump item.json unreadable: "+err.Error()
			out = append(out, r)
			continue
		}
		liveBody, err := client.getRaw(ctx, client.userPath("builds/"+mapped.NewID))
		if err != nil {
			r.status, r.detail = statusMissing, err.Error()
			out = append(out, r)
			continue
		}
		var liveFull api.Build
		if err := json.Unmarshal(liveBody, &liveFull); err != nil {
			r.status, r.detail = statusFieldMismatch, "live build unparseable: "+err.Error()
			out = append(out, r)
			continue
		}
		if detail := compareBuildRefs(dumpFull, liveFull, m); detail != "" {
			r.status, r.detail = statusFieldMismatch, detail
			out = append(out, r)
			continue
		}
		if st, detail := verifyArrayImages(ctx, itemDir, liveBody, mapped); st != statusOK {
			r.status, r.detail = st, detail
			out = append(out, r)
			continue
		}
		r.status = statusOK
		out = append(out, r)
	}
	return out
}

// compareBuildRefs checks that the live build's remapped references match what
// the id map says they should be. Returns "" on match.
func compareBuildRefs(dump, live api.Build, m *idMap) string {
	dumpType := dump.BuildType
	if dumpType == "" {
		dumpType = repository.BuildTypeMX
	}
	if live.BuildType != dumpType {
		return fmt.Sprintf("build_type: dump %s, live %s", dumpType, live.BuildType)
	}

	kb, ok := m.Keyboards[dump.Keyboard.Id]
	if !ok || live.Keyboard.Id != kb.NewID {
		want := ""
		if ok {
			want = kb.NewID
		}
		return fmt.Sprintf("keyboard ref: want %s, live has %s", want, live.Keyboard.Id)
	}

	var dumpPlate, livePlate *string
	if dump.Plate != nil {
		dumpPlate = &dump.Plate.Id
	}
	if live.Plate != nil {
		livePlate = &live.Plate.Id
	}
	if detail := comparePartRef("plate", dumpPlate, livePlate, kb.Plates); detail != "" {
		return detail
	}
	var dumpPCB, livePCB *string
	if dump.Pcb != nil {
		dumpPCB = &dump.Pcb.Id
	}
	if live.Pcb != nil {
		livePCB = &live.Pcb.Id
	}
	if detail := comparePartRef("pcb", dumpPCB, livePCB, kb.PCBs); detail != "" {
		return detail
	}

	dumpSwitches := derefSwitches(dump.Switches)
	liveSwitches := derefSwitches(live.Switches)
	if len(dumpSwitches) != len(liveSwitches) {
		return fmt.Sprintf("switch entry count: dump %d, live %d", len(dumpSwitches), len(liveSwitches))
	}
	for i := range dumpSwitches {
		want := ""
		if sw, ok := m.Switches[dumpSwitches[i].Switch.Id]; ok {
			want = sw.NewID
		}
		got := liveSwitches[i].Switch.Id
		if want == "" || got != want {
			return fmt.Sprintf("switch[%d] ref: want %s, live has %s", i, want, got)
		}
		if dumpSwitches[i].Count != liveSwitches[i].Count {
			return fmt.Sprintf("switch[%d] count: dump %d, live %d", i, dumpSwitches[i].Count, liveSwitches[i].Count)
		}
	}

	dumpECParts := derefECParts(dump.EcParts)
	liveECParts := derefECParts(live.EcParts)
	if len(dumpECParts) != len(liveECParts) {
		return fmt.Sprintf("EC part entry count: dump %d, live %d", len(dumpECParts), len(liveECParts))
	}
	for i := range dumpECParts {
		want := ""
		if part, ok := m.ECParts[dumpECParts[i].EcPart.Id]; ok {
			want = part.NewID
		}
		got := liveECParts[i].EcPart.Id
		if want == "" || got != want {
			return fmt.Sprintf("ec_part[%d] ref: want %s, live has %s", i, want, got)
		}
		if dumpECParts[i].Count != liveECParts[i].Count {
			return fmt.Sprintf("ec_part[%d] count: dump %d, live %d", i, dumpECParts[i].Count, liveECParts[i].Count)
		}
	}

	dumpKits := flattenKits(dump.KeycapSets)
	liveKits := flattenKits(live.KeycapSets)
	if len(dumpKits) != len(liveKits) {
		return fmt.Sprintf("keycap kit count: dump %d, live %d", len(dumpKits), len(liveKits))
	}
	for i := range dumpKits {
		wantSet, wantKit := "", ""
		if set, ok := m.KeycapSets[dumpKits[i].KeycapSet]; ok {
			wantSet = set.NewID
			wantKit = set.Kits[dumpKits[i].Kit]
		}
		if wantSet == "" || liveKits[i].KeycapSet != wantSet || wantKit == "" || liveKits[i].Kit != wantKit {
			return fmt.Sprintf("keycap kit %d ref: want set %s kit %s, live has set %s kit %s", i, wantSet, wantKit, liveKits[i].KeycapSet, liveKits[i].Kit)
		}
	}
	return ""
}

// comparePartRef checks a build's plate or PCB ref against the keyboard's
// part id map. Returns "" on match.
func comparePartRef(field string, dump, live *string, ids map[string]string) string {
	switch {
	case dump == nil && live == nil:
		return ""
	case dump == nil:
		return fmt.Sprintf("%s ref: dump has none, live has %s", field, *live)
	case live == nil:
		return fmt.Sprintf("%s ref: want %s, live has none", field, ids[*dump])
	case ids[*dump] == "" || ids[*dump] != *live:
		return fmt.Sprintf("%s ref: want %s, live has %s", field, ids[*dump], *live)
	}
	return ""
}

func derefSwitches(p *[]api.BuildSwitchEntryResolved) []api.BuildSwitchEntryResolved {
	if p == nil {
		return nil
	}
	return *p
}

func derefECParts(p *[]api.BuildECPartEntryResolved) []api.BuildECPartEntryResolved {
	if p == nil {
		return nil
	}
	return *p
}

func flattenKits(p *[]api.BuildKeycapSetRef) []api.BuildKeycapKitEntry {
	if p == nil {
		return nil
	}
	var out []api.BuildKeycapKitEntry
	for _, ks := range *p {
		for _, kit := range ks.Kits {
			out = append(out, api.BuildKeycapKitEntry{KeycapSet: ks.Id, Kit: kit.KitId})
		}
	}
	return out
}

// ---- image verification ----

// verifyArrayImages checks that the live entity has one image per dump
// manifest entry, matched by the id map, each hash-matching the dump.
func verifyArrayImages(ctx context.Context, itemDir string, liveBody []byte, mapped *mappedEntity) (verifyStatus, string) {
	manifestPath := filepath.Join(itemDir, "images", "images.manifest.json")
	var manifest []imageManifestEntry
	if _, err := os.Stat(manifestPath); err == nil {
		if err := readJSONFile(manifestPath, &manifest); err != nil {
			return statusImageHashMismatch, err.Error()
		}
	}

	liveRefs, err := arrayImageRefs(liveBody)
	if err != nil {
		return statusImageHashMismatch, err.Error()
	}
	if len(liveRefs) != len(manifest) {
		return statusImageCountMismatch, fmt.Sprintf("dump has %d image(s), live has %d", len(manifest), len(liveRefs))
	}

	liveByID := map[string]string{}
	for _, ref := range liveRefs {
		liveByID[ref.imageID] = ref.url
	}
	for _, entry := range manifest {
		newID, ok := mapped.Images[entry.ImageID]
		if !ok {
			return statusImageHashMismatch, "id map has no restored id for dumped image " + entry.ImageID
		}
		url, ok := liveByID[newID]
		if !ok {
			return statusImageHashMismatch, "restored image " + newID + " is not on the live entity"
		}
		if st, detail := checkURLHash(ctx, url, entry.SHA256); st != statusOK {
			return st, detail
		}
	}
	return statusOK, ""
}

// verifyKitImages checks each dumped kit image against the live set.
func verifyKitImages(ctx context.Context, setDir string, liveBody []byte, mapped *mappedKeycaps) (verifyStatus, string) {
	kitsDir := filepath.Join(setDir, "kits")
	entries, err := os.ReadDir(kitsDir)
	if os.IsNotExist(err) {
		return statusOK, ""
	}
	if err != nil {
		return statusImageHashMismatch, err.Error()
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		oldKitID := e.Name()
		manifestPath := filepath.Join(kitsDir, oldKitID, "image.manifest.json")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			continue
		}
		var entry imageManifestEntry
		if err := readJSONFile(manifestPath, &entry); err != nil {
			return statusImageHashMismatch, err.Error()
		}
		newKitID, ok := mapped.Kits[oldKitID]
		if !ok {
			return statusImageHashMismatch, "id map has no restored kit id for " + oldKitID
		}
		url, err := kitImageURL(liveBody, newKitID)
		if err != nil {
			return statusImageHashMismatch, err.Error()
		}
		if st, detail := checkURLHash(ctx, url, entry.SHA256); st != statusOK {
			return st, detail
		}
	}
	return statusOK, ""
}

// verifySingleImage checks a single-slot image (switch/EC part/profile) against
// liveURL. name is "image" or "avatar".
func verifySingleImage(ctx context.Context, dir, name, liveURL string) (verifyStatus, string) {
	manifestPath := filepath.Join(dir, name+".manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		if liveURL != "" {
			return statusImageCountMismatch, "dump has no " + name + " but live does"
		}
		return statusOK, ""
	}
	var entry imageManifestEntry
	if err := readJSONFile(manifestPath, &entry); err != nil {
		return statusImageHashMismatch, err.Error()
	}
	if liveURL == "" {
		return statusImageCountMismatch, "dump has " + name + " but live does not"
	}
	return checkURLHash(ctx, liveURL, entry.SHA256)
}

// checkURLHash downloads url and compares the bytes' SHA-256 to wantSHA.
func checkURLHash(ctx context.Context, url, wantSHA string) (verifyStatus, string) {
	data, _, err := downloadImage(ctx, url)
	if err != nil {
		return statusImageHashMismatch, "download failed: " + err.Error()
	}
	if got := sha256Hex(data); got != wantSHA {
		return statusImageHashMismatch, fmt.Sprintf("want %s, got %s", wantSHA, got)
	}
	return statusOK, ""
}
