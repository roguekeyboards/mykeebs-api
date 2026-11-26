package schema_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"
)

type KeyboardValidateSuite struct {
	suite.Suite
}

func TestKeyboardValidateSuite(t *testing.T) {
	suite.Run(t, new(KeyboardValidateSuite))
}

func (s *KeyboardValidateSuite) TestValidInput_NoErrors() {
	in := schema.KeyboardInput{
		Brand: "Mode", Name: "Sonnet", Visibility: "public",
		Purchase: &schema.KeyboardPurchaseInput{Price: new(450.0), DeliveryDate: new("2026-10-01")},
	}

	s.Empty(validate.Check(schema.CreateKeyboardInput{KeyboardInput: in}))
	s.Empty(validate.Check(schema.UpdateKeyboardInput{KeyboardID: "kb-1", KeyboardInput: in}))
}

func (s *KeyboardValidateSuite) TestInvalidInput_ReportsEveryField() {
	in := schema.KeyboardInput{
		Purchase:   &schema.KeyboardPurchaseInput{Price: new(-1.0), OrderDate: new("2026-13-01")},
		Visibility: "friends",
	}

	s.Equal([]validate.FieldError{
		{Field: "keyboard_id", Reason: "must not be blank"},
		{Field: "brand", Reason: "must not be blank"},
		{Field: "name", Reason: "must not be blank"},
		{Field: "purchase.price", Reason: "must be at least 0"},
		{Field: "purchase.order_date", Reason: `"2026-13-01" must be a date in YYYY-MM-DD form`},
		{Field: "visibility", Reason: "must be one of: public, authenticated, private"},
	}, validate.Check(schema.UpdateKeyboardInput{KeyboardInput: in}))
}

func (s *KeyboardValidateSuite) TestIDInputs_RequireTheIDs() {
	for name, tc := range map[string]struct {
		in   validate.Validatable
		want []validate.FieldError
	}{
		"get":         {schema.GetKeyboardInput{}, []validate.FieldError{{Field: "keyboard_id", Reason: "must not be blank"}}},
		"list images": {schema.ListKeyboardImagesInput{}, []validate.FieldError{{Field: "keyboard_id", Reason: "must not be blank"}}},
		"delete image": {schema.DeleteKeyboardImageInput{}, []validate.FieldError{
			{Field: "keyboard_id", Reason: "must not be blank"},
			{Field: "image_id", Reason: "must not be blank"},
		}},
		"delete": {schema.DeleteKeyboardInput{OnDelete: "detach"}, []validate.FieldError{
			{Field: "keyboard_id", Reason: "must not be blank"},
			{Field: "on_delete", Reason: "must be block or cascade"},
		}},
		"add image": {schema.AddKeyboardImageInput{}, []validate.FieldError{
			{Field: "keyboard_id", Reason: "must not be blank"},
			{Field: "size_bytes", Reason: "must be between 1 and 1572864 (1.5 MB); resize or recompress the image first"},
		}},
	} {
		s.Equal(tc.want, validate.Check(tc.in), name)
	}
}

func (s *KeyboardValidateSuite) TestPlateAndPCB_ValidInput_NoErrors() {
	purchase := &schema.KeyboardPartPurchaseInput{Price: new(0.0), OrderDate: new("2026-09-21")}

	s.Empty(validate.Check(schema.CreateKeyboardPlateInput{KeyboardID: "kb-1", KeyboardPlateInput: schema.KeyboardPlateInput{Material: "AL", Thickness: new(0.0), Purchase: purchase}}))
	s.Empty(validate.Check(schema.CreateKeyboardPCBInput{KeyboardID: "kb-1", KeyboardPCBInput: schema.KeyboardPCBInput{Thickness: new(1.6), Purchase: purchase}}))
}

func (s *KeyboardValidateSuite) TestPlate_InvalidInput_ReportsEveryField() {
	s.Equal([]validate.FieldError{
		{Field: "keyboard_id", Reason: "must not be blank"},
		{Field: "plate_id", Reason: "must not be blank"},
		{Field: "material", Reason: "must not be blank"},
		{Field: "thickness", Reason: "must be at least 0"},
		{Field: "purchase.price", Reason: "must be at most 100000000000000"},
		{Field: "purchase.delivery_date", Reason: `"x" must be a date in YYYY-MM-DD form`},
		{Field: "purchase.estimated_delivery_date", Reason: `"Q4" must be a date in YYYY-MM-DD form`},
	}, validate.Check(schema.UpdateKeyboardPlateInput{KeyboardPlateInput: schema.KeyboardPlateInput{
		Thickness: new(-1.5),
		Purchase:  &schema.KeyboardPartPurchaseInput{Price: new(float64(validate.MaxMoney) + 1), DeliveryDate: new("x"), EstimatedDeliveryDate: new("Q4")},
	}}))
}

func (s *KeyboardValidateSuite) TestPCB_InvalidInput_ReportsEveryField() {
	s.Equal([]validate.FieldError{
		{Field: "keyboard_id", Reason: "must not be blank"},
		{Field: "pcb_id", Reason: "must not be blank"},
		{Field: "thickness", Reason: "must be at least 0"},
		{Field: "purchase.price", Reason: "must be at least 0"},
	}, validate.Check(schema.UpdateKeyboardPCBInput{KeyboardPCBInput: schema.KeyboardPCBInput{
		Thickness: new(-1.6),
		Purchase:  &schema.KeyboardPartPurchaseInput{Price: new(-5.0)},
	}}))
}

func (s *KeyboardValidateSuite) TestDeleteParts_RequireIDsAndAKnownOnDelete() {
	want := func(id string) []validate.FieldError {
		return []validate.FieldError{
			{Field: "keyboard_id", Reason: "must not be blank"},
			{Field: id, Reason: "must not be blank"},
			{Field: "on_delete", Reason: "must be block or cascade"},
		}
	}

	s.Equal(want("plate_id"), validate.Check(schema.DeleteKeyboardPlateInput{OnDelete: "detach"}))
	s.Equal(want("pcb_id"), validate.Check(schema.DeleteKeyboardPCBInput{OnDelete: "detach"}))
	s.Equal([]validate.FieldError{{Field: "keyboard_id", Reason: "must not be blank"}},
		validate.Check(schema.CreateKeyboardPCBInput{}))
}
