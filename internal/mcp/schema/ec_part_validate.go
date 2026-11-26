package schema

import (
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// Validate implements [validate.Validatable].
func (in ListECPartsInput) Validate(v *validate.V) {
	checkListLimit(v, in.Limit)
	if in.PartType != "" {
		validate.OneOf(v, "part_type", repository.ECPartType(in.PartType), repository.ECPartTypes)
	}
}

// Validate implements [validate.Validatable].
func (in GetECPartInput) Validate(v *validate.V) {
	v.NotBlank("ec_part_id", in.ECPartID)
}

// Validate implements [validate.Validatable].
func (in ECPartInput) Validate(v *validate.V) {
	v.NotBlank("brand", in.Brand)
	v.NotBlank("name", in.Name)
	validate.OneOf(v, "part_type", repository.ECPartType(in.PartType), repository.ECPartTypes)
	if in.Domes != nil {
		v.NonNegative("domes.weight", in.Domes.Weight)
	}
	if in.Sliders != nil {
		checkECPartSize(v, "sliders.size", in.Sliders.Size)
		if in.Sliders.Stem != nil {
			validate.OneOf(v, "sliders.stem", repository.ECSliderStem(*in.Sliders.Stem), repository.ECSliderStems)
		}
	}
	if in.Housings != nil {
		checkECPartSize(v, "housings.size", in.Housings.Size)
	}
	if in.Rings != nil {
		v.NonNegative("rings.thickness", in.Rings.Thickness)
	}
	for i, p := range in.Purchases {
		p.Validate(v.Index("purchases", i))
	}
	checkVisibility(v, in.Visibility)
}

func checkECPartSize(v *validate.V, field string, size *string) {
	if size != nil {
		validate.OneOf(v, field, repository.ECPartSize(*size), repository.ECPartSizes)
	}
}

// Validate implements [validate.Validatable].
func (in ECPartPurchaseInput) Validate(v *validate.V) {
	checkPurchase(v, in.Price, in.OrderDate, in.DeliveryDate, in.EstimatedDeliveryDate)
	v.Positive("quantity", in.Quantity)
}

// Validate implements [validate.Validatable].
func (in UpdateECPartInput) Validate(v *validate.V) {
	v.NotBlank("ec_part_id", in.ECPartID)
	in.ECPartInput.Validate(v)
}

// Validate implements [validate.Validatable].
func (in DeleteECPartInput) Validate(v *validate.V) {
	v.NotBlank("ec_part_id", in.ECPartID)
	checkOnDelete(v, in.OnDelete)
}

// Validate implements [validate.Validatable].
func (in SetECPartImageInput) Validate(v *validate.V) {
	v.NotBlank("ec_part_id", in.ECPartID)
	checkImageSize(v, in.SizeBytes)
}

// Validate implements [validate.Validatable].
func (in DeleteECPartImageInput) Validate(v *validate.V) {
	v.NotBlank("ec_part_id", in.ECPartID)
}
