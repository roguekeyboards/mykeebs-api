package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// imageRef is one image to download: its signed image URL, plus the
// server-generated id for array-image entities (empty for single-slot).
type imageRef struct {
	imageID string
	url     string
}

// entitySpec describes one simple dumpable collection (one whose GET body maps
// straight to a dump directory: keyboards, switches, EC parts, builds). Keycap
// sets and the profile (profile.go) need bespoke handling and don't use this.
type entitySpec struct {
	// name is both the dump subdirectory and the /v1/users/{sub}/ suffix.
	name string
	// arrayImages is true for an images[] array, false for a single slot.
	arrayImages bool
	// imageRefs extracts every image reference from a full GET body.
	imageRefs func(body []byte) ([]imageRef, error)
}

var (
	keyboardsSpec = entitySpec{name: "keyboards", arrayImages: true, imageRefs: arrayImageRefs}
	switchesSpec  = entitySpec{name: "switches", arrayImages: false, imageRefs: singleImageRef}
	ecPartsSpec   = entitySpec{name: "ec-parts", arrayImages: false, imageRefs: singleImageRef}
	buildsSpec    = entitySpec{name: "builds", arrayImages: true, imageRefs: arrayImageRefs}
)

// idField pulls a top-level "id" string out of a raw JSON object.
func idField(raw json.RawMessage) (string, error) {
	var v struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", fmt.Errorf("reading id: %w", err)
	}
	if v.ID == "" {
		return "", fmt.Errorf("item has no id: %s", raw)
	}
	return v.ID, nil
}

// arrayImageRefs extracts images[] = [{image_id,url}] from a GET body.
func arrayImageRefs(body []byte) ([]imageRef, error) {
	var v struct {
		Images []struct {
			ImageID string `json:"image_id"`
			URL     string `json:"url"`
		} `json:"images"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("reading images[]: %w", err)
	}
	refs := make([]imageRef, 0, len(v.Images))
	for _, im := range v.Images {
		refs = append(refs, imageRef{imageID: im.ImageID, url: im.URL})
	}
	return refs, nil
}

// singleImageRef extracts image = {url} | null from a GET body.
func singleImageRef(body []byte) ([]imageRef, error) {
	var v struct {
		Image *struct {
			URL string `json:"url"`
		} `json:"image"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("reading image: %w", err)
	}
	if v.Image == nil || v.Image.URL == "" {
		return nil, nil
	}
	return []imageRef{{url: v.Image.URL}}, nil
}

// Run executes the dump.
func (c *DumpCmd) Run(ctx context.Context) error {
	token, err := c.token(ctx)
	if err != nil {
		return err
	}
	client, err := newAPIClient(c.BaseURL, token)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(c.Out, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", c.Out, err)
	}

	counts := map[string]int{}
	totalImages := 0

	// Builds reference the other entities; dump it last so its resolved body
	// is captured after everything it points at (not strictly required, since
	// the resolved GET body is stored verbatim, but it keeps the tree tidy).
	conv := &imageConverter{}
	for _, spec := range []entitySpec{keyboardsSpec, switchesSpec, ecPartsSpec} {
		n, imgs, err := dumpSimpleCollection(ctx, client, c.Out, spec, conv)
		if err != nil {
			return err
		}
		counts[strings.ReplaceAll(spec.name, "-", "_")] = n
		totalImages += imgs
	}

	ksCount, ksImgs, err := dumpKeycapSets(ctx, client, c.Out, conv)
	if err != nil {
		return err
	}
	counts["keycap_sets"] = ksCount
	totalImages += ksImgs

	hasProfile, avatars, err := dumpProfile(ctx, client, c.Out, conv)
	if err != nil {
		return err
	}
	if hasProfile {
		counts["profile"] = 1
	}
	totalImages += avatars

	buildsN, buildsImgs, err := dumpSimpleCollection(ctx, client, c.Out, buildsSpec, conv)
	if err != nil {
		return err
	}
	counts["builds"] = buildsN
	totalImages += buildsImgs

	if err := dumpLookups(ctx, client, c.Out); err != nil {
		return err
	}

	counts["images"] = totalImages
	counts["images_converted"] = conv.converted
	manifest := dumpManifest{
		ToolVersion:   Version,
		DumpedAt:      time.Now(),
		SourceBaseURL: client.baseURL,
		SourceSubject: client.subject,
		Counts:        counts,
	}
	if err := writeJSONFile(filepath.Join(c.Out, "manifest.json"), manifest); err != nil {
		return err
	}

	fmt.Printf("\ndump complete\n")
	fmt.Printf("  source:  %s (subject %s)\n", client.baseURL, client.subject)
	fmt.Printf("  output:  %s\n\n", c.Out)
	for _, k := range []string{"profile", "keyboards", "switches", "ec_parts", "keycap_sets", "builds", "images", "images_converted"} {
		fmt.Printf("  %-16s %d\n", k, counts[k])
	}
	fmt.Printf("\nEach collection count above equals the number the API's list endpoint\n")
	fmt.Printf("returned; cross-check against list_keyboards / list_switches /\n")
	fmt.Printf("list_ec_parts / list_keycap_sets / list_builds if you want independent\n")
	fmt.Printf("confirmation.\n")
	return nil
}

