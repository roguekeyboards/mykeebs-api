package schema

import "github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"

// Validate implements [validate.Validatable].
func (in GetKeycapSetInput) Validate(v *validate.V) {
	v.NotBlank("keycap_set_id", in.KeycapSetID)
}

// Validate implements [validate.Validatable].
func (in KeycapSetInput) Validate(v *validate.V) {
	v.NotBlank("brand", in.Brand)
	v.NotBlank("name", in.Name)
	checkVisibility(v, in.Visibility)
}

// Validate implements [validate.Validatable].
func (in UpdateKeycapSetInput) Validate(v *validate.V) {
	v.NotBlank("keycap_set_id", in.KeycapSetID)
	in.KeycapSetInput.Validate(v)
}

// Validate implements [validate.Validatable].
func (in DeleteKeycapSetInput) Validate(v *validate.V) {
	v.NotBlank("keycap_set_id", in.KeycapSetID)
	checkOnDelete(v, in.OnDelete)
}

// Validate implements [validate.Validatable].
func (in KeycapKitInput) Validate(v *validate.V) {
	v.NotBlank("name", in.Name)
	if in.Purchase != nil {
		in.Purchase.Validate(v.At("purchase"))
	}
}

// Validate implements [validate.Validatable].
func (in KeycapKitPurchaseInput) Validate(v *validate.V) {
	checkPurchase(v, in.Price, in.OrderDate, in.DeliveryDate)
}

// Validate implements [validate.Validatable].
func (in CreateKeycapKitInput) Validate(v *validate.V) {
	v.NotBlank("keycap_set_id", in.KeycapSetID)
	in.KeycapKitInput.Validate(v)
}

// Validate implements [validate.Validatable].
func (in UpdateKeycapKitInput) Validate(v *validate.V) {
	v.NotBlank("keycap_set_id", in.KeycapSetID)
	v.NotBlank("kit_id", in.KitID)
	in.KeycapKitInput.Validate(v)
}

// Validate implements [validate.Validatable].
func (in DeleteKeycapKitInput) Validate(v *validate.V) {
	v.NotBlank("keycap_set_id", in.KeycapSetID)
	v.NotBlank("kit_id", in.KitID)
	checkOnDelete(v, in.OnDelete)
}

// Validate implements [validate.Validatable].
func (in SetKeycapKitImageInput) Validate(v *validate.V) {
	v.NotBlank("keycap_set_id", in.KeycapSetID)
	v.NotBlank("kit_id", in.KitID)
	checkImageSize(v, in.SizeBytes)
}

// Validate implements [validate.Validatable].
func (in DeleteKeycapKitImageInput) Validate(v *validate.V) {
	v.NotBlank("keycap_set_id", in.KeycapSetID)
	v.NotBlank("kit_id", in.KitID)
}

// Validate implements [validate.Validatable].
func (in ListKeycapSetsInput) Validate(v *validate.V) {
	checkListLimit(v, in.Limit)
}
