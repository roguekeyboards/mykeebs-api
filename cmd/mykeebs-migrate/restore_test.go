package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
)

type RestoreSuite struct {
	suite.Suite
}

func TestRestoreSuite(t *testing.T) {
	suite.Run(t, new(RestoreSuite))
}

func (s *RestoreSuite) fullMap() *idMap {
	return &idMap{
		Keyboards: map[string]*mappedEntity{"kb-old": {
			NewID:  "kb-new",
			Plates: map[string]string{"plate-old": "plate-new"},
			PCBs:   map[string]string{"pcb-old": "pcb-new"},
		}},
		Switches: map[string]*mappedEntity{"sw-old": {NewID: "sw-new"}},
		ECParts:  map[string]*mappedEntity{"ec-old": {NewID: "ec-new"}},
		KeycapSets: map[string]*mappedKeycaps{
			"set-old": {NewID: "set-new", Kits: map[string]string{"kit-old": "kit-new"}},
		},
		Builds: map[string]*mappedEntity{},
	}
}

func (s *RestoreSuite) TestBuildInputFromResolved_RemapsEveryReference() {
	full := api.Build{
		Id:         "b-old",
		Keyboard:   api.BuildKeyboardRef{Id: "kb-old", Brand: "B", Name: "N"},
		Plate:      &api.BuildPlateRef{Id: "plate-old", Material: "Brass"},
		Pcb:        &api.BuildPCBRef{Id: "pcb-old"},
		Visibility: new(api.Visibility("public")),
		Switches: &[]api.BuildSwitchEntryResolved{
			{Count: 70, Switch: api.BuildSwitchRef{Id: "sw-old", Name: "S", Type: "linear"}},
		},
		KeycapSets: &[]api.BuildKeycapSetRef{
			{Id: "set-old", Brand: "GMK", Name: "X", Kits: []api.BuildKeycapKitRef{{KitId: "kit-old", Name: "Base"}}},
		},
	}

	got, err := buildInputFromResolved(full, s.fullMap())
	s.Require().NoError(err)
	s.Equal("kb-new", got.Keyboard)
	s.Equal(new("plate-new"), got.Plate)
	s.Equal(new("pcb-new"), got.Pcb)
	s.Equal("public", string(got.Visibility))

	s.Require().NotNil(got.Switches)
	s.Require().Len(*got.Switches, 1)
	s.Equal("sw-new", (*got.Switches)[0].Switch)
	s.Equal(70, (*got.Switches)[0].Count)

	s.Require().NotNil(got.KeycapKits)
	s.Require().Len(*got.KeycapKits, 1)
	s.Equal("set-new", (*got.KeycapKits)[0].KeycapSet)
	s.Equal("kit-new", (*got.KeycapKits)[0].Kit)
}

func (s *RestoreSuite) TestBuildInputFromResolved_RemapsECParts() {
	full := api.Build{
		Keyboard:   api.BuildKeyboardRef{Id: "kb-old"},
		BuildType:  "EC",
		Visibility: new(api.Visibility("public")),
		EcParts: &[]api.BuildECPartEntryResolved{
			{Count: 87, EcPart: api.BuildECPartRef{Id: "ec-old", PartType: api.ECPartTypeDomes}},
		},
	}

	got, err := buildInputFromResolved(full, s.fullMap())

	s.Require().NoError(err)
	s.Equal("EC", got.BuildType)
	s.Equal(&[]api.BuildECPartEntry{{EcPart: "ec-new", Count: 87}}, got.EcParts)
	s.Nil(got.Switches)
}

func (s *RestoreSuite) TestBuildInputFromResolved_UnmappedECPartIsError() {
	full := api.Build{
		Keyboard:   api.BuildKeyboardRef{Id: "kb-old"},
		Visibility: new(api.Visibility("public")),
		EcParts: &[]api.BuildECPartEntryResolved{
			{Count: 1, EcPart: api.BuildECPartRef{Id: "ec-unknown"}},
		},
	}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().ErrorContains(err, "ec-unknown")
}

