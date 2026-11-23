package lookup_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type ValidateBuildSuite struct {
	suite.Suite
}

func TestValidateBuildSuite(t *testing.T) {
	suite.Run(t, new(ValidateBuildSuite))
}

func (s *ValidateBuildSuite) TestAllFieldsUnset_SkipsValidation() {
	b := repository.Build{}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Empty(errs)
}

func (s *ValidateBuildSuite) TestValidFields_ReturnsNoErrors() {
	mountType := "Gasket Mount"
	durometer := "40A"
	stabName := "Durock v3"
	stabMount := "Screw-in"
	b := repository.Build{
		CaseMountType: &repository.BuildCaseMountType{Type: &mountType, Durometer: &durometer},
		Stabs:         &repository.BuildStabs{Name: &stabName, MountType: &stabMount},
	}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Empty(errs)
}

func (s *ValidateBuildSuite) TestInvalidCaseMountType_ReturnsFieldError() {
	mountType := "NotAMountType"
	b := repository.Build{CaseMountType: &repository.BuildCaseMountType{Type: &mountType}}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Equal([]lookup.FieldError{
		{Field: "case_mount_type.type", Value: "NotAMountType", Reason: `"NotAMountType" is not an approved build_case_mount_type value`},
	}, errs)
}

func (s *ValidateBuildSuite) TestInvalidDurometer_ReturnsFieldError() {
	durometer := "NotADurometer"
	b := repository.Build{CaseMountType: &repository.BuildCaseMountType{Durometer: &durometer}}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Equal([]lookup.FieldError{
		{Field: "case_mount_type.durometer", Value: "NotADurometer", Reason: `"NotADurometer" is not an approved build_durometer value`},
	}, errs)
}

func (s *ValidateBuildSuite) TestDurometerWithMountTypeWithoutDurometerSupport_ReturnsFieldError() {
	mountType := "Top Mount"
	durometer := "40A"
	b := repository.Build{CaseMountType: &repository.BuildCaseMountType{Type: &mountType, Durometer: &durometer}}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Equal([]lookup.FieldError{
		{Field: "case_mount_type.durometer", Value: "40A", Reason: `"40A" needs a case_mount_type.type that supports durometer`},
	}, errs)
}

func (s *ValidateBuildSuite) TestDurometerWithoutMountType_ReturnsFieldError() {
	durometer := "40A"
	b := repository.Build{CaseMountType: &repository.BuildCaseMountType{Durometer: &durometer}}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Equal([]lookup.FieldError{
		{Field: "case_mount_type.durometer", Value: "40A", Reason: `"40A" needs a case_mount_type.type that supports durometer`},
	}, errs)
}

func (s *ValidateBuildSuite) TestDurometerWithInvalidMountType_ReportsOnlyTheMountType() {
	mountType := "NotAMountType"
	durometer := "40A"
	b := repository.Build{CaseMountType: &repository.BuildCaseMountType{Type: &mountType, Durometer: &durometer}}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Equal([]lookup.FieldError{
		{Field: "case_mount_type.type", Value: "NotAMountType", Reason: `"NotAMountType" is not an approved build_case_mount_type value`},
	}, errs)
}

func (s *ValidateBuildSuite) TestInvalidStabName_ReturnsFieldError() {
	name := "NotAStab"
	b := repository.Build{Stabs: &repository.BuildStabs{Name: &name}}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Equal([]lookup.FieldError{
		{Field: "stabs.name", Value: "NotAStab", Reason: `"NotAStab" is not an approved build_stabilizer value`},
	}, errs)
}

func (s *ValidateBuildSuite) TestInvalidStabMountType_ReturnsFieldError() {
	mountType := "NotAMountType"
	b := repository.Build{Stabs: &repository.BuildStabs{MountType: &mountType}}

	errs := lookup.ValidateBuild(s.T().Context(), b)
	s.Equal([]lookup.FieldError{
		{Field: "stabs.mount_type", Value: "NotAMountType", Reason: `"NotAMountType" is not an approved build_stabilizer_mount_type value`},
	}, errs)
}

func (s *ValidateBuildSuite) TestBuildType_EmptyCountsAsMXAndUnapprovedIsReported() {
	s.Empty(lookup.ValidateBuild(s.T().Context(), repository.Build{}))
	s.Equal([]lookup.FieldError{
		{Field: "build_type", Value: "Topre", Reason: `"Topre" is not an approved build_type value`},
	}, lookup.ValidateBuild(s.T().Context(), repository.Build{BuildType: "Topre"}))
}
