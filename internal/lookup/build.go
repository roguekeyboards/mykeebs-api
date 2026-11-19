package lookup

import (
	"context"
	"fmt"
	"slices"

	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// ValidateBuild treats an empty BuildType as MX. It does not check whether
// b.Keyboard/Switches/ECParts/KeycapKits reference entities that actually
// exist - that needs live repository
// data, not this package's static lookup-category model, so it's handled
// separately by
// [github.com/roguekeyboards/mykeebs-api/internal/buildrefs.ValidateReferences].
func ValidateBuild(ctx context.Context, b repository.Build) []FieldError {
	var checks []fieldCheck
	add := func(field string, value *string, category Category) {
		if value == nil {
			return
		}
		checks = append(checks, fieldCheck{Field: field, Value: *value, Category: category})
	}

	buildType := b.EffectiveBuildType()
	add("build_type", &buildType, CategoryBuildType)
	if b.Stabs != nil {
		add("stabs.name", b.Stabs.Name, CategoryBuildStabilizer)
		add("stabs.mount_type", b.Stabs.MountType, CategoryBuildStabilizerMountType)
	}

	fieldErrs := validateFields(ctx, checks)

	if b.CaseMountType != nil {
		fieldErrs = append(fieldErrs, validateBuildCaseMountType(ctx, *b.CaseMountType)...)
	}

	return append(fieldErrs, validateBuildParts(b)...)
}

func validateBuildParts(b repository.Build) []FieldError {
	buildType := b.EffectiveBuildType()
	if buildType == repository.BuildTypeEC {
		if len(b.Switches) > 0 {
			return []FieldError{{Field: "switches", Reason: "not allowed when build_type is EC; list ec_parts instead"}}
		}
		return nil
	}
	if len(b.ECParts) > 0 {
		return []FieldError{{Field: "ec_parts", Reason: fmt.Sprintf("only allowed when build_type is EC, not %s; list switches instead", buildType)}}
	}
	return nil
}

// validateBuildCaseMountType checks case_mount_type.type against
// CategoryBuildCaseMountType's named-object entries (not a plain string
// list, so it can't go through the generic fieldCheck/validateFields path -
// mirrors [validateKeyboardLayout]'s handling of [CategoryKeyboardLayout]).
// An approved case_mount_type.durometer is also cross-checked against the
// type, the way layout is cross-checked against size: it's rejected unless
// the type is set and supports_durometer. An unapproved type skips the
// cross-check, having already been reported.
func validateBuildCaseMountType(ctx context.Context, cmt repository.BuildCaseMountType) []FieldError {
	var errs []FieldError

	var mountType *CaseMountTypeValue
	if cmt.Type != nil {
		category := CategoryBuildCaseMountType
		l, ok := GetCategory(ctx, category)
		var values []CaseMountTypeValue
		if ok {
			values = l.CaseMountTypeValues()
		}
		idx := slices.IndexFunc(values, func(v CaseMountTypeValue) bool { return v.Name == *cmt.Type })
		if idx == -1 {
			errs = append(errs, notApproved("case_mount_type.type", *cmt.Type, category))
		} else {
			mountType = &values[idx]
		}
	}

	if cmt.Durometer != nil {
		durometerErrs := validateFields(ctx, []fieldCheck{
			{Field: "case_mount_type.durometer", Value: *cmt.Durometer, Category: CategoryBuildDurometer},
		})
		if len(durometerErrs) == 0 && (cmt.Type == nil || mountType != nil && !mountType.SupportsDurometer) {
			durometerErrs = append(durometerErrs, FieldError{
				Field:  "case_mount_type.durometer",
				Value:  *cmt.Durometer,
				Reason: fmt.Sprintf("%q needs a case_mount_type.type that supports durometer", *cmt.Durometer),
			})
		}
		errs = append(errs, durometerErrs...)
	}

	return errs
}
