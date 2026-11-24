package schema_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"
)

type SwitchValidateSuite struct {
	suite.Suite
}

func TestSwitchValidateSuite(t *testing.T) {
	suite.Run(t, new(SwitchValidateSuite))
}

func validSwitchInput() schema.SwitchInput {
	return schema.SwitchInput{Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: "private"}
}

func (s *SwitchValidateSuite) TestValidInput_NoErrors() {
	in := validSwitchInput()
	in.Pins = new(5)
	in.Force = &schema.SwitchForce{Actuation: new(0.0), BottomOut: new(67.0)}
	in.Spring = &schema.SwitchSpring{PreTravel: new(2.0), TotalTravel: new(4.0), Length: new(22.0)}
	in.Purchases = []schema.SwitchPurchaseInput{{Price: new(0.0), Quantity: new(1), OrderDate: new("2026-09-21")}}

	s.Empty(validate.Check(schema.CreateSwitchInput{SwitchInput: in}))
	s.Empty(validate.Check(schema.UpdateSwitchInput{SwitchID: "sw-1", SwitchInput: in}))
}

func (s *SwitchValidateSuite) TestInvalidInput_ReportsEveryField() {
	in := schema.SwitchInput{
		Brand:      " ",
		Pins:       new(0),
		Force:      &schema.SwitchForce{Actuation: new(-1.0), BottomOut: new(-1.0)},
		Spring:     &schema.SwitchSpring{PreTravel: new(-1.0), TotalTravel: new(-1.0), Length: new(-1.0)},
		Purchases:  []schema.SwitchPurchaseInput{{Price: new(-1.0), Quantity: new(0), OrderDate: new("21/09/2026"), DeliveryDate: new("soon")}},
		Visibility: "everyone",
	}

	s.Equal([]validate.FieldError{
		{Field: "switch_id", Reason: "must not be blank"},
		{Field: "brand", Reason: "must not be blank"},
		{Field: "name", Reason: "must not be blank"},
		{Field: "pins", Reason: "must be at least 1"},
		{Field: "force.actuation", Reason: "must be at least 0"},
		{Field: "force.bottom_out", Reason: "must be at least 0"},
		{Field: "spring.pre_travel", Reason: "must be at least 0"},
		{Field: "spring.total_travel", Reason: "must be at least 0"},
		{Field: "spring.length", Reason: "must be at least 0"},
		{Field: "purchases[0].price", Reason: "must be at least 0"},
		{Field: "purchases[0].order_date", Reason: `"21/09/2026" must be a date in YYYY-MM-DD form`},
		{Field: "purchases[0].delivery_date", Reason: `"soon" must be a date in YYYY-MM-DD form`},
		{Field: "purchases[0].quantity", Reason: "must be at least 1"},
		{Field: "visibility", Reason: "must be one of: public, authenticated, private"},
	}, validate.Check(schema.UpdateSwitchInput{SwitchInput: in}))
}

func (s *SwitchValidateSuite) TestIDInputs_RequireTheID() {
	for name, in := range map[string]validate.Validatable{
		"get":          schema.GetSwitchInput{},
		"delete":       schema.DeleteSwitchInput{},
		"delete image": schema.DeleteSwitchImageInput{},
	} {
		s.Equal([]validate.FieldError{{Field: "switch_id", Reason: "must not be blank"}}, validate.Check(in), name)
	}
}

func (s *SwitchValidateSuite) TestDelete_RejectsUnknownOnDelete() {
	s.Empty(validate.Check(schema.DeleteSwitchInput{SwitchID: "sw-1", OnDelete: "cascade"}))
	s.Equal([]validate.FieldError{{Field: "on_delete", Reason: "must be block or cascade"}},
		validate.Check(schema.DeleteSwitchInput{SwitchID: "sw-1", OnDelete: "detach"}))
}

func (s *SwitchValidateSuite) TestSetImage_RejectsSizeOutsideTheCap() {
	s.Empty(validate.Check(schema.SetSwitchImageInput{SwitchID: "sw-1", SizeBytes: 1}))
	for _, size := range []int64{0, 1536*1024 + 1} {
		errs := validate.Check(schema.SetSwitchImageInput{SwitchID: "sw-1", SizeBytes: size})
		s.Require().Len(errs, 1, size)
		s.Equal("size_bytes", errs[0].Field, size)
	}
}
