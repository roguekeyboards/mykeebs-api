package ecpartvalidate_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/ecpartvalidate"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type ValidateSuite struct {
	suite.Suite
}

func TestValidateSuite(t *testing.T) {
	suite.Run(t, new(ValidateSuite))
}

func (s *ValidateSuite) TestTypeWithoutSubObject_NoErrors() {
	for _, t := range repository.ECPartTypes {
		s.Empty(ecpartvalidate.Validate(repository.ECPart{PartType: t}), t)
	}
}

func (s *ValidateSuite) TestEveryTypeWithItsOwnSubObject_NoErrors() {
	for _, p := range []repository.ECPart{
		{PartType: repository.ECPartTypeDomes, Domes: &repository.ECDomes{}},
		{PartType: repository.ECPartTypeSliders, Sliders: &repository.ECSliders{}},
		{PartType: repository.ECPartTypeHousings, Housings: &repository.ECHousings{}},
		{PartType: repository.ECPartTypeSprings, Springs: &repository.ECSprings{}},
		{PartType: repository.ECPartTypeRings, Rings: &repository.ECRings{}},
	} {
		s.Empty(ecpartvalidate.Validate(p), p.PartType)
	}
}

func (s *ValidateSuite) TestOtherTypesSubObjects_EachFlagged() {
	p := repository.ECPart{
		PartType: repository.ECPartTypeDomes,
		Domes:    &repository.ECDomes{},
		Sliders:  &repository.ECSliders{},
		Rings:    &repository.ECRings{},
	}

	s.Equal([]ecpartvalidate.FieldError{
		{Field: "sliders", Reason: "only allowed when part_type is sliders"},
		{Field: "rings", Reason: "only allowed when part_type is rings"},
	}, ecpartvalidate.Validate(p))
}
