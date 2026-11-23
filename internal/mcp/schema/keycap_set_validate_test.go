package schema_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"
)

type KeycapSetValidateSuite struct {
	suite.Suite
}

func TestKeycapSetValidateSuite(t *testing.T) {
	suite.Run(t, new(KeycapSetValidateSuite))
}

func (s *KeycapSetValidateSuite) TestValidInput_NoErrors() {
	set := schema.KeycapSetInput{Brand: "GMK", Name: "Olivia", Visibility: "public"}
	kit := schema.KeycapKitInput{Name: "Base", Purchase: &schema.KeycapKitPurchaseInput{Price: new(140.0), OrderDate: new("2026-01-05")}}

	s.Empty(validate.Check(schema.CreateKeycapSetInput{KeycapSetInput: set}))
	s.Empty(validate.Check(schema.UpdateKeycapSetInput{KeycapSetID: "ks-1", KeycapSetInput: set}))
	s.Empty(validate.Check(schema.CreateKeycapKitInput{KeycapSetID: "ks-1", KeycapKitInput: kit}))
	s.Empty(validate.Check(schema.UpdateKeycapKitInput{KeycapSetID: "ks-1", KitID: "k-1", KeycapKitInput: kit}))
}

func (s *KeycapSetValidateSuite) TestInvalidSet_ReportsEveryField() {
	s.Equal([]validate.FieldError{
		{Field: "keycap_set_id", Reason: "must not be blank"},
		{Field: "brand", Reason: "must not be blank"},
		{Field: "name", Reason: "must not be blank"},
		{Field: "visibility", Reason: "must be one of: public, authenticated, private"},
	}, validate.Check(schema.UpdateKeycapSetInput{}))
}

func (s *KeycapSetValidateSuite) TestInvalidKit_ReportsEveryField() {
	s.Equal([]validate.FieldError{
		{Field: "keycap_set_id", Reason: "must not be blank"},
		{Field: "kit_id", Reason: "must not be blank"},
		{Field: "name", Reason: "must not be blank"},
		{Field: "purchase.price", Reason: "must be at least 0"},
		{Field: "purchase.order_date", Reason: `"Jan 5" must be a date in YYYY-MM-DD form`},
	}, validate.Check(schema.UpdateKeycapKitInput{KeycapKitInput: schema.KeycapKitInput{
		Purchase: &schema.KeycapKitPurchaseInput{Price: new(-1.0), OrderDate: new("Jan 5")},
	}}))
}

func (s *KeycapSetValidateSuite) TestIDInputs_RequireTheIDs() {
	setID := validate.FieldError{Field: "keycap_set_id", Reason: "must not be blank"}
	kitID := validate.FieldError{Field: "kit_id", Reason: "must not be blank"}
	onDelete := validate.FieldError{Field: "on_delete", Reason: "must be block or cascade"}
	size := validate.FieldError{Field: "size_bytes", Reason: "must be between 1 and 1572864 (1.5 MB); resize or recompress the image first"}

	for name, tc := range map[string]struct {
		in   validate.Validatable
		want []validate.FieldError
	}{
		"get":          {schema.GetKeycapSetInput{}, []validate.FieldError{setID}},
		"delete set":   {schema.DeleteKeycapSetInput{OnDelete: "detach"}, []validate.FieldError{setID, onDelete}},
		"delete kit":   {schema.DeleteKeycapKitInput{OnDelete: "detach"}, []validate.FieldError{setID, kitID, onDelete}},
		"set image":    {schema.SetKeycapKitImageInput{}, []validate.FieldError{setID, kitID, size}},
		"delete image": {schema.DeleteKeycapKitImageInput{}, []validate.FieldError{setID, kitID}},
	} {
		s.Equal(tc.want, validate.Check(tc.in), name)
	}
}