func (s *RestoreSuite) TestRestoreECParts_CreatesPartAndUploadsImage() {
	dumpDir := s.T().TempDir()
	itemDir := filepath.Join(dumpDir, "ec-parts", "ec-old")
	s.Require().NoError(os.MkdirAll(itemDir, 0o750))
	s.Require().NoError(os.WriteFile(filepath.Join(itemDir, "item.json"), []byte(
		`{"id":"ec-old","brand":"DynaCap","name":"Medium Doems","part_type":"domes","visibility":"public",`+
			`"domes":{"weight":45},"purchase":{"price":40,"quantity":110,"currency":"USD"},"image":{"url":"x"}}`,
	), 0o600))
	img := []byte("ec part image bytes")
	s.Require().NoError(os.WriteFile(filepath.Join(itemDir, "image.webp"), img, 0o600))
	manifest, err := json.Marshal(imageManifestEntry{
		Filename: "image.webp", ContentType: "image/webp", Bytes: len(img), SHA256: sha256Hex(img),
	})
	s.Require().NoError(err)
	s.Require().NoError(os.WriteFile(filepath.Join(itemDir, "image.manifest.json"), manifest, 0o600))

	var calls []string
	var created map[string]any
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/users/u-1/ec-parts":
			_ = json.NewDecoder(r.Body).Decode(&created)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"ec-new","brand":"DynaCap","name":"Medium Doems","part_type":"domes"}`))
		case r.Method == http.MethodPost:
			_, _ = fmt.Fprintf(w, `{"upload_url":%q}`, srv.URL+"/blob")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/u-1/ec-parts/ec-new":
			_, _ = fmt.Fprintf(w, `{"id":"ec-new","image":{"url":%q}}`, srv.URL+"/blob")
		case r.Method == http.MethodGet && r.URL.Path == "/blob":
			_, _ = w.Write(img)
		}
	}))
	defer srv.Close()

	client := &apiClient{baseURL: srv.URL, subject: "u-1", http: srv.Client()}
	m := newIDMap(s.T().TempDir(), "u-1")

	err = restoreECParts(context.Background(), client, dumpDir, m)

	s.Require().NoError(err)
	s.Equal("ec-new", m.ECParts["ec-old"].NewID)
	s.Equal(map[string]any{
		"brand": "DynaCap", "name": "Medium Doems", "part_type": "domes", "visibility": "public",
		"domes":    map[string]any{"weight": float64(45)},
		"purchase": map[string]any{"price": float64(40), "quantity": float64(110)},
	}, created)
	s.Equal([]string{
		"POST /v1/users/u-1/ec-parts",
		"POST /v1/users/u-1/ec-parts/ec-new/image",
		"PUT /blob",
		"GET /v1/users/u-1/ec-parts/ec-new",
		"GET /blob",
	}, calls)
}

func (s *RestoreSuite) TestBuildInputFromResolved_UnmappedKeyboardIsError() {
	full := api.Build{Keyboard: api.BuildKeyboardRef{Id: "kb-unknown"}, Visibility: new(api.Visibility("public"))}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "kb-unknown")
}

func (s *RestoreSuite) TestBuildInputFromResolved_UnmappedPlateIsError() {
	full := api.Build{
		Keyboard:   api.BuildKeyboardRef{Id: "kb-old"},
		Visibility: new(api.Visibility("public")),
		Plate:      &api.BuildPlateRef{Id: "plate-unknown", Material: "AL"},
	}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "plate-unknown")
}

func (s *RestoreSuite) TestBuildInputFromResolved_UnmappedSwitchIsError() {
	full := api.Build{
		Keyboard:   api.BuildKeyboardRef{Id: "kb-old"},
		Visibility: new(api.Visibility("public")),
		Switches: &[]api.BuildSwitchEntryResolved{
			{Count: 1, Switch: api.BuildSwitchRef{Id: "sw-unknown"}},
		},
	}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "sw-unknown")
}

func (s *RestoreSuite) TestBuildInputFromResolved_UnmappedKitIsError() {
	full := api.Build{
		Keyboard:   api.BuildKeyboardRef{Id: "kb-old"},
		Visibility: new(api.Visibility("public")),
		KeycapSets: &[]api.BuildKeycapSetRef{
			{Id: "set-old", Kits: []api.BuildKeycapKitRef{{KitId: "kit-unknown"}}},
		},
	}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "kit-unknown")
}

func (s *RestoreSuite) TestBuildInputFromResolved_MissingVisibilityIsError() {
	full := api.Build{Keyboard: api.BuildKeyboardRef{Id: "kb-old"}}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "no visibility")
}

func (s *RestoreSuite) TestBuildInputFromResolved_MissingKeyboardIsError() {
	_, err := buildInputFromResolved(api.Build{Visibility: new(api.Visibility("public"))}, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "no keyboard")
}

func (s *RestoreSuite) TestRestoreKeyboards_AddsPartsInDumpedOrderAndMapsTheirIDs() {
	dumpDir := s.T().TempDir()
	itemDir := filepath.Join(dumpDir, "keyboards", "kb-old")
	s.Require().NoError(os.MkdirAll(itemDir, 0o750))
	s.Require().NoError(os.WriteFile(filepath.Join(itemDir, "item.json"), []byte(
		`{"id":"kb-old","brand":"Acme","name":"One","visibility":"private",`+
			`"plates":[{"id":"plate-a","material":"FR4"},{"id":"plate-b","material":"PC"}],`+
			`"pcbs":[{"id":"pcb-a","assembly":"Hotswap"}]}`,
	), 0o600))

	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		calls = append(calls, fmt.Sprintf("%s %s %v", r.Method, r.URL.Path, body["material"]))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		switch {
		case strings.HasSuffix(r.URL.Path, "/plates"):
			_, _ = fmt.Fprintf(w, `{"id":"new-%v","material":%q}`, body["material"], body["material"])
		case strings.HasSuffix(r.URL.Path, "/pcbs"):
			_, _ = w.Write([]byte(`{"id":"new-pcb"}`))
		default:
			_, _ = w.Write([]byte(`{"id":"kb-new","brand":"Acme","name":"One","visibility":"private"}`))
		}
	}))
	defer srv.Close()

	client := &apiClient{baseURL: srv.URL, subject: "u-1", http: srv.Client()}
	m := newIDMap(s.T().TempDir(), "u-1")

	err := restoreKeyboards(context.Background(), client, dumpDir, m)

	s.Require().NoError(err)
	s.Equal([]string{
		"POST /v1/users/u-1/keyboards <nil>",
		"POST /v1/users/u-1/keyboards/kb-new/plates FR4",
		"POST /v1/users/u-1/keyboards/kb-new/plates PC",
		"POST /v1/users/u-1/keyboards/kb-new/pcbs <nil>",
	}, calls)
	s.Equal(map[string]string{"plate-a": "new-FR4", "plate-b": "new-PC"}, m.Keyboards["kb-old"].Plates)
	s.Equal(map[string]string{"pcb-a": "new-pcb"}, m.Keyboards["kb-old"].PCBs)
}

func (s *RestoreSuite) TestRestoreKeyboards_ResumeSkipsPartsAlreadyMapped() {
	dumpDir := s.T().TempDir()
	itemDir := filepath.Join(dumpDir, "keyboards", "kb-old")
	s.Require().NoError(os.MkdirAll(itemDir, 0o750))
	s.Require().NoError(os.WriteFile(filepath.Join(itemDir, "item.json"), []byte(
		`{"id":"kb-old","brand":"Acme","name":"One","visibility":"private",`+
			`"plates":[{"id":"plate-a","material":"FR4"},{"id":"plate-b","material":"PC"}]}`,
	), 0o600))

	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"new-b","material":"PC"}`))
	}))
	defer srv.Close()

	client := &apiClient{baseURL: srv.URL, subject: "u-1", http: srv.Client()}
	m := newIDMap(s.T().TempDir(), "u-1")
	m.Keyboards["kb-old"] = &mappedEntity{NewID: "kb-new", Images: map[string]string{}, Plates: map[string]string{"plate-a": "new-a"}}

	err := restoreKeyboards(context.Background(), client, dumpDir, m)

	s.Require().NoError(err)
	s.Equal([]string{"POST /v1/users/u-1/keyboards/kb-new/plates"}, calls)
	s.Equal(map[string]string{"plate-a": "new-a", "plate-b": "new-b"}, m.Keyboards["kb-old"].Plates)
}

