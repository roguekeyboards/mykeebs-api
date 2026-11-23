package schema

import "github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"

// Validate implements [validate.Validatable].
func (in GetSwitchInput) Validate(v *validate.V) {
	v.NotBlank("switch_id", in.SwitchID)
}

// Validate implements [validate.Validatable].
func (in SwitchInput) Validate(v *validate.V) {
	v.NotBlank("brand", in.Brand)
	v.NotBlank("name", in.Name)
	v.Positive("pins", in.Pins)
	if in.Force != nil {
		in.Force.Validate(v.At("force"))
	}
	if in.Spring != nil {
		in.Spring.Validate(v.At("spring"))
	}
	if in.Purchase != nil {
		in.Purchase.Validate(v.At("purchase"))
	}
	checkVisibility(v, in.Visibility)
}

// Validate implements [validate.Validatable].
func (in SwitchForce) Validate(v *validate.V) {
	v.NonNegative("actuation", in.Actuation)
	v.NonNegative("bottom_out", in.BottomOut)
}

// Validate implements [validate.Validatable].
func (in SwitchSpring) Validate(v *validate.V) {
	v.NonNegative("pre_travel", in.PreTravel)
	v.NonNegative("total_travel", in.TotalTravel)
	v.NonNegative("length", in.Length)
}

// Validate implements [validate.Validatable].
func (in SwitchPurchaseInput) Validate(v *validate.V) {
	checkPurchase(v, in.Price, in.OrderDate, in.DeliveryDate)
	v.Positive("quantity", in.Quantity)
}

// Validate implements [validate.Validatable].
func (in UpdateSwitchInput) Validate(v *validate.V) {
	v.NotBlank("switch_id", in.SwitchID)
	in.SwitchInput.Validate(v)
}

// Validate implements [validate.Validatable].
func (in DeleteSwitchInput) Validate(v *validate.V) {
	v.NotBlank("switch_id", in.SwitchID)
	checkOnDelete(v, in.OnDelete)
}

// Validate implements [validate.Validatable].
func (in SetSwitchImageInput) Validate(v *validate.V) {
	v.NotBlank("switch_id", in.SwitchID)
	checkImageSize(v, in.SizeBytes)
}

// Validate implements [validate.Validatable].
func (in DeleteSwitchImageInput) Validate(v *validate.V) {
	v.NotBlank("switch_id", in.SwitchID)
}

// Validate implements [validate.Validatable].
func (in ListSwitchesInput) Validate(v *validate.V) {
	checkListLimit(v, in.Limit)
}
