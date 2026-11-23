package schema

import "github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"

// Validate implements [validate.Validatable].
func (in GetBuildInput) Validate(v *validate.V) {
	v.NotBlank("build_id", in.BuildID)
}

// Validate implements [validate.Validatable].
func (in BuildInput) Validate(v *validate.V) {
	v.NotBlank("keyboard", in.Keyboard)
	if in.Stabs != nil {
		v.Money("stabs.price", in.Stabs.Price)
	}
	for i, e := range in.Switches {
		sv := v.Index("switches", i)
		sv.NotBlank("switch", e.Switch)
		sv.Positive("count", &e.Count)
	}
	for i, e := range in.ECParts {
		ev := v.Index("ec_parts", i)
		ev.NotBlank("ec_part", e.ECPart)
		ev.Positive("count", &e.Count)
	}
	for i, e := range in.KeycapKits {
		kv := v.Index("keycap_kits", i)
		kv.NotBlank("keycap_set", e.KeycapSet)
		kv.NotBlank("kit", e.Kit)
	}
	v.Date("build_date", in.BuildDate)
	checkVisibility(v, in.Visibility)
}

// Validate implements [validate.Validatable].
func (in UpdateBuildInput) Validate(v *validate.V) {
	v.NotBlank("build_id", in.BuildID)
	in.BuildInput.Validate(v)
}

// Validate implements [validate.Validatable].
func (in DeleteBuildInput) Validate(v *validate.V) {
	v.NotBlank("build_id", in.BuildID)
}

// Validate implements [validate.Validatable].
func (in AddBuildImageInput) Validate(v *validate.V) {
	v.NotBlank("build_id", in.BuildID)
	checkImageSize(v, in.SizeBytes)
}

// Validate implements [validate.Validatable].
func (in DeleteBuildImageInput) Validate(v *validate.V) {
	v.NotBlank("build_id", in.BuildID)
	v.NotBlank("image_id", in.ImageID)
}

// Validate implements [validate.Validatable].
func (in ListBuildImagesInput) Validate(v *validate.V) {
	v.NotBlank("build_id", in.BuildID)
}

// Validate implements [validate.Validatable].
func (in ListBuildsInput) Validate(v *validate.V) {
	checkListLimit(v, in.Limit)
}
