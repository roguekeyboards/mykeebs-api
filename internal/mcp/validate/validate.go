// Package validate checks MCP tool input against the rules api/openapi.yaml
// states, which REST's request validator enforces before a handler runs and
// MCP has no equivalent for. Each input type in
// [github.com/roguekeyboards/mykeebs-api/internal/mcp/schema] states its
// own rules in a Validate method; approved lookup values are checked
// separately by [github.com/roguekeyboards/mykeebs-api/internal/lookup].
package validate

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// MaxMoney is api/openapi.yaml's Money maximum.
const MaxMoney = 100_000_000_000_000

// DateLayout matches api/openapi.yaml's `format: date` and how
// openapi_types.Date parses a stored value.
const DateLayout = "2006-01-02"

// FieldError reports that Field, a dotted JSON path such as
// "purchase.price" or "switches[0].count", failed for Reason.
type FieldError struct {
	Field  string
	Reason string
}

// Validatable is an input type that states its own rules.
type Validatable interface {
	Validate(v *V)
}

// V collects every FieldError rather than stopping at the first, with
// field names relative to its path.
type V struct {
	path string
	errs *[]FieldError
}

// Check returns every rule x breaks.
func Check(x Validatable) []FieldError {
	var errs []FieldError
	x.Validate(&V{errs: &errs})
	return errs
}

// Error joins Check's result into one error, or returns nil.
func Error(x Validatable) error {
	errs := Check(x)
	if len(errs) == 0 {
		return nil
	}

	reasons := make([]string, len(errs))
	for i, fe := range errs {
		reasons[i] = fe.Field + ": " + fe.Reason
	}
	return errors.New(strings.Join(reasons, "; "))
}

// At returns a V for the object at field.
func (v *V) At(field string) *V {
	return &V{path: v.name(field), errs: v.errs}
}

// Index returns a V for element i of the array at field.
func (v *V) Index(field string, i int) *V {
	return v.At(fmt.Sprintf("%s[%d]", field, i))
}

// Fail records that field failed for reason.
func (v *V) Fail(field, reason string) {
	*v.errs = append(*v.errs, FieldError{Field: v.name(field), Reason: reason})
}

func (v *V) name(field string) string {
	if v.path == "" {
		return field
	}
	return v.path + "." + field
}

// NotBlank fails a string that is empty or only whitespace.
func (v *V) NotBlank(field, s string) {
	if strings.TrimSpace(s) == "" {
		v.Fail(field, "must not be blank")
	}
}

// Date fails a set s that isn't a YYYY-MM-DD date.
func (v *V) Date(field string, s *string) {
	if s == nil {
		return
	}
	if _, err := time.Parse(DateLayout, *s); err != nil {
		v.Fail(field, fmt.Sprintf("%q must be a date in YYYY-MM-DD form", *s))
	}
}

// Money fails a set f outside 0 to MaxMoney.
func (v *V) Money(field string, f *float64) {
	if f != nil && *f > MaxMoney {
		v.Fail(field, fmt.Sprintf("must be at most %d", MaxMoney))
		return
	}
	v.NonNegative(field, f)
}

// NonNegative fails a set f below 0.
func (v *V) NonNegative(field string, f *float64) {
	if f != nil && *f < 0 {
		v.Fail(field, "must be at least 0")
	}
}

// Positive fails a set n below 1.
func (v *V) Positive(field string, n *int) {
	if n != nil && *n < 1 {
		v.Fail(field, "must be at least 1")
	}
}

// OneOf fails an s that isn't in allowed.
func OneOf[T ~string](v *V, field string, s T, allowed []T) {
	if slices.Contains(allowed, s) {
		return
	}
	names := make([]string, len(allowed))
	for i, a := range allowed {
		names[i] = string(a)
	}
	v.Fail(field, "must be one of: "+strings.Join(names, ", "))
}
