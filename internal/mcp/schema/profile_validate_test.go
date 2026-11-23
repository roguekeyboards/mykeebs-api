package schema_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"
)

type ProfileValidateSuite struct {
	suite.Suite
}

func TestProfileValidateSuite(t *testing.T) {
	suite.Run(t, new(ProfileValidateSuite))
}

func (s *ProfileValidateSuite) TestGet_RequiresIdentifier() {
	s.Empty(validate.Check(schema.GetProfileInput{Identifier: "jay"}))
	s.Equal([]validate.FieldError{{Field: "identifier", Reason: "must not be blank"}}, validate.Check(schema.GetProfileInput{Identifier: " "}))
}

func (s *ProfileValidateSuite) TestList_FiltersAreMutuallyExclusive() {
	s.Empty(validate.Check(schema.ListProfilesInput{Username: "ja"}))
	s.Empty(validate.Check(schema.ListProfilesInput{DiscordUsername: "ja"}))
	s.Equal([]validate.FieldError{{Field: "discord_username", Reason: "mutually exclusive with username"}},
		validate.Check(schema.ListProfilesInput{Username: "ja", DiscordUsername: "ja"}))
}

func (s *ProfileValidateSuite) TestSetImage_RejectsSizeOutsideTheCap() {
	s.Empty(validate.Check(schema.SetProfileImageInput{SizeBytes: 1536 * 1024}))
	s.Len(validate.Check(schema.SetProfileImageInput{}), 1)
}

func (s *ProfileValidateSuite) TestGetLookup_RequiresCategory() {
	s.Empty(validate.Check(schema.GetLookupInput{Category: "vendor"}))
	s.Equal([]validate.FieldError{{Field: "category", Reason: "must not be blank"}}, validate.Check(schema.GetLookupInput{}))
}
