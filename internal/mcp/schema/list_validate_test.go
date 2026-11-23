package schema_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"
)

type ListValidateSuite struct {
	suite.Suite
}

func TestListValidateSuite(t *testing.T) {
	suite.Run(t, new(ListValidateSuite))
}

func listInputs(limit *int) map[string]validate.Validatable {
	return map[string]validate.Validatable{
		"switches":    schema.ListSwitchesInput{Limit: limit},
		"keyboards":   schema.ListKeyboardsInput{Limit: limit},
		"keycap sets": schema.ListKeycapSetsInput{Limit: limit},
		"ec parts":    schema.ListECPartsInput{Limit: limit},
		"builds":      schema.ListBuildsInput{Limit: limit},
		"profiles":    schema.ListProfilesInput{Limit: limit},
	}
}

func (s *ListValidateSuite) TestUnsetOrInRangeLimit_NoErrors() {
	for _, limit := range []*int{nil, new(1), new(100)} {
		for name, in := range listInputs(limit) {
			s.Empty(validate.Check(in), name)
		}
	}
}

func (s *ListValidateSuite) TestLimitOutOfRange_Rejected() {
	for _, limit := range []int{0, -1, 101} {
		for name, in := range listInputs(&limit) {
			s.Equal([]validate.FieldError{{Field: "limit", Reason: "must be between 1 and 100"}}, validate.Check(in), name)
		}
	}
}
