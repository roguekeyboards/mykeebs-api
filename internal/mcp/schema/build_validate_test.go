package schema_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"
)

type BuildValidateSuite struct {
	suite.Suite
}

func TestBuildValidateSuite(t *testing.T) {
	suite.Run(t, new(BuildValidateSuite))
}

func (s *BuildValidateSuite) TestValidInput_NoErrors() {
	in := schema.BuildInput{
		Keyboard:   "kb-1",
		Stabs:      &schema.BuildStabsInput{Price: new(0.0)},
		Switches:   []schema.BuildSwitchEntry{{Switch: "sw-1", Count: 1}},
		ECParts:    []schema.BuildECPartEntry{{ECPart: "p-1", Count: 60}},
		KeycapKits: []schema.BuildKeycapKitEntry{{KeycapSet: "ks-1", Kit: "base"}},
		BuildDate:  new("2026-11-22"),
		Visibility: "authenticated",
	}

	s.Empty(validate.Check(schema.CreateBuildInput{BuildInput: in}))
	s.Empty(validate.Check(schema.UpdateBuildInput{BuildID: "b-1", BuildInput: in}))
}

func (s *BuildValidateSuite) TestInvalidInput_ReportsEveryField() {
	in := schema.BuildInput{
		Keyboard:   " ",
		Stabs:      &schema.BuildStabsInput{Price: new(-1.0)},
		Switches:   []schema.BuildSwitchEntry{{Switch: "sw-1", Count: 1}, {Switch: "", Count: 0}},
		ECParts:    []schema.BuildECPartEntry{{ECPart: " ", Count: -1}},
		KeycapKits: []schema.BuildKeycapKitEntry{{KeycapSet: "ks-1", Kit: "base"}, {KeycapSet: "", Kit: " "}},
		BuildDate:  new("22/11/2026"),
		Visibility: "",
	}

	s.Equal([]validate.FieldError{
		{Field: "build_id", Reason: "must not be blank"},
		{Field: "keyboard", Reason: "must not be blank"},
		{Field: "stabs.price", Reason: "must be at least 0"},
		{Field: "switches[1].switch", Reason: "must not be blank"},
		{Field: "switches[1].count", Reason: "must be at least 1"},
		{Field: "ec_parts[0].ec_part", Reason: "must not be blank"},
		{Field: "ec_parts[0].count", Reason: "must be at least 1"},
		{Field: "keycap_kits[1].keycap_set", Reason: "must not be blank"},
		{Field: "keycap_kits[1].kit", Reason: "must not be blank"},
		{Field: "build_date", Reason: `"22/11/2026" must be a date in YYYY-MM-DD form`},
		{Field: "visibility", Reason: "must be one of: public, authenticated, private"},
	}, validate.Check(schema.UpdateBuildInput{BuildInput: in}))
}

func (s *BuildValidateSuite) TestIDInputs_RequireTheIDs() {
	id := validate.FieldError{Field: "build_id", Reason: "must not be blank"}

	for name, tc := range map[string]struct {
		in   validate.Validatable
		want []validate.FieldError
	}{
		"get":          {schema.GetBuildInput{}, []validate.FieldError{id}},
		"delete":       {schema.DeleteBuildInput{}, []validate.FieldError{id}},
		"list images":  {schema.ListBuildImagesInput{}, []validate.FieldError{id}},
		"delete image": {schema.DeleteBuildImageInput{}, []validate.FieldError{id, {Field: "image_id", Reason: "must not be blank"}}},
		"add image": {schema.AddBuildImageInput{SizeBytes: 1536*1024 + 1}, []validate.FieldError{
			id, {Field: "size_bytes", Reason: "must be between 1 and 1572864 (1.5 MB); resize or recompress the image first"},
		}},
	} {
		s.Equal(tc.want, validate.Check(tc.in), name)
	}
}
