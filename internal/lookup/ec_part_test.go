package lookup_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type ValidateECPartSuite struct {
	suite.Suite
}

func TestValidateECPartSuite(t *testing.T) {
	suite.Run(t, new(ValidateECPartSuite))
}

func (s *ValidateECPartSuite) TestOptionalFieldsUnset_SkipsValidation() {
	s.Empty(lookup.ValidateECPart(s.T().Context(), repository.ECPart{PartType: repository.ECPartTypeDomes}))
}

func (s *ValidateECPartSuite) TestApprovedMaterialsForEveryType_ReturnNoErrors() {
	for _, p := range []repository.ECPart{
		{PartType: repository.ECPartTypeDomes, Domes: &repository.ECDomes{Material: new("Silicone")}},
		{PartType: repository.ECPartTypeSliders, Sliders: &repository.ECSliders{Material: new("POM")}},
		{PartType: repository.ECPartTypeHousings, Housings: &repository.ECHousings{Material: new("Polycarbonate")}},
		{PartType: repository.ECPartTypeSprings, Springs: &repository.ECSprings{Material: new("Nickel-Plated")}},
		{PartType: repository.ECPartTypeRings, Rings: &repository.ECRings{Material: new("Poron")}},
	} {
		s.Empty(lookup.ValidateECPart(s.T().Context(), p))
	}
}

func (s *ValidateECPartSuite) TestUnapprovedMaterials_ReturnFieldErrors() {
	for _, tc := range []struct {
		part repository.ECPart
		want lookup.FieldError
	}{
		{
			repository.ECPart{PartType: repository.ECPartTypeDomes, Domes: &repository.ECDomes{Material: new("Steel")}},
			lookup.FieldError{Field: "domes.material", Value: "Steel", Reason: `"Steel" is not an approved ec_dome_material value`},
		},
		{
			repository.ECPart{PartType: repository.ECPartTypeSliders, Sliders: &repository.ECSliders{Material: new("Wood")}},
			lookup.FieldError{Field: "sliders.material", Value: "Wood", Reason: `"Wood" is not an approved switch_material value`},
		},
		{
			repository.ECPart{PartType: repository.ECPartTypeHousings, Housings: &repository.ECHousings{Material: new("Wood")}},
			lookup.FieldError{Field: "housings.material", Value: "Wood", Reason: `"Wood" is not an approved switch_material value`},
		},
		{
			repository.ECPart{PartType: repository.ECPartTypeSprings, Springs: &repository.ECSprings{Material: new("Silicone")}},
			lookup.FieldError{Field: "springs.material", Value: "Silicone", Reason: `"Silicone" is not an approved switch_spring_material value`},
		},
		{
			repository.ECPart{PartType: repository.ECPartTypeRings, Rings: &repository.ECRings{Material: new("POM")}},
			lookup.FieldError{Field: "rings.material", Value: "POM", Reason: `"POM" is not an approved ec_ring_material value`},
		},
	} {
		s.Equal([]lookup.FieldError{tc.want}, lookup.ValidateECPart(s.T().Context(), tc.part))
	}
}

func (s *ValidateECPartSuite) TestUnapprovedPurchaseValues_ReturnFieldErrors() {
	p := repository.ECPart{PartType: repository.ECPartTypeSprings, Purchase: repository.ECPartPurchase{Vendor: new("NotAVendor"), OrderStatus: new("NotAStatus")}}

	s.Equal([]lookup.FieldError{
		{Field: "purchase.vendor", Value: "NotAVendor", Reason: `"NotAVendor" is not an approved vendor value`},
		{Field: "purchase.order_status", Value: "NotAStatus", Reason: `"NotAStatus" is not an approved order_status value`},
	}, lookup.ValidateECPart(s.T().Context(), p))
}

func fieldNames(errs []lookup.FieldError) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Field
	}
	return out
}

func (s *ValidateECPartSuite) TestStructure_TypeWithoutSubObject_NoErrors() {
	for _, t := range repository.ECPartTypes {
		s.Empty(lookup.ValidateECPart(s.T().Context(), repository.ECPart{PartType: t}), t)
	}
}

