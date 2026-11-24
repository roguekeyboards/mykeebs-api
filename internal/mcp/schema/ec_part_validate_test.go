package schema_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"
)

type ECPartValidateSuite struct {
	suite.Suite
}

func TestECPartValidateSuite(t *testing.T) {
	suite.Run(t, new(ECPartValidateSuite))
}

func (s *ECPartValidateSuite) TestValidInputForEveryType_NoErrors() {
	purchases := []schema.ECPartPurchaseInput{{Price: new(30.0), Quantity: new(90), OrderDate: new("2026-11-01")}}

	for _, in := range []schema.ECPartInput{
		{PartType: "domes", Domes: &schema.ECDomes{Weight: new(0.0)}},
		{PartType: "sliders", Sliders: &schema.ECSliders{Size: new("1U"), Stem: new("MX")}},
		{PartType: "housings", Housings: &schema.ECHousings{Size: new("2U")}},
		{PartType: "springs", Springs: &schema.ECSprings{}},
		{PartType: "rings", Rings: &schema.ECRings{Thickness: new(0.0)}},
	} {
		in.Brand, in.Name, in.Visibility, in.Purchases = "Deskthority", "Part", "public", purchases
		s.Empty(validate.Check(schema.CreateECPartInput{ECPartInput: in}), in.PartType)
	}
}

func (s *ECPartValidateSuite) TestInvalidInput_ReportsEveryField() {
	in := schema.ECPartInput{
		PartType:   "keycaps",
		Domes:      &schema.ECDomes{Weight: new(-45.0)},
		Sliders:    &schema.ECSliders{Size: new("3U"), Stem: new("Alps")},
		Housings:   &schema.ECHousings{Size: new("1u")},
		Rings:      &schema.ECRings{Thickness: new(-0.2)},
		Purchases:  []schema.ECPartPurchaseInput{{Price: new(-1.0), Quantity: new(0)}},
		Visibility: "friends",
	}

	s.Equal([]validate.FieldError{
		{Field: "ec_part_id", Reason: "must not be blank"},
		{Field: "brand", Reason: "must not be blank"},
		{Field: "name", Reason: "must not be blank"},
		{Field: "part_type", Reason: "must be one of: domes, sliders, housings, springs, rings"},
		{Field: "domes.weight", Reason: "must be at least 0"},
		{Field: "sliders.size", Reason: "must be one of: 1U, 2U"},
		{Field: "sliders.stem", Reason: "must be one of: MX, OEM"},
		{Field: "housings.size", Reason: "must be one of: 1U, 2U"},
		{Field: "rings.thickness", Reason: "must be at least 0"},
		{Field: "purchases[0].price", Reason: "must be at least 0"},
		{Field: "purchases[0].quantity", Reason: "must be at least 1"},
		{Field: "visibility", Reason: "must be one of: public, authenticated, private"},
	}, validate.Check(schema.UpdateECPartInput{ECPartInput: in}))
}

func (s *ECPartValidateSuite) TestListFilter_UnsetOrKnownPartTypeOnly() {
	s.Empty(validate.Check(schema.ListECPartsInput{}))
	s.Empty(validate.Check(schema.ListECPartsInput{PartType: "rings"}))
	s.Equal([]validate.FieldError{{Field: "part_type", Reason: "must be one of: domes, sliders, housings, springs, rings"}},
		validate.Check(schema.ListECPartsInput{PartType: "keycaps"}))
}

func (s *ECPartValidateSuite) TestIDInputs_RequireTheID() {
	id := validate.FieldError{Field: "ec_part_id", Reason: "must not be blank"}

	for name, tc := range map[string]struct {
		in   validate.Validatable
		want []validate.FieldError
	}{
		"get":          {schema.GetECPartInput{}, []validate.FieldError{id}},
		"delete":       {schema.DeleteECPartInput{OnDelete: "detach"}, []validate.FieldError{id, {Field: "on_delete", Reason: "must be block or cascade"}}},
		"set image":    {schema.SetECPartImageInput{SizeBytes: 1}, []validate.FieldError{id}},
		"delete image": {schema.DeleteECPartImageInput{}, []validate.FieldError{id}},
	} {
		s.Equal(tc.want, validate.Check(tc.in), name)
	}
}