// token is the bearer token the dump runs as: an impersonated session's with
// --impersonate, otherwise --token or the login cache.
func (c *DumpCmd) token(ctx context.Context) (string, error) {
	if !c.Impersonate {
		return resolveToken(c.Token, c.Issuer)
	}
	if c.Issuer == "" || c.PublicToken == "" {
		return "", errors.New("--impersonate needs --issuer and --public-token")
	}
	return impersonate(ctx, c.Issuer, c.PublicToken, c.Port)
}

// dumpSimpleCollection dumps one collection with either an images[] array or a
// single image slot. Returns (item count, image count).
func dumpSimpleCollection(ctx context.Context, client *apiClient, outDir string, spec entitySpec, conv *imageConverter) (int, int, error) {
	fmt.Printf("listing %s...\n", spec.name)
	items, err := client.listAll(ctx, client.userPath(spec.name))
	if err != nil {
		return 0, 0, fmt.Errorf("listing %s: %w", spec.name, err)
	}
	total := len(items)
	fmt.Printf("  %d %s listed\n", total, spec.name)

	imageCount := 0
	for i, summary := range items {
		id, err := idField(summary)
		if err != nil {
			return 0, 0, fmt.Errorf("%s: %w", spec.name, err)
		}
		itemDir := filepath.Join(outDir, spec.name, id)
		if err := os.MkdirAll(itemDir, 0o700); err != nil {
			return 0, 0, fmt.Errorf("creating %s: %w", itemDir, err)
		}

		body, err := client.getRaw(ctx, client.userPath(spec.name+"/"+id))
		if err != nil {
			return 0, 0, fmt.Errorf("fetching %s %s: %w", spec.name, id, err)
		}
		if err := os.WriteFile(filepath.Join(itemDir, "item.json"), body, 0o600); err != nil {
			return 0, 0, fmt.Errorf("writing %s item.json: %w", id, err)
		}

		refs, err := spec.imageRefs(body)
		if err != nil {
			return 0, 0, fmt.Errorf("%s %s: %w", spec.name, id, err)
		}
		n, err := saveImageRefs(ctx, itemDir, spec.arrayImages, refs, conv)
		if err != nil {
			return 0, 0, fmt.Errorf("%s %s: %w", spec.name, id, err)
		}
		imageCount += n
		fmt.Printf("  %s %d/%d %s (%d image%s)\n", spec.name, i+1, total, id, n, plural(n))
	}
	return total, imageCount, nil
}

// plural returns "s" unless n == 1, for simple count messages.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// saveImageRefs downloads each ref into dir, writing image files plus a
// manifest. For arrayImages it writes images/<image_id>.<ext> +
// images/images.manifest.json; otherwise image.<ext> + image.manifest.json.
// Returns the number of images written.
func saveImageRefs(ctx context.Context, dir string, arrayImages bool, refs []imageRef, conv *imageConverter) (int, error) {
	if len(refs) == 0 {
		return 0, nil
	}

	if arrayImages {
		imgDir := filepath.Join(dir, "images")
		if err := os.MkdirAll(imgDir, 0o700); err != nil {
			return 0, fmt.Errorf("creating %s: %w", imgDir, err)
		}
		manifest := make([]imageManifestEntry, 0, len(refs))
		for _, ref := range refs {
			entry, err := downloadOne(ctx, imgDir, ref.imageID, ref, conv)
			if err != nil {
				return 0, err
			}
			manifest = append(manifest, entry)
		}
		if err := writeJSONFile(filepath.Join(imgDir, "images.manifest.json"), manifest); err != nil {
			return 0, err
		}
		return len(manifest), nil
	}

	// single slot
	ref := refs[0]
	entry, err := downloadOne(ctx, dir, "image", ref, conv)
	if err != nil {
		return 0, err
	}
	if err := writeJSONFile(filepath.Join(dir, "image.manifest.json"), entry); err != nil {
		return 0, err
	}
	return 1, nil
}

