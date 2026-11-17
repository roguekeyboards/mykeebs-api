package lookup_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type ValidateKeyboardSuite struct {
	suite.Suite
}

func TestValidateKeyboardSuite(t *testing.T) {
	suite.Run(t, new(ValidateKeyboardSuite))
}

func (s *ValidateKeyboardSuite) TestAllFieldsUnset_SkipsValidation() {
	kb := repository.Keyboard{}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Empty(errs)
}

func (s *ValidateKeyboardSuite) TestInvalidSize_ReturnsFieldError() {
	size := "not-a-size"
	kb := repository.Keyboard{Size: &size}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Equal([]lookup.FieldError{
		{Field: "size", Value: "not-a-size", Reason: `"not-a-size" is not an approved keyboard_size value`},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestValidSizeAndLayout_ReturnsNoErrors() {
	size := "60%"
	layoutName := "WK"
	kb := repository.Keyboard{Size: &size, Layout: &layoutName}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Empty(errs)
}

func (s *ValidateKeyboardSuite) TestLayoutNotValidForSize_ReturnsFieldError() {
	size := "40%"
	layoutName := "WK"
	kb := repository.Keyboard{Size: &size, Layout: &layoutName}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Equal([]lookup.FieldError{
		{Field: "layout", Value: "WK", Reason: `"WK" is not a valid layout for size "40%"`},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestNumpadWithRowLayout_ReturnsNoErrors() {
	size := "Numpad"
	for _, layoutName := range []string{"5 Row", "6 Row"} {
		kb := repository.Keyboard{Size: &size, Layout: &layoutName}

		s.Empty(lookup.ValidateKeyboard(s.T().Context(), kb), layoutName)
	}
}

func (s *ValidateKeyboardSuite) TestNumpadWithFullSizeLayout_ReturnsFieldError() {
	size := "Numpad"
	layoutName := "WK"
	kb := repository.Keyboard{Size: &size, Layout: &layoutName}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Equal([]lookup.FieldError{
		{Field: "layout", Value: "WK", Reason: `"WK" is not a valid layout for size "Numpad"`},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestRowLayoutWithNonNumpadSize_ReturnsFieldError() {
	size := "60%"
	layoutName := "5 Row"
	kb := repository.Keyboard{Size: &size, Layout: &layoutName}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Equal([]lookup.FieldError{
		{Field: "layout", Value: "5 Row", Reason: `"5 Row" is not a valid layout for size "60%"`},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestInvalidSize_SkipsLayoutSizeCrossCheck() {
	size := "not-a-size"
	layoutName := "WK"
	kb := repository.Keyboard{Size: &size, Layout: &layoutName}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Equal([]lookup.FieldError{
		{Field: "size", Value: "not-a-size", Reason: `"not-a-size" is not an approved keyboard_size value`},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestInvalidLayoutName_ReturnsFieldError() {
	layoutName := "NotALayout"
	kb := repository.Keyboard{Layout: &layoutName}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Equal([]lookup.FieldError{
		{Field: "layout", Value: "NotALayout", Reason: `"NotALayout" is not an approved keyboard_layout value`},
	}, errs)
}

type ValidateKeyboardPartsSuite struct {
	suite.Suite
}

func TestValidateKeyboardPartsSuite(t *testing.T) {
	suite.Run(t, new(ValidateKeyboardPartsSuite))
}

func (s *ValidateKeyboardPartsSuite) TestValidPlate_ReturnsNoErrors() {
	s.Empty(lookup.ValidateKeyboardPlate(s.T().Context(), repository.KeyboardPlate{Material: "AL"}))
}

func (s *ValidateKeyboardPartsSuite) TestInvalidPlate_ReturnsFieldErrors() {
	bad := "NotAValue"
	plate := repository.KeyboardPlate{
		Material: "NotAMaterial",
		Purchase: repository.KeyboardPurchase{Vendor: &bad, OrderStatus: &bad},
	}

	errs := lookup.ValidateKeyboardPlate(s.T().Context(), plate)
	s.ElementsMatch([]lookup.FieldError{
		{Field: "material", Value: "NotAMaterial", Reason: `"NotAMaterial" is not an approved keyboard_plate_material value`},
		{Field: "purchase.vendor", Value: bad, Reason: `"NotAValue" is not an approved vendor value`},
		{Field: "purchase.order_status", Value: bad, Reason: `"NotAValue" is not an approved order_status value`},
	}, errs)
}

func (s *ValidateKeyboardPartsSuite) TestUnsetPCBFields_SkipsValidation() {
	s.Empty(lookup.ValidateKeyboardPCB(s.T().Context(), repository.KeyboardPCB{}))
}

func (s *ValidateKeyboardPartsSuite) TestInvalidPCB_ReturnsFieldErrors() {
	bad := "NotAValue"
	pcb := repository.KeyboardPCB{
		Firmware: &bad, Assembly: &bad, Connectivity: &bad,
		Purchase: repository.KeyboardPurchase{OrderStatus: &bad},
	}

	errs := lookup.ValidateKeyboardPCB(s.T().Context(), pcb)
	s.ElementsMatch([]lookup.FieldError{
		{Field: "firmware", Value: bad, Reason: `"NotAValue" is not an approved keyboard_pcb_firmware value`},
		{Field: "assembly", Value: bad, Reason: `"NotAValue" is not an approved keyboard_pcb_assembly_type value`},
		{Field: "connectivity", Value: bad, Reason: `"NotAValue" is not an approved keyboard_pcb_connectivity_type value`},
		{Field: "purchase.order_status", Value: bad, Reason: `"NotAValue" is not an approved order_status value`},
	}, errs)
}

func (s *ValidateKeyboardPartsSuite) TestValidPCB_ReturnsNoErrors() {
	v, a, c := "QMK/VIA", "EC", "Tri-Mode"
	s.Empty(lookup.ValidateKeyboardPCB(s.T().Context(), repository.KeyboardPCB{Firmware: &v, Assembly: &a, Connectivity: &c}))
}
