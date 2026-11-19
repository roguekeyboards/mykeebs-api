package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// dumpManifest is dump/manifest.json: metadata about a dump run, written last.
type dumpManifest struct {
	ToolVersion   string         `json:"tool_version"`
	DumpedAt      time.Time      `json:"dumped_at"`
	SourceBaseURL string         `json:"source_base_url"`
	SourceSubject string         `json:"source_subject"`
	Counts        map[string]int `json:"counts"`
}

// imageManifestEntry describes one downloaded image file. For array-image
// entities (keyboards, builds) the dir holds an images.manifest.json that is a
// JSON array of these; for single-slot entities it holds one object.
type imageManifestEntry struct {
	// ImageID is the server-generated id, present only for array-image
	// entities. Empty for single-slot images (switch/EC part/kit/avatar).
	ImageID     string `json:"image_id,omitempty"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Bytes       int    `json:"bytes"`
	SHA256      string `json:"sha256"`
}

// idMap is dump/id-map.json: old-id -> new-id for every server-generated id a
// restore creates. Written incrementally so a failed restore can resume.
type idMap struct {
	TargetSubject string                    `json:"target_subject"`
	RestoredAt    time.Time                 `json:"restored_at"`
	Keyboards     map[string]*mappedEntity  `json:"keyboards"`
	Switches      map[string]*mappedEntity  `json:"switches"`
	ECParts       map[string]*mappedEntity  `json:"ec_parts"`
	KeycapSets    map[string]*mappedKeycaps `json:"keycap_sets"`
	Builds        map[string]*mappedEntity  `json:"builds"`

	path string
}

// mappedEntity is a restored entity's new id, plus (for array-image entities)
// the per-image old-id -> new-id map, and for a keyboard the same for its
// plates and PCBs.
type mappedEntity struct {
	NewID  string            `json:"new_id"`
	Images map[string]string `json:"images,omitempty"`
	Plates map[string]string `json:"plates,omitempty"`
	PCBs   map[string]string `json:"pcbs,omitempty"`
}

// mappedKeycaps is a restored keycap set: its new id plus the per-set
// old-kit-id -> new-kit-id map (kit ids are server-generated per set).
type mappedKeycaps struct {
	NewID string            `json:"new_id"`
	Kits  map[string]string `json:"kits"`
}

// newIDMap returns an empty map bound to dir/id-map.json.
func newIDMap(dir, targetSubject string) *idMap {
	return &idMap{
		TargetSubject: targetSubject,
		RestoredAt:    time.Now(),
		Keyboards:     map[string]*mappedEntity{},
		Switches:      map[string]*mappedEntity{},
		ECParts:       map[string]*mappedEntity{},
		KeycapSets:    map[string]*mappedKeycaps{},
		Builds:        map[string]*mappedEntity{},
		path:          filepath.Join(dir, "id-map.json"),
	}
}

// loadOrNewIDMap reads dir/id-map.json if present (so a re-run resumes), or
// returns a fresh one. A present map whose target subject differs from
// targetSubject is an error: mixing restores into two identities in one
// directory would corrupt the reference remapping.
func loadOrNewIDMap(dir, targetSubject string) (*idMap, error) {
	path := filepath.Join(dir, "id-map.json")
	data, err := os.ReadFile(path) //nolint:gosec // path is inside the dump dir.
	if os.IsNotExist(err) {
		return newIDMap(dir, targetSubject), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var m idMap
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if m.TargetSubject != "" && m.TargetSubject != targetSubject {
		return nil, fmt.Errorf("%s was written for subject %q but this restore is for %q; use a fresh dump copy", path, m.TargetSubject, targetSubject)
	}
	m.TargetSubject = targetSubject
	m.path = path
	m.ensureMaps()
	return &m, nil
}

func (m *idMap) ensureMaps() {
	if m.Keyboards == nil {
		m.Keyboards = map[string]*mappedEntity{}
	}
	if m.Switches == nil {
		m.Switches = map[string]*mappedEntity{}
	}
	if m.ECParts == nil {
		m.ECParts = map[string]*mappedEntity{}
	}
	if m.KeycapSets == nil {
		m.KeycapSets = map[string]*mappedKeycaps{}
	}
	if m.Builds == nil {
		m.Builds = map[string]*mappedEntity{}
	}
}

// save writes the map to its bound path, pretty-printed.
func (m *idMap) save() error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding id-map: %w", err)
	}
	if err := os.WriteFile(m.path, data, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", m.path, err)
	}
	return nil
}

// writeJSONFile marshals v pretty-printed to path.
func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// readJSONFile unmarshals path into v.
func readJSONFile(path string, v any) error {
	data, err := os.ReadFile(path) //nolint:gosec // path is inside the dump dir.
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	return nil
}