// writeKitDump writes a one-set, one-kit dump whose kit has an image, and
// returns the dump dir and the image bytes.
func (s *RestoreSuite) writeKitDump() (string, []byte) {
	dumpDir := s.T().TempDir()
	setDir := filepath.Join(dumpDir, "keycap-sets", "set-old")
	kitDir := filepath.Join(setDir, "kits", "kit-old")
	s.Require().NoError(os.MkdirAll(kitDir, 0o750))
	s.Require().NoError(os.WriteFile(filepath.Join(setDir, "item.json"), []byte(
		`{"id":"set-old","name":"Botanical","visibility":"private","kits":[{"kit_id":"kit-old","name":"Aesthetic"}]}`,
	), 0o600))
	img := []byte("kit image bytes")
	s.Require().NoError(os.WriteFile(filepath.Join(kitDir, "image.webp"), img, 0o600))
	manifest, err := json.Marshal(imageManifestEntry{
		Filename: "image.webp", ContentType: "image/webp", Bytes: len(img), SHA256: sha256Hex(img),
	})
	s.Require().NoError(err)
	s.Require().NoError(os.WriteFile(filepath.Join(kitDir, "image.manifest.json"), manifest, 0o600))
	return dumpDir, img
}

// kitServer serves the restored set, with its kit's image present once
// uploaded (or from the start when hasImage), and records each request.
func (s *RestoreSuite) kitServer(img []byte, hasImage bool, calls *[]string) *httptest.Server {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls = append(*calls, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/users/u-1/keycap-sets/set-new":
			image := ""
			if hasImage {
				image = fmt.Sprintf(`,"image":{"url":%q}`, srv.URL+"/blob")
			}
			_, _ = fmt.Fprintf(w, `{"id":"set-new","kits":[{"kit_id":"kit-new"%s}]}`, image)
		case r.Method == http.MethodPost:
			_, _ = fmt.Fprintf(w, `{"upload_url":%q}`, srv.URL+"/blob")
		case r.Method == http.MethodPut:
			hasImage = true
		case r.Method == http.MethodGet && r.URL.Path == "/blob":
			_, _ = w.Write(img)
		}
	}))
	return srv
}

