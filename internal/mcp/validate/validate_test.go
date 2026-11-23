package validate_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"
)

type ValidateSuite struct {
	suite.Suite
}

func TestValidateSuite(t *testing.T) {
	suite.Run(t, new(ValidateSuite))
}

type rules func(v *validate.V)

func (r rules) Validate(v *validate.V) { r(v) }

func (s *ValidateSuite) TestValuesWithinRules_NoErrors() {
	s.Empty(validate.Check(rules(func(v *validate.V) {
		v.NotBlank("brand", "Gateron")
		v.Date("order_date", new("2026-09-21"))
		v.Date("delivery_date", nil)
		v.Money("price", new(0.0))
		v.Money("max", new(float64(validate.MaxMoney)))
		v.NonNegative("weight", new(0.0))
		v.Positive("count", new(1))
		v.Positive("unset", nil)
		validate.OneOf(v, "size", "1U", []string{"1U", "2U"})
	})))
}

func (s *ValidateSuite) TestValuesBreakingRules_ReportEveryField() {
	s.Equal([]validate.FieldError{
		{Field: "brand", Reason: "must not be blank"},
		{Field: "order_date", Reason: `"21/09/2026" must be a date in YYYY-MM-DD form`},
		{Field: "price", Reason: "must be at least 0"},
		{Field: "max", Reason: "must be at most 100000000000000"},
		{Field: "weight", Reason: "must be at least 0"},
		{Field: "count", Reason: "must be at least 1"},
		{Field: "size", Reason: "must be one of: 1U, 2U"},
	}, validate.Check(rules(func(v *validate.V) {
		v.NotBlank("brand", "  ")
		v.Date("order_date", new("21/09/2026"))
		v.Money("price", new(-0.01))
		v.Money("max", new(float64(validate.MaxMoney)+1))
		v.NonNegative("weight", new(-1.0))
		v.Positive("count", new(0))
		validate.OneOf(v, "size", "3U", []string{"1U", "2U"})
	})))
}

func (s *ValidateSuite) TestNestedFields_ArePrefixedWithTheirPath() {
	s.Equal([]validate.FieldError{
		{Field: "purchase.price", Reason: "must be at least 0"},
		{Field: "switches[1].count", Reason: "must be at least 1"},
	}, validate.Check(rules(func(v *validate.V) {
		v.At("purchase").Money("price", new(-1.0))
		v.Index("switches", 1).Positive("count", new(0))
	})))
}

func (s *ValidateSuite) TestError_JoinsEveryFieldError() {
	s.Require().NoError(validate.Error(rules(func(*validate.V) {})))

	s.EqualError(validate.Error(rules(func(v *validate.V) {
		v.NotBlank("brand", "")
		v.NotBlank("name", "")
	})), "brand: must not be blank; name: must not be blank")
}
