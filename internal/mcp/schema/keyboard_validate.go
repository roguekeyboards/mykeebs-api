package schema

import "github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"

// Validate implements [validate.Validatable].
func (in GetKeyboardInput) Validate(v *validate.V) {
	v.NotBlank("keyboard_id", in.KeyboardID)
}

// Validate implements [validate.Validatable].
func (in KeyboardInput) Validate(v *validate.V) {
	v.NotBlank("brand", in.Brand)
	v.NotBlank("name", in.Name)
	if in.Purchase != nil {
		in.Purchase.Validate(v.At("purchase"))
	}
	checkVisibility(v, in.Visibility)
}

// Validate implements [validate.Validatable].
func (in KeyboardPurchaseInput) Validate(v *validate.V) {
	checkPurchase(v, in.Price, in.OrderDate, in.DeliveryDate)
}

// Validate implements [validate.Validatable].
func (in UpdateKeyboardInput) Validate(v *validate.V) {
	v.NotBlank("keyboard_id", in.KeyboardID)
	in.KeyboardInput.Validate(v)
}

// Validate implements [validate.Validatable].
func (in DeleteKeyboardInput) Validate(v *validate.V) {
	v.NotBlank("keyboard_id", in.KeyboardID)
	checkOnDelete(v, in.OnDelete)
}

// Validate implements [validate.Validatable].
func (in ListKeyboardImagesInput) Validate(v *validate.V) {
	v.NotBlank("keyboard_id", in.KeyboardID)
}

// Validate implements [validate.Validatable].
func (in AddKeyboardImageInput) Validate(v *validate.V) {
	v.NotBlank("keyboard_id", in.KeyboardID)
	checkImageSize(v, in.SizeBytes)
}

// Validate implements [validate.Validatable].
func (in DeleteKeyboardImageInput) Validate(v *validate.V) {
	v.NotBlank("keyboard_id", in.KeyboardID)
	v.NotBlank("image_id", in.ImageID)
}

// Validate implements [validate.Validatable].
func (in KeyboardPlateInput) Validate(v *validate.V) {
	v.NotBlank("material", in.Material)
	v.NonNegative("thickness", in.Thickness)
	if in.Purchase != nil {
		in.Purchase.Validate(v.At("purchase"))
	}
}

// Validate implements [validate.Validatable].
func (in KeyboardPCBInput) Validate(v *validate.V) {
	v.NonNegative("thickness", in.Thickness)
	if in.Purchase != nil {
		in.Purchase.Validate(v.At("purchase"))
	}
}

// Validate implements [validate.Validatable].
func (in KeyboardPartPurchaseInput) Validate(v *validate.V) {
	checkPurchase(v, in.Price, in.OrderDate, in.DeliveryDate)
}

// Validate implements [validate.Validatable].
func (in CreateKeyboardPlateInput) Validate(v *validate.V) {
	v.NotBlank("keyboard_id", in.KeyboardID)
	in.KeyboardPlateInput.Validate(v)
}

// Validate implements [validate.Validatable].
func (in UpdateKeyboardPlateInput) Validate(v *validate.V) {
	v.NotBlank("keyboard_id", in.KeyboardID)
	v.NotBlank("plate_id", in.PlateID)
	in.KeyboardPlateInput.Validate(v)
}

// Validate implements [validate.Validatable].
func (in DeleteKeyboardPlateInput) Validate(v *validate.V) {
	v.NotBlank("keyboard_id", in.KeyboardID)
	v.NotBlank("plate_id", in.PlateID)
	checkOnDelete(v, in.OnDelete)
}

// Validate implements [validate.Validatable].
func (in CreateKeyboardPCBInput) Validate(v *validate.V) {
	v.NotBlank("keyboard_id", in.KeyboardID)
	in.KeyboardPCBInput.Validate(v)
}

// Validate implements [validate.Validatable].
func (in UpdateKeyboardPCBInput) Validate(v *validate.V) {
	v.NotBlank("keyboard_id", in.KeyboardID)
	v.NotBlank("pcb_id", in.PCBID)
	in.KeyboardPCBInput.Validate(v)
}

// Validate implements [validate.Validatable].
func (in DeleteKeyboardPCBInput) Validate(v *validate.V) {
	v.NotBlank("keyboard_id", in.KeyboardID)
	v.NotBlank("pcb_id", in.PCBID)
	checkOnDelete(v, in.OnDelete)
}

// Validate implements [validate.Validatable].
func (in ListKeyboardsInput) Validate(v *validate.V) {
	checkListLimit(v, in.Limit)
}