// downloadOne fetches ref, converts it with conv, writes
// <dir>/<basename>.<ext>, verifies the file on disk, and returns its manifest
// entry. basename is the image id for array images, or "image"/"avatar" for
// single slots.
func downloadOne(ctx context.Context, dir, basename string, ref imageRef, conv *imageConverter) (imageManifestEntry, error) {
	data, _, err := downloadImage(ctx, ref.url)
	if err != nil {
		return imageManifestEntry{}, fmt.Errorf("downloading image %s: %w", basename, err)
	}
	data, err = conv.convert(data)
	if err != nil {
		return imageManifestEntry{}, fmt.Errorf("converting image %s: %w", basename, err)
	}
	filename := basename + ".webp"
	sha := sha256Hex(data)
	if err := writeAndVerify(filepath.Join(dir, filename), data, sha); err != nil {
		return imageManifestEntry{}, err
	}
	return imageManifestEntry{
		ImageID:     ref.imageID,
		Filename:    filename,
		ContentType: "image/webp",
		Bytes:       len(data),
		SHA256:      sha,
	}, nil
}

// dumpKeycapSets dumps keycap sets with their nested kits[] and per-kit
// images. Returns (set count, image count).
func dumpKeycapSets(ctx context.Context, client *apiClient, outDir string, conv *imageConverter) (int, int, error) {
	fmt.Println("listing keycap-sets...")
	items, err := client.listAll(ctx, client.userPath("keycap-sets"))
	if err != nil {
		return 0, 0, fmt.Errorf("listing keycap sets: %w", err)
	}
	total := len(items)
	fmt.Printf("  %d keycap-sets listed\n", total)

	imageCount := 0
	for i, summary := range items {
		id, err := idField(summary)
		if err != nil {
			return 0, 0, fmt.Errorf("keycap sets: %w", err)
		}
		setDir := filepath.Join(outDir, "keycap-sets", id)
		if err := os.MkdirAll(setDir, 0o700); err != nil {
			return 0, 0, fmt.Errorf("creating %s: %w", setDir, err)
		}

		body, err := client.getRaw(ctx, client.userPath("keycap-sets/"+id))
		if err != nil {
			return 0, 0, fmt.Errorf("fetching keycap set %s: %w", id, err)
		}
		if err := os.WriteFile(filepath.Join(setDir, "item.json"), body, 0o600); err != nil {
			return 0, 0, fmt.Errorf("writing keycap set %s item.json: %w", id, err)
		}

		var set struct {
			Kits []struct {
				KitID string `json:"kit_id"`
				Image *struct {
					URL string `json:"url"`
				} `json:"image"`
			} `json:"kits"`
		}
		if err := json.Unmarshal(body, &set); err != nil {
			return 0, 0, fmt.Errorf("reading keycap set %s kits: %w", id, err)
		}
		kitImages := 0
		for _, kit := range set.Kits {
			if kit.Image == nil || kit.Image.URL == "" {
				continue
			}
			kitDir := filepath.Join(setDir, "kits", kit.KitID)
			if err := os.MkdirAll(kitDir, 0o700); err != nil {
				return 0, 0, fmt.Errorf("creating %s: %w", kitDir, err)
			}
			entry, err := downloadOne(ctx, kitDir, "image", imageRef{url: kit.Image.URL}, conv)
			if err != nil {
				return 0, 0, fmt.Errorf("keycap set %s kit %s: %w", id, kit.KitID, err)
			}
			if err := writeJSONFile(filepath.Join(kitDir, "image.manifest.json"), entry); err != nil {
				return 0, 0, err
			}
			kitImages++
		}
		imageCount += kitImages
		fmt.Printf("  keycap-sets %d/%d %s (%d kit%s, %d kit image%s)\n",
			i+1, total, id, len(set.Kits), plural(len(set.Kits)), kitImages, plural(kitImages))
	}
	return total, imageCount, nil
}

// dumpLookups writes lookups/lookups.json for reference/diff. Never restored.
func dumpLookups(ctx context.Context, client *apiClient, outDir string) error {
	body, err := client.getRaw(ctx, "/v1/lookups")
	if err != nil {
		return fmt.Errorf("fetching lookups: %w", err)
	}
	dir := filepath.Join(outDir, "lookups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lookups.json"), body, 0o600); err != nil {
		return fmt.Errorf("writing lookups.json: %w", err)
	}
	return nil
}
