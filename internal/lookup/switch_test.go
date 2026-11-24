package lookup_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type ValidateSwitchSuite struct {
	suite.Suite
}

func TestValidateSwitchSuite(t *testing.T) {
	suite.Run(t, new(ValidateSwitchSuite))
}

func (s *ValidateSwitchSuite) TestOptionalFieldsUnset_SkipsValidation() {
	sw := repository.Switch{Type: "Linear"}

	errs := lookup.ValidateSwitch(s.T().Context(), sw)
	s.Empty(errs)
}

func (s *ValidateSwitchSuite) TestValidFields_ReturnsNoErrors() {
	vendor := "Amazon"
	sw := repository.Switch{
		Type:      "Linear",
		Purchases: repository.BulkPurchases{{Vendor: &vendor}},
	}

	errs := lookup.ValidateSwitch(s.T().Context(), sw)
	s.Empty(errs)
}

func (s *ValidateSwitchSuite) TestInvalidType_ReturnsFieldError() {
	sw := repository.Switch{Type: "NotAType"}

	errs := lookup.ValidateSwitch(s.T().Context(), sw)
	s.Equal([]lookup.FieldError{
		{Field: "type", Value: "NotAType", Reason: `"NotAType" is not an approved switch_type value`},
	}, errs)
}

func (s *ValidateSwitchSuite) TestInvalidOrderStatus_ReturnsFieldError() {
	status := "NotAStatus"
	sw := repository.Switch{
		Type:      "Linear",
		Purchases: repository.BulkPurchases{{OrderStatus: &status}},
	}

	errs := lookup.ValidateSwitch(s.T().Context(), sw)
	s.Equal([]lookup.FieldError{
		{Field: "purchases[0].order_status", Value: "NotAStatus", Reason: `"NotAStatus" is not an approved order_status value`},
	}, errs)
}
