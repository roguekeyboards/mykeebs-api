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
	p := repository.ECPart{PartType: repository.ECPartTypeSprings, Purchases: repository.BulkPurchases{{Vendor: new("NotAVendor"), OrderStatus: new("NotAStatus")}}}

	s.Equal([]lookup.FieldError{
		{Field: "purchases[0].vendor", Value: "NotAVendor", Reason: `"NotAVendor" is not an approved vendor value`},
		{Field: "purchases[0].order_status", Value: "NotAStatus", Reason: `"NotAStatus" is not an approved order_status value`},
	}, lookup.ValidateECPart(s.T().Context(), p))
}