func (s *RestoreSuite) TestRestoreKeycapSets_ResumeUploadsMissingKitImage() {
	dumpDir, img := s.writeKitDump()
	var calls []string
	srv := s.kitServer(img, false, &calls)
	defer srv.Close()

	client := &apiClient{baseURL: srv.URL, subject: "u-1", http: srv.Client()}
	m := newIDMap(s.T().TempDir(), "u-1")
	m.KeycapSets["set-old"] = &mappedKeycaps{NewID: "set-new", Kits: map[string]string{"kit-old": "kit-new"}}

	err := restoreKeycapSets(context.Background(), client, dumpDir, m)

	s.Require().NoError(err)
	s.Contains(calls, "POST /v1/users/u-1/keycap-sets/set-new/kits/kit-new/image")
	s.Contains(calls, "PUT /blob")
}

func (s *RestoreSuite) TestRestoreKeycapSets_ResumeSkipsKitImageAlreadyPresent() {
	dumpDir, img := s.writeKitDump()
	var calls []string
	srv := s.kitServer(img, true, &calls)
	defer srv.Close()

	client := &apiClient{baseURL: srv.URL, subject: "u-1", http: srv.Client()}
	m := newIDMap(s.T().TempDir(), "u-1")
	m.KeycapSets["set-old"] = &mappedKeycaps{NewID: "set-new", Kits: map[string]string{"kit-old": "kit-new"}}

	err := restoreKeycapSets(context.Background(), client, dumpDir, m)

	s.Require().NoError(err)
	s.Equal([]string{"GET /v1/users/u-1/keycap-sets/set-new"}, calls)
}