func (s *ValidateECPartSuite) TestStructure_EveryTypeWithItsOwnSubObject_NoErrors() {
	for _, p := range []repository.ECPart{
		{PartType: repository.ECPartTypeDomes, Domes: &repository.ECDomes{Weight: new(45.0)}},
		{PartType: repository.ECPartTypeSliders, Sliders: &repository.ECSliders{Size: new(repository.ECPartSize1U), Stem: new(repository.ECSliderStemMX)}},
		{PartType: repository.ECPartTypeHousings, Housings: &repository.ECHousings{Size: new(repository.ECPartSize2U)}},
		{PartType: repository.ECPartTypeSprings, Springs: &repository.ECSprings{Material: new("Nickel-Plated")}},
		{PartType: repository.ECPartTypeRings, Rings: &repository.ECRings{Thickness: new(0.5)}},
	} {
		s.Empty(lookup.ValidateECPart(s.T().Context(), p), p.PartType)
	}
}

func (s *ValidateECPartSuite) TestStructure_UnknownPartType_Flagged() {
	s.Equal([]string{"part_type"}, fieldNames(lookup.ValidateECPart(s.T().Context(), repository.ECPart{PartType: "keycaps"})))
}

func (s *ValidateECPartSuite) TestStructure_MissingPartType_Flagged() {
	s.Equal([]string{"part_type"}, fieldNames(lookup.ValidateECPart(s.T().Context(), repository.ECPart{})))
}

func (s *ValidateECPartSuite) TestStructure_OtherTypesSubObjects_EachFlagged() {
	p := repository.ECPart{
		PartType: repository.ECPartTypeDomes,
		Domes:    &repository.ECDomes{},
		Sliders:  &repository.ECSliders{},
		Rings:    &repository.ECRings{},
	}

	s.Equal([]string{"sliders", "rings"}, fieldNames(lookup.ValidateECPart(s.T().Context(), p)))
}

func (s *ValidateECPartSuite) TestStructure_UnknownSizesAndStem_Flagged() {
	sliders := repository.ECPart{
		PartType: repository.ECPartTypeSliders,
		Sliders:  &repository.ECSliders{Size: new(repository.ECPartSize("3U")), Stem: new(repository.ECSliderStem("Alps"))},
	}
	housings := repository.ECPart{
		PartType: repository.ECPartTypeHousings,
		Housings: &repository.ECHousings{Size: new(repository.ECPartSize("1u"))},
	}

	s.Equal([]string{"sliders.size", "sliders.stem"}, fieldNames(lookup.ValidateECPart(s.T().Context(), sliders)))
	s.Equal([]string{"housings.size"}, fieldNames(lookup.ValidateECPart(s.T().Context(), housings)))
}

func (s *ValidateECPartSuite) TestStructure_Reasons_ListTheAllowedValues() {
	p := repository.ECPart{
		PartType: repository.ECPartTypeSliders,
		Sliders:  &repository.ECSliders{Size: new(repository.ECPartSize("3U")), Stem: new(repository.ECSliderStem("Alps"))},
	}

	s.Equal([]lookup.FieldError{
		{Field: "sliders.size", Value: "3U", Reason: "must be one of: 1U, 2U"},
		{Field: "sliders.stem", Value: "Alps", Reason: "must be one of: MX, OEM"},
	}, lookup.ValidateECPart(s.T().Context(), p))
	s.Equal([]lookup.FieldError{
		{Field: "part_type", Value: "keycaps", Reason: "must be one of: domes, sliders, housings, springs, rings"},
	}, lookup.ValidateECPart(s.T().Context(), repository.ECPart{PartType: "keycaps"}))
}

func (s *ValidateECPartSuite) TestValidateECPartType() {
	s.Empty(lookup.ValidateECPartType(repository.ECPartTypeRings))
	s.Equal([]lookup.FieldError{
		{Field: "part_type", Value: "keycaps", Reason: "must be one of: domes, sliders, housings, springs, rings"},
	}, lookup.ValidateECPartType("keycaps"))
}
